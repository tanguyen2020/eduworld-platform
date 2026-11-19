package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// TranslationJob is a row of content.translation_jobs (F025 US5).
type TranslationJob struct {
	ID           uuid.UUID
	Owner        contentmodel.Owner
	Kind         string // export | import
	Status       string // queued | running | succeeded | failed
	Format       string // xliff | csv
	SourceLang   string
	TargetLang   string
	Request      json.RawMessage
	InputAssetID *uuid.UUID
	ResultKey    *string
	Report       json.RawMessage
	RequestedBy  uuid.UUID
	CreatedAt    time.Time
	FinishedAt   *time.Time
}

// TranslationJobStore persists translation jobs under the scope of ctx (platform jobs under PlatformScope).
type TranslationJobStore interface {
	InsertTranslationJob(ctx context.Context, j TranslationJob) error
	TranslationJob(ctx context.Context, id uuid.UUID) (TranslationJob, error)
	// UpdateTranslationJob saves status, result key, report and finished_at.
	UpdateTranslationJob(ctx context.Context, j TranslationJob) error
	// QueuedTranslationJobs lists queued jobs of every owner, oldest first (ctx carries tenancy.Bypass(platform_job)).
	QueuedTranslationJobs(ctx context.Context, limit int) ([]TranslationJob, error)
}
