package job

import (
	"context"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
)

// Intervals of the F016 jobs [Proposed] (the hour of the cleanup comes from inactive_student_cleanup_days).
const (
	StudentCleanupInterval = 24 * time.Hour // student-inactive-cleanup
	SagaRetryInterval      = time.Minute    // saga-retry (resume due runs, SagaStuck gauge)
	SagaStuckAfter         = 2 * time.Hour  // [Proposed] threshold of the SagaStuck alert
)

// StudentInactiveCleanup is the job student-inactive-cleanup.
func StudentInactiveCleanup(c command.StudentCleanup) RunOnce { return c.RunAll }

// SagaRetry is the job saga-retry over the given definitions.
func SagaRetry(e saga.Engine, defs map[string]saga.Definition) RunOnce {
	return func(ctx context.Context) (int, error) { return e.ResumeDue(ctx, defs, 100, SagaStuckAfter) }
}
