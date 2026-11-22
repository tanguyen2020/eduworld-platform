package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// BoardKey identifies one period of one leaderboard in one scope (the Valkey key
// ew:lb:<tenant_id>:<leaderboard_id>:<scope_key>:<period_key>, VALKEY_DESIGN §3.1).
type BoardKey struct {
	Tenant uuid.UUID
	Board  uuid.UUID
	Scope  string
	Period string
}

// BoardMember is one member of a board as computed from the ledgers: its value and the tie-break tick (who reached the
// value first ranks above).
type BoardMember struct {
	Student uuid.UUID
	Value   float64
	Tick    float64
}

// LeaderboardStore is the Valkey cache of the open periods (ADR-0004: Rebuildable — PostgreSQL ledgers are the source
// of truth). A board is "built" once its meta key exists; an increment on a board not built is dropped (the rebuild
// reads it from the ledger) and the board is queued for rebuild.
type LeaderboardStore interface {
	// Incr adds delta to the value of student (value never below 0) with a new tick; false when the board is not built.
	Incr(ctx context.Context, k BoardKey, student uuid.UUID, delta, tick float64, ttl time.Duration) (bool, error)
	// Set sets the value of student (rating boards); false when the board is not built.
	Set(ctx context.Context, k BoardKey, student uuid.UUID, value, tick float64, ttl time.Duration) (bool, error)
	// Remove takes student out of the board (rating become provisional again, sanction).
	Remove(ctx context.Context, k BoardKey, student uuid.UUID) error
	// Built reports whether the board is built.
	Built(ctx context.Context, k BoardKey) (bool, error)
	// Count returns the number of members.
	Count(ctx context.Context, k BoardKey) (int, error)
	// Range returns the members from rank from to rank to (1-based, inclusive), best first.
	Range(ctx context.Context, k BoardKey, from, to int) ([]ldb.Ranked, error)
	// RankOf returns the rank and value of student (nil when not a member).
	RankOf(ctx context.Context, k BoardKey, student uuid.UUID) (*ldb.Ranked, error)
	// Scores returns the values of students that are members (friends boards: ZMSCORE).
	Scores(ctx context.Context, k BoardKey, students []uuid.UUID) (map[uuid.UUID]BoardMember, error)
	// Members returns every member with its value (reconciliation).
	Members(ctx context.Context, k BoardKey) ([]BoardMember, error)
	// Replace atomically replaces the board by members and marks it built.
	Replace(ctx context.Context, k BoardKey, members []BoardMember, ttl time.Duration) error
	// QueueRebuild asks the job leaderboard-rebuild to rebuild k.
	QueueRebuild(ctx context.Context, k BoardKey) error
	// PopRebuilds takes up to n queued boards.
	PopRebuilds(ctx context.Context, n int) ([]BoardKey, error)
	// BuiltBoards lists the built boards (reconciliation), at most n.
	BuiltBoards(ctx context.Context, n int) ([]BoardKey, error)
}

// TierPush sends competition.tier.updated to the student's game client (realtime gateway): tier only, never a rating
// number (F106-AC-07); there is no "you were overtaken" message (COMPETITION_SYSTEM §9.4).
type TierPush interface {
	Send(ctx context.Context, to []uuid.UUID, typ string, payload any) error
}

// RankingMetrics are the eduworld_rating_* / eduworld_ldb_* metrics (plan §Quan sát): no school or student label.
type RankingMetrics interface {
	RatingUpdated(mode string)
	PointsCapped()
	Rebuild(reason string)
	Drift()
	PeriodClose(d time.Duration)
}

// RatingSource reads the ratings of the active season for matchmaking (implements RatingReader with the published
// Ranking Rules defaults).
type RatingSource interface {
	ActiveRatings(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject uuid.UUID) (map[uuid.UUID]ranking.Rating, error)
}
