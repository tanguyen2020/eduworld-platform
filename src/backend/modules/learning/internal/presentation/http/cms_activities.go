package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// CmsActivityQueries are the CMS reads of activities (query.CmsActivities) and the registry (query.ActivityTypes), F030 US4.
type CmsActivityQueries interface {
	Get(ctx context.Context, id uuid.UUID) (query.CmsActivityView, error)
	List(ctx context.Context, f read.DocFilter, in query.PageInput) (query.Page[query.CmsActivityView], error)
}

// ActivityTypeQueries lists the Activity Registry (query.ActivityTypes).
type ActivityTypeQueries interface {
	List(ctx context.Context) ([]activity.Info, error)
}

const cmsActivities = "/cms/learning/activities"

func init() {
	for id, o := range map[string]httpx.Operation{
		"listCmsActivities":      op("listCmsActivities", http.MethodGet, cmsActivities, httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"createCmsActivity":      op("createCmsActivity", http.MethodPost, cmsActivities, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
		"getCmsActivity":         op("getCmsActivity", http.MethodGet, cmsActivities+"/{activity_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"updateCmsActivityDraft": op("updateCmsActivityDraft", http.MethodPatch, cmsActivities+"/{activity_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"listActivityTypes":      op("listActivityTypes", http.MethodGet, "/cms/learning/activity-types", httpx.TenantOptional, httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var cmsActivityRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCmsActivities":      Handlers.listCmsActivities,
	"createCmsActivity":      Handlers.createCmsActivity,
	"getCmsActivity":         Handlers.getCmsActivity,
	"updateCmsActivityDraft": Handlers.updateCmsActivityDraft,
	"listActivityTypes":      Handlers.listActivityTypes,
}

// activityDefinition is ActivityDefinition: the activity fields of the version body plus its metadata columns.
func activityDefinition(d query.DocView, current *int) map[string]any {
	v := d.Version
	a := attemptview.ParseBody(v.Body)
	out := map[string]any{"activity_id": d.Doc.ID, "activity_version_id": v.ID, "version": v.VersionNo, "activity_type": a.ActivityType,
		"title": v.Title, "status": string(v.Status), "owner_scope": string(d.Doc.Owner.Scope), "grade_levels": nonNil(v.GradeLevels),
		"objective_ids": nonNilIDs(v.ObjectiveIDs), "current_published_version": current, "updated_at": v.UpdatedAt,
		"content_language": v.ContentLanguage}
	if d.Doc.Owner.TenantID != nil {
		out["tenant_id"] = *d.Doc.Owner.TenantID
	}
	if v.SubjectID != nil {
		out["subject_id"] = *v.SubjectID
	}
	if a.Difficulty != nil {
		out["difficulty"] = *a.Difficulty
	}
	if len(a.Config) > 0 {
		out["config"] = a.Config
	}
	if v.TranslationStatus != nil {
		out["translation_status"] = *v.TranslationStatus
	}
	return out
}

func nonNil(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func nonNilIDs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

func (h Handlers) createCmsActivity(w http.ResponseWriter, r *http.Request) {
	f, err := readDoc(r, contentmodel.TypeActivity)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.Create(r.Context(), contentmodel.TypeActivity, f)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, activityDefinition(query.DocView{Doc: res.Doc, Version: res.Version}, nil))
}

func (h Handlers) getCmsActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "activity_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.CmsActivities.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.Version.RowVersion)
	writeJSON(w, http.StatusOK, activityDefinition(v.DocView, v.CurrentPublishedVersion))
}

func (h Handlers) listCmsActivities(w http.ResponseWriter, r *http.Request) {
	f := read.DocFilter{Status: r.URL.Query().Get("status"), GradeLevel: queryInt(r, "grade_level"), ActivityType: r.URL.Query().Get("activity_type")}
	var err error
	if f.SubjectID, err = queryUUID(r, "subject_id"); err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.CmsActivities.List(r.Context(), f, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v query.CmsActivityView) map[string]any {
		return activityDefinition(v.DocView, v.CurrentPublishedVersion)
	})
}

// updateCmsActivityDraft edits the open draft of an activity (the newest version; If-Match is its ETag): a published
// version is never edited (409 CONTENT_NOT_EDITABLE, create a new version through the content endpoints).
func (h Handlers) updateCmsActivityDraft(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "activity_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	f, err := readDoc(r, contentmodel.TypeActivity)
	if err != nil {
		fail(w, r, err)
		return
	}
	cur, err := h.CmsActivities.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.UpdateDraft(r.Context(), command.UpdateDocDraftInput{Type: contentmodel.TypeActivity, ID: id, VersionNo: cur.Version.VersionNo,
		IfMatch: rv, Fields: f})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusOK, activityDefinition(query.DocView{Doc: res.Doc, Version: res.Version}, cur.CurrentPublishedVersion))
}

// activityTypeInfo is ActivityTypeInfo.
type activityTypeInfo struct {
	ActivityType         string   `json:"activity_type"`
	Label                string   `json:"label"`
	SchemaURL            string   `json:"schema_url"`
	SchemaVersion        int      `json:"schema_version"`
	EvaluationStrategies []string `json:"evaluation_strategies"`
	MinClientVersion     string   `json:"min_client_version"`
	Status               string   `json:"status"`
}

func (h Handlers) listActivityTypes(w http.ResponseWriter, r *http.Request) {
	infos, err := h.ActivityTypes.List(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]activityTypeInfo, len(infos))
	for i, in := range infos {
		data[i] = activityTypeInfo{ActivityType: string(in.Type), Label: in.Label, SchemaURL: in.SchemaURL, SchemaVersion: in.SchemaVersion,
			EvaluationStrategies: in.Strategies, MinClientVersion: in.MinClientVersion, Status: string(in.Status)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
