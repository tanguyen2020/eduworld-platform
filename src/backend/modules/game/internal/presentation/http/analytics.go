package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AnalyticsIngest is postTelemetry (command.AnalyticsIngest, F080 US1).
type AnalyticsIngest interface {
	PostTelemetry(ctx context.Context, b command.TelemetryBatch) (int, error)
}

// AnalyticsQueries are the Game Analytics reads (query.GameAnalytics, F080 US4/US5).
type AnalyticsQueries interface {
	GetMetrics(ctx context.Context, codes []string, f query.AnalyticsFilter) (query.MetricsView, error)
	GetRetention(ctx context.Context, f query.AnalyticsFilter) (query.RetentionView, error)
	GetFunnel(ctx context.Context, code string, f query.AnalyticsFilter) (query.FunnelView, error)
	GetContentStats(ctx context.Context, kind, sort string, limit int, cursor string, f query.AnalyticsFilter) (query.ContentPage, error)
	GetPerformance(ctx context.Context, f query.AnalyticsFilter) (query.PerformanceView, error)
	GetTenantSummary(ctx context.Context, in query.TenantSummaryInput) (query.TenantSummaryView, error)
}

// AnalyticsExports are createGameAnalyticsExport / getGameAnalyticsExport (command.AnalyticsExports).
type AnalyticsExports interface {
	Create(ctx context.Context, report string, params json.RawMessage) (command.ExportView, error)
	Get(ctx context.Context, id uuid.UUID) (command.ExportView, error)
}

// LiveOpsOverviewHandler is getLiveOpsOverview (query.LiveOpsOverview).
type LiveOpsOverviewHandler interface {
	Get(ctx context.Context, tenantIDs []uuid.UUID) (query.OverviewView, error)
}

// TelemetryFlag answers ClientConfig.product_telemetry_enabled (query.ProductTelemetryFlag).
type TelemetryFlag interface {
	ProductTelemetry(ctx context.Context, tenant *uuid.UUID) bool
}

// AnalyticsHandlers are the F080 handlers; nil Ingest = not mounted (binaries without the analytics key).
type AnalyticsHandlers struct {
	Ingest   AnalyticsIngest
	Queries  AnalyticsQueries
	Exports  AnalyticsExports
	Overview LiveOpsOverviewHandler
}

// AnalyticsAdminOperations are the platform routes of F080 (internal gateway, portal client, platform role, R20).
var AnalyticsAdminOperations = []string{"getGameMetrics", "getGameRetention", "getGameFunnel", "getGameContentStats", "getGamePerformance",
	"createGameAnalyticsExport", "getGameAnalyticsExport", "getLiveOpsOverview"}

var analyticsOperations = []httpx.Operation{
	op("postTelemetry", http.MethodPost, "/game/telemetry", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusAccepted, httpx.GuardGamePlayer),
	op("getTenantGameSummary", http.MethodGet, "/tenant/analytics/game/summary", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff),
	op("getGameMetrics", http.MethodGet, "/admin/analytics/game/metrics", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getGameRetention", http.MethodGet, "/admin/analytics/game/retention", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getGameFunnel", http.MethodGet, "/admin/analytics/game/funnels/{funnel_code}", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getGameContentStats", http.MethodGet, "/admin/analytics/game/content-stats", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getGamePerformance", http.MethodGet, "/admin/analytics/game/performance", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("createGameAnalyticsExport", http.MethodPost, "/admin/analytics/game/exports", httpx.TenantOptional, httpx.IdempotencyRequired,
		http.StatusAccepted, httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getGameAnalyticsExport", http.MethodGet, "/admin/analytics/game/exports/{export_id}", httpx.TenantNone, httpx.IdempotencyNone,
		http.StatusOK, httpx.GuardPortalStaff|httpx.GuardPlatformRole),
	op("getLiveOpsOverview", http.MethodGet, "/admin/liveops/overview", httpx.TenantNone, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardPortalStaff|httpx.GuardPlatformRole),
}

func init() {
	for _, o := range analyticsOperations {
		Operations[o.ID] = o
	}
	PlayerOperations = append(PlayerOperations, "postTelemetry")
}

func (h Handlers) analyticsRoutes() map[string]http.HandlerFunc {
	a := h.Analytics
	if a.Ingest == nil || a.Queries == nil || a.Exports == nil || a.Overview == nil {
		return nil
	}
	return map[string]http.HandlerFunc{
		"postTelemetry":             a.postTelemetry,
		"getTenantGameSummary":      a.getTenantGameSummary,
		"getGameMetrics":            a.getGameMetrics,
		"getGameRetention":          a.getGameRetention,
		"getGameFunnel":             a.getGameFunnel,
		"getGameContentStats":       a.getGameContentStats,
		"getGamePerformance":        a.getGamePerformance,
		"createGameAnalyticsExport": a.createExport,
		"getGameAnalyticsExport":    a.getExport,
		"getLiveOpsOverview":        a.getLiveOpsOverview,
	}
}

// list returns the values of a repeated (form, explode) parameter; comma lists are accepted too.
func list(r *http.Request, name string) []string {
	var out []string
	for _, v := range r.URL.Query()[name] {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func dateParam(r *http.Request, name string) (time.Time, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return t, problem.Invalid(name, problem.SubInvalid, "YYYY-MM-DD")
	}
	return t, nil
}

func uuidList(r *http.Request, name string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, s := range list(r, name) {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, problem.Invalid(name, problem.SubInvalid, "UUIDs")
		}
		out = append(out, id)
	}
	return out, nil
}

func analyticsFilter(r *http.Request, fromName, toName string) (query.AnalyticsFilter, error) {
	from, err := dateParam(r, fromName)
	if err != nil {
		return query.AnalyticsFilter{}, err
	}
	to, err := dateParam(r, toName)
	if err != nil {
		return query.AnalyticsFilter{}, err
	}
	tenants, err := uuidList(r, "tenant_ids")
	if err != nil {
		return query.AnalyticsFilter{}, err
	}
	schools, err := uuidList(r, "school_ids")
	if err != nil {
		return query.AnalyticsFilter{}, err
	}
	f := query.AnalyticsFilter{From: from, To: to, Grain: r.URL.Query().Get("grain"), TenantIDs: tenants, GradeBands: list(r, "grade_bands"),
		Platforms: list(r, "platforms"), Locales: list(r, "locales"), DeviceTiers: list(r, "device_tiers"), ClientVersions: list(r, "client_versions")}
	for _, s := range schools {
		f.Schools = append(f.Schools, s.String())
	}
	return f, nil
}

// failDash adds Retry-After to 429 DASHBOARD_BUSY.
func failDash(w http.ResponseWriter, r *http.Request, err error) {
	var be *query.BusyError
	if errors.As(err, &be) {
		w.Header().Set("Retry-After", be.Seconds())
	}
	fail(w, r, err)
}

func (a AnalyticsHandlers) postTelemetry(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Platform      string           `json:"platform"`
		ClientVersion string           `json:"client_version"`
		DeviceTier    string           `json:"device_tier"`
		Preset        string           `json:"preset"`
		Renderer      string           `json:"renderer"`
		Locale        string           `json:"locale"`
		Events        []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	if _, err := a.Ingest.PostTelemetry(r.Context(), command.TelemetryBatch{Platform: b.Platform, ClientVersion: b.ClientVersion,
		DeviceTier: b.DeviceTier, Preset: b.Preset, Renderer: b.Renderer, Locale: b.Locale, Events: b.Events}); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (a AnalyticsHandlers) getTenantGameSummary(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "from", "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	schools, _ := uuidList(r, "school_ids")
	out, err := a.Queries.GetTenantSummary(r.Context(), query.TenantSummaryInput{From: f.From, To: f.To, Grain: f.Grain, SchoolIDs: schools,
		GradeBands: f.GradeBands})
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getGameMetrics(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "from", "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Queries.GetMetrics(r.Context(), list(r, "metric_codes"), f)
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getGameRetention(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "cohort_from", "cohort_to")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Queries.GetRetention(r.Context(), f)
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getGameFunnel(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "from", "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Queries.GetFunnel(r.Context(), chi.URLParam(r, "funnel_code"), f)
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getGameContentStats(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "from", "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := a.Queries.GetContentStats(r.Context(), q.Get("kind"), q.Get("sort"), limit, q.Get("cursor"), f)
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getGamePerformance(w http.ResponseWriter, r *http.Request) {
	f, err := analyticsFilter(r, "from", "to")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Queries.GetPerformance(r.Context(), f)
	if err != nil {
		failDash(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) createExport(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Report string          `json:"report"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	out, err := a.Exports.Create(r.Context(), b.Report, b.Params)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (a AnalyticsHandlers) getExport(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "export_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Exports.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a AnalyticsHandlers) getLiveOpsOverview(w http.ResponseWriter, r *http.Request) {
	tenants, err := uuidList(r, "tenant_ids")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := a.Overview.Get(r.Context(), tenants)
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) && pe.Code == problem.Unavailable {
			w.Header().Set("Retry-After", "15")
		}
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}
