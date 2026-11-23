package job

import "context"

// Names of the F074 jobs.
const (
	CurveRollout = "curve-rollout"
	XPReconcile  = "xp-reconcile"
)

// ProgressionRunner is the use case behind the progression jobs (command.Progression).
type ProgressionRunner interface {
	RolloutCurve(ctx context.Context, batch int) (int, error)
	ReconcileXP(ctx context.Context, batch int) (int, error)
}

// CurveRolloutJob is one run of curve-rollout (F074-AC-21, AC-22): profiles not yet on the current curve version.
func CurveRolloutJob(r ProgressionRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.RolloutCurve(ctx, Batch) }
}

// XPReconcileJob is one run of the nightly xp-reconcile (F074-AC-03): mismatches are reported, never corrected.
func XPReconcileJob(r ProgressionRunner) RunOnce {
	return func(ctx context.Context) (int, error) { return r.ReconcileXP(ctx, Batch) }
}
