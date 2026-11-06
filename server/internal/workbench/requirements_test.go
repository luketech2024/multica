package workbench

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func edit(t *testing.T, old []byte, input Requirements, confirm bool, actor string) ([]byte, Requirements) {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := EditRequirements(old, raw, confirm, "member-1", actor, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRequirements(updated)
	if err != nil {
		t.Fatal(err)
	}
	return updated, parsed
}

func TestRequirementsIdentifiersNeverReuseDeletedNumbers(t *testing.T) {
	raw, criteria := edit(t, nil, Requirements{V: 1, Items: []Item{{Text: "First"}, {Text: "Second"}}}, false, "member")
	if criteria.Items[0].ID != "A-01" || criteria.Items[1].ID != "A-02" {
		t.Fatal(criteria.Items)
	}
	criteria.Items = criteria.Items[:1]
	raw, criteria = edit(t, raw, criteria, false, "member")
	criteria.Items = append(criteria.Items, Item{Text: "Third"})
	_, criteria = edit(t, raw, criteria, false, "member")
	if criteria.Items[1].ID != "A-03" {
		t.Fatal(criteria.Items)
	}
}

func TestRequirementsEditingClearsCheckAndServerOwnsAttribution(t *testing.T) {
	raw, criteria := edit(t, nil, Requirements{V: 1, Items: []Item{{Text: "First"}}}, false, "member")
	criteria.Items[0].Checked = &Check{At: "forged", By: "another-member"}
	raw, criteria = edit(t, raw, criteria, false, "member")
	if criteria.Items[0].Checked.By != "member-1" || criteria.Items[0].Checked.At == "forged" {
		t.Fatal(criteria.Items)
	}
	criteria.Items[0].Text = "Changed"
	_, criteria = edit(t, raw, criteria, false, "member")
	if criteria.Items[0].Checked != nil {
		t.Fatal("editing retained a stale check")
	}
}

func TestRequirementsConfirmationLocksSnapshot(t *testing.T) {
	raw, criteria := edit(t, nil, Requirements{V: 1, Items: []Item{{Text: "First"}}, Bounds: []string{"No billing"}}, false, "member")
	criteria.Items[0].Checked = &Check{}
	raw, criteria = edit(t, raw, criteria, true, "member")
	if criteria.Gate.Spec == nil || criteria.Gate.Spec.PassedBy != "member-1" {
		t.Fatal(criteria)
	}
	if len(criteria.Bounds) != 1 || criteria.Bounds[0] != "No billing" {
		t.Fatal(criteria.Bounds)
	}
	if _, err := EditRequirements(raw, []byte(`{"v":1,"items":[]}`), false, "member-1", "member", time.Now()); err == nil {
		t.Fatal("confirmed criteria were editable")
	}
}

func TestRequirementsRejectUnsafeConfirmation(t *testing.T) {
	for _, text := range []string{"Valid but unchecked", "TBD", "待补充"} {
		t.Run(text, func(t *testing.T) {
			raw, criteria := edit(t, nil, Requirements{V: 1, Items: []Item{{Text: text}}}, false, "member")
			if text != "Valid but unchecked" {
				criteria.Items[0].Checked = &Check{}
			}
			input, _ := json.Marshal(criteria)
			if _, err := EditRequirements(raw, input, true, "member-1", "member", time.Now()); err == nil {
				t.Fatal("unsafe confirmation accepted")
			}
		})
	}
	if _, err := EditRequirements(nil, []byte(`{"v":1,"items":[]}`), true, "member-1", "member", time.Now()); err == nil {
		t.Fatal("empty confirmation accepted")
	}
}

func TestRequirementsRejectAgentConfirmationAndForgedIDs(t *testing.T) {
	raw, criteria := edit(t, nil, Requirements{V: 1, Items: []Item{{Text: "First"}}}, false, "member")
	criteria.Items[0].Checked = &Check{}
	input, _ := json.Marshal(criteria)
	if _, err := EditRequirements(raw, input, true, "agent-1", "agent", time.Now()); err == nil {
		t.Fatal("agent confirmed requirements")
	}
	_, agentEdit := edit(t, raw, criteria, false, "agent")
	if agentEdit.Items[0].Checked != nil {
		t.Fatal("agent forged a human check")
	}
	criteria.Items[0].ID = "A-99"
	input, _ = json.Marshal(criteria)
	if _, err := EditRequirements(raw, input, false, "member-1", "member", time.Now()); err == nil {
		t.Fatal("forged ID accepted")
	}
}

func TestRequirementsRejectMalformedDataWithoutErasingIt(t *testing.T) {
	for _, raw := range []string{"null", " null\n", "{}", "[]", `{"items":[]}`} {
		if _, err := EditRequirements(nil, []byte(raw), false, "member-1", "member", time.Now()); err == nil {
			t.Fatalf("accepted missing request schema version: %q", raw)
		}
	}
	for _, raw := range []string{`{"v":2}`, `{"v":1,"items":[{"id":"wrong"}]}`, `{"v":1,"items":[{"id":"A-999999999999999999999"}]}`} {
		if _, err := ParseRequirements([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"null", "[]", ""} {
		criteria, err := ParseRequirements([]byte(raw))
		if err != nil || criteria.NextItemNo != 1 {
			t.Fatalf("legacy empty value failed: %s", raw)
		}
	}
	if _, err := EditRequirements(nil, []byte(`{"v":1,"items":[{"text":"`+strings.Repeat("x", 121)+`"}]}`), false, "member-1", "member", time.Now()); err == nil {
		t.Fatal("unbounded text accepted")
	}
}
