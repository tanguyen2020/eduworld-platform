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

// PeriodKeyset is the position after the last closed period of a page of getLeaderboardHistory (newest first).
type PeriodKeyset struct {
	Captured time.Time
	Period   string
}

// SnapshotView is the result of SnapshotWindow: the capture instant, the member count, the ranked rows asked (by
// rank) and the viewer's own row.
type SnapshotView struct {
	Captured time.Time
	Total    int
	Rows     []ldb.Ranked
	Me       *ldb.Ranked
}

// RankWindowQuery is the part of a board to rank in SQL: ranks From..To (1-based) and, when Viewer is set, the
// viewer and Around ranks on each side of it.
type RankWindowQuery struct {
	From, To int
	Viewer   *uuid.UUID
	Around   int
}

// RankWindow is the result of RankingWindow: the member count and the ranked rows asked, ordered by rank.
type RankWindow struct {
	Total int
	Rows  []ldb.Ranked
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
	// RankingWindow computes the shown part of a board from the ledgers in SQL (the stale path while the Valkey board is
	// rebuilt, F106-AC-14): the member count, the members ranked [From, To] and, with a viewer, its row and the members
	// ranked [its rank − Around, its rank + Around] — never the whole board (API_DESIGN §5).
	RankingWindow(ctx context.Context, q port.BoardQuery, w RankWindowQuery) (RankWindow, error)
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
	// SnapshotPeriods returns one page of the closed periods of a board in a scope, newest first: keyset (captured_at,
	// period_key) after After and LIMIT limit in SQL (API_DESIGN §5); Rows hold the collective row of the period only.
	SnapshotPeriods(ctx context.Context, tenant, board uuid.UUID, scope string, after *PeriodKeyset, limit int) ([]SnapshotPeriod, error)
	// SnapshotWindow reads the shown part of one closed period in SQL (nil when not snapshotted): the member count, the
	// members ranked [From, To] and, with a viewer, its own row (any rank) and the members ranked [its rank − Around,
	// its rank + Around] — rows only from the shown top of the snapshot (rank ≤ 100), never the whole period.
	SnapshotWindow(ctx context.Context, tenant, board uuid.UUID, scope, period string, w RankWindowQuery) (*SnapshotView, error)
}
