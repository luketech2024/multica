package workbench

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type DeliveryTask struct {
	ID       string
	Revision int64
	Status   string
	Handoff  Handoff
}

type DeliveryBlocker struct {
	Code    string `json:"code"`
	IssueID string `json:"issue_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Role    string `json:"role,omitempty"`
}

type DeliveryCheck struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Check string `json:"check"`
}

type DeliveryReview struct {
	Action string          `json:"action"`
	Reason string          `json:"reason"`
	Checks []DeliveryCheck `json:"checks"`
	At     string          `json:"at"`
	By     string          `json:"by"`
}

func DeliveryChecks(criteria Requirements) []DeliveryCheck {
	checks := []DeliveryCheck{}
	if criteria.Template == nil {
		return checks
	}
	for index, rule := range criteria.Template.Content.Forbidden {
		checks = append(checks, DeliveryCheck{ID: fmt.Sprintf("forbidden-%d", index), Text: rule.Rule, Check: rule.Check})
	}
	for index, rule := range criteria.Template.Content.DefinitionOfDone {
		checks = append(checks, DeliveryCheck{ID: fmt.Sprintf("dod-%d", index), Text: rule})
	}
	return checks
}

func DeliveryBlockers(criteria Requirements, children []DeliveryTask, active bool, description string) []DeliveryBlocker {
	blockers := []DeliveryBlocker{}
	if criteria.Gate.Spec == nil || criteria.Team == nil || criteria.Plan == nil || criteria.Plan.Version != criteria.Version || criteria.Change != nil || criteria.NeedsResplit || len(criteria.Plan.Issues) == 0 || len(criteria.Items) == 0 {
		return append(blockers, DeliveryBlocker{Code: "plan"})
	}
	for _, item := range criteria.Items {
		if item.Checked == nil || len(item.Covers) == 0 {
			return append(blockers, DeliveryBlocker{Code: "plan"})
		}
		for _, child := range item.Covers {
			if !slices.Contains(criteria.Plan.Issues, child) {
				return append(blockers, DeliveryBlocker{Code: "plan"})
			}
		}
	}
	if active {
		blockers = append(blockers, DeliveryBlocker{Code: "running"})
	}
	tasks := map[string]DeliveryTask{}
	for _, child := range children {
		tasks[child.ID] = child
	}
	roles := map[string]bool{}
	for _, id := range criteria.Plan.Issues {
		child, exists := tasks[id]
		if !exists || child.Handoff.Superseded || child.Handoff.RequirementVersion != criteria.Version {
			blockers = append(blockers, DeliveryBlocker{Code: "task_missing", IssueID: id})
			continue
		}
		roles[child.Handoff.Role] = true
		if child.Status != "done" {
			blockers = append(blockers, DeliveryBlocker{Code: "task_incomplete", IssueID: id})
			continue
		}
		evidence, exists := criteria.Delivery[id]
		if !exists {
			blockers = append(blockers, DeliveryBlocker{Code: "evidence_missing", IssueID: id})
			continue
		}
		if evidence.DescriptionHash != DeliveryDescriptionHash(description) || evidence.IssueID != id || evidence.IssueRevision != child.Revision || evidence.Version != criteria.Version || evidence.Role != child.Handoff.Role || evidence.RepoID != child.Handoff.RepoID || evidence.RecordedAt == "" || evidence.RecordedBy == "" || ValidateDeliveryEvidence(criteria, child.Handoff, evidence) != nil {
			blockers = append(blockers, DeliveryBlocker{Code: "evidence_stale", IssueID: id})
			continue
		}
		if !evidence.Passed {
			blockers = append(blockers, DeliveryBlocker{Code: "evidence_failed", IssueID: id})
		}
	}
	for _, role := range []string{"dev", "test", "sec", "ops"} {
		if criteria.Team.Config.Roles[role].Enabled && !roles[role] {
			blockers = append(blockers, DeliveryBlocker{Code: "role_missing", Role: role})
		}
	}
	return blockers
}

func ReviewDelivery(criteria Requirements, status, action, reason, actor string, checked []string, blockers []DeliveryBlocker, now time.Time) (Requirements, string, error) {
	stamp := now.UTC().Format(time.RFC3339Nano)
	review := &DeliveryReview{Action: action, Reason: strings.TrimSpace(reason), Checks: []DeliveryCheck{}, At: stamp, By: actor}
	switch action {
	case "accept":
		if status != "in_review" || len(blockers) != 0 {
			return criteria, status, errors.New("delivery is not ready for acceptance")
		}
		for _, check := range DeliveryChecks(criteria) {
			if !slices.Contains(checked, check.ID) {
				return criteria, status, errors.New("verify the project prohibitions and definition of done before acceptance")
			}
			review.Checks = append(review.Checks, check)
		}
		criteria.Gate.Accept = &GateCheck{PassedAt: stamp, PassedBy: actor}
		status = "done"
	case "reject":
		if status != "in_review" || review.Reason == "" || len([]rune(review.Reason)) > 2000 {
			return criteria, status, errors.New("requesting changes requires a reviewable task and a reason of 1 to 2000 characters")
		}
		criteria.Gate.Accept = nil
		criteria.Delivery = nil
		status = "in_progress"
	case "undo":
		if status != "done" || criteria.Gate.Accept == nil || criteria.Gate.Accept.PassedBy != actor {
			return criteria, status, errors.New("only the accepting member can undo this acceptance")
		}
		accepted, err := time.Parse(time.RFC3339Nano, criteria.Gate.Accept.PassedAt)
		if err != nil || now.Before(accepted) || now.After(accepted.Add(5*time.Second)) {
			return criteria, status, errors.New("the five-second acceptance undo window has expired")
		}
		criteria.Gate.Accept = nil
		status = "in_review"
	default:
		return criteria, status, errors.New("unknown delivery review action")
	}
	criteria.Review = review
	return criteria, status, nil
}
