package workbench

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Check struct {
	At string `json:"at"`
	By string `json:"by"`
}

type Item struct {
	Category string   `json:"category,omitempty"`
	Section  string   `json:"section,omitempty"`
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Origin   string   `json:"origin"`
	Covers   []string `json:"covers"`
	Checked  *Check   `json:"checked"`
	Note     *string  `json:"note"`
}

type GateCheck struct {
	PassedAt string `json:"passed_at"`
	PassedBy string `json:"passed_by"`
}

type Gates struct {
	Spec   *GateCheck `json:"spec"`
	Accept *GateCheck `json:"accept"`
}

type Requirements struct {
	Phase                     string                      `json:"phase,omitempty"`
	DraftRevision             int64                       `json:"draft_revision,omitempty"`
	ReadyRevision             int64                       `json:"ready_revision,omitempty"`
	DiscussionRevision        int64                       `json:"discussion_revision,omitempty"`
	CoveredDiscussionRevision int64                       `json:"covered_discussion_revision,omitempty"`
	Review                    *DeliveryReview             `json:"review,omitempty"`
	Delivery                  map[string]DeliveryEvidence `json:"delivery,omitempty"`
	Plan                      *ExecutionPlan              `json:"plan,omitempty"`
	NeedsResplit              bool                        `json:"needs_resplit,omitempty"`
	ResumeRuns                []string                    `json:"resume_runs,omitempty"`
	ResumeError               string                      `json:"resume_error,omitempty"`
	Change                    *RequirementChange          `json:"change,omitempty"`
	Template                  *ProjectTemplate            `json:"template,omitempty"`
	Team                      *ProjectTeamSnapshot        `json:"team,omitempty"`
	V                         int                         `json:"v"`
	Version                   int                         `json:"version"`
	NextItemNo                int                         `json:"next_item_no"`
	Items                     []Item                      `json:"items"`
	Bounds                    []string                    `json:"bounds"`
	Gate                      Gates                       `json:"gate"`
}

func RequirementsNeedConfirmation(criteria Requirements, status string, isChild bool) bool {
	return !isChild && criteria.Team != nil && RequirementsReady(criteria) && criteria.Gate.Spec == nil &&
		(status == "todo" || status == "backlog" || status == "paused") &&
		(criteria.Change == nil || criteria.Change.Phase == "editing")
}

func RequirementsReady(criteria Requirements) bool {
	return len(criteria.Items) > 0 && (criteria.Phase == "" ||
		(criteria.Phase == "ready" && criteria.DraftRevision > 0 && criteria.ReadyRevision == criteria.DraftRevision && criteria.CoveredDiscussionRevision == criteria.DiscussionRevision))
}

var itemID = regexp.MustCompile(`^A-([0-9]+)$`)
var placeholder = regexp.MustCompile(`(?i)\b(TBD|TODO)\b|待明确|待补充|待确认|\{[^{}]*\}`)

func ParseRequirements(raw []byte) (Requirements, error) {
	result := Requirements{V: 1, Version: 1, NextItemNo: 1, Items: []Item{}, Bounds: []string{}}
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.V != 1 {
		return result, errors.New("unsupported acceptance criteria format")
	}
	for _, item := range result.Items {
		parts := itemID.FindStringSubmatch(item.ID)
		if len(parts) != 2 {
			return result, errors.New("stored acceptance item has an invalid identifier")
		}
		number, err := strconv.Atoi(parts[1])
		if err != nil || number < 1 || number >= 1000000 {
			return result, errors.New("acceptance item identifier is out of range")
		}
		result.NextItemNo = max(result.NextItemNo, number+1)
	}
	return result, nil
}

// EditRequirements owns identifiers, check attribution and the confirmation lock.
// The caller commits the result with the issue revision as a compare-and-swap.
func EditRequirements(currentJSON, incomingJSON []byte, confirm bool, actorID, actorType string, now time.Time) ([]byte, error) {
	current, err := ParseRequirements(currentJSON)
	if err != nil {
		return nil, err
	}
	if current.Gate.Spec != nil {
		return nil, errors.New("confirmed requirements are read-only; request a requirements change first")
	}
	if current.Change != nil && current.Change.Phase != "editing" {
		return nil, errors.New("requirements cannot be edited until execution has stopped")
	}
	if confirm && actorType != "member" {
		return nil, errors.New("only a member can confirm requirements")
	}
	input := current
	if len(incomingJSON) > 0 {
		input = Requirements{}
		if json.Unmarshal(incomingJSON, &input) != nil || input.V != 1 {
			return nil, errors.New("acceptance criteria must be a version 1 object")
		}
	}
	if len(input.Items) > 200 || len(input.Bounds) > 100 {
		return nil, errors.New("too many acceptance items or boundaries")
	}
	previous := make(map[string]Item, len(current.Items))
	for _, item := range current.Items {
		previous[item.ID] = item
	}
	seen := make(map[string]bool, len(input.Items))
	boundsChanged := !slices.Equal(current.Bounds, input.Bounds)
	changed := len(current.Items) != len(input.Items) || boundsChanged
	stamp := now.UTC().Format(time.RFC3339Nano)
	for index := range input.Items {
		item := &input.Items[index]
		item.Text = strings.TrimSpace(item.Text)
		if len(item.Section) > 200 || (item.Category != "" && item.Category != "purpose" && item.Category != "scope" && item.Category != "behavior" && item.Category != "constraints" && item.Category != "acceptance") {
			return nil, errors.New("invalid requirement category or section")
		}
		if item.Text == "" || len([]rune(item.Text)) > 120 {
			return nil, errors.New("acceptance item text must contain 1 to 120 characters")
		}
		old, exists := previous[item.ID]
		changed = changed || !exists || item.Text != old.Text || item.Category != old.Category || item.Section != old.Section || !slices.Equal(item.Covers, old.Covers) || !equalNote(item.Note, old.Note)
		if item.ID == "" {
			item.ID = fmt.Sprintf("A-%02d", current.NextItemNo)
			current.NextItemNo++
			item.Origin = "user"
			if actorType == "agent" {
				item.Origin = "product"
			}
			item.Checked = nil
		} else if !exists || seen[item.ID] {
			return nil, errors.New("acceptance identifiers must be existing and unique")
		} else {
			item.Origin = old.Origin
			switch {
			case item.Text != old.Text || item.Category != old.Category || item.Section != old.Section || !slices.Equal(item.Covers, old.Covers) || !equalNote(item.Note, old.Note):
				item.Checked = nil
			case actorType != "member":
				item.Checked = old.Checked
			case item.Checked != nil && old.Checked == nil:
				item.Checked = &Check{At: stamp, By: actorID}
			case item.Checked != nil:
				item.Checked = old.Checked
			}
		}
		seen[item.ID] = true
		if boundsChanged {
			item.Checked = nil
		}
	}
	if current.Change == nil && (current.Phase != "" || input.Phase != "" || changed && current.Team != nil) {
		if changed {
			current.DraftRevision++
			current.Phase = "draft"
			current.ReadyRevision = 0
		}
		if actorType == "agent" && len(incomingJSON) > 0 {
			if input.CoveredDiscussionRevision != current.DiscussionRevision {
				return nil, errors.New("discussion changed; read the latest messages before updating requirements")
			}
			current.CoveredDiscussionRevision = input.CoveredDiscussionRevision
			switch input.Phase {
			case "ready":
				if len(input.Items) == 0 {
					return nil, errors.New("submit a nonempty requirements draft")
				}
				current.DraftRevision = max(current.DraftRevision, 1)
				current.Phase = "ready"
				current.ReadyRevision = current.DraftRevision
			case "clarifying", "draft":
				current.Phase = input.Phase
				current.ReadyRevision = 0
			case "":
				current.Phase = "draft"
				current.ReadyRevision = 0
			default:
				return nil, errors.New("unsupported requirements phase")
			}
		}
	}
	if confirm {
		candidate := current
		candidate.Items = input.Items
		if current.Change == nil && !RequirementsReady(candidate) {
			return nil, errors.New("requirements are still being discussed; wait for the current draft to be submitted")
		}
		if len(input.Items) == 0 {
			return nil, errors.New("at least one acceptance item is required")
		}
		for _, item := range input.Items {
			if item.Checked == nil || placeholder.MatchString(item.Text) {
				return nil, fmt.Errorf("%s must be checked and contain no placeholder", item.ID)
			}
		}
		current.Gate.Spec = &GateCheck{PassedAt: stamp, PassedBy: actorID}
		if current.Change != nil {
			current.NeedsResplit = true
			current.Change = nil
		}
	}
	current.Items = input.Items
	current.Bounds = input.Bounds
	if current.Items == nil {
		current.Items = []Item{}
	}
	if current.Bounds == nil {
		current.Bounds = []string{}
	}
	return json.Marshal(current)
}

func equalNote(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
