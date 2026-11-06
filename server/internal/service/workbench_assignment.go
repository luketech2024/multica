package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func assignInitialWorkbenchRole(ctx context.Context, q *db.Queries, params *IssueCreateParams, criteria workbench.Requirements) error {
	if params.ParentIssueID.Valid || params.CreatorType != "member" || criteria.Team == nil {
		return nil
	}
	if params.UseProjectAssignee != nil && !*params.UseProjectAssignee {
		return nil
	}
	if params.UseProjectAssignee == nil && (params.AssigneeType.Valid || params.AssigneeID.Valid) {
		return nil
	}
	roleName := "sched"
	if len(criteria.Items) == 0 {
		roleName = "product"
	}
	role := criteria.Team.Config.Roles[roleName]
	if role.AgentID == nil {
		params.AssigneeType = pgtype.Text{String: "member", Valid: true}
		params.AssigneeID = params.CreatorID
		return nil
	}
	id, err := util.ParseUUID(*role.AgentID)
	if err != nil || !role.Enabled {
		return fmt.Errorf("%w: project %s role is unavailable", ErrIssueRequirements, roleName)
	}
	agent, err := q.GetAgent(ctx, id)
	if err != nil || agent.WorkspaceID != params.WorkspaceID || agent.ArchivedAt.Valid {
		return fmt.Errorf("%w: project %s agent is unavailable", ErrIssueRequirements, roleName)
	}
	if role.RuntimeID != nil && *role.RuntimeID != util.UUIDToString(agent.RuntimeID) {
		return fmt.Errorf("%w: project %s runtime no longer matches the configured agent", ErrIssueRequirements, roleName)
	}
	allowed, err := memberMayInvokeAgent(ctx, q, agent, params.CreatorID, params.WorkspaceID)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: project %s agent cannot be invoked by this member", ErrIssueRequirements, roleName)
	}
	params.AssigneeType = pgtype.Text{String: "agent", Valid: true}
	params.AssigneeID = agent.ID
	return nil
}
