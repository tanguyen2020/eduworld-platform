package command

import (
	"context"
	"log/slog"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// PartitionMaintainer is the job chat-partition-maintainer (F117 T034): it keeps daily partitions of
// chat.gameplay_messages and chat.ephemeral_messages ahead (SECURITY DEFINER chat.ensure_message_partitions), so that the
// purge can drop a whole day (DATABASE_DESIGN §3.8b). It never drops anything.
type PartitionMaintainer struct {
	UoW     write.UnitOfWork
	Store   port.PurgeStore
	Metrics port.PurgeMetrics
	Days    int // partitions created from today (EW_CHAT_PARTITION_DAYS_AHEAD)
	Now     func() time.Time
}

// Run creates the missing partitions and reports the days covered.
func (h PartitionMaintainer) Run(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.job.PartitionMaintainer")
	defer func() { done(err) }()
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var ahead map[string]int
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		n, ahead, err = h.Store.EnsurePartitions(ctx, day, max(h.Days, 2))
		return err
	})
	if err != nil {
		return 0, err
	}
	for table, d := range ahead {
		h.Metrics.PartitionsAhead(table, d)
		if d < 2 {
			slog.WarnContext(ctx, "chat: message partitions missing ahead", "table", table, "days_ahead", d, "alert", "P3")
		}
	}
	return n, nil
}
