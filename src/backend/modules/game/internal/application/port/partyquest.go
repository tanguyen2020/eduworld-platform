package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// PartyQuestContent reads the published quests a party may play (F103): by code (activity_ref of the Party Lobby) and
// the party quests (party_mode ≠ solo) assigned to a tenant.
type PartyQuestContent interface {
	// QuestByCode returns the current version of the quest of a code (nil when unknown or unpublished).
	QuestByCode(ctx context.Context, code string) (*QuestRow, error)
	// PartyQuests lists the party quests assigned to tenant, of the area when given, by code; at most limit rows in the
	// database.
	PartyQuests(ctx context.Context, tenant uuid.UUID, areaID string, limit int) ([]QuestRow, error)
}

// ClueRender is the render of the clue card of a role (display blocks of a published learning material, no answer).
type ClueRender struct {
	Title  map[string]string `json:"title,omitempty"`
	Blocks []map[string]any  `json:"blocks"`
}

// ClueRenders returns the renders of the clue cards of roles of an interdependent step (getLearningMaterialRenders of
// module learning behind a cache ew:game:clue:<quest_version_id>:<step_id>:<role>, research R3).
// ErrBossLearningUnavailable when module learning cannot answer.
type ClueRenders interface {
	Renders(ctx context.Context, versionID uuid.UUID, stepID string, roles []quest.Role) (map[string]ClueRender, error)
}

// PartyQuestReads reads the co-op quest state for the queries (read session; explicit tenant + deleted_at, R27).
type PartyQuestReads interface {
	// ActiveProgress returns the active progress of a student on a quest (nil when none).
	ActiveProgress(ctx context.Context, tenant, student, questID uuid.UUID) (*quest.Progress, error)
	// ActiveSession returns the active session of a party on a quest a student is a present member of (nil when none).
	ActiveSession(ctx context.Context, tenant, party, questID, student uuid.UUID) (*partyquest.Session, error)
}

// QuestMessages pushes the co-op quest messages (quest.roles.assigned, quest.clue.shared) to one member (best effort).
type QuestMessages interface {
	QuestMessage(ctx context.Context, student uuid.UUID, typ string, payload map[string]any, at time.Time) error
}
