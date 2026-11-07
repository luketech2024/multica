package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	inboxwriter "github.com/multica-ai/multica/server/internal/inbox"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (s *TaskService) RemindExpiringCredentials(ctx context.Context) error {
	now := time.Now().UTC()
	cursor := pgtype.UUID{Valid: true}
	for {
		credentials, err := s.Queries.ListExpiringWorkbenchCredentials(ctx, db.ListExpiringWorkbenchCredentialsParams{
			NowAt: pgtype.Timestamptz{Time: now, Valid: true}, UntilAt: pgtype.Timestamptz{Time: now.Add(7 * 24 * time.Hour), Valid: true}, AfterID: cursor,
		})
		if err != nil {
			return err
		}
		for _, credential := range credentials {
			cursor = credential.ID
			if err := s.remindExpiringCredential(ctx, credential, now); err != nil {
				return err
			}
		}
		if len(credentials) < 100 {
			return nil
		}
	}
}

func (s *TaskService) remindExpiringCredential(ctx context.Context, candidate db.ListExpiringWorkbenchCredentialsRow, now time.Time) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err := q.LockWorkspaceForChatSessionCreate(ctx, candidate.WorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if candidate.ProjectID.Valid {
		if _, err := q.LockProjectForChatSessionCreate(ctx, db.LockProjectForChatSessionCreateParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ProjectID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
	}
	credential, err := q.GetWorkbenchCredentialForUpdate(ctx, db.GetWorkbenchCredentialForUpdateParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !credential.ExpiresAt.Valid || !credential.ExpiresAt.Time.After(now) || credential.ExpiresAt.Time.After(now.Add(7*24*time.Hour)) {
		return nil
	}
	managers, err := q.ListWorkspaceManagerUserIDs(ctx, credential.WorkspaceID)
	if err != nil {
		return err
	}
	expires := credential.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	var notices []db.InboxItem
	for _, manager := range managers {
		exists, err := q.HasCredentialExpiryNotice(ctx, db.HasCredentialExpiryNoticeParams{
			WorkspaceID: credential.WorkspaceID, RecipientID: manager, CredentialID: util.UUIDToString(credential.ID), ExpiresAt: expires,
		})
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		details, _ := json.Marshal(map[string]string{"source": "workbench_credential", "workbench": "true",
			"credential_id": util.UUIDToString(credential.ID), "expires_at": expires})
		notice, inserted, err := inboxwriter.CreateIfEligible(ctx, q, db.CreateInboxItemParams{
			ID: dbid.NewV7(), WorkspaceID: credential.WorkspaceID, RecipientType: "member", RecipientID: manager,
			Type: "review_requested", Severity: "action_required", Title: credential.Label, ActorType: util.StrToText("system"), Details: details,
		})
		if err != nil {
			return err
		}
		if inserted {
			notices = append(notices, notice)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, notice := range notices {
		(&IssueWakeupService{Tasks: s}).publishSystemInbox(notice, "")
	}
	return nil
}
