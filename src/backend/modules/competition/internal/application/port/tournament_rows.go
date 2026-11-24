package port

import (
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
)

// Rows of the tournaments shared by the write and read contexts (F107): the write repositories and the read models both
// return them (package write aliases them).

// TemplateRow is the version of a tournament/challenge template in effect (competition.game_configs, current published
// version) with whether it is assigned to the tenant asking (infra.tenant_resource_assignments, ADR-0029).
type TemplateRow struct {
	ConfigID   uuid.UUID
	VersionID  uuid.UUID
	VersionNo  int
	ConfigType string
	Code       string
	Template   challenge.Template
	Assigned   bool
}

// ChallengeRow is one row of competition.challenges.
type ChallengeRow struct {
	ID                   uuid.UUID
	Tenant               uuid.UUID
	School               uuid.UUID
	TemplateConfigID     uuid.UUID
	TemplateVersionID    uuid.UUID
	Kind                 string
	Format               string
	Name                 string
	Grade                *int
	Listed               []uuid.UUID
	Exclude              []uuid.UUID
	SubjectID            uuid.UUID
	QuestionSetVersionID *uuid.UUID
	Params               map[string]any
	StartsAt             time.Time
	EndsAt               time.Time
	JoinOpensAt          time.Time
	Status               string
	TournamentID         *uuid.UUID
	MatchIDs             []uuid.UUID
	CancelReason         *string
	CreatedBy            uuid.UUID
	CreatedAt            time.Time
	Version              int64
}

// TournamentRow is one row of competition.tournaments.
type TournamentRow struct {
	ID               uuid.UUID
	Tenant           uuid.UUID
	School           *uuid.UUID
	OwnerScope       string
	OwnerID          uuid.UUID
	ChallengeID      *uuid.UUID
	TemplateConfigID uuid.UUID
	RuleVersionID    uuid.UUID
	Name             string
	Format           string
	ParticipantType  string
	TeamSize         *int
	Mode             string
	SubjectID        uuid.UUID
	TopicIDs         []uuid.UUID
	Grades           []int
	RegOpenAt        time.Time
	RegCloseAt       time.Time
	CheckInOpensAt   time.Time
	StartsAt         time.Time
	MaxEntries       int
	MinEntries       int
	SeedingPolicy    string
	SeedValue        *int64
	AutoConfirm      bool
	RewardRules      []challenge.RewardRule
	RewardSummary    *string
	RoundGap         time.Duration
	Status           string
	CancelReason     *string
	CompletedAt      *time.Time
	RewardedAt       *time.Time
	CreatedAt        time.Time
	Version          int64
}

// EntryRow is one row of competition.tournament_entries.
type EntryRow struct {
	ID           uuid.UUID
	Tournament   uuid.UUID
	Tenant       uuid.UUID
	School       uuid.UUID
	Student      *uuid.UUID
	Party        *uuid.UUID
	Members      []uuid.UUID
	Seed         *int
	Group        *uuid.UUID
	Status       string
	RegisteredBy uuid.UUID
	CheckedInAt  *time.Time
	Version      int64
}

// GroupRow is one row of competition.tournament_groups.
type GroupRow struct {
	ID         uuid.UUID
	Tournament uuid.UUID
	Tenant     uuid.UUID
	Code       string
	Standings  []tournament.Row
}

// StudentInfo is what eligibility needs of a projected student: school, grade, active, restricted from competition
// (competition_restrictions, multiplayer sanctions) and whether the hours of its school exclude now.
type StudentInfo struct {
	ID         uuid.UUID
	School     uuid.UUID
	Grade      int
	Active     bool
	Restricted bool
	OffHours   bool
}

// AssignmentRow is one active row of infra.tenant_resource_assignments.
type AssignmentRow struct {
	ID           uuid.UUID
	ResourceType string
	ResourceID   uuid.UUID
	Tenant       uuid.UUID
	AssignedBy   uuid.UUID
	AssignedAt   time.Time
	RevokedAt    *time.Time
}

// Challenge statuses.
const (
	ChallengeScheduled = "scheduled"
	ChallengeRunning   = "running"
	ChallengeCompleted = "completed"
	ChallengeCancelled = "cancelled"
)
