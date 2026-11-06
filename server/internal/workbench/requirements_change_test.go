package workbench

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRequirementsChangeStopEditCancelAndLatestBaseline(t *testing.T) {
	baseline := Requirements{V: 1, Version: 1, NextItemNo: 2, Items: []Item{{ID: "A-01", Text: "Version one", Origin: "user", Covers: []string{"child"}, Checked: &Check{At: "before", By: "member"}}}, Bounds: []string{"Keep billing"}, Gate: Gates{Spec: &GateCheck{PassedAt: "before", PassedBy: "member"}}}
	raw, _ := json.Marshal(baseline)
	raw, err := StartRequirementsChange(raw, "in_progress", "New requirement", "member", []string{"run"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ChangeRequirements(raw, "edit", false); err == nil {
		t.Fatal("editing while stopping was allowed")
	}
	if _, _, err := ChangeRequirements(raw, "cancel", false); err == nil {
		t.Fatal("resuming without reconciliation was allowed")
	}
	raw, _, err = ChangeRequirements(raw, "reconcile", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err = ChangeRequirements(raw, "edit", true)
	if err != nil {
		t.Fatal(err)
	}
	edited, _ := ParseRequirements(raw)
	if edited.Version != 2 || edited.Gate.Spec != nil || edited.Items[0].Checked != nil || len(edited.Items[0].Covers) != 0 {
		t.Fatal("old confirmation or coverage survived change")
	}
	edited.Items[0].Text = "Version two"
	raw, _ = edit(t, raw, edited, false, "member")
	restored, status, err := ChangeRequirements(raw, "cancel", true)
	if err != nil || status != "in_progress" {
		t.Fatalf("restore failed: %s %v", status, err)
	}
	restoredCriteria, _ := ParseRequirements(restored)
	if restoredCriteria.Items[0].Text != "Version one" || restoredCriteria.Gate.Spec == nil || restoredCriteria.Version != 2 || restoredCriteria.NextItemNo != 2 {
		t.Fatal("baseline or monotonic counters were lost")
	}
	edited.Items[0].Checked = &Check{}
	raw, confirmed := edit(t, raw, edited, true, "member")
	if confirmed.Change != nil {
		t.Fatal("confirmation did not finish the change")
	}
	raw, err = StartRequirementsChange(raw, "in_progress", "Next change", "member", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	next, _ := ParseRequirements(raw)
	if next.Change.Baseline.Items[0].Text != "Version two" || next.Change.Baseline.Version != 2 {
		t.Fatal("second change reused the first baseline")
	}
}
