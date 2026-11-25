package questsql

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

var _ port.PartyQuestContent = Content{}

// F103: the quests a party may play (Party Lobby activity_ref = quest code; party_mode ≠ solo).

// QuestByCode implements port.PartyQuestContent.
func (Content) QuestByCode(ctx context.Context, code string) (*port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform content (G tables, no tenant_id); deleted_at filtered on both tables
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		WHERE d.code = $1 AND d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'`, code)
	if err != nil {
		return nil, fmt.Errorf("quest by code: %w", err)
	}
	rs, err := scanQuests(rows)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	return &rs[0], nil
}

// PartyQuests implements port.PartyQuestContent.
func (Content) PartyQuests(ctx context.Context, tenant uuid.UUID, areaID string, limit int) ([]port.QuestRow, error) {
	t, err := q(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok platform quests (G tables) joined with the active assignments of the session tenant ($1, explicit; infra without RLS)
	rows, err := t.QueryContext(ctx, `SELECT `+versionCols+` FROM game.quest_definitions d
		JOIN game.quest_versions v ON v.quest_version_id = d.current_version_id
		JOIN infra.tenant_resource_assignments a ON a.resource_type = 'quest' AND a.resource_id = d.quest_id AND a.tenant_id = $1
			AND a.deleted_at IS NULL
		WHERE d.deleted_at IS NULL AND v.deleted_at IS NULL AND v.status = 'published'
			AND COALESCE(v.payload->>'party_mode', 'solo') <> 'solo' AND ($2::text = '' OR v.area_id = $2)
		ORDER BY d.code LIMIT $3`, tenant, areaID, limit)
	if err != nil {
		return nil, fmt.Errorf("party quests: %w", err)
	}
	return scanQuests(rows)
}
