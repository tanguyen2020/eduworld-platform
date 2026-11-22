package port

import (
	"time"

	"github.com/google/uuid"

	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/ranking"
)

// The F106 rows shared by the write ports (port/write aliases them) and the read side (port/read, query): the read
// context never imports port/write (F003-AC-10).

// RatingRow is competition.ratings: the Game Rating of a student per (mode, subject, season). Never returned to a
// student as a number (F106-AC-07).
type RatingRow struct {
	ID          uuid.UUID
	Tenant      uuid.UUID
	School      uuid.UUID
	Student     uuid.UUID
	Mode        string
	Subject     uuid.UUID
	Season      uuid.UUID
	Grade       int
	Rating      ranking.Rating
	Games       int
	Tier        *string
	Peak        *string
	LastPlayed  *time.Time
	RuleVersion uuid.UUID
	// Version is the optimistic version (0 for a new row).
	Version int64
}

// SeasonRow is competition.seasons.
type SeasonRow struct {
	ID        uuid.UUID
	Code      string
	Name      string
	Theme     *string
	StartsAt  time.Time
	EndsAt    time.Time
	Status    string
	StartedAt *time.Time
	EndedAt   *time.Time
}

// Bounds returns the season as the domain needs it for its period key.
func (s SeasonRow) Bounds() *ldb.Season {
	return &ldb.Season{ID: s.ID, StartsAt: s.StartsAt, Ends: s.EndsAt}
}

// Placement is the projected placement of a student (ref.students: school and grade, ADR-0095).
type Placement struct {
	School uuid.UUID
	Grade  int
	Active bool
}

// SchoolPolicy is the leaderboard policy of a school (ref.schools.settings_subset, F015/F052):
// disabled_leaderboard_definitions (codes), school_leaderboard_enabled (false turns off every school/grade board) and
// allow_cross_school_listing (default false).
type SchoolPolicy struct {
	Disabled         []string
	SchoolBoardsOff  bool
	CrossSchoolAllow bool
}

// Allows reports whether the school shows definition d to its students (F106-AC-18, AC-19).
func (p SchoolPolicy) Allows(d ldb.Definition) bool {
	if d.SchoolScoped() {
		if p.SchoolBoardsOff {
			return false
		}
		for _, c := range p.Disabled {
			if c == d.Code {
				return false
			}
		}
	}
	return !d.CrossSchool() || p.CrossSchoolAllow
}

// BoardQuery is one board period in one scope computed from the PostgreSQL ledgers (the source of truth, ADR-0004).
type BoardQuery struct {
	Def    ldb.Definition
	Tenant uuid.UUID
	School *uuid.UUID
	Grade  *int
	Start  time.Time
	End    time.Time
	// Season of a rating board.
	Season *uuid.UUID
	// Students limits the board (friends).
	Students []uuid.UUID
	// Provisional excludes provisional ratings from a rating board (F106-AC-05).
	Provisional ranking.Provisional
	// At is the time the sanctions in force are taken (sanctioned students are out of the period, F106-AC-18).
	At time.Time
}

// SnapshotRow is one row of competition.leaderboard_snapshots: a ranked member, or the collective row of a grade_goal
// board (Student and Rank nil).
type SnapshotRow struct {
	Tenant   uuid.UUID
	Board    uuid.UUID
	Scope    string
	School   *uuid.UUID
	Grade    *int
	Period   string
	Student  *uuid.UUID
	Rank     *int
	Value    float64
	Target   *float64
	Reached  *bool
	Members  *int
	Captured time.Time
}
