package workbench

import (
	"testing"
	"time"
)

func TestDeliveryReviewChecksRulesAndFiveSecondUndo(t *testing.T) {
	now := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	criteria := Requirements{Template: &ProjectTemplate{Content: BusinessTemplate{
		Forbidden: []TemplateProhibition{{Rule: "No secrets", Check: "Secret scan"}}, DefinitionOfDone: []string{"Regression passed"},
	}}}
	if _, _, err := ReviewDelivery(criteria, "in_review", "accept", "", "member", nil, nil, now); err == nil {
		t.Fatal("acceptance bypassed the project checklist")
	}
	checks := []string{"forbidden-0", "dod-0"}
	if _, _, err := ReviewDelivery(criteria, "in_review", "accept", "", "member", checks, []DeliveryBlocker{{Code: "running"}}, now); err == nil {
		t.Fatal("accepted active execution")
	}
	accepted, status, err := ReviewDelivery(criteria, "in_review", "accept", "", "member", checks, nil, now)
	if err != nil || status != "done" || accepted.Gate.Accept.PassedBy != "member" || len(accepted.Review.Checks) != 2 {
		t.Fatalf("acceptance did not save the verified checklist: %+v %s %v", accepted, status, err)
	}
	if _, _, err := ReviewDelivery(accepted, "done", "undo", "", "other", nil, nil, now); err == nil {
		t.Fatal("another member undid acceptance")
	}
	if _, _, err := ReviewDelivery(accepted, "done", "undo", "", "member", nil, nil, now.Add(6*time.Second)); err == nil {
		t.Fatal("expired undo succeeded")
	}
	undone, status, err := ReviewDelivery(accepted, "done", "undo", "", "member", nil, nil, now.Add(time.Second))
	if err != nil || status != "in_review" || undone.Gate.Accept != nil {
		t.Fatal("undo did not restore review")
	}
	if _, _, err := ReviewDelivery(undone, "in_review", "reject", "  ", "member", nil, nil, now); err == nil {
		t.Fatal("rejection without a reason succeeded")
	}
	undone.Delivery = map[string]DeliveryEvidence{"child": {Passed: true}}
	rejected, status, err := ReviewDelivery(undone, "in_review", "reject", "Fix export", "member", nil, nil, now)
	if err != nil || status != "in_progress" || rejected.Delivery != nil || rejected.Review.Reason != "Fix export" {
		t.Fatal("requesting changes did not invalidate this round's conclusions")
	}
}

func TestDeliveryReadinessRequiresPlannedTasksAndFreshReports(t *testing.T) {
	config := DefaultProjectRoles()
	criteria := Requirements{Version: 1, Gate: Gates{Spec: &GateCheck{}},
		Items: []Item{{ID: "A-01", Checked: &Check{}, Covers: []string{"child"}}},
		Team:  &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: config}},
		Plan:  &ExecutionPlan{Version: 1, Issues: []string{"child"}},
		Delivery: map[string]DeliveryEvidence{"child": {
			DescriptionHash: DeliveryDescriptionHash(""),
			IssueID:         "child", IssueRevision: 3, Version: 1, Role: "test", Summary: "Passed", Passed: true,
			RecordedAt: "now", RecordedBy: "member", Reports: []DeliveryReport{{Name: "report", URL: "https://example.test/report"}},
		}},
	}
	child := DeliveryTask{ID: "child", Revision: 3, Status: "done", Handoff: Handoff{V: 2, Role: "test", RequirementVersion: 1, Expected: []string{"report"}}}
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 0 {
		t.Fatalf("unplanned roles blocked delivery: %+v", blockers)
	}
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, "Changed task context"); len(blockers) != 1 || blockers[0].Code != "evidence_stale" {
		t.Fatal("changed parent description retained old conclusions")
	}
	child.Revision++
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 1 || blockers[0].Code != "evidence_stale" {
		t.Fatalf("changed subtask retained old evidence: %+v", blockers)
	}
	child.Revision--
	child.Handoff.Role = "ops"
	evidence := criteria.Delivery["child"]
	evidence.Role = "ops"
	criteria.Delivery["child"] = evidence
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 0 {
		t.Fatalf("operations-only delivery was blocked: %+v", blockers)
	}
	criteria.Plan.Issues = append(criteria.Plan.Issues, "security")
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 1 || blockers[0].Code != "task_missing" || blockers[0].IssueID != "security" {
		t.Fatalf("missing planned security task was ignored: %+v", blockers)
	}
}

func TestDeliveryFingerprintDistinguishesConversationFromTaskChanges(t *testing.T) {
	child := DeliveryTask{ID: "child", Title: "Deploy service", Description: "Use the selected branch", Revision: 3, Status: "done",
		Handoff: Handoff{V: 2, Role: "ops", RequirementVersion: 1, Expected: []string{"report"}}}
	criteria := Requirements{Version: 1, Gate: Gates{Spec: &GateCheck{}},
		Items: []Item{{ID: "A-01", Checked: &Check{}, Covers: []string{"child"}}},
		Team:  &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: DefaultProjectRoles()}},
		Plan:  &ExecutionPlan{Version: 1, Issues: []string{"child"}},
		Delivery: map[string]DeliveryEvidence{"child": {
			TaskHash: DeliveryTaskHash(child), DescriptionHash: DeliveryDescriptionHash(""),
			IssueID: "child", IssueRevision: 3, Version: 1, Role: "ops", Summary: "Verified", Passed: true,
			RecordedAt: "now", RecordedBy: "member", Reports: []DeliveryReport{{Name: "report", URL: "https://example.test/report"}},
		}},
	}
	for _, test := range []struct {
		name    string
		change  func(*DeliveryTask)
		blocker string
	}{
		{"conversation revision", func(task *DeliveryTask) { task.Revision++ }, ""},
		{"title", func(task *DeliveryTask) { task.Title = "Deploy another service" }, "evidence_stale"},
		{"description", func(task *DeliveryTask) { task.Description = "Use another branch" }, "evidence_stale"},
		{"handoff", func(task *DeliveryTask) { task.Handoff.Covers = []string{"A-02"} }, "evidence_stale"},
		{"reopened task", func(task *DeliveryTask) { task.Status = "in_progress" }, "task_incomplete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			updated := child
			test.change(&updated)
			blockers := DeliveryBlockers(criteria, []DeliveryTask{updated}, false, "")
			if test.blocker == "" {
				if len(blockers) != 0 {
					t.Fatalf("conversation invalidated verified evidence: %+v", blockers)
				}
			} else if len(blockers) != 1 || blockers[0].Code != test.blocker {
				t.Fatalf("blockers = %+v, want %s", blockers, test.blocker)
			}
		})
	}
}
