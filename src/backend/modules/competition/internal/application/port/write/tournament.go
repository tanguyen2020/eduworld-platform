package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
)

// TemplateRow is port.TemplateRow.
type TemplateRow = port.TemplateRow

// ChallengeRow is port.ChallengeRow.
type ChallengeRow = port.ChallengeRow

// Challenge statuses (port).
const (
	ChallengeScheduled = port.ChallengeScheduled
	ChallengeRunning   = port.ChallengeRunning
	ChallengeCompleted = port.ChallengeCompleted
	ChallengeCancelled = port.ChallengeCancelled
)

// TournamentRow is port.TournamentRow.
type TournamentRow = port.TournamentRow

// EntryRow is port.EntryRow.
type EntryRow = port.EntryRow

// GroupRow is port.GroupRow.
type GroupRow = port.GroupRow

// StudentInfo is port.StudentInfo.
type StudentInfo = port.StudentInfo

// ResultRow is one row of competition.tournament_results.
type ResultRow struct {
	Tournament uuid.UUID
	Tenant     uuid.UUID
	Entry      uuid.UUID
	School     uuid.UUID
	Placement  tournament.Placement
}

// RewardRow is one row of competition.tournament_rewards.
type RewardRow struct {
	Tournament uuid.UUID
	Tenant     uuid.UUID
	Entry      uuid.UUID
	RuleID     string
	Students   []uuid.UUID
	RewardRef  map[string]string
	Points     int
	At         time.Time
}

// DueRef is a tournament, challenge or node a job must look at, with its tenant.
type DueRef struct {
	Tenant uuid.UUID
	ID     uuid.UUID
}

// TournamentRepository is the persistence of challenges, tournaments, entries, groups, nodes, results and rewards
// (eduworld_competition, F107). Every method runs in the transaction of ctx (tenant RLS; the Due* listings run under
// SystemPlatformScope to find the work of every tenant, then each item is handled in its tenant).
type TournamentRepository interface {
	// Template returns the template config id in effect for tenant (nil when unknown, not a template or not published).
	Template(ctx context.Context, tenant, configID uuid.UUID) (*TemplateRow, error)
	// TemplateVersion returns a pinned template version (the tournament keeps running on it, F107 edge case).
	TemplateVersion(ctx context.Context, versionID uuid.UUID) (*challenge.Template, error)
	// SchoolActive reports whether school is an active school of tenant (ref.schools).
	SchoolActive(ctx context.Context, tenant, school uuid.UUID) (bool, error)
	// SchoolOpen reports whether the access hours of school (OQ-17) contain every instant of at.
	SchoolOpen(ctx context.Context, tenant, school uuid.UUID, at []time.Time) (bool, error)
	// GradeStudents lists the active students of grade in school (ref.students, ADR-0095).
	GradeStudents(ctx context.Context, tenant, school uuid.UUID, grade int) ([]uuid.UUID, error)
	// Students returns the projection of the students (unknown ids absent).
	Students(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID, now time.Time) (map[uuid.UUID]StudentInfo, error)
	// PartyMembers returns the leader and the members of an active party of tenant (nil when unknown).
	PartyMembers(ctx context.Context, tenant, party uuid.UUID) (*uuid.UUID, []uuid.UUID, error)

	InsertChallenge(ctx context.Context, c ChallengeRow) error
	Challenge(ctx context.Context, tenant, id uuid.UUID, lock bool) (*ChallengeRow, error)
	UpdateChallenge(ctx context.Context, c ChallengeRow) error

	InsertTournament(ctx context.Context, t TournamentRow) error
	Tournament(ctx context.Context, tenant, id uuid.UUID, lock bool) (*TournamentRow, error)
	UpdateTournament(ctx context.Context, t TournamentRow) error

	Entries(ctx context.Context, tenant, tournamentID uuid.UUID) ([]EntryRow, error)
	Entry(ctx context.Context, tenant, tournamentID, entryID uuid.UUID, lock bool) (*EntryRow, error)
	// EntryOfMember returns the entry (any status) holding student, nil when none.
	EntryOfMember(ctx context.Context, tenant, tournamentID, student uuid.UUID) (*EntryRow, error)
	InsertEntry(ctx context.Context, e EntryRow) error
	UpdateEntry(ctx context.Context, e EntryRow) error

	Groups(ctx context.Context, tenant, tournamentID uuid.UUID) ([]GroupRow, error)
	InsertGroup(ctx context.Context, g GroupRow) error
	UpdateGroupStandings(ctx context.Context, g GroupRow) error

	Nodes(ctx context.Context, tenant, tournamentID uuid.UUID, lock bool) ([]*tournament.Node, error)
	InsertNode(ctx context.Context, tenant, tournamentID uuid.UUID, n *tournament.Node) error
	UpdateNode(ctx context.Context, tenant uuid.UUID, n *tournament.Node) error
	// NodeOfMatch returns the tournament and node of a match (nil, nil when the match is no tournament match).
	NodeOfMatch(ctx context.Context, tenant, matchID uuid.UUID) (*uuid.UUID, *uuid.UUID, error)
	// MatchCreated reports whether the match of a proposal exists (every side accepted the ready check).
	MatchCreated(ctx context.Context, matchID uuid.UUID) (bool, error)
	// RoomOf returns the room of matchIDs holding student (nil when none).
	RoomOf(ctx context.Context, tenant uuid.UUID, matchIDs []uuid.UUID, student uuid.UUID) (*uuid.UUID, error)

	InsertResult(ctx context.Context, r ResultRow) (bool, error)
	InsertReward(ctx context.Context, r RewardRow) (bool, error)

	DueTournaments(ctx context.Context, now time.Time, limit int) ([]DueRef, error)
	DueChallenges(ctx context.Context, now time.Time, limit int) ([]DueRef, error)
}

// AssignmentRow is port.AssignmentRow.
type AssignmentRow = port.AssignmentRow

// AssignmentRepository is the persistence of the template assignments (F107-AC-29), under CrossTenantScope or
// PlatformScope (competition_platform).
type AssignmentRepository interface {
	// TemplateConfig returns the config type of a live platform template config (empty when it is none).
	TemplateConfig(ctx context.Context, configID uuid.UUID) (string, error)
	ActiveTenants(ctx context.Context, tenants []uuid.UUID) (map[uuid.UUID]bool, error)
	Active(ctx context.Context, resourceType string, resourceID uuid.UUID, tenants []uuid.UUID) ([]AssignmentRow, error)
	Insert(ctx context.Context, a AssignmentRow) error
	Revoke(ctx context.Context, id uuid.UUID, by uuid.UUID, at time.Time) error
}
