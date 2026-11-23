package job

import "context"

// Names of the live-ops jobs (F077).
const (
	EventScheduler = "event-scheduler"
	GoalRecompute  = "goal-recompute"
)

// LiveOpsRunner is the use case behind the live-ops jobs (command.LiveOps).
type LiveOpsRunner interface {
	RunScheduler(ctx context.Context, batch int) (int, error)
	RecomputeGoals(ctx context.Context, batch int) (int, error)
}

// EventSchedulerJob is one tick of event-scheduler (F077-AC-01, AC-05, AC-11): under the singleton lock
// ew:lock:job:event-scheduler, due runs move (scheduled → live → ended → archived) and newer versions in effect are
// pinned; a transition happens once whatever the instances.
func EventSchedulerJob(r LiveOpsRunner, batch int) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RunScheduler(ctx, batch) }
}

// GoalRecomputeJob is one run of goal-recompute (F077-AC-14): the community goal totals are recomputed from the
// contributions (the truth) and the Valkey counters rewritten.
func GoalRecomputeJob(r LiveOpsRunner, batch int) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RecomputeGoals(ctx, batch) }
}
