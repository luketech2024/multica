package handler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workbench"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func createOneSpecReleaseDraft(ctx context.Context, q *db.Queries, issue db.Issue, creator pgtype.UUID, input workbench.ReleaseOrderInput) (db.WorkbenchReleaseOrder, error) {
	input.ProjectID = uuidToString(issue.ProjectID)
	if input.Delivery == nil {
		input.Delivery = &workbench.ApplicationRelease{Title: issue.Title, Mode: "orchestrated", Changes: []workbench.ReleaseChange{}}
	}
	if len(input.Tasks) > 0 {
		return db.WorkbenchReleaseOrder{}, errors.New("a workflow release draft uses application changes")
	}
	if err := input.Normalize(); err != nil {
		return db.WorkbenchReleaseOrder{}, err
	}
	config, _, err := readProjectDeployment(ctx, q, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return db.WorkbenchReleaseOrder{}, err
	}
	for _, change := range input.Delivery.Changes {
		found := false
		for _, app := range config.Applications {
			if app.ID == change.ApplicationID {
				found = true
				break
			}
		}
		if !found {
			return db.WorkbenchReleaseOrder{}, errors.New("release application must belong to this project")
		}
	}
	number, err := q.NextWorkbenchReleaseNumber(ctx, issue.WorkspaceID)
	if err != nil {
		return db.WorkbenchReleaseOrder{}, err
	}
	id := dbid.NewV7()
	manifest, _ := json.Marshal(workbench.ReleaseManifest{Delivery: input.Delivery, Tasks: []workbench.ReleaseTaskSnapshot{}, Repositories: []workbench.ReleaseRepositorySnapshot{}})
	return q.CreateWorkbenchRelease(ctx, db.CreateWorkbenchReleaseParams{ID: id, WorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID, Number: number, Env: input.Env, RequestID: dbid.NewV7(), RequestHash: input.Digest(), Manifest: manifest, CreatedBy: creator})
}
