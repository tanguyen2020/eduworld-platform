package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/boss"
)

// BossRow is a published boss version (current or pinned).
type BossRow struct {
	BossID    uuid.UUID
	VersionID uuid.UUID
	VersionNo int
	Version   boss.Version
}

// BossFilter is the database filter of listBosses (F103-AC-01): the bosses assigned to the tenant whose current
// version serves the grade, of the area given; keyset on boss_id after After, at most Limit rows (the caller asks
// limit + 1).
type BossFilter struct {
	Tenant uuid.UUID
	Grade  int
	AreaID string
	After  *uuid.UUID
	Limit  int
}

// BossContent reads the published bosses and their tenant assignment in the transaction of ctx (read or write):
// explicit soft-delete conditions (R27).
type BossContent interface {
	// AvailableBosses is the keyset page of BossFilter (database paging, never sliced in memory).
	AvailableBosses(ctx context.Context, f BossFilter) ([]BossRow, error)
	// PartyBosses lists the bosses a party may play (mode ≠ solo) assigned to tenant, of the area when given, by code;
	// at most limit rows in the database.
	PartyBosses(ctx context.Context, tenant uuid.UUID, areaID string, limit int) ([]BossRow, error)
	// BossAssigned reports a boss assigned to tenant (active assignment).
	BossAssigned(ctx context.Context, tenant, bossID uuid.UUID) (bool, error)
	// CurrentBoss returns the current version of a boss (nil when unknown or unpublished).
	CurrentBoss(ctx context.Context, bossID uuid.UUID) (*BossRow, error)
	// BossByCode returns the current version of the boss of a code (nil when unknown or unpublished).
	BossByCode(ctx context.Context, code string) (*BossRow, error)
	// BossVersion returns a version by id (pinned versions stay readable after a new publish).
	BossVersion(ctx context.Context, versionID uuid.UUID) (*BossRow, error)
	// KnownBosses returns which of ids are published bosses (references of quests and live-ops events).
	KnownBosses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// BossUsage is what a student did on a boss: encounters started in the period (every one except those abandoned by
// a learning failure, which never count — edge case of the spec) and whether it was ever defeated.
type BossUsage struct {
	Encounters int
	Defeated   bool
}

// BossPlanRow is the question plan of one member, requirement and round (game.boss_phase_plans).
type BossPlanRow struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	EncounterID   uuid.UUID
	PhaseID       string
	RetryNo       int
	StudentID     uuid.UUID
	RequirementID string
	PlanID        uuid.UUID
	Items         []BossPlanItem
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// BossPlanItem is one question of a plan: render payload only (no answer data, ADR-0016).
type BossPlanItem struct {
	Index             int            `json:"index"`
	QuestionVersionID uuid.UUID      `json:"question_version_id"`
	QuestionType      string         `json:"question_type"`
	TimeLimitSeconds  int            `json:"time_limit_seconds"`
	Render            map[string]any `json:"render"`
}

// BossAnswerRow is the graded result of one answer (game.boss_answer_results, append-only).
type BossAnswerRow struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	EncounterID   uuid.UUID
	StudentID     uuid.UUID
	PlanID        uuid.UUID
	Index         int
	PhaseID       string
	RetryNo       int
	RequirementID string
	IsCorrect     bool
	Counted       bool
	RapidGuess    bool
	ReceivedAt    time.Time
}

// BossReads reads encounters, plans and answers for the queries (read session): explicit tenant + deleted_at (R27).
type BossReads interface {
	// Encounter returns an encounter of tenant with its members and contributions (nil when unknown).
	Encounter(ctx context.Context, tenant, id uuid.UUID) (*boss.Encounter, error)
	// Plans returns the plans of a round (every member).
	Plans(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) ([]BossPlanRow, error)
	// Round returns the counted correct answers of a round per member and requirement.
	Round(ctx context.Context, tenant, encounter uuid.UUID, phaseID string, retry int) (map[uuid.UUID]map[string]int, error)
	// Answered returns the (plan, index) answered by a student in an encounter.
	Answered(ctx context.Context, tenant, encounter, student uuid.UUID) (map[uuid.UUID]map[int]bool, error)
	// Usage is the usage of a boss by a student in tenant for a period.
	Usage(ctx context.Context, tenant, student, bossID uuid.UUID, period string) (BossUsage, error)
	// Nicknames returns the nicknames of students (game profiles; R16: never a real name).
	Nicknames(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]string, error)
}

// BossQuestionPlan is the request of createQuestionPlan for one member, one requirement, one round (research R1).
type BossQuestionPlan struct {
	PlanKey         uuid.UUID // idempotency key (UUIDv5 of encounter, phase, round, student, requirement) sent as match_id
	EncounterID     uuid.UUID
	StudentID       uuid.UUID
	GradeLevel      int
	ActivityID      uuid.UUID
	ActivityVersion *int
	QuestionCount   int
	TimePerQuestion int
}

// BossPlan is the answer of createQuestionPlan.
type BossPlan struct {
	PlanID    uuid.UUID
	ExpiresAt time.Time
	Items     []BossPlanItem
}

// BossScore is the answer of scorePlanAnswer.
type BossScore struct {
	IsCorrect  bool
	Late       bool
	RapidGuess bool
}

// ErrBossLearningUnavailable is module learning not answering (deadline, 5xx, not wired).
var ErrBossLearningUnavailable = errors.New("boss: module learning unavailable")

// BossLearning is createQuestionPlan and scorePlanAnswer of the learning facade (x-allowed-callers: game; deadline,
// TenantScope, original Principal and Idempotency-Key through pkg/modcall). Grading and learning records stay in module
// learning (ADR-0016).
type BossLearning interface {
	CreateBossPlan(ctx context.Context, in BossQuestionPlan) (BossPlan, error)
	ScoreBossAnswer(ctx context.Context, planID uuid.UUID, index int, student uuid.UUID, response []byte, receivedAt time.Time) (BossScore, error)
}

// BossRealtime pushes the boss messages to the connections of one member (Valkey ew:rt:user:<id>, best effort).
type BossRealtime interface {
	BossMessage(ctx context.Context, student uuid.UUID, typ string, payload map[string]any, at time.Time) error
}

// OnlineMembers reports which students of tenant are online: an active game session whose last heartbeat is within the
// heartbeat timeout (F103-AC-11 grace starts when a member goes offline).
type OnlineMembers interface {
	Online(ctx context.Context, tenant uuid.UUID, students []uuid.UUID, since time.Time) (map[uuid.UUID]bool, error)
}

// GameRoom is a room of the shared room host (pkg/roomhost) for a live boss encounter or party quest session: announced
// when it starts (an owner node takes it and fires its timer at the due time), closed when it ends.
type GameRoom interface {
	Open(ctx context.Context, tenant, id uuid.UUID, due time.Time) error
	Close(ctx context.Context, id uuid.UUID) error
}

// NicknameReader returns the nicknames of students (game profiles; R16: never a real name).
type NicknameReader interface {
	Nicknames(ctx context.Context, tenant uuid.UUID, students []uuid.UUID) (map[uuid.UUID]string, error)
}
