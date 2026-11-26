package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TranslationCommands are exportGameTranslations / importGameTranslations (command.Translations, F082 US4).
type TranslationCommands interface {
	Export(ctx context.Context, in command.ExportInput) (command.JobView, error)
	Import(ctx context.Context, in command.ImportInput) (command.JobView, error)
}

// TranslationQueries is getGameTranslationJob (query.TranslationJobs).
type TranslationQueries interface {
	Get(ctx context.Context, id uuid.UUID) (command.JobView, error)
}

// TranslationHandlers serve /game-admin/translations* (internal gateway, portal staff with a platform role — route
// guards; game.content.translate in each use case, AGENTS R7, R20).
type TranslationHandlers struct {
	Commands TranslationCommands
	Queries  TranslationQueries
}

func init() {
	for _, o := range []httpx.Operation{
		adminOp("exportGameTranslations", http.MethodPost, "/game-admin/translations:export", httpx.IdempotencyRequired, http.StatusAccepted),
		adminOp("importGameTranslations", http.MethodPost, "/game-admin/translations:import", httpx.IdempotencyRequired, http.StatusAccepted),
		adminOp("getGameTranslationJob", http.MethodGet, "/game-admin/translations/jobs/{job_id}", httpx.IdempotencyNone, http.StatusOK),
	} {
		Operations[o.ID] = o
	}
}

func registerTranslations(s *httpx.Server, h TranslationHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("game: the translation handlers are required")
	}
	for id, fn := range map[string]http.HandlerFunc{"exportGameTranslations": h.export, "importGameTranslations": h.importFile,
		"getGameTranslationJob": h.job} {
		if err := s.Handle(Operations[id], fn); err != nil {
			return err
		}
	}
	return nil
}

func (h TranslationHandlers) export(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ContentTypes   []string    `json:"content_types"`
		ContentIDs     []uuid.UUID `json:"content_ids"`
		SourceLanguage string      `json:"source_language"`
		TargetLanguage string      `json:"target_language"`
		Format         string      `json:"format"`
		OnlyMissing    *bool       `json:"only_missing"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	only := true
	if b.OnlyMissing != nil {
		only = *b.OnlyMissing
	}
	out, err := h.Commands.Export(r.Context(), command.ExportInput{ContentTypes: b.ContentTypes, ContentIDs: b.ContentIDs,
		SourceLanguage: b.SourceLanguage, TargetLanguage: b.TargetLanguage, Format: b.Format, OnlyMissing: only})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h TranslationHandlers) importFile(w http.ResponseWriter, r *http.Request) {
	var b struct {
		AssetID        uuid.UUID `json:"asset_id"`
		Format         string    `json:"format"`
		TargetLanguage string    `json:"target_language"`
		DryRun         bool      `json:"dry_run"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if b.AssetID == uuid.Nil {
		fail(w, r, problem.Invalid("asset_id", problem.SubRequired, "the uploaded file"))
		return
	}
	out, err := h.Commands.Import(r.Context(), command.ImportInput{AssetID: b.AssetID, Format: b.Format, TargetLanguage: b.TargetLanguage,
		DryRun: b.DryRun})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h TranslationHandlers) job(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "job_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Queries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
