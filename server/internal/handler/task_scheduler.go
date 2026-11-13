package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A human-selected discussion agent coordinates this task when the project has
// no explicit scheduler. Persist the choice in the task snapshot, not the project.
func bindSelectedTaskScheduler(ctx context.Context, q *db.Queries, issue db.Issue, criteria *workbench.Requirements) error {
	if criteria.Team == nil || issue.ParentIssueID.Valid {
		return nil
	}
	role := criteria.Team.Config.Roles["sched"]
	if !role.Enabled || role.AgentID != nil || issue.AssigneeType.String != "agent" || !issue.AssigneeID.Valid {
		return nil
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: issue.AssigneeID, WorkspaceID: issue.WorkspaceID})
	if err != nil || agent.ArchivedAt.Valid {
		return errors.New("the selected task coordinator is unavailable; select an active agent before confirming requirements")
	}
	if !agent.RuntimeID.Valid {
		return errors.New("the selected task coordinator has no runtime; configure its runtime before confirming requirements")
	}
	if strings.HasPrefix(agent.SystemKey.String, "onespec:") && agent.SystemKey.String != "onespec:sched" {
		return errors.New("configure the project scheduling role before confirming requirements; this platform agent has a different role")
	}
	id, runtime := uuidToString(agent.ID), uuidToString(agent.RuntimeID)
	role.AgentID = &id
	role.RuntimeID = &runtime
	criteria.Team.Config.Roles["sched"] = role
	return nil
}
