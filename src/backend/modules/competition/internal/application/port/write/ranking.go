package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/points"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// RatingRow is port.RatingRow.
type RatingRow = port.RatingRow

// SeasonRow is port.SeasonRow.
type SeasonRow = port.SeasonRow

// PointEntry is one row of competition.competition_point_ledger.
type PointEntry struct {
	Tenant      uuid.UUID
	School      uuid.UUID
	Student     uuid.UUID
	Grade       int
	Season      uuid.UUID
	Subject     *uuid.UUID
	Topic       *uuid.UUID
	Mode        *string
	Award       points.Award
	SourceType  string
	SourceID    uuid.UUID
	RuleVersion uuid.UUID
	Day         time.Time
	At          time.Time
}

// Contribution is one row of competition.leaderboard_contributions.
type Contribution struct {
	Tenant      uuid.UUID
	School      uuid.UUID
	Student     uuid.UUID
	Grade       int
	Metric      string
	Amount      int
	SourceType  string
	SourceID    string
	Day         time.Time
	RuleVersion *uuid.UUID
	At          time.Time
}

// StudentRef is port.Placement.
type StudentRef = port.Placement

// SchoolPolicy is port.SchoolPolicy.
type SchoolPolicy = port.SchoolPolicy

// RankingRepository is the persistence of ratings, Competition Points and contributions (inside the transaction of
// ctx; reads of the platform configuration are G tables).
type RankingRepository interface {
	// Rules returns the published Ranking Rules in effect at at and its version id.
	Rules(ctx context.Context, at time.Time) (ranking.Rules, uuid.UUID, error)
	// SeasonAt returns the season containing at (active or ended), else the active season; nil when there is none.
	SeasonAt(ctx context.Context, at time.Time) (*SeasonRow, error)
	// LockRatings returns the ratings of students for (mode, subject, season), locked for update.
	LockRatings(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, mode string, subject, season uuid.UUID) (map[uuid.UUID]RatingRow, error)
	// PreviousRating returns the latest rating of an earlier season (the lazy soft reset of a first match in a season).
	PreviousRating(ctx context.Context, tenant, student uuid.UUID, mode string, subject uuid.UUID, before time.Time) (*RatingRow, error)
	// SaveRating inserts (Version 0) or updates a rating.
	SaveRating(ctx context.Context, r RatingRow) error
	// InsertRatingChange records the change of a rating by a match; false when (match_id, student_id) exists.
	InsertRatingChange(ctx context.Context, match uuid.UUID, before, after RatingRow) (bool, error)
	// BeforeTiers returns the tier of each student before match (rating_changes.before) when it was rated already.
	BeforeTiers(ctx context.Context, match uuid.UUID) (map[uuid.UUID]*string, error)
	// MatchTopic returns the topic of a match (competition.matches), nil when none.
	MatchTopic(ctx context.Context, match uuid.UUID) (*uuid.UUID, error)
	// PointDay returns the ledger of a student on day (sum and match entries).
	PointDay(ctx context.Context, tenant, student uuid.UUID, day time.Time) (points.Day, error)
	// InsertPoints appends a ledger entry; false when (source_type, source_id, student_id) exists.
	InsertPoints(ctx context.Context, e PointEntry) (bool, error)
	// SeasonPoints returns the Competition Points of a student in a season.
	SeasonPoints(ctx context.Context, tenant, student, season uuid.UUID) (int, error)
	// Student returns the placement of a student (nil when not projected).
	Student(ctx context.Context, tenant, student uuid.UUID) (*StudentRef, error)
	// EffortDay returns the effort of a student on day.
	EffortDay(ctx context.Context, tenant, student uuid.UUID, day time.Time) (ldb.EffortDay, error)
	// InsertContribution appends a contribution; false when (metric, source_type, source_id, student_id) exists.
	InsertContribution(ctx context.Context, c Contribution) (bool, error)
	// Definitions returns the live leaderboard definitions.
	Definitions(ctx context.Context) ([]ldb.Definition, error)
	// SchoolPolicy returns the leaderboard policy of a school.
	SchoolPolicy(ctx context.Context, tenant, school uuid.UUID) (SchoolPolicy, error)
	// Restricted returns the students with a restrict_competition sanction in force at at (excluded from the period).
	Restricted(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, at time.Time) (map[uuid.UUID]bool, error)
}
