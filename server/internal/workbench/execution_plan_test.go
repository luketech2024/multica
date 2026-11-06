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
	if err := ValidateExecutionPlan(valid(), criteria, map[string]bool{"api": true}); err != nil {
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
			err := ValidateExecutionPlan(tasks, criteria, map[string]bool{"api": true})
			if err == nil {
				t.Fatal("invalid plan accepted")
			}
			if name == "cycle" && !strings.Contains(err.Error(), "T-01 → T-02 → T-01") {
				t.Fatal(err)
			}
		})
	}
	criteria.Team.Config.Roles["test"] = ProjectRole{Enabled: false}
	if err := ValidateExecutionPlan(valid(), criteria, map[string]bool{"api": true}); err == nil {
		t.Fatal("disabled role accepted")
	}
}
