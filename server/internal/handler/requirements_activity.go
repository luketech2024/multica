package handler

import (
	"context"
	"encoding/json"
	"errors"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func recordRequirementsActivity(ctx context.Context, q *db.Queries, before, after db.Issue) (db.ActivityLog, error) {
	actor, ok := ctx.Value(wakeupActorKey{}).(wakeupActor)
	if !ok {
		return db.ActivityLog{}, errors.New("requirements audit requires an authenticated actor")
	}
	details, err := json.Marshal(map[string]any{
		"before":   json.RawMessage(before.AcceptanceCriteria),
		"after":    json.RawMessage(after.AcceptanceCriteria),
		"revision": after.Revision,
	})
	if err != nil {
		return db.ActivityLog{}, err
	}
	return q.CreateActivity(ctx, db.CreateActivityParams{
		ID: dbid.NewV7(), WorkspaceID: after.WorkspaceID, IssueID: after.ID,
		ActorType: strToText(actor.kind), ActorID: optionalActorUUID(actor.id),
		Action: "requirements_updated", Details: details,
	})
}

func (h *Handler) publishRequirementsActivity(activity db.ActivityLog) {
	if !activity.ID.Valid {
		return
	}
	h.publish(protocol.EventActivityCreated, uuidToString(activity.WorkspaceID), activity.ActorType.String, uuidToString(activity.ActorID), map[string]any{
		"issue_id": uuidToString(activity.IssueID),
		"entry": map[string]any{
			"type": "activity", "id": uuidToString(activity.ID),
			"actor_type": activity.ActorType.String, "actor_id": uuidToString(activity.ActorID),
			"action": activity.Action, "details": json.RawMessage(activity.Details),
			"created_at": timestampToString(activity.CreatedAt),
		},
	})
}
