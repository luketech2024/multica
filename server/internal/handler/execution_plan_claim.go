package handler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) resolveIssueClaimProjectContext(ctx context.Context, issue db.Issue, runtime db.AgentRuntime) (claimProjectContext, error) {
	if len(issue.Properties) == 0 {
		return h.resolveClaimProjectContext(ctx, issue.ProjectID, issue.WorkspaceID)
	}
	var properties struct {
		Handoff *workbench.Handoff `json:"workbench_handoff"`
	}
	if err := json.Unmarshal(issue.Properties, &properties); err != nil {
		return claimProjectContext{}, err
	}
	if properties.Handoff == nil {
		return h.resolveClaimProjectContext(ctx, issue.ProjectID, issue.WorkspaceID)
	}
	handoff := properties.Handoff
	if handoff.V != 2 || handoff.Superseded {
		return claimProjectContext{}, errors.New("invalid or retired workbench handoff")
	}
	project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return claimProjectContext{}, err
	}
	result := claimProjectContext{ProjectID: uuidToString(project.ID), Title: project.Title, Description: project.Description.String}
	if handoff.RepoID == "" {
		if handoff.Role == "dev" {
			return claimProjectContext{}, errors.New("development handoff requires a repository")
		}
		return result, nil
	}
	rows, err := h.Queries.ListProjectResourcesInWorkspace(ctx, db.ListProjectResourcesInWorkspaceParams{ProjectID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return claimProjectContext{}, err
	}
	for _, row := range rows {
		if uuidToString(row.ID) == handoff.RepoID {
			if row.ResourceType == "local_directory" {
				var ref localDirectoryRef
				if json.Unmarshal(row.ResourceRef, &ref) != nil || ref.DaemonID == "" || ref.DaemonID != runtime.DaemonID.String {
					return claimProjectContext{}, errors.New("the selected local repository belongs to another runtime machine")
				}
			}
			result.Resources, result.Repos = projectResourcesForClaim([]db.ProjectResource{row})
			return result, nil
		}
	}
	return claimProjectContext{}, errors.New("handoff repository no longer belongs to this project")
}
