package workbench

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type RepositoryCommands struct {
	Install string   `json:"install"`
	Build   string   `json:"build"`
	Test    string   `json:"test"`
	Lint    []string `json:"lint"`
	Scan    string   `json:"scan"`
}

type StackTemplate struct {
	ID       string             `json:"id"`
	Family   string             `json:"family"`
	Runtime  string             `json:"runtime"`
	Derived  RepositoryCommands `json:"derived"`
	Requires []string           `json:"requires"`
}

type StackTag struct {
	ID                  string                                `json:"id"`
	Family              string                                `json:"family,omitempty"`
	Template            string                                `json:"template,omitempty"`
	CompatibleTemplates []string                              `json:"compatible_templates,omitempty"`
	Runtime             string                                `json:"runtime,omitempty"`
	Derived             map[string]json.RawMessage            `json:"derived,omitempty"`
	ByFamily            map[string]map[string]json.RawMessage `json:"by_family,omitempty"`
	Requires            []string                              `json:"requires,omitempty"`
	Checks              []string                              `json:"checks,omitempty"`
}

type RepositoryStackCatalog struct {
	Templates  []StackTemplate `json:"templates"`
	Tags       []StackTag      `json:"tags"`
	Deliveries []string        `json:"deliveries"`
}

//go:embed repository_stacks.json
var repositoryStackCatalogJSON []byte

func RepositoryStacks() (RepositoryStackCatalog, error) {
	var catalog RepositoryStackCatalog
	err := json.Unmarshal(repositoryStackCatalogJSON, &catalog)
	return catalog, err
}

type ManualRepositoryCommand struct {
	Value string `json:"value"`
	By    string `json:"by"`
}

type RepositoryStack struct {
	V         int                                `json:"v"`
	Template  string                             `json:"template"`
	Tags      []string                           `json:"tags"`
	Detected  json.RawMessage                    `json:"detected"`
	Derived   RepositoryCommands                 `json:"derived"`
	Overrides map[string]ManualRepositoryCommand `json:"overrides"`
	Runtime   string                             `json:"runtime"`
	Requires  []string                           `json:"requires"`
	Checks    []string                           `json:"checks"`
	Baseline  json.RawMessage                    `json:"baseline"`
}

type RepositoryProfile struct {
	Revision int64           `json:"revision"`
	Stack    RepositoryStack `json:"stack"`
	Delivery []string        `json:"delivery"`
}

type RepositoryProfileInput struct {
	ExpectedRevision int64             `json:"expected_revision"`
	Template         string            `json:"template"`
	Tags             []string          `json:"tags"`
	Delivery         []string          `json:"delivery"`
	Overrides        map[string]string `json:"overrides"`
}

func ReadRepositoryProfile(ref []byte) (RepositoryProfile, error) {
	var stored struct {
		Revision int64            `json:"workbench_profile_revision"`
		Stack    *RepositoryStack `json:"stack"`
		Delivery []string         `json:"delivery"`
	}
	if json.Unmarshal(ref, &stored) != nil {
		return RepositoryProfile{}, errors.New("invalid repository reference")
	}
	if stored.Stack == nil {
		profile, err := BuildRepositoryProfile(RepositoryProfileInput{Template: "custom", Delivery: []string{"none"}}, RepositoryProfile{})
		profile.Revision = 0
		return profile, err
	}
	if stored.Stack.V != 3 || stored.Revision < 1 {
		return RepositoryProfile{}, errors.New("unsupported repository profile")
	}
	return RepositoryProfile{Revision: stored.Revision, Stack: *stored.Stack, Delivery: stored.Delivery}, nil
}

func BuildRepositoryProfile(input RepositoryProfileInput, previous RepositoryProfile) (RepositoryProfile, error) {
	catalog, err := RepositoryStacks()
	if err != nil {
		return RepositoryProfile{}, err
	}
	templates := map[string]StackTemplate{}
	for _, template := range catalog.Templates {
		templates[template.ID] = template
	}
	if _, ok := templates[input.Template]; !ok {
		return RepositoryProfile{}, errors.New("unknown stack template")
	}
	if len(input.Tags) > len(catalog.Tags) || len(input.Delivery) == 0 || len(input.Delivery) > len(catalog.Deliveries) {
		return RepositoryProfile{}, errors.New("select supported tags and at least one delivery type")
	}
	selected := map[string]bool{}
	for _, tag := range input.Tags {
		if selected[tag] {
			return RepositoryProfile{}, fmt.Errorf("duplicate tag %s", tag)
		}
		selected[tag] = true
	}
	family := ""
	resolvedTemplate := input.Template
	tags := []string{}
	for _, rule := range catalog.Tags {
		if !selected[rule.ID] {
			continue
		}
		delete(selected, rule.ID)
		tags = append(tags, rule.ID)
		if rule.Family != "" {
			if family != "" && family != rule.Family {
				return RepositoryProfile{}, errors.New("conflicting language tags")
			}
			family = rule.Family
		}
		if rule.Template != "" && !slices.Contains(rule.CompatibleTemplates, resolvedTemplate) {
			resolvedTemplate = rule.Template
		}
	}
	if len(selected) > 0 {
		return RepositoryProfile{}, errors.New("unknown stack tag")
	}
	if slices.Contains(tags, "java21") && slices.Contains(tags, "java8") {
		return RepositoryProfile{}, errors.New("select one Java version")
	}
	base := templates[resolvedTemplate]
	if family == "" {
		family = base.Family
	}
	stack := RepositoryStack{V: 3, Template: resolvedTemplate, Tags: tags, Derived: base.Derived,
		Overrides: map[string]ManualRepositoryCommand{}, Runtime: base.Runtime, Requires: append([]string{}, base.Requires...), Checks: []string{},
		Detected: previous.Stack.Detected, Baseline: previous.Stack.Baseline}
	for _, rule := range catalog.Tags {
		if !slices.Contains(tags, rule.ID) {
			continue
		}
		if err := applyRepositoryCommands(&stack.Derived, rule.Derived); err != nil {
			return RepositoryProfile{}, err
		}
		if err := applyRepositoryCommands(&stack.Derived, rule.ByFamily[family]); err != nil {
			return RepositoryProfile{}, err
		}
		stack.Requires = append(stack.Requires, rule.Requires...)
		stack.Checks = append(stack.Checks, rule.Checks...)
		if rule.Runtime != "" {
			stack.Runtime = rule.Runtime
		}
	}
	for key, value := range input.Overrides {
		if !slices.Contains([]string{"install", "build", "test", "lint", "scan"}, key) || len(value) > 4096 || strings.ContainsRune(value, 0) {
			return RepositoryProfile{}, errors.New("invalid manual command override")
		}
		stack.Overrides[key] = ManualRepositoryCommand{Value: value, By: "manual"}
	}
	slices.Sort(stack.Requires)
	stack.Requires = slices.Compact(stack.Requires)
	slices.Sort(stack.Checks)
	stack.Checks = slices.Compact(stack.Checks)
	deliverySeen := map[string]bool{}
	for _, delivery := range input.Delivery {
		if !slices.Contains(catalog.Deliveries, delivery) || deliverySeen[delivery] {
			return RepositoryProfile{}, errors.New("unknown or duplicate delivery type")
		}
		deliverySeen[delivery] = true
	}
	if deliverySeen["none"] && len(input.Delivery) != 1 {
		return RepositoryProfile{}, errors.New("none cannot be combined with another delivery type")
	}
	return RepositoryProfile{Revision: previous.Revision + 1, Stack: stack, Delivery: input.Delivery}, nil
}

func applyRepositoryCommands(commands *RepositoryCommands, patch map[string]json.RawMessage) error {
	raw, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	if len(patch) == 0 {
		return nil
	}
	return json.Unmarshal(raw, commands)
}

func WriteRepositoryProfile(ref []byte, profile RepositoryProfile) ([]byte, error) {
	var values map[string]json.RawMessage
	if json.Unmarshal(ref, &values) != nil || values == nil {
		return nil, errors.New("invalid repository reference")
	}
	values["workbench_profile_revision"], _ = json.Marshal(profile.Revision)
	values["stack"], _ = json.Marshal(profile.Stack)
	values["delivery"], _ = json.Marshal(profile.Delivery)
	return json.Marshal(values)
}

// Generic repository edits cannot remove or forge the profile owned by its endpoint.
func RepositorySourceChanged(current, replacement []byte) bool {
	type source struct {
		URL       string `json:"url"`
		Ref       string `json:"ref"`
		LocalPath string `json:"local_path"`
		DaemonID  string `json:"daemon_id"`
	}
	var before, after source
	return json.Unmarshal(current, &before) != nil || json.Unmarshal(replacement, &after) != nil || before != after
}

func PreserveRepositoryProfile(current, replacement []byte) ([]byte, error) {
	var saved, next map[string]json.RawMessage
	if json.Unmarshal(current, &saved) != nil || json.Unmarshal(replacement, &next) != nil || next == nil {
		return nil, errors.New("invalid repository reference")
	}
	for _, key := range []string{"workbench_profile_revision", "stack", "delivery"} {
		delete(next, key)
		if value, ok := saved[key]; ok {
			next[key] = value
		}
	}
	if saved["stack"] != nil && RepositorySourceChanged(current, replacement) {
		profile, err := ReadRepositoryProfile(current)
		if err != nil {
			return nil, err
		}
		profile.Stack.Detected, profile.Stack.Baseline = nil, nil
		profile.Revision++
		raw, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		return WriteRepositoryProfile(raw, profile)
	}
	return json.Marshal(next)
}
