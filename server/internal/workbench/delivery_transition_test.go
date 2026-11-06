package workbench

import "testing"

func TestDeliveryTransitionRequiresFreshCompleteRoleConclusions(t *testing.T) {
	roles := DefaultProjectRoles()
	roles.Roles["dev"] = ProjectRole{}
	roles.Roles["ops"] = ProjectRole{}
	criteria := Requirements{Version: 1, Gate: Gates{Spec: &GateCheck{}},
		Items: []Item{{ID: "A-01", Checked: &Check{}, Covers: []string{"test", "sec"}}},
		Team:  &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: roles}},
		Plan:  &ExecutionPlan{Version: 1, Issues: []string{"test", "sec"}}, Delivery: map[string]DeliveryEvidence{},
	}
	tasks := []DeliveryTask{}
	for _, role := range []string{"test", "sec"} {
		tasks = append(tasks, DeliveryTask{ID: role, Revision: 1, Status: "done", Handoff: Handoff{V: 2, Role: role, RequirementVersion: 1, Expected: []string{"report"}}})
		criteria.Delivery[role] = DeliveryEvidence{IssueID: role, IssueRevision: 1, Version: 1, Role: role, Passed: true,
			DescriptionHash: DeliveryDescriptionHash(""), Summary: "checked", RecordedAt: "now", RecordedBy: "member",
			Reports: []DeliveryReport{{Name: "report", URL: "https://example.test/report"}},
		}
	}
	assert := func(active bool, description, want string) {
		t.Helper()
		status, _ := DeliveryTransition(criteria, tasks, active, description, "in_progress")
		if status != want {
			t.Fatalf("transition = %q, want %q", status, want)
		}
	}
	assert(false, "", "in_review")
	if status, reason := DeliveryTransition(criteria, tasks, false, "", "todo"); status != "in_progress" || reason != "plan_started" {
		t.Fatal("completed current children did not advance their waiting parent")
	}
	assert(true, "", "")
	assert(false, "changed", "")
	evidence := criteria.Delivery["sec"]
	evidence.Passed = false
	criteria.Delivery["sec"] = evidence
	assert(false, "", "blocked")
	tasks[1].Revision++
	assert(false, "", "")
	tasks[1].Revision--
	evidence = criteria.Delivery["test"]
	evidence.Passed = false
	criteria.Delivery["test"] = evidence
	assert(false, "", "")
	criteria.Team.Config.Roles["sec"] = ProjectRole{}
	criteria.Plan.Issues = []string{"test"}
	criteria.Items[0].Covers = []string{"test"}
	assert(false, "", "")
	evidence.Passed = true
	criteria.Delivery["test"] = evidence
	assert(false, "", "in_review")
}
