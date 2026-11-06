package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	inboxwriter "github.com/multica-ai/multica/server/internal/inbox"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (s *TaskService) ReconcileSecurityAcceptances(ctx context.Context) error {
	now := time.Now().UTC()
	cursor := pgtype.UUID{Valid: true}
	for {
		checks, err := s.Queries.ListSecurityAcceptanceCandidates(ctx, db.ListSecurityAcceptanceCandidatesParams{
			UntilAt: pgtype.Timestamptz{Time: now.Add(72 * time.Hour), Valid: true}, AfterID: cursor,
		})
		if err != nil {
			return err
		}
		for _, check := range checks {
			cursor = check.ID
			if err := s.reconcileSecurityAcceptance(ctx, check, now); err != nil {
				return err
			}
		}
		if len(checks) < 100 {
			return nil
		}
	}
}

func (s *TaskService) reconcileSecurityAcceptance(ctx context.Context, candidate db.SecurityCheck, now time.Time) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	// Match repository deletion and scan ingestion lock order.
	if _, err := q.LockProjectForChatSessionCreate(ctx, db.LockProjectForChatSessionCreateParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ProjectID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := q.GetProjectResourceInWorkspaceForUpdate(ctx, db.GetProjectResourceInWorkspaceForUpdateParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.RepoID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	check, err := q.LockSecurityCheck(ctx, db.LockSecurityCheckParams{WorkspaceID: candidate.WorkspaceID, ID: candidate.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if check.Status != "accepted" || !check.AcceptedUntil.Valid || check.AcceptedUntil.Time.After(now.Add(72*time.Hour)) {
		return nil
	}
	if !check.AcceptedUntil.Time.After(now) {
		after, err := q.UpdateSecurityCheck(ctx, db.UpdateSecurityCheckParams{WorkspaceID: check.WorkspaceID, ID: check.ID,
			ExpectedRevision: check.Revision, Status: "unresolved", FixVersion: check.FixVersion,
			FixRequestedAt: check.FixRequestedAt, FixedScanID: check.FixedScanID, AcceptedUntil: check.AcceptedUntil, AcceptanceReason: check.AcceptanceReason})
		if err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"check_id": util.UUIDToString(check.ID), "project_id": util.UUIDToString(check.ProjectID), "before": check, "after": after})
		if _, err := q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: check.WorkspaceID,
			ActorType: util.StrToText("system"), Action: "security_acceptance_expired", Details: details}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	managers, err := q.ListWorkspaceManagerUserIDs(ctx, check.WorkspaceID)
	if err != nil {
		return err
	}
	expires := check.AcceptedUntil.Time.UTC().Format(time.RFC3339Nano)
	var notices []db.InboxItem
	for _, manager := range managers {
		exists, err := q.HasSecurityAcceptanceNotice(ctx, db.HasSecurityAcceptanceNoticeParams{WorkspaceID: check.WorkspaceID,
			RecipientID: manager, CheckID: util.UUIDToString(check.ID), AcceptedUntil: expires})
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		details, _ := json.Marshal(map[string]string{"source": "workbench_security", "workbench": "true",
			"check_id": util.UUIDToString(check.ID), "accepted_until": expires, "reason": "acceptance_expiring"})
		notice, inserted, err := inboxwriter.CreateIfEligible(ctx, q, db.CreateInboxItemParams{ID: dbid.NewV7(), WorkspaceID: check.WorkspaceID,
			RecipientType: "member", RecipientID: manager, Type: "review_requested", Severity: "action_required",
			Title: fmt.Sprintf("SEC-%02d %s", check.Number, check.Title), ActorType: util.StrToText("system"), Details: details})
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
