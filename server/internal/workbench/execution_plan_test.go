package workbench

import (
	"strings"
	"testing"
)

func TestExecutionPlanCoverageRolesRepositoriesAndCycles(t *testing.T) {
	criteria := Requirements{Gate: Gates{Spec: &GateCheck{}}, Team: &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: DefaultProjectRoles()}}, Items: []Item{{ID: "A-01"}, {ID: "A-02"}}}
	valid := func() []PlanTask {
		return []PlanTask{
			{Key: "T-01", Title: "API", Role: "dev", RepoID: "api", Covers: []string{"A-01"}, Expected: []string{"diff"}},
			{Key: "T-02", Title: "Verify", Role: "test", RepoID: "api", Covers: []string{"A-02"}, DependsOn: []string{"T-01"}, Expected: []string{"selftest"}},
		}
	}
	if err := ValidateExecutionPlan(valid(), criteria, map[string]bool{"api": true}, nil); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]PlanTask){
		"cycle":                    func(tasks []PlanTask) { tasks[0].DependsOn = []string{"T-02"} },
		"unknown dependency":       func(tasks []PlanTask) { tasks[0].DependsOn = []string{"other-project"} },
		"unknown coverage":         func(tasks []PlanTask) { tasks[1].Covers = []string{"A-03"} },
		"missing coverage":         func(tasks []PlanTask) { tasks[1].Covers = []string{"A-01"} },
		"development without repo": func(tasks []PlanTask) { tasks[0].RepoID = "" },
		"foreign repo":             func(tasks []PlanTask) { tasks[0].RepoID = "other-project" },
		"unknown role":             func(tasks []PlanTask) { tasks[0].Role = "root" },
	} {
		t.Run(name, func(t *testing.T) {
			tasks := valid()
			mutate(tasks)
			err := ValidateExecutionPlan(tasks, criteria, map[string]bool{"api": true}, nil)
			if err == nil {
				t.Fatal("invalid plan accepted")
			}
			if name == "cycle" && !strings.Contains(err.Error(), "T-01 → T-02 → T-01") {
				t.Fatal(err)
			}
		})
	}
	criteria.Team.Config.Roles["test"] = ProjectRole{Enabled: false}
	if err := ValidateExecutionPlan(valid(), criteria, map[string]bool{"api": true}, nil); err == nil {
		t.Fatal("disabled role accepted")
	}
}

func TestExecutionPlanExistingDependencies(t *testing.T) {
	criteria := Requirements{Gate: Gates{Spec: &GateCheck{}}, Team: &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: DefaultProjectRoles()}}, Items: []Item{{ID: "A-01"}}}
	tasks := []PlanTask{{Key: "T-01", Title: "Document", Role: "product", Covers: []string{"A-01"}, Expected: []string{"spec"}, ExistingDependencies: []string{"existing"}}}
	if err := ValidateExecutionPlan(tasks, criteria, nil, map[string][]string{"existing": nil}); err != nil {
		t.Fatal(err)
	}
	tasks[0].DependsOn = []string{"existing"}
	if err := ValidateExecutionPlan(tasks, criteria, nil, map[string][]string{"existing": nil}); err == nil {
		t.Fatal("persisted IDs were accepted as draft dependency keys")
	}
	tasks[0].DependsOn = nil
	for name, graph := range map[string]map[string][]string{
		"missing":                       {},
		"missing transitive dependency": {"existing": {"missing"}},
		"cycle":                         {"existing": {"other"}, "other": {"existing"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateExecutionPlan(tasks, criteria, nil, graph); err == nil {
				t.Fatal("invalid existing dependencies accepted")
			}
		})
	}
}
