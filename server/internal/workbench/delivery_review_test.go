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

func TestDeliveryReadinessRequiresEnabledRolesAndFreshReports(t *testing.T) {
	config := DefaultProjectRoles()
	for _, role := range []string{"dev", "sec", "ops"} {
		config.Roles[role] = ProjectRole{Enabled: false}
	}
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
		t.Fatalf("disabled roles blocked delivery: %+v", blockers)
	}
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, "Changed task context"); len(blockers) != 1 || blockers[0].Code != "evidence_stale" {
		t.Fatal("changed parent description retained old conclusions")
	}
	child.Revision++
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 1 || blockers[0].Code != "evidence_stale" {
		t.Fatalf("changed subtask retained old evidence: %+v", blockers)
	}
	config.Roles["sec"] = ProjectRole{Enabled: true}
	if blockers := DeliveryBlockers(criteria, []DeliveryTask{child}, false, ""); len(blockers) != 2 || blockers[1].Role != "sec" {
		t.Fatalf("enabled security role was skipped: %+v", blockers)
	}
}
