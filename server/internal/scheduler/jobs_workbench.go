package scheduler

import (
	"context"
	"time"
)

type WorkbenchDispatcher interface {
	DispatchWorkbenchPlans(context.Context) error
}

type SecurityAcceptanceReconciler interface {
	ReconcileSecurityAcceptances(context.Context) error
}

func SecurityAcceptanceJob(reconciler SecurityAcceptanceReconciler) JobSpec {
	return JobSpec{
		Name: "security_acceptance_expiry", Cadence: time.Hour, CatchUpMode: CatchUpLatestOnly, CatchUpWindow: 24 * time.Hour,
		RunTimeout: 10 * time.Minute, StaleTimeout: 15 * time.Minute, HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true, MaxAttempts: 3, Scopes: StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, _ HandlerInput) (HandlerResult, error) {
			return HandlerResult{}, reconciler.ReconcileSecurityAcceptances(ctx)
		},
	}
}

func WorkbenchDispatchJob(dispatcher WorkbenchDispatcher) JobSpec {
	return JobSpec{
		Name: "workbench_plan_dispatch", Cadence: 10 * time.Second, CatchUpMode: CatchUpLatestOnly, CatchUpWindow: time.Hour,
		RunTimeout: 30 * time.Second, StaleTimeout: time.Minute, HeartbeatInterval: 10 * time.Second,
		AllowStaleReentry: true, MaxAttempts: 1, Scopes: StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, _ HandlerInput) (HandlerResult, error) {
			return HandlerResult{}, dispatcher.DispatchWorkbenchPlans(ctx)
		},
	}
}
