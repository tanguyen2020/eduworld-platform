package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// RunCursor is the keyset cursor of the runs of a student (starts_at, event_run_id).
type RunCursor struct {
	StartsAt time.Time
	ID       uuid.UUID
}

// StudentRunsFilter selects the runs a student sees (F077-AC-07): live and suspended runs, ended runs before claim_until
// (live), scheduled runs from upcoming before starts_at (upcoming), of the tenant's audience (school and grade filtered by
// the caller).
type StudentRunsFilter struct {
	Tenant   uuid.UUID
	School   uuid.UUID
	Grade    int
	Now      time.Time
	Upcoming time.Duration
	Live     bool
	Soon     bool
	After    *RunCursor
	Limit    int
}

// SeasonHistoryRow is one past season of a student.
type SeasonHistoryRow struct {
	port.SeasonProgressRow
	SeasonName string
	EndedAt    time.Time
}

// SeasonCursor is the keyset cursor of the season history (ended_at desc, season_id).
type SeasonCursor struct {
	EndedAt time.Time
	ID      uuid.UUID
}

// SchoolEventsFilter selects the school events of a tenant (school-limited staff see their schools only).
type SchoolEventsFilter struct {
	Tenant  uuid.UUID
	Schools []uuid.UUID // nil = every school and the tenant-wide events
	Status  string
	After   *RunCursor // created_at, event_id
	Limit   int
}

// LiveOpsReadModel reads the live-ops state in a READ ONLY session (F077).
type LiveOpsReadModel interface {
	port.LiveOpsReader
	// StudentRuns lists the runs a student may see, keyset on (starts_at, event_run_id), audience filtered in SQL.
	StudentRuns(ctx context.Context, f StudentRunsFilter) ([]port.EventRunRow, error)
	// Templates lists the current published templates assigned to tenant, keyset on code.
	Templates(ctx context.Context, tenant uuid.UUID, afterCode string, limit int) ([]port.TemplateRow, error)
	// SchoolEvents lists the school events of a tenant, newest first (keyset on created_at, event_id).
	SchoolEvents(ctx context.Context, f SchoolEventsFilter) ([]port.SchoolEventRow, error)
	// SchoolEvent returns a school event of tenant with its run, nil when unknown.
	SchoolEvent(ctx context.Context, tenant, event uuid.UUID) (*port.SchoolEventRow, error)
	// SeasonHistory lists the past seasons of a student (ended seasons with progress), keyset on (ended_at desc, season_id).
	SeasonHistory(ctx context.Context, tenant, student uuid.UUID, after *SeasonCursor, limit int) ([]SeasonHistoryRow, error)
}
