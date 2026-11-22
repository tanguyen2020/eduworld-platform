package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// SnapshotPeriod is one closed period of a board in a scope (competition.leaderboard_snapshots).
type SnapshotPeriod struct {
	Period   string
	Captured time.Time
	Rows     []port.SnapshotRow
}

// RankingReader is the read side of ranking, leaderboards and seasons (READ ONLY transaction of ctx).
type RankingReader interface {
	Rules(ctx context.Context, at time.Time) (ranking.Rules, error)
	ActiveSeason(ctx context.Context) (*port.SeasonRow, error)
	Season(ctx context.Context, id uuid.UUID) (*port.SeasonRow, error)
	// Seasons returns the live seasons, newest first.
	Seasons(ctx context.Context, limit, offset int) ([]port.SeasonRow, error)
	// MyRatings returns the ratings of a student in a season.
	MyRatings(ctx context.Context, tenant, student, season uuid.UUID) ([]port.RatingRow, error)
	// PeakTiers returns the peak tier codes of a student per season (every mode and subject).
	PeakTiers(ctx context.Context, tenant, student uuid.UUID, seasons []uuid.UUID) (map[uuid.UUID][]string, error)
	Definitions(ctx context.Context) ([]ldb.Definition, error)
	Student(ctx context.Context, tenant, student uuid.UUID) (*port.Placement, error)
	SchoolPolicy(ctx context.Context, tenant, school uuid.UUID) (port.SchoolPolicy, error)
	// SchoolKnown reports whether school belongs to tenant (ref.schools or a projected student of it).
	SchoolKnown(ctx context.Context, tenant, school uuid.UUID) (bool, error)
	// Ranking computes a board from the ledgers (the stale path while the Valkey board is rebuilt, F106-AC-14).
	Ranking(ctx context.Context, q port.BoardQuery) ([]port.BoardMember, error)
	GradeGoal(ctx context.Context, tenant, school uuid.UUID, grade int, start, end time.Time) (float64, int, error)
	// MyEffort returns the effort of a student credited to one grade of one school in [start, end).
	MyEffort(ctx context.Context, tenant, student, school uuid.UUID, grade int, start, end time.Time) (float64, error)
	// Friends returns the most recent friends of a student (at most max).
	Friends(ctx context.Context, tenant, student uuid.UUID, max int) ([]uuid.UUID, error)
	Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error)
	// Excluded returns the students out of every board: sanctioned (restrict_competition), inactive or cleaned up.
	Excluded(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, at time.Time) (map[uuid.UUID]bool, error)
	// Tiers returns the tier codes of students for a rating board.
	Tiers(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject, season uuid.UUID) (map[uuid.UUID]*string, error)
	// Snapshots returns the closed periods of a board in a scope, newest first.
	Snapshots(ctx context.Context, tenant, board uuid.UUID, scope string, limit, offset int) ([]SnapshotPeriod, error)
	// SnapshotRank returns the snapshot row of one member of a closed period (its rank beyond the top 100).
	SnapshotRank(ctx context.Context, tenant, board uuid.UUID, scope, period string, student uuid.UUID) (*port.SnapshotRow, error)
	// Snapshot returns one closed period (nil when not snapshotted).
	Snapshot(ctx context.Context, tenant, board uuid.UUID, scope, period string) (*SnapshotPeriod, error)
}
