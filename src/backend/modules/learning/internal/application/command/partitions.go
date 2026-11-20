package command

import (
	"context"
	"log/slog"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Partition horizon of the learning tables (DATABASE_DESIGN §5: three months ahead; F031-AC-04 requires two).
const (
	PartitionMonthsAhead    = 3
	PartitionMinMonthsAhead = 2
)

// PartitionMetrics exposes eduworld_learning_events_partition_months_ahead{table}; nil = not exposed.
type PartitionMetrics interface {
	MonthsAhead(table string, months int)
}

// LearningPartitions is the job learning-events-partitions (F031-AC-04): it keeps the current month and the next
// PartitionMonthsAhead months of every monthly partitioned learning table, and warns (log + gauge, alert
// LearningPartitionsMissing) while fewer than PartitionMinMonthsAhead future months exist.
type LearningPartitions struct {
	Maint   write.PartitionMaintenance
	Metrics PartitionMetrics
	Clock   func() time.Time
}

// RunOnce creates the missing partitions and returns how many were created.
func (h LearningPartitions) RunOnce(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.LearningEventsPartitions")
	defer func() { done(err) }()
	t := now(h.Clock).UTC()
	month := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	n, ensureErr := h.Maint.EnsurePartitions(ctx, month, PartitionMonthsAhead+1)
	for _, table := range write.PartitionedTables {
		months, err := h.Maint.MonthsAhead(ctx, table, month)
		if err != nil {
			return n, err
		}
		ahead := max(months-1, 0) // future months after the current one
		if h.Metrics != nil {
			h.Metrics.MonthsAhead(table, ahead)
		}
		if ahead < PartitionMinMonthsAhead {
			slog.WarnContext(ctx, "learning partitions running out", "table", table, "months_ahead", ahead)
		}
	}
	return n, ensureErr
}
