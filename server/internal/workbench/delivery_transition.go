package workbench

import "slices"

// DeliveryTransition derives a parent transition from the same evidence checks
// used by human acceptance. Incomplete or stale reports never advance a parent.
func DeliveryTransition(criteria Requirements, tasks []DeliveryTask, active bool, description, currentStatus string) (string, string) {
	if currentStatus == "todo" {
		if criteria.Gate.Spec == nil || criteria.Team == nil || criteria.Plan == nil || criteria.Plan.Version != criteria.Version || criteria.Change != nil || criteria.NeedsResplit {
			return "", ""
		}
		for _, task := range tasks {
			if slices.Contains(criteria.Plan.Issues, task.ID) && !task.Handoff.Superseded && task.Handoff.RequirementVersion == criteria.Version &&
				slices.Contains([]string{"in_progress", "in_review", "blocked", "done"}, task.Status) {
				return "in_progress", "plan_started"
			}
		}
		return "", ""
	}
	if currentStatus != "in_progress" && currentStatus != "in_review" {
		return "", ""
	}
	blockers := DeliveryBlockers(criteria, tasks, active, description)
	if len(blockers) == 0 {
		return "in_review", "delivery_ready"
	}
	for _, blocker := range blockers {
		if blocker.Code != "evidence_failed" {
			return "", ""
		}
	}
	passed := map[string]bool{}
	for _, id := range criteria.Plan.Issues {
		evidence := criteria.Delivery[id]
		if evidence.Role == "test" || evidence.Role == "sec" {
			previous, exists := passed[evidence.Role]
			passed[evidence.Role] = (!exists || previous) && evidence.Passed
		}
	}
	if len(passed) == 2 && passed["test"] != passed["sec"] {
		return "blocked", "test_security_conflict"
	}
	return "", ""
}
