package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func ResolveOneSpecFlow(ctx context.Context, q *db.Queries, issue db.Issue, kind string) (workbench.OneSpecFlow, error) {
	if !issue.ProjectID.Valid || issue.ParentIssueID.Valid {
		return workbench.OneSpecFlow{}, pgx.ErrNoRows
	}
	config, err := q.GetOneSpecProject(ctx, db.GetOneSpecProjectParams{WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID})
	if err != nil {
		return workbench.OneSpecFlow{}, err
	}
	if !config.Enabled {
		return workbench.OneSpecFlow{}, pgx.ErrNoRows
	}
	if len(workbench.OneSpecStages(kind)) == 0 {
		return workbench.OneSpecFlow{}, errors.New("unknown workflow kind")
	}
	runtime := config.RuntimeID
	if !runtime.Valid {
		defaults, e := q.GetOneSpecWorkspace(ctx, issue.WorkspaceID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return workbench.OneSpecFlow{}, e
		}
		runtime = defaults.DefaultRuntimeID
	}
	project, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return workbench.OneSpecFlow{}, err
	}
	team, _, err := workbench.LoadProjectRoles(ctx, q, project)
	if err != nil {
		return workbench.OneSpecFlow{}, err
	}
	flow := workbench.OneSpecFlow{V: 1, Kind: kind, Phase: "requirements", Status: "preparing", RuleVersion: workbench.OneSpecVersion, Team: team, Artifacts: []workbench.OneSpecArtifact{}, Decisions: []workbench.OneSpecDecision{}, GitOperations: []workbench.OneSpecGitOperation{}}
	if runtime.Valid {
		flow.RuntimeID = util.UUIDToString(runtime)
	}
	if config.OverviewRepositoryID.Valid {
		flow.OverviewRepositoryID = util.UUIDToString(config.OverviewRepositoryID)
	}
	if issue.CreatorType == "member" {
		flow.ApproverID = util.UUIDToString(issue.CreatorID)
	} else if project.LeadType.String == "member" && project.LeadID.Valid {
		flow.ApproverID = util.UUIDToString(project.LeadID)
	}
	return flow, nil
}

func InitializeOneSpecFlow(ctx context.Context, q *db.Queries, issue db.Issue, kind string) (db.OnespecFlow, error) {
	flow, err := ResolveOneSpecFlow(ctx, q, issue, kind)
	if err != nil {
		return db.OnespecFlow{}, err
	}
	raw, _ := json.Marshal(flow)
	return q.CreateOneSpecFlow(ctx, db.CreateOneSpecFlowParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, State: raw})
}
