package port

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// QuestRow is a published quest version (current or pinned).
type QuestRow struct {
	QuestID   uuid.UUID
	VersionID uuid.UUID
	VersionNo int
	SHA256    string
	Version   quest.Version
}

// QuestHistory is what a student did on one quest (every version): the latest progress and the completions.
type QuestHistory struct {
	Latest           *quest.Progress
	Completed        int
	LastCompletedAt  *time.Time
	CompletedPeriods map[string]bool // period keys with a completed progress
}

// QuestContent reads the published quests, their tenant assignment and the quest progress of a student in the
// transaction of ctx (read or write): explicit tenant and soft-delete conditions (R27).
type QuestContent interface {
	// AssignedQuests returns the current versions of the quests assigned to tenant whose grade_levels hold grade (0 =
	// every grade: the content manifest of the tenant).
	AssignedQuests(ctx context.Context, tenant uuid.UUID, grade int) ([]QuestRow, error)
	// Assigned reports a quest assigned to tenant (active assignment).
	Assigned(ctx context.Context, tenant, questID uuid.UUID) (bool, error)
	// CurrentQuest returns the current version of a quest, nil when unknown or unpublished.
	CurrentQuest(ctx context.Context, questID uuid.UUID) (*QuestRow, error)
	// CurrentQuests returns the current versions of every published quest (platform).
	CurrentQuests(ctx context.Context) ([]QuestRow, error)
	// QuestVersion returns a version by id (pinned versions stay readable after a new publish).
	QuestVersion(ctx context.Context, versionID uuid.UUID) (*QuestRow, error)
	// Histories returns the history of every quest the student has a progress of in tenant.
	Histories(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]*QuestHistory, error)
	// Discovered returns the interactables the student found (exploration_progress).
	Discovered(ctx context.Context, tenant, student uuid.UUID) (map[string]bool, error)
}

// LearningAttempt is the request of createAttempt (facade learning): mode quest (a quest step, F072) or minigame (a
// free mini-game play, F073).
type LearningAttempt struct {
	StudentID        uuid.UUID
	ContentID        uuid.UUID
	ContentVersionID *uuid.UUID
	// ContentType is the ActivityRef content type: "" = activity (F072); minigame_config for a mini-game (F073, the
	// version is then always pinned by module game).
	ContentType     string
	QuestProgressID uuid.UUID
	QuestVersionID  uuid.UUID
	StepID          string
	GameSessionID   *uuid.UUID
	// MiniGamePlayID is the play of a free mini-game: mode minigame, context minigame_session (nil = mode quest).
	MiniGamePlayID *uuid.UUID
}

// Learning content types of createAttempt.
const (
	ContentActivity       = "activity"
	ContentMiniGameConfig = "minigame_config"
)

// LearningAttemptCreated is AttemptCreated of the facade.
type LearningAttemptCreated struct {
	AttemptID        uuid.UUID
	Status           string
	QuestionCount    int
	StartedAt        time.Time
	DeadlineAt       *time.Time
	ContentVersionID *uuid.UUID
}

// ErrLearningUnavailable is a facade that cannot answer (deadline, error): startQuestActivity answers 503 with
// Retry-After and the quest progress does not change (F072-AC-18).
var ErrLearningUnavailable = errors.New("game: module learning unavailable")

// LearningAttempts is createAttempt of the learning facade (x-allowed-callers: game; pkg/modcall forwards the
// TenantScope, the original Principal and the Idempotency-Key of the request).
type LearningAttempts interface {
	CreateAttempt(ctx context.Context, in LearningAttempt) (LearningAttemptCreated, error)
}

// QuestRealtime pushes quest.progress.updated to the connections of a student (best effort after commit).
type QuestRealtime interface {
	QuestProgressUpdated(ctx context.Context, student uuid.UUID, payload map[string]any, at time.Time) error
}

// PayloadValidator checks a payload against a JSON Schema (quest.schema.json v2, embedded copy in infrastructure).
type PayloadValidator interface {
	Validate(raw []byte) error
}
