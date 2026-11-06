package workbench

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ProjectRoleConfiguration struct {
	Config   ProjectRoles `json:"config"`
	Revision int64        `json:"revision"`
}

type ProjectTeamSnapshot struct {
	ProjectID string `json:"project_id"`
	ProjectRoleConfiguration
}

func LoadProjectRoles(ctx context.Context, q *db.Queries, project db.Project) (ProjectRoleConfiguration, db.ProjectResource, error) {
	resource, err := q.GetProjectWorkbenchRoles(ctx, db.GetProjectWorkbenchRolesParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectRoleConfiguration{Config: DefaultProjectRoles(), Revision: 1}, resource, nil
	}
	if err != nil {
		return ProjectRoleConfiguration{}, resource, err
	}
	var response ProjectRoleConfiguration
	if err = json.Unmarshal(resource.ResourceRef, &response); err != nil || response.Revision < 1 {
		return response, resource, errors.New("invalid saved project roles")
	}
	raw, _ := json.Marshal(response.Config)
	_, err = ParseProjectRoles(raw)
	return response, resource, err
}
