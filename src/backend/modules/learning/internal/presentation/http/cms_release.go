package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ReleaseCommands are the release use cases (command.Releases, F026).
type ReleaseCommands interface {
	Publish(ctx context.Context, in command.PublishContentInput) (dto.ContentReleaseView, error)
	Rollback(ctx context.Context, in command.RollbackInput) (dto.ContentReleaseView, error)
	Unpublish(ctx context.Context, in command.UnpublishInput) (dto.ContentReleaseView, error)
	CancelSchedule(ctx context.Context, in command.CancelScheduleInput) (dto.ContentReleaseView, error)
}

// ReleaseQueries are the reads of the release (query.Releases, F026).
type ReleaseQueries interface {
	History(ctx context.Context, typ string, id uuid.UUID, status string, in query.PageInput) (query.VersionHistory, error)
	GetVersion(ctx context.Context, typ string, id uuid.UUID, no int) (dto.ContentVersionDetail, error)
	Diff(ctx context.Context, typ string, id uuid.UUID, no, with int) (query.VersionDiff, error)
	ListReleases(ctx context.Context, typ string, id uuid.UUID, in query.PageInput) (query.Page[dto.ContentReleaseView], error)
	Usages(ctx context.Context, typ string, id uuid.UUID, in query.PageInput) (query.Page[dto.ContentUsageView], error)
}

const cmsContent = "/cms/learning/content/{content_type}/{content_id}"

// ReleaseOperations of F026 (merged into Operations).
var ReleaseOperations = map[string]httpx.Operation{
	"publishContentVersion": op("publishContentVersion", http.MethodPost, cmsContent+"/versions/{version}:publish", httpx.TenantOptional,
		httpx.IdempotencyRequired, 200),
	"rollbackContent":     op("rollbackContent", http.MethodPost, cmsContent+":rollback", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"unpublishContent":    op("unpublishContent", http.MethodPost, cmsContent+":unpublish", httpx.TenantOptional, httpx.IdempotencyRequired, 200),
	"listContentVersions": op("listContentVersions", http.MethodGet, cmsContent+"/versions", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getContentVersion":   op("getContentVersion", http.MethodGet, cmsContent+"/versions/{version}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"diffContentVersions": op("diffContentVersions", http.MethodGet, cmsContent+"/versions/{version}:diff", httpx.TenantOptional,
		httpx.IdempotencyNone, 200),
	"listContentReleases": op("listContentReleases", http.MethodGet, cmsContent+"/releases", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"getContentUsages":    op("getContentUsages", http.MethodGet, cmsContent+"/usages", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	"cancelScheduledPublication": op("cancelScheduledPublication", http.MethodPost, cmsContent+"/releases/{release_id}:cancel",
		httpx.TenantOptional, httpx.IdempotencyRequired, 200),
}

func init() {
	for id, o := range ReleaseOperations {
		Operations[id] = o
	}
	workflowRoutes["publishContentVersion"] = Handlers.publishContent
	workflowRoutes["rollbackContent"] = Handlers.rollbackContent
	workflowRoutes["unpublishContent"] = Handlers.unpublishContent
	workflowRoutes["listContentVersions"] = Handlers.listContentVersions
	workflowRoutes["getContentVersion"] = Handlers.getContentVersion
	workflowRoutes["diffContentVersions"] = Handlers.diffContentVersions
	workflowRoutes["listContentReleases"] = Handlers.listContentReleases
	workflowRoutes["getContentUsages"] = Handlers.getContentUsages
	workflowRoutes["cancelScheduledPublication"] = Handlers.cancelSchedule
}

func contentRef(r *http.Request) (string, uuid.UUID, error) {
	id, err := pathUUID(r, "content_id")
	return chi.URLParam(r, "content_type"), id, err
}

func release(w http.ResponseWriter, r *http.Request, out dto.ContentReleaseView, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	status := http.StatusOK
	if out.Scheduled {
		status = http.StatusAccepted
	}
	writeJSON(w, status, out)
}

func (h Handlers) publishContent(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.VersionMismatch {
			err = problem.New(problem.ContentVersionConflict, "")
		}
		fail(w, r, err)
		return
	}
	var b struct {
		PublishAt  *time.Time `json:"publish_at"`
		ChangeNote *string    `json:"change_note"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &b); err != nil {
			fail(w, r, err)
			return
		}
	}
	out, err := h.Releases.Publish(r.Context(), command.PublishContentInput{Type: typ, ContentID: id, VersionNo: n, IfMatch: rv, PublishAt: b.PublishAt,
		ChangeNote: b.ChangeNote})
	release(w, r, out, err)
}

func (h Handlers) rollbackContent(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ToVersion int    `json:"to_version"`
		Reason    string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Releases.Rollback(r.Context(), command.RollbackInput{Type: typ, ContentID: id, ToVersion: b.ToVersion, Reason: b.Reason})
	release(w, r, out, err)
}

func (h Handlers) unpublishContent(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
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
	out, err := h.Releases.Unpublish(r.Context(), command.UnpublishInput{Type: typ, ContentID: id, Reason: b.Reason})
	release(w, r, out, err)
}

func (h Handlers) cancelSchedule(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	sid, err := pathUUID(r, "release_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Releases.CancelSchedule(r.Context(), command.CancelScheduleInput{Type: typ, ContentID: id, ScheduleID: sid})
	release(w, r, out, err)
}

func (h Handlers) listContentVersions(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ReleaseQueries.History(r.Context(), typ, id, r.URL.Query().Get("status"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getContentVersion(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.ReleaseQueries.GetVersion(r.Context(), typ, id, n)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, out.RowVersion)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) diffContentVersions(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	n, err := pathVersion(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	with, err := strconv.Atoi(r.URL.Query().Get("with"))
	if err != nil || with < 1 {
		fail(w, r, problem.Invalid("with", problem.SubInvalid, "must be a version number"))
		return
	}
	out, err := h.ReleaseQueries.Diff(r.Context(), typ, id, n, with)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listContentReleases(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.ReleaseQueries.ListReleases(r.Context(), typ, id, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.ContentReleaseView) dto.ContentReleaseView { return v })
}

func (h Handlers) getContentUsages(w http.ResponseWriter, r *http.Request) {
	typ, id, err := contentRef(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.ReleaseQueries.Usages(r.Context(), typ, id, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.ContentUsageView) dto.ContentUsageView { return v })
}
