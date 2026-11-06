package workbench

import (
	"strings"
	"testing"
)

func TestDeliveryEvidenceRequiresCurrentAttributedOutputs(t *testing.T) {
	criteria := Requirements{Version: 2, Gate: Gates{Spec: &GateCheck{}},
		Team: &ProjectTeamSnapshot{ProjectRoleConfiguration: ProjectRoleConfiguration{Config: DefaultProjectRoles()}},
		Plan: &ExecutionPlan{Version: 2, Issues: []string{"child"}},
	}
	handoff := Handoff{V: 2, RequirementVersion: 2, Role: "dev", RepoID: "repo", Expected: []string{"diff", "selftest"}}
	evidence := DeliveryEvidence{IssueID: "child", Summary: "Export verified", Commit: strings.Repeat("a", 40), Passed: true,
		Reports: []DeliveryReport{{Name: "diff", URL: "https://example.test/compare"}, {Name: "selftest", URL: "https://example.test/report"}},
		Changes: &DeliveryChanges{Files: 3, Additions: 21, Deletions: 4}, PreviewURL: "https://preview.example.test/export",
	}
	if err := ValidateDeliveryEvidence(criteria, handoff, evidence); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*Requirements, *Handoff, *DeliveryEvidence)
	}{
		{"old version", func(_ *Requirements, h *Handoff, _ *DeliveryEvidence) { h.RequirementVersion = 1 }},
		{"different task", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) { e.IssueID = "other" }},
		{"retired", func(_ *Requirements, h *Handoff, _ *DeliveryEvidence) { h.Superseded = true }},
		{"needs resplit", func(c *Requirements, _ *Handoff, _ *DeliveryEvidence) { c.NeedsResplit = true }},
		{"missing report", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) { e.Reports = e.Reports[:1] }},
		{"untrusted link", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) { e.PreviewURL = "javascript:alert(1)" }},
		{"credential link", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) {
			e.PreviewURL = "https://user:secret@example.test"
		}},
		{"short hash", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) { e.Commit = "abcdef" }},
		{"no statistics", func(_ *Requirements, _ *Handoff, e *DeliveryEvidence) { e.Changes = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, h, e := criteria, handoff, evidence
			test.edit(&c, &h, &e)
			if ValidateDeliveryEvidence(c, h, e) == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}
}
