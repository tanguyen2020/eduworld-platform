package write

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// PartitionMaint implements write.PartitionMaintenance through learning.ensure_monthly_partitions and
// learning.monthly_partitions_ahead (DB is the learning_partition_admin pool: EXECUTE on those functions only).
type PartitionMaint struct {
	DB *sql.DB
}

var _ write.PartitionMaintenance = PartitionMaint{}

// EnsurePartitions implements write.PartitionMaintenance.
func (m PartitionMaint) EnsurePartitions(ctx context.Context, from time.Time, months int) (int, error) {
	var n int
	if err := m.DB.QueryRowContext(ctx, `SELECT learning.ensure_monthly_partitions($1::date, $2)`, from.UTC().Format(time.DateOnly), months).
		Scan(&n); err != nil {
		return 0, fmt.Errorf("ensure learning partitions: %w", err)
	}
	return n, nil
}

// MonthsAhead implements write.PartitionMaintenance.
func (m PartitionMaint) MonthsAhead(ctx context.Context, table string, from time.Time) (int, error) {
	var n int
	if err := m.DB.QueryRowContext(ctx, `SELECT learning.monthly_partitions_ahead($1, $2::date)`, table, from.UTC().Format(time.DateOnly)).
		Scan(&n); err != nil {
		return 0, fmt.Errorf("learning partitions ahead: %w", err)
	}
	return n, nil
}
