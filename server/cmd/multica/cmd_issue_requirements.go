package main

import (
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/workbench"
	"github.com/spf13/cobra"
)

func applyIssueCreateRequirements(cmd *cobra.Command, body map[string]any) error {
	items, _ := cmd.Flags().GetStringArray("criterion")
	bounds, _ := cmd.Flags().GetStringArray("boundary")
	external, _ := cmd.Flags().GetString("external-ticket")
	unassigned, _ := cmd.Flags().GetBool("unassigned")
	_, parent := body["parent_issue_id"]
	_, project := body["project_id"]
	if unassigned {
		if cmd.Flags().Changed("assignee") || cmd.Flags().Changed("assignee-id") || cmd.Flags().Changed("assignee-picker") {
			return fmt.Errorf("--unassigned cannot be combined with assignee flags")
		}
		body["use_project_assignee"] = false
	}
	if external != "" {
		if parent {
			return fmt.Errorf("external ticket numbers belong to parent tasks")
		}
		if err := workbench.ValidateExternalTicket(external); err != nil {
			return err
		}
		body["external_ticket_no"] = external
	}
	if len(items) == 0 && len(bounds) == 0 && (!project || parent) {
		return nil
	}
	if status, _ := body["status"].(string); status != "" && status != "todo" && status != "backlog" {
		return fmt.Errorf("new requirements must start in todo or backlog")
	}
	if len(items) > 200 || len(bounds) > 100 {
		return fmt.Errorf("at most 200 acceptance criteria and 100 boundaries are allowed")
	}
	criteria := workbench.Requirements{V: 1, Items: []workbench.Item{}, Bounds: []string{}}
	for _, text := range items {
		text = strings.TrimSpace(text)
		if text == "" || len([]rune(text)) > 120 {
			return fmt.Errorf("each --criterion requires 1 to 120 characters")
		}
		criteria.Items = append(criteria.Items, workbench.Item{Text: text})
	}
	for _, text := range bounds {
		text = strings.TrimSpace(text)
		if text == "" {
			return fmt.Errorf("--boundary cannot be empty")
		}
		criteria.Bounds = append(criteria.Bounds, text)
	}
	body["acceptance_criteria"] = criteria
	return nil
}
