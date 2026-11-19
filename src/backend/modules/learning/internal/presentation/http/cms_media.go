package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// MediaCommands are the write use cases of the media library (command.Media, F027).
type MediaCommands interface {
	PresignMediaUpload(ctx context.Context, in command.PresignInput) (command.PresignResult, error)
	CompleteMediaUpload(ctx context.Context, id uuid.UUID) (dto.MediaView, error)
}

// MediaQueries are the reads of the media library (query.MediaLibrary).
type MediaQueries interface {
	List(ctx context.Context, f read.MediaFilter, in query.PageInput) (query.Page[dto.MediaView], error)
	Get(ctx context.Context, id uuid.UUID) (dto.MediaView, error)
}

// PreviewCommands are the preview use cases (command.Preview, F027 US4).
type PreviewCommands interface {
	CreateSession(ctx context.Context, in command.PreviewInput) (command.PreviewView, error)
	SubmitResponse(ctx context.Context, in command.PreviewResponseInput) (command.PreviewResult, error)
}

const (
	cmsMedia   = "/cms/media"
	cmsPreview = "/cms/learning/preview/sessions"
)

// MediaOperations of F027 (merged into Operations).
var MediaOperations = map[string]httpx.Operation{
	"presignMediaUpload":           op("presignMediaUpload", http.MethodPost, cmsMedia+":presign-upload", httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"completeMediaUpload":          op("completeMediaUpload", http.MethodPost, cmsMedia+"/{asset_id}:complete-upload", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"listMediaAssets":              op("listMediaAssets", http.MethodGet, cmsMedia, httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getMediaAsset":                op("getMediaAsset", http.MethodGet, cmsMedia+"/{asset_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"createLearningPreviewSession": op("createLearningPreviewSession", http.MethodPost, cmsPreview, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
	"submitPreviewResponse":        op("submitPreviewResponse", http.MethodPost, cmsPreview+"/{preview_session_id}/responses", httpx.TenantOptional, httpx.IdempotencyNone, 200),
}

func init() {
	for id, o := range MediaOperations {
		Operations[id] = o
	}
}

var mediaRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"presignMediaUpload":           Handlers.presignMediaUpload,
	"completeMediaUpload":          Handlers.completeMediaUpload,
	"listMediaAssets":              Handlers.listMediaAssets,
	"getMediaAsset":                Handlers.getMediaAsset,
	"createLearningPreviewSession": Handlers.createLearningPreviewSession,
	"submitPreviewResponse":        Handlers.submitPreviewResponse,
}

type presignBody struct {
	Kind         string     `json:"kind"`
	Filename     string     `json:"filename"`
	MIME         string     `json:"mime_type"`
	Size         int64      `json:"size_bytes"`
	SHA256       string     `json:"sha256"`
	License      string     `json:"license"`
	AltText      string     `json:"alt_text"`
	ExamMaterial bool       `json:"is_exam_material"`
	OwnerScope   string     `json:"owner_scope"`
	TenantID     *uuid.UUID `json:"tenant_id"`
}

func (h Handlers) presignMediaUpload(w http.ResponseWriter, r *http.Request) {
	var b presignBody
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Media.PresignMediaUpload(r.Context(), command.PresignInput{Kind: b.Kind, Filename: b.Filename, MIME: b.MIME, Size: b.Size,
		SHA256: b.SHA256, License: b.License, AltText: b.AltText, ExamMaterial: b.ExamMaterial, OwnerScope: b.OwnerScope, TenantID: b.TenantID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) completeMediaUpload(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "asset_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Media.CompleteMediaUpload(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listMediaAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := h.MediaQueries.List(r.Context(), read.MediaFilter{Kind: q.Get("kind"), Status: q.Get("status"), Q: q.Get("q")}, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.MediaView) dto.MediaView { return v })
}

func (h Handlers) getMediaAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "asset_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.MediaQueries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) createLearningPreviewSession(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ContentType     string    `json:"content_type"`
		ContentID       uuid.UUID `json:"content_id"`
		Version         int       `json:"version"`
		DisplayLanguage string    `json:"display_language"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Preview.CreateSession(r.Context(), command.PreviewInput{ContentType: b.ContentType, ContentID: b.ContentID, Version: b.Version,
		DisplayLanguage: b.DisplayLanguage})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) submitPreviewResponse(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "preview_session_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Position int             `json:"position"`
		Response json.RawMessage `json:"response"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Preview.SubmitResponse(r.Context(), command.PreviewResponseInput{SessionID: id, Position: b.Position, Response: b.Response})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
