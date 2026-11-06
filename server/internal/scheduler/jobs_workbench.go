package scheduler

import (
	"context"
	"time"
)

type WorkbenchDispatcher interface {
	DispatchWorkbenchPlans(context.Context) error
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
