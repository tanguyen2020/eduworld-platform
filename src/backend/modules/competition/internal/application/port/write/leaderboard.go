package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// BoardQuery is port.BoardQuery.
type BoardQuery = port.BoardQuery

// ScopeRef is a (tenant, school, grade) with data in a period.
type ScopeRef struct {
	Tenant uuid.UUID
	School uuid.UUID
	Grade  int
}

// SnapshotRow is port.SnapshotRow.
type SnapshotRow = port.SnapshotRow

// LeaderboardRepository is the persistence of the leaderboard jobs and use cases (transaction of ctx).
type LeaderboardRepository interface {
	// Definitions returns the live definitions.
	Definitions(ctx context.Context) ([]ldb.Definition, error)
	// Ranking computes a board from the ledgers, best first (ties: who reached the value first).
	Ranking(ctx context.Context, q BoardQuery) ([]port.BoardMember, error)
	// Scopes lists the (tenant, school, grade) with entries of the metric of d in [start, end) (SystemPlatformScope:
	// ids only).
	Scopes(ctx context.Context, d ldb.Definition, start, end time.Time) ([]ScopeRef, error)
	// GradeGoal returns the effort of one grade of one school in [start, end) and its number of contributors.
	GradeGoal(ctx context.Context, tenant, school uuid.UUID, grade int, start, end time.Time) (float64, int, error)
	// SnapshotTaken reports whether the period of the board in the scope was snapshotted already.
	SnapshotTaken(ctx context.Context, board uuid.UUID, scope, period string) (bool, error)
	// InsertSnapshot appends the snapshot rows of one board period (one scope).
	InsertSnapshot(ctx context.Context, rows []SnapshotRow) error
	// PublishedDefinitions returns the definitions of the published leaderboard-definitions configuration in effect.
	PublishedDefinitions(ctx context.Context, at time.Time) ([]ldb.Definition, *uuid.UUID, error)
	// UpsertDefinition materializes a published definition (SystemPlatformScope).
	UpsertDefinition(ctx context.Context, d ldb.Definition, version *uuid.UUID) (bool, error)
}

// SeasonRepository is the persistence of the season scheduler (platform G table seasons; season_results and the new
// season ratings per tenant).
type SeasonRepository interface {
	// Seasons returns the live seasons, oldest first.
	Seasons(ctx context.Context) ([]SeasonRow, error)
	// SetStatus moves a season from → to at at; false when it is not in from (idempotency of the transitions).
	SetStatus(ctx context.Context, season uuid.UUID, from, to string, at time.Time) (bool, error)
	// TenantsWithRatings returns the tenants holding ratings of season (SystemPlatformScope, ids only).
	TenantsWithRatings(ctx context.Context, season uuid.UUID) ([]uuid.UUID, error)
	// WriteResults stores the season results of the tenant from its ratings and ledger (idempotent); returns the rows.
	WriteResults(ctx context.Context, tenant, season uuid.UUID) (int, error)
	// SoftReset creates the ratings of season to from the ratings of season from (idempotent) with step.
	SoftReset(ctx context.Context, tenant, from, to uuid.UUID, rule uuid.UUID, step func(ranking.Rating) ranking.Rating) (int, error)
	// InflateInactive applies the no-games step to the ratings of season neither played nor inflated since before.
	InflateInactive(ctx context.Context, tenant, season uuid.UUID, before time.Time, step func(ranking.Rating) ranking.Rating) (int, error)
}
