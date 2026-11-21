package write

import (
	"context"
	"fmt"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
)

// MasteryRepo implements command.MasteryWriter on the projection ref.mastery_summary (F071 T011).
type MasteryRepo struct{}

var _ command.MasteryWriter = MasteryRepo{}

// UpsertMastery implements command.MasteryWriter: upsert by source_version (an older event never overwrites a newer row).
func (MasteryRepo) UpsertMastery(ctx context.Context, m command.MasteryUpdate) (bool, error) {
	// tenancyguard:ok projection ref.mastery_summary written in the tenant of the event (RLS insert/update policies); no soft-delete
	ok, err := exec(ctx, `INSERT INTO ref.mastery_summary AS s (tenant_id, student_id, objective_id, level, value, source_version)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, student_id, objective_id) DO UPDATE SET level = EXCLUDED.level, value = EXCLUDED.value,
			source_version = EXCLUDED.source_version
		WHERE s.source_version < EXCLUDED.source_version`, m.TenantID, m.StudentID, m.ObjectiveID, m.Level, m.Value, m.SourceVersion)
	if err != nil {
		return false, fmt.Errorf("mastery summary: %w", err)
	}
	return ok, nil
}
