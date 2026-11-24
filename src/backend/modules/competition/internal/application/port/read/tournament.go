package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
)

// Keyset is the position after the last row of a page ordered by (starts_at, id) descending.
type Keyset struct {
	At time.Time
	ID uuid.UUID
}

// TemplateKeyset is the position after the last template of a page ordered by (code, config_id).
type TemplateKeyset struct {
	Code string
	ID   uuid.UUID
}

// ChallengeFilter narrows listChallenges: the schools of the organizer (nil = every school of the tenant), a grade, a
// status.
type ChallengeFilter struct {
	Schools []uuid.UUID
	Grade   *int
	Status  *string
}

// StudentScope is a student reading the tournaments: its school and grade (ADR-0095) and whether it wants only the
// tournaments it has an entry in.
type StudentScope struct {
	Student uuid.UUID
	School  uuid.UUID
	Grade   int
	Mine    bool
}

// TournamentFilter narrows listTournaments.
type TournamentFilter struct {
	Schools []uuid.UUID // organizer: its schools (nil = every school)
	School  *uuid.UUID  // organizer: one school asked
	Status  *string
	Student *StudentScope
}

// TournamentReader is the read model of the tournaments (F107 query side): keyset pages cut in SQL (LIMIT n+1), no
// write. Every method runs in the read session of ctx (tenant RLS).
type TournamentReader interface {
	ChallengePage(ctx context.Context, tenant uuid.UUID, f ChallengeFilter, after *Keyset, limit int) ([]port.ChallengeRow, error)
	Challenge(ctx context.Context, tenant, id uuid.UUID) (*port.ChallengeRow, error)
	// ChallengeParticipants counts the participants of challenges: entries of their tournament, or the students of their rooms.
	ChallengeParticipants(ctx context.Context, tenant uuid.UUID, cs []port.ChallengeRow) (map[uuid.UUID]int, error)
	// AudienceSize counts the effective audience of a challenge (grade students ∪ listed − excluded, ADR-0095).
	AudienceSize(ctx context.Context, c port.ChallengeRow) (int, error)
	// MyChallenges pages the challenges of a student's audience not completed nor cancelled (F107-AC-27).
	MyChallenges(ctx context.Context, tenant uuid.UUID, s StudentScope, after *Keyset, limit int) ([]port.ChallengeRow, error)
	// TemplatePage pages the published templates assigned to tenant (filters: a grade, a subject).
	TemplatePage(ctx context.Context, tenant uuid.UUID, grade *int, subject *uuid.UUID, after *TemplateKeyset, limit int) ([]port.TemplateRow, error)

	TournamentPage(ctx context.Context, tenant uuid.UUID, f TournamentFilter, after *Keyset, limit int) ([]port.TournamentRow, error)
	Tournament(ctx context.Context, tenant, id uuid.UUID) (*port.TournamentRow, error)
	EntryCounts(ctx context.Context, tenant uuid.UUID, tournaments []uuid.UUID) (map[uuid.UUID]int, error)
	Entries(ctx context.Context, tenant, tournament uuid.UUID) ([]port.EntryRow, error)
	Nodes(ctx context.Context, tenant, tournament uuid.UUID) ([]*trn.Node, error)
	Groups(ctx context.Context, tenant, tournament uuid.UUID) ([]port.GroupRow, error)
	Results(ctx context.Context, tenant, tournament uuid.UUID) (map[uuid.UUID]int, error)
	Student(ctx context.Context, tenant, id uuid.UUID) (*port.StudentInfo, error)
	Cards(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]port.Card, error)
}

// AssignmentReader pages the active assignments of a template (listResourceAssignments, platform read).
type AssignmentReader interface {
	AssignmentPage(ctx context.Context, resourceType string, resourceID uuid.UUID, after *port.AssignmentRow, limit int) ([]port.AssignmentRow, error)
}
