package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// ExperimentCommands are the write use cases of the A/B experiments (command.Experiments, F035 US5).
type ExperimentCommands interface {
	CreateLearningExperiment(ctx context.Context, in command.ExperimentInput) (dto.LearningExperimentView, error)
	UpdateLearningExperiment(ctx context.Context, id uuid.UUID, ifMatch int64, in command.ExperimentPatch) (dto.LearningExperimentView, error)
	StartLearningExperiment(ctx context.Context, id uuid.UUID, exposure string) (dto.LearningExperimentView, error)
	StopLearningExperiment(ctx context.Context, id uuid.UUID, reason string) (dto.LearningExperimentView, error)
}

// ExperimentQueries are the reads of the A/B experiments (query.Experiments).
type ExperimentQueries interface {
	ListLearningExperiments(ctx context.Context, status, kind string, in query.PageInput) (query.Page[dto.LearningExperimentView], error)
	GetLearningExperiment(ctx context.Context, id uuid.UUID) (dto.LearningExperimentView, error)
	GetLearningExperimentResults(ctx context.Context, id uuid.UUID) (dto.ExperimentResultsView, error)
}

// ExperimentHandlers are the use cases of F035 US5 served over HTTP (Handlers.Experiments).
type ExperimentHandlers struct {
	Commands ExperimentCommands
	Queries  ExperimentQueries
}

const (
	experimentsPath = "/admin/learning/experiments"
	experimentPath  = experimentsPath + "/{experiment_id}"
)

func init() {
	for id, o := range map[string]httpx.Operation{
		"listLearningExperiments":      op("listLearningExperiments", http.MethodGet, experimentsPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"createLearningExperiment":     op("createLearningExperiment", http.MethodPost, experimentsPath, httpx.TenantNone, httpx.IdempotencyRequired, 201),
		"getLearningExperiment":        op("getLearningExperiment", http.MethodGet, experimentPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"updateLearningExperiment":     op("updateLearningExperiment", http.MethodPatch, experimentPath, httpx.TenantNone, httpx.IdempotencyNone, 200),
		"startLearningExperiment":      op("startLearningExperiment", http.MethodPost, experimentPath+":start", httpx.TenantNone, httpx.IdempotencyRequired, 200),
		"stopLearningExperiment":       op("stopLearningExperiment", http.MethodPost, experimentPath+":stop", httpx.TenantNone, httpx.IdempotencyRequired, 200),
		"getLearningExperimentResults": op("getLearningExperimentResults", http.MethodGet, experimentPath+"/results", httpx.TenantNone, httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var experimentRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listLearningExperiments":      Handlers.listLearningExperiments,
	"createLearningExperiment":     Handlers.createLearningExperiment,
	"getLearningExperiment":        Handlers.getLearningExperiment,
	"updateLearningExperiment":     Handlers.updateLearningExperiment,
	"startLearningExperiment":      Handlers.startLearningExperiment,
	"stopLearningExperiment":       Handlers.stopLearningExperiment,
	"getLearningExperimentResults": Handlers.getLearningExperimentResults,
}

func revisionTag(w http.ResponseWriter, v dto.LearningExperimentView) { etag(w, int64(v.Revision)) }

func (h Handlers) listLearningExperiments(w http.ResponseWriter, r *http.Request) {
	p, err := h.Experiments.Queries.ListLearningExperiments(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("kind"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	listItems(w, p)
}

func (h Handlers) createLearningExperiment(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Code      string           `json:"code"`
		Kind      string           `json:"kind"`
		Unit      string           `json:"unit"`
		Arms      []experiment.Arm `json:"arms"`
		TenantIDs []uuid.UUID      `json:"tenant_ids"`
		StudyID   *uuid.UUID       `json:"study_id"`
		StartAt   time.Time        `json:"start_at"`
		EndAt     time.Time        `json:"end_at"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Commands.CreateLearningExperiment(r.Context(), command.ExperimentInput{Code: b.Code, Kind: b.Kind, Unit: b.Unit,
		Arms: b.Arms, TenantIDs: b.TenantIDs, StudyID: b.StudyID, StartAt: b.StartAt, EndAt: b.EndAt, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		fail(w, r, err)
		return
	}
	revisionTag(w, v)
	writeJSON(w, http.StatusCreated, v)
}

func (h Handlers) getLearningExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "experiment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Queries.GetLearningExperiment(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	revisionTag(w, v)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) updateLearningExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "experiment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rev, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	pt, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.ExperimentPatch
	if in.Unit, err = optionalOf[string](pt, "unit"); err == nil {
		if _, err = pt.into("arms", &in.Arms); err == nil {
			if _, err = pt.into("tenant_ids", &in.TenantIDs); err == nil {
				if in.StudyID, err = nullable[uuid.UUID](pt, "study_id"); err == nil {
					if in.StartAt, err = optionalOf[time.Time](pt, "start_at"); err == nil {
						in.EndAt, err = optionalOf[time.Time](pt, "end_at")
					}
				}
			}
		}
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Commands.UpdateLearningExperiment(r.Context(), id, rev, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	revisionTag(w, v)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) startLearningExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "experiment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Exposure string `json:"exposure"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Commands.StartLearningExperiment(r.Context(), id, b.Exposure)
	if err != nil {
		fail(w, r, err)
		return
	}
	revisionTag(w, v)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) stopLearningExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "experiment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Commands.StopLearningExperiment(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	revisionTag(w, v)
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getLearningExperimentResults(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "experiment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Experiments.Queries.GetLearningExperimentResults(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
