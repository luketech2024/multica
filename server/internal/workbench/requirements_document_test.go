package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestRequirementsDocumentVersionAndVerification(t *testing.T) {
	content := "# Requirements\nServe the homepage.\n"
	digest := sha256.Sum256([]byte(content))
	document := &RequirementsDocument{Content: content, Ref: OneSpecArtifactRef{RepositoryID: "repo", Path: "onespec/changes/task/prd.md", Commit: strings.Repeat("a", 40), Digest: hex.EncodeToString(digest[:])}, Verified: true}
	flow := OneSpecFlow{Phase: "requirements", RuleVersion: OneSpecVersion}
	artifact := OneSpecArtifact{Stage: "requirements", Summary: "Homepage", Refs: []OneSpecArtifactRef{document.Ref}}
	if err := flow.Submit(artifact, "agent", time.Now()); err == nil {
		t.Fatal("accepted missing readable document")
	}
	artifact.Document = document
	if err := flow.Submit(artifact, "agent", time.Now()); err != nil {
		t.Fatal(err)
	}
	if flow.Latest("requirements").Document.Verified {
		t.Fatal("trusted agent-supplied verification")
	}
	if err := flow.Decide("requirements", 1, true, "", "user", time.Now()); err == nil {
		t.Fatal("confirmed unverified file")
	}
	flow.Artifacts[0].Document.Verified = true
	if err := flow.Decide("requirements", 1, true, "", "user", time.Now()); err != nil {
		t.Fatal(err)
	}
	document.Content += "changed"
	if err := document.Validate(false); err == nil {
		t.Fatal("accepted incorrect digest")
	}
}

func TestRequirementsDocumentLegacyAndPrototype(t *testing.T) {
	flow := OneSpecFlow{Phase: "requirements", RuleVersion: "1.0.0"}
	if err := flow.Submit(oneSpecTestArtifact("requirements"), "agent", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := flow.Decide("requirements", 1, true, "", "user", time.Now()); err != nil {
		t.Fatal(err)
	}
	artifact := OneSpecArtifact{Document: &RequirementsDocument{Verified: true, Prototype: &RequirementsDocument{Verified: false}}}
	if artifact.DocumentReady() {
		t.Fatal("unverified prototype was confirmable")
	}
}

func TestRequirementsDocumentResetsAffectedChecks(t *testing.T) {
	before := &RequirementsDocument{Content: "# PRD\n## Scope\nHomepage\n## Contact\nEmail"}
	after := &RequirementsDocument{Content: "# PRD\n## Scope\nHomepage\n## Contact\nPhone"}
	if RequirementItemDocumentChanged(before, after, "Scope") {
		t.Fatal("unchanged section lost confirmation")
	}
	for _, section := range []string{"Contact", "", "Missing"} {
		if !RequirementItemDocumentChanged(before, after, section) {
			t.Fatal("unmapped or changed item kept confirmation")
		}
	}
	after.Prototype = &RequirementsDocument{Content: "<button>New action</button>"}
	if !RequirementItemDocumentChanged(before, after, "Scope") {
		t.Fatal("prototype change retained old checks")
	}
}
