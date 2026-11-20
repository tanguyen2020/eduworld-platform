package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/simulation"
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
	Templates(ctx context.Context) ([]simulation.Descriptor, error)
}

const cmsActivities = "/cms/learning/activities"

func init() {
	for id, o := range map[string]httpx.Operation{
		"listCmsActivities":      op("listCmsActivities", http.MethodGet, cmsActivities, httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"createCmsActivity":      op("createCmsActivity", http.MethodPost, cmsActivities, httpx.TenantOptional, httpx.IdempotencyRequired, 201),
		"getCmsActivity":         op("getCmsActivity", http.MethodGet, cmsActivities+"/{activity_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"updateCmsActivityDraft": op("updateCmsActivityDraft", http.MethodPatch, cmsActivities+"/{activity_id}", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"createCmsActivityVersion": op("createCmsActivityVersion", http.MethodPost, cmsActivities+"/{activity_id}/versions", httpx.TenantOptional,
			httpx.IdempotencyRequired, 201),
		"listActivityTypes": op("listActivityTypes", http.MethodGet, "/cms/learning/activity-types", httpx.TenantOptional, httpx.IdempotencyNone, 200),
		"listSimulationTemplates": op("listSimulationTemplates", http.MethodGet, "/cms/learning/simulation-templates", httpx.TenantOptional,
			httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var cmsActivityRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCmsActivities":        Handlers.listCmsActivities,
	"createCmsActivity":        Handlers.createCmsActivity,
	"getCmsActivity":           Handlers.getCmsActivity,
	"updateCmsActivityDraft":   Handlers.updateCmsActivityDraft,
	"createCmsActivityVersion": Handlers.createCmsActivityVersion,
	"listActivityTypes":        Handlers.listActivityTypes,
	"listSimulationTemplates":  Handlers.listSimulationTemplates,
}

// activityDefinition is ActivityDefinition (command.ActivityJSON, also the idempotent replay body).
// activityDefinition is ActivityDefinition with the assigned flag of the session tenant (F058-AC-27).
func activityDefinition(r *http.Request, d query.DocView, current *int) map[string]any {
	return command.ActivityJSONFor(d.Doc, d.Version, current, sessionTenantOf(r))
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
	writeJSON(w, http.StatusCreated, activityDefinition(r, query.DocView{Doc: res.Doc, Version: res.Version}, nil))
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
	writeJSON(w, http.StatusOK, activityDefinition(r, v.DocView, v.CurrentPublishedVersion))
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
		return activityDefinition(r, v.DocView, v.CurrentPublishedVersion)
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
	writeJSON(w, http.StatusOK, activityDefinition(r, query.DocView{Doc: res.Doc, Version: res.Version}, cur.CurrentPublishedVersion))
}

// createCmsActivityVersion opens a new draft from a version of the activity (OQ F030 owner 2026-11-20, same pattern as
// createQuestionVersion): a published version is never edited (R11), its content is copied into version n+1 draft.
func (h Handlers) createCmsActivityVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "activity_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		FromVersion int    `json:"from_version"`
		Note        string `json:"note"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	cur, err := h.CmsActivities.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := h.Docs.NewVersion(r.Context(), command.NewDocVersionInput{Type: contentmodel.TypeActivity, ID: id, FromVersion: b.FromVersion, Note: b.Note})
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, res.Version.RowVersion)
	writeJSON(w, http.StatusCreated, activityDefinition(r, query.DocView{Doc: res.Doc, Version: res.Version}, cur.CurrentPublishedVersion))
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

type simulationTemplate struct {
	TemplateCode     string                 `json:"template_code"`
	TemplateVersion  int                    `json:"template_version"`
	Title            string                 `json:"title"`
	SimulationKind   string                 `json:"simulation_kind"`
	ParameterSchema  json.RawMessage        `json:"parameter_schema"`
	Actions          []simulation.ActionDef `json:"actions"`
	Metrics          []simulation.MetricDef `json:"metrics"`
	CheckerAvailable bool                   `json:"checker_available"`
	Status           string                 `json:"status"`
	MinClientVersion string                 `json:"min_client_version"`
}

// listSimulationTemplates lists the packaged template versions (F066-AC-26, research R2: read-only, shipped with the
// release); the title follows Accept-Language (vi by default).
func (h Handlers) listSimulationTemplates(w http.ResponseWriter, r *http.Request) {
	list, err := h.ActivityTypes.Templates(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	lang := "vi"
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		lang = "en"
	}
	data := make([]simulationTemplate, len(list))
	for i, d := range list {
		title := d.Title[lang]
		if title == "" {
			title = d.Title["vi"]
		}
		data[i] = simulationTemplate{TemplateCode: d.TemplateCode, TemplateVersion: d.TemplateVersion, Title: title, SimulationKind: d.SimulationKind,
			ParameterSchema: d.ParameterSchema, Actions: d.Actions, Metrics: d.Metrics, CheckerAvailable: d.CheckerAvailable, Status: d.Status,
			MinClientVersion: d.MinClientVersion}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
