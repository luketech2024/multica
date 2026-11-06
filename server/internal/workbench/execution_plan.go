package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// HandoffJSON exposes the reserved object separately from scalar custom fields.
func HandoffJSON(properties []byte) json.RawMessage {
	var values map[string]json.RawMessage
	if json.Unmarshal(properties, &values) != nil {
		return nil
	}
	return values["workbench_handoff"]
}

type PlanTask struct {
	Key                  string   `json:"key"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Role                 string   `json:"role"`
	RepoID               string   `json:"repo_id"`
	Covers               []string `json:"covers"`
	DependsOn            []string `json:"depends_on"`
	ExistingDependencies []string `json:"existing_dependencies,omitempty"`
	SerialGroup          string   `json:"serial_group"`
	Expected             []string `json:"expected"`
}

type ExecutionPlan struct {
	Version int      `json:"version"`
	Key     string   `json:"key"`
	Digest  string   `json:"digest"`
	Issues  []string `json:"issues"`
}

type Handoff struct {
	V                  int      `json:"v"`
	RequirementVersion int      `json:"requirement_version"`
	Role               string   `json:"role"`
	RepoID             string   `json:"repo_id"`
	Covers             []string `json:"covers"`
	DependsOn          []string `json:"depends_on"`
	SerialGroup        string   `json:"serial_group"`
	MaxRetry           int      `json:"max_retry"`
	Expected           []string `json:"expected"`
	Superseded         bool     `json:"superseded"`
}

// Draft dependencies use plan keys, while existing dependencies use persisted
// issue IDs. Both become issue IDs when the complete plan is saved.
func ValidateExecutionPlan(tasks []PlanTask, criteria Requirements, repos map[string]bool, existing map[string][]string) error {
	if len(tasks) == 0 || len(tasks) > 100 {
		return errors.New("an execution plan requires 1 to 100 subtasks")
	}
	if criteria.Gate.Spec == nil || criteria.Change != nil || criteria.Team == nil {
		return errors.New("confirm requirements and select a project team before planning")
	}
	items := map[string]bool{}
	for _, item := range criteria.Items {
		items[item.ID] = false
	}
	graph := map[string][]string{}
	for id, dependencies := range existing {
		for _, dependency := range dependencies {
			if _, found := existing[dependency]; !found {
				return fmt.Errorf("%s has an unavailable existing dependency %s", id, dependency)
			}
		}
		graph[id] = dependencies
	}
	planKeys := map[string]bool{}
	for _, task := range tasks {
		if strings.TrimSpace(task.Key) == "" || len(task.Key) > 64 {
			return errors.New("each subtask requires a key of 1 to 64 bytes")
		}
		if _, exists := graph[task.Key]; exists {
			return fmt.Errorf("duplicate subtask key %s", task.Key)
		}
		planKeys[task.Key] = true
		if strings.TrimSpace(task.Title) == "" || len([]rune(task.Title)) > 200 || len([]rune(task.Description)) > 20000 {
			return fmt.Errorf("%s has an empty or oversized title or description", task.Key)
		}
		if !criteria.Team.Config.Roles[task.Role].Enabled {
			return fmt.Errorf("%s uses a role that is not enabled", task.Key)
		}
		if (task.Role == "dev" && task.RepoID == "") || (task.RepoID != "" && !repos[task.RepoID]) {
			return fmt.Errorf("%s must select a repository from this project", task.Key)
		}
		if len(task.Covers) == 0 || len(task.Covers) > len(items) {
			return fmt.Errorf("%s needs acceptance item coverage", task.Key)
		}
		covered := map[string]bool{}
		for _, id := range task.Covers {
			if _, exists := items[id]; !exists || covered[id] {
				return fmt.Errorf("%s has unknown or duplicate coverage %s", task.Key, id)
			}
			covered[id], items[id] = true, true
		}
		if len(task.Expected) == 0 || len(task.Expected) > 20 || len([]rune(task.SerialGroup)) > 100 {
			return fmt.Errorf("%s needs 1 to 20 expected deliverables and a bounded serial group", task.Key)
		}
		for _, expected := range task.Expected {
			if strings.TrimSpace(expected) == "" || len([]rune(expected)) > 200 {
				return fmt.Errorf("%s has an empty or oversized deliverable", task.Key)
			}
		}
		if len(task.ExistingDependencies) > 100 {
			return fmt.Errorf("%s has more than 100 existing dependencies", task.Key)
		}
		graph[task.Key] = append(append([]string{}, task.DependsOn...), task.ExistingDependencies...)
	}
	for id, covered := range items {
		if !covered {
			return fmt.Errorf("%s has no subtask coverage", id)
		}
	}
	for _, task := range tasks {
		for _, dependency := range task.DependsOn {
			if !planKeys[dependency] {
				return fmt.Errorf("%s has an unknown draft dependency %s", task.Key, dependency)
			}
		}
		for _, dependency := range task.ExistingDependencies {
			if _, found := existing[dependency]; !found {
				return fmt.Errorf("%s has an unavailable existing dependency %s", task.Key, dependency)
			}
		}
	}
	colors := map[string]int{}
	var visit func(string, []string) error
	visit = func(key string, path []string) error {
		if colors[key] == 1 {
			return fmt.Errorf("dependency cycle: %s", strings.Join(append(path, key), " → "))
		}
		if colors[key] == 2 {
			return nil
		}
		colors[key] = 1
		seen := map[string]bool{}
		for _, dependency := range graph[key] {
			if _, exists := graph[dependency]; !exists || seen[dependency] {
				return fmt.Errorf("%s has an unknown or duplicate dependency %s", key, dependency)
			}
			seen[dependency] = true
			if err := visit(dependency, append(path, key)); err != nil {
				return err
			}
		}
		colors[key] = 2
		return nil
	}
	for _, task := range tasks {
		if err := visit(task.Key, nil); err != nil {
			return err
		}
	}
	return nil
}
