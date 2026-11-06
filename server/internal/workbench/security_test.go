package workbench

import (
	"strings"
	"testing"
	"time"
)

func TestSecurityScanAndRecheckRejectIncompleteOrMismatchedEvidence(t *testing.T) {
	now := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)
	valid := SecurityScanInput{IssueRevision: 1, RepoID: "repo", RunID: "scan/42", ArtifactVersion: "v2", Commit: strings.Repeat("a", 40),
		ScannedAt: now.Add(-time.Minute), Status: "completed", ReportURL: "https://example.test/scan/42", Categories: append([]string{}, SecurityCategories...), Findings: []SecurityFinding{}}
	if err := valid.Validate(now); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*SecurityScanInput){
		func(scan *SecurityScanInput) { scan.ScannedAt = now.Add(time.Second) },
		func(scan *SecurityScanInput) { scan.ScannedAt = time.Time{} },
		func(scan *SecurityScanInput) { scan.Commit = "HEAD" },
		func(scan *SecurityScanInput) { scan.Findings = nil },
		func(scan *SecurityScanInput) { scan.Categories = []string{"deps", "deps", "authz", "injection"} },
		func(scan *SecurityScanInput) { scan.ReportURL = "javascript:alert(1)" },
	} {
		scan := valid
		change(&scan)
		if scan.Validate(now) == nil {
			t.Fatal("invalid scan was accepted")
		}
	}
	requested := now.Add(-2 * time.Minute)
	if err := ValidateSecurityRecheck(valid, "repo", "v2", "risk", requested); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*SecurityScanInput){
		func(scan *SecurityScanInput) { scan.RepoID = "other-repo" },
		func(scan *SecurityScanInput) { scan.ArtifactVersion = "v1" },
		func(scan *SecurityScanInput) { scan.Status = "failed" },
		func(scan *SecurityScanInput) { scan.ScannedAt = requested.Add(-time.Second) },
		func(scan *SecurityScanInput) { scan.Findings = []SecurityFinding{{Fingerprint: "risk"}} },
	} {
		scan := valid
		change(&scan)
		if ValidateSecurityRecheck(scan, "repo", "v2", "risk", requested) == nil {
			t.Fatal("mismatched recheck was accepted")
		}
	}
}

func TestSecurityAcceptanceRequiresLowSeverityReasonAndBoundedDuration(t *testing.T) {
	for _, severity := range []string{"high", "medium"} {
		if ValidateSecurityAcceptance(severity, "Reviewed", 7) == nil {
			t.Fatal("blocking severity was accepted")
		}
	}
	if ValidateSecurityAcceptance("low", " ", 7) == nil || ValidateSecurityAcceptance("low", "Reviewed", 365) == nil {
		t.Fatal("invalid acceptance was accepted")
	}
	for _, days := range []int{7, 30, 90} {
		if err := ValidateSecurityAcceptance("low", "Reviewed with compensating control", days); err != nil {
			t.Fatal(err)
		}
	}
}
