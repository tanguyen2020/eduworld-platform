package job

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// AssignmentRunner opens and closes the school assignments (command.Assignments, F041).
type AssignmentRunner interface {
	RunScheduler(ctx context.Context, limit int) (command.SchedulerReport, error)
}

// AssignmentScheduler is the job assignment-scheduler of the worker profile jobs (F041-AC-18, AC-20): every minute, the
// scheduled assignments whose open_at is past open, the open ones past their due date plus late window close.
type AssignmentScheduler struct {
	Runner AssignmentRunner
	Batch  int
}

// RunOnce opens and closes up to Batch assignments of each kind and returns how many changed.
func (w AssignmentScheduler) RunOnce(ctx context.Context) (int, error) {
	rep, err := w.Runner.RunScheduler(ctx, w.Batch)
	return rep.Opened + rep.Closed, err
}
