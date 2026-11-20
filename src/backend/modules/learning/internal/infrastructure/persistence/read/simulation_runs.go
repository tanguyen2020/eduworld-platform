package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
)

// SimulationRunModel implements read.SimulationRunReadModel on assessment.simulation_runs_live (F066).
type SimulationRunModel struct{}

var _ read.SimulationRunReadModel = SimulationRunModel{}

// RunCount implements read.SimulationRunReadModel.
func (SimulationRunModel) RunCount(ctx context.Context, attemptID uuid.UUID) (int, error) {
	q, err := txOf(ctx)
	if err != nil {
		return 0, err
	}
	// tenancyguard:ok tenant of the session (RLS + infra.current_tenant_id()); the _live view has deleted_at IS NULL
	rows, err := q.QueryContext(ctx, `SELECT count(*) FROM assessment.simulation_runs_live
		WHERE tenant_id = infra.current_tenant_id() AND attempt_id = $1 AND deleted_at IS NULL`, attemptID)
	if err != nil {
		return 0, fmt.Errorf("simulation runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("simulation runs: %w", err)
		}
	}
	return n, rows.Err()
}
