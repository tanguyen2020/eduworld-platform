package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// TranslationCommands queue translation jobs (command.Translations, F025 US5).
type TranslationCommands interface {
	Create(ctx context.Context, in command.TranslationJobInput) (dto.TranslationJobView, error)
}

// TranslationQueries read translation jobs (query.TranslationJobs).
type TranslationQueries interface {
	Get(ctx context.Context, id uuid.UUID) (dto.TranslationJobView, error)
}

// TranslationOperations of F025 US5 (merged into Operations).
var TranslationOperations = map[string]httpx.Operation{
	"exportLearningTranslations": op("exportLearningTranslations", http.MethodPost, "/cms/learning/translations:export", httpx.TenantOptional,
		httpx.IdempotencyRequired, 202),
	"importLearningTranslations": op("importLearningTranslations", http.MethodPost, "/cms/learning/translations:import", httpx.TenantOptional,
		httpx.IdempotencyRequired, 202),
	"getLearningTranslationJob": op("getLearningTranslationJob", http.MethodGet, "/cms/learning/translations/jobs/{job_id}", httpx.TenantOptional,
		httpx.IdempotencyNone, 200),
}

func init() {
	for id, o := range TranslationOperations {
		Operations[id] = o
	}
	workflowRoutes["exportLearningTranslations"] = Handlers.exportTranslations
	workflowRoutes["importLearningTranslations"] = Handlers.importTranslations
	workflowRoutes["getLearningTranslationJob"] = Handlers.getTranslationJob
}

type translationBody struct {
	ContentTypes   []string    `json:"content_types"`
	ContentIDs     []uuid.UUID `json:"content_ids"`
	SourceLanguage string      `json:"source_language"`
	TargetLanguage string      `json:"target_language"`
	Format         string      `json:"format"`
	OnlyMissing    *bool       `json:"only_missing"`
	AssetID        *uuid.UUID  `json:"asset_id"`
	DryRun         bool        `json:"dry_run"`
}

func (h Handlers) createTranslationJob(w http.ResponseWriter, r *http.Request, kind string) {
	var b translationBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	only := true
	if b.OnlyMissing != nil {
		only = *b.OnlyMissing
	}
	out, err := h.Translations.Create(r.Context(), command.TranslationJobInput{Kind: kind, Format: b.Format, SourceLang: b.SourceLanguage,
		TargetLang: b.TargetLanguage, ContentTypes: b.ContentTypes, ContentIDs: b.ContentIDs, OnlyMissing: only, AssetID: b.AssetID, DryRun: b.DryRun})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h Handlers) exportTranslations(w http.ResponseWriter, r *http.Request) {
	h.createTranslationJob(w, r, "export")
}

func (h Handlers) importTranslations(w http.ResponseWriter, r *http.Request) {
	h.createTranslationJob(w, r, "import")
}

func (h Handlers) getTranslationJob(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "job_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.TranslationJobs.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
