package workbench

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

var SecurityCategories = []string{"secrets", "deps", "authz", "injection"}
var securityRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,199}$`)
var securityCWE = regexp.MustCompile(`^CWE-[0-9]+$`)

type SecurityFinding struct {
	Fingerprint    string `json:"fingerprint"`
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	Title          string `json:"title"`
	CWE            string `json:"cwe"`
	Location       string `json:"location"`
	Recommendation string `json:"recommendation"`
}

type SecurityScanInput struct {
	IssueID         string            `json:"issue_id"`
	IssueRevision   int64             `json:"issue_revision"`
	RepoID          string            `json:"repo_id"`
	RunID           string            `json:"run_id"`
	ArtifactVersion string            `json:"artifact_version"`
	Commit          string            `json:"commit"`
	ScannedAt       time.Time         `json:"scanned_at"`
	Status          string            `json:"status"`
	ReportURL       string            `json:"report_url"`
	Categories      []string          `json:"categories"`
	Findings        []SecurityFinding `json:"findings"`
}

func (scan *SecurityScanInput) Validate(now time.Time) error {
	scan.RunID = strings.TrimSpace(scan.RunID)
	scan.ArtifactVersion = strings.TrimSpace(scan.ArtifactVersion)
	if !securityRunID.MatchString(scan.RunID) || scan.ArtifactVersion == "" || len(scan.ArtifactVersion) > 200 || !sourceCommit.MatchString(scan.Commit) {
		return errors.New("scan run id, artifact version and full source commit are required")
	}
	if scan.IssueRevision < 1 || scan.ScannedAt.IsZero() || scan.ScannedAt.After(now) {
		return errors.New("scan requires a task revision and a valid past scan time")
	}
	if !slices.Contains([]string{"completed", "failed"}, scan.Status) || !validEvidenceURL(scan.ReportURL) {
		return errors.New("scan status and an HTTP(S) report URL are required")
	}
	if len(scan.Categories) != len(SecurityCategories) {
		return errors.New("scan must cover all four security categories")
	}
	seenCategories := map[string]bool{}
	for _, category := range scan.Categories {
		if !slices.Contains(SecurityCategories, category) || seenCategories[category] {
			return errors.New("scan categories must be unique and supported")
		}
		seenCategories[category] = true
	}
	if scan.Findings == nil || len(scan.Findings) > 200 {
		return errors.New("scan findings must be an explicit array with at most 200 entries")
	}
	seenFindings := map[string]bool{}
	for _, finding := range scan.Findings {
		if strings.TrimSpace(finding.Fingerprint) == "" || len(finding.Fingerprint) > 200 || seenFindings[finding.Fingerprint] {
			return errors.New("finding fingerprints must be unique and contain 1 to 200 bytes")
		}
		seenFindings[finding.Fingerprint] = true
		if !slices.Contains([]string{"high", "medium", "low"}, finding.Severity) || !seenCategories[finding.Category] {
			return errors.New("finding severity and category are invalid")
		}
		if strings.TrimSpace(finding.Title) == "" || len([]rune(finding.Title)) > 300 || strings.TrimSpace(finding.Location) == "" || len(finding.Location) > 1024 ||
			strings.TrimSpace(finding.Recommendation) == "" || len([]rune(finding.Recommendation)) > 4000 {
			return errors.New("finding title, location and recommendation are required within their length limits")
		}
		if finding.CWE != "" && !securityCWE.MatchString(finding.CWE) {
			return errors.New("CWE must use the CWE-number format")
		}
	}
	return nil
}

func ValidateSecurityAcceptance(severity, reason string, days int) error {
	if severity != "low" {
		return errors.New("only low severity risks can be accepted")
	}
	if strings.TrimSpace(reason) == "" || len([]rune(reason)) > 2000 || !slices.Contains([]int{7, 30, 90}, days) {
		return errors.New("risk acceptance requires a reason and a 7, 30 or 90 day duration")
	}
	return nil
}

func ValidateSecurityRecheck(scan SecurityScanInput, repository, version, fingerprint string, requestedAt time.Time) error {
	if scan.Status != "completed" || scan.RepoID != repository || scan.ArtifactVersion != version || scan.ScannedAt.Before(requestedAt) {
		return errors.New("recheck requires a completed newer scan of the same repository and target artifact")
	}
	for _, finding := range scan.Findings {
		if finding.Fingerprint == fingerprint {
			return errors.New("the recheck still reports this risk")
		}
	}
	return nil
}
