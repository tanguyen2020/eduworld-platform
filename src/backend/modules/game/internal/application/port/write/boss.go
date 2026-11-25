package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// ItemBoss is the boss item type of the release core (F103).
const ItemBoss = "boss"

// BossStore writes the boss encounters in the tenant transaction of ctx (explicit tenant conditions; RLS as well).
// The encounter row is locked (SELECT … FOR UPDATE) before every change: commands of the members and timers of the
// room host are serialized per encounter.
type BossStore interface {
	// InsertEncounter adds an encounter with its members; false when party_activity_id exists already.
	InsertEncounter(ctx context.Context, e *boss.Encounter) (bool, error)
	// Lock returns an encounter of tenant with its members and contributions, locked; nil when unknown.
	Lock(ctx context.Context, tenant, id uuid.UUID) (*boss.Encounter, error)
	// ByPartyActivity returns the encounter of a party activity (idempotency of startPartyActivity), nil when none.
	ByPartyActivity(ctx context.Context, tenant, partyActivity uuid.UUID) (*boss.Encounter, error)
	// Save writes the encounter, its members and contributions (state_version is the new one).
	Save(ctx context.Context, e *boss.Encounter) error
	// LiveOfStudent lists the live encounters a student is a present member of.
	LiveOfStudent(ctx context.Context, tenant, student uuid.UUID) ([]uuid.UUID, error)
	// LiveOfParty lists the live encounters of a party.
	LiveOfParty(ctx context.Context, tenant, party uuid.UUID) ([]uuid.UUID, error)
	// Plans returns the plans of a round.
	Plans(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) ([]port.BossPlanRow, error)
	// PlanByID returns the plan of a learning plan id (nil when none).
	PlanByID(ctx context.Context, tenant, planID uuid.UUID) (*port.BossPlanRow, error)
	// InsertPlan adds a plan once per (encounter, phase, round, student, requirement).
	InsertPlan(ctx context.Context, p port.BossPlanRow) error
	// InsertAnswer appends a graded answer; false when (plan, index, student) is there already (counted once).
	InsertAnswer(ctx context.Context, a port.BossAnswerRow) (bool, error)
	// Answer returns the graded answer of (plan, index, student), nil when none.
	Answer(ctx context.Context, tenant, planID uuid.UUID, index int, student uuid.UUID) (*port.BossAnswerRow, error)
	// Round returns the counted correct answers of a round per member and requirement.
	Round(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) (map[uuid.UUID]map[string]int, error)
	// Usage is the usage of a boss by a student in tenant for a period.
	Usage(ctx context.Context, tenant, student, bossID uuid.UUID, period string) (port.BossUsage, error)
}

// PartyQuestStore writes the shared progress of the co-op quests in the tenant transaction of ctx. The session row is
// locked before every change (research R4: the members' QuestProgress rows change in the same transaction, locked in
// join order after the session — never the other way round).
type PartyQuestStore interface {
	// Insert adds a session; false when party_activity_id exists already.
	Insert(ctx context.Context, s *partyquest.Session) (bool, error)
	// Lock returns a session of tenant, locked; nil when unknown.
	Lock(ctx context.Context, tenant, id uuid.UUID) (*partyquest.Session, error)
	// ByPartyActivity returns the session of a party activity, nil when none.
	ByPartyActivity(ctx context.Context, tenant, partyActivity uuid.UUID) (*partyquest.Session, error)
	// ActiveFor returns the active session of a party on a quest a student is a present member of (locked), nil when none.
	ActiveFor(ctx context.Context, tenant, party, questID, student uuid.UUID) (*partyquest.Session, error)
	// ActiveOfParty lists the active sessions of a party.
	ActiveOfParty(ctx context.Context, tenant, party uuid.UUID) ([]uuid.UUID, error)
	// Save writes the session (state, members, shared steps, roles, clues).
	Save(ctx context.Context, s *partyquest.Session) error
	// AddContribution adds metric to the contribution of a member on a collective step once per attempt; returns the
	// totals of every member of the step after the change (false when the attempt was counted before).
	AddContribution(ctx context.Context, tenant, session, student uuid.UUID, stepID string, attempt uuid.UUID, metric int,
		minContribution int) (map[uuid.UUID]int, bool, error)
	// Contributions returns the totals of the members on a step.
	Contributions(ctx context.Context, tenant, session uuid.UUID, stepID string) (map[uuid.UUID]int, error)
	// ProgressRef reads a progress without locking it (the consumers lock the session first, then the members).
	ProgressRef(ctx context.Context, tenant, progressID uuid.UUID) (*quest.Progress, error)
	// ProgressRefActive reads the active progress of a student on a quest without locking it (nil when none).
	ProgressRefActive(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error)
	// AttemptRef reads the step attempt of a learning attempt without locking it (nil when not a quest step).
	AttemptRef(ctx context.Context, tenant, attemptID uuid.UUID) (*StepAttempt, error)
}
