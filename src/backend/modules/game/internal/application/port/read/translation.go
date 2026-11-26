package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
)

// TranslationJobRow is one game.translation_jobs row as getGameTranslationJob reads it (F082).
type TranslationJobRow struct {
	ID             uuid.UUID
	Kind           string
	Status         string
	Format         string
	TargetLanguage string
	DryRun         bool
	ResultKey      *string
	Report         *i18ntext.Report
	ErrorMessage   *string
	RequestedBy    uuid.UUID
	CreatedAt      time.Time
	FinishedAt     *time.Time
}

// TranslationJobReader reads translation jobs (platform G table, deleted_at filtered).
type TranslationJobReader interface {
	TranslationJob(ctx context.Context, id uuid.UUID) (*TranslationJobRow, error)
}
