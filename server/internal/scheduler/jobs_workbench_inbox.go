package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const WorkbenchInboxRetention = 90 * 24 * time.Hour

func WorkbenchInboxPruneJob(queries *db.Queries) JobSpec {
	return JobSpec{
		Name: "prune_workbench_inbox", Cadence: time.Hour, CatchUpMode: CatchUpLatestOnly, CatchUpWindow: 24 * time.Hour,
		RunTimeout: 10 * time.Minute, StaleTimeout: 15 * time.Minute, HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true, MaxAttempts: 3, RetryBackoff: []time.Duration{time.Minute, 5 * time.Minute},
		Scopes: StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, in HandlerInput) (HandlerResult, error) {
			cutoff := in.PlanTime.Add(-WorkbenchInboxRetention)
			result := HandlerResult{Result: map[string]any{"cutoff": cutoff.UTC().Format(time.RFC3339)}}
			for batch := 0; batch < 200; batch++ {
				deleted, err := queries.PruneWorkbenchInbox(ctx, db.PruneWorkbenchInboxParams{
					Cutoff: pgtype.Timestamptz{Time: cutoff, Valid: true}, BatchSize: 5000,
				})
				if err != nil {
					return result, fmt.Errorf("prune workbench inbox: %w", err)
				}
				result.RowsAffected += deleted
				if deleted < 5000 {
					break
				}
				if in.Heartbeat != nil {
					if err := in.Heartbeat(ctx); err != nil {
						return result, err
					}
				}
			}
			return result, nil
		},
	}
}
