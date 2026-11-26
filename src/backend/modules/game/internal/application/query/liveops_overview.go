package query

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// KPIThresholds are liveops_kpi_thresholds (CONFIG_DEFAULTS, DESIGN_SYSTEM §4.3, MONITORING): warning / incident levels of
// the Live dashboard KPIs; target_ccu is the planned capacity.
type KPIThresholds struct {
	TargetCCU            float64
	CCUWarnRatio         float64 // 0.9 of target_ccu
	MatchWaitP95Warn     float64 // seconds
	MatchWaitP95Incident float64
	Error5xxWarn         float64 // ratio
	Error5xxIncident     float64
	ReportsWarn          float64 // reports in the last hour
	ReportsIncident      float64
	SISAgeWarnHours      float64
	SISAgeIncidentHours  float64
	RollbacksWarn        float64
}

// DefaultKPIThresholds are the CONFIG_DEFAULTS values.
func DefaultKPIThresholds() KPIThresholds {
	return KPIThresholds{TargetCCU: 10000, CCUWarnRatio: 0.9, MatchWaitP95Warn: 60, MatchWaitP95Incident: 120, Error5xxWarn: 0.01,
		Error5xxIncident: 0.05, ReportsWarn: 10, ReportsIncident: 50, SISAgeWarnHours: 26, SISAgeIncidentHours: 48, RollbacksWarn: 1}
}

// KPIDef is one entry of the fixed query catalog of the Live dashboard (F080-AC-17: no free query from the portal).
type KPIDef struct {
	Code      string
	Unit      string
	Query     string
	Breakdown string // label of the breakdown (query returns one sample per value), "" = single value
	Warn      func(t KPIThresholds) *float64
	Incident  func(t KPIThresholds) *float64
}

func fp(v float64) *float64 { return &v }

// KPICatalog is the PromQL catalog (metric names of MONITORING; labels never carry a user id or content — F080-AC-18).
var KPICatalog = []KPIDef{
	{Code: "ccu", Unit: "players", Query: `sum(eduworld_rt_connections)`,
		Warn: func(t KPIThresholds) *float64 { return fp(t.TargetCCU * t.CCUWarnRatio) }, Incident: func(t KPIThresholds) *float64 { return fp(t.TargetCCU) }},
	{Code: "game_sessions_active", Unit: "sessions", Query: `sum(eduworld_game_sessions_active)`},
	{Code: "matches_started_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_mm_matches_created_total[5m])) * 60`},
	{Code: "matchmaking_queue", Unit: "players", Query: `sum by (mode) (eduworld_mm_queue_size)`, Breakdown: "mode"},
	{Code: "matchmaking_wait_p95_seconds", Unit: "seconds", Query: `histogram_quantile(0.95, sum by (le) (rate(eduworld_mm_wait_seconds_bucket[5m])))`,
		Warn: func(t KPIThresholds) *float64 { return fp(t.MatchWaitP95Warn) }, Incident: func(t KPIThresholds) *float64 { return fp(t.MatchWaitP95Incident) }},
	{Code: "match_duration_p50_seconds", Unit: "seconds", Query: `histogram_quantile(0.5, sum by (le) (rate(eduworld_match_duration_seconds_bucket[1h])))`},
	{Code: "logins_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_auth_game_login_total[5m])) * 60`},
	{Code: "chat_messages_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_chat_messages_total[5m])) * 60`},
	{Code: "chat_blocked_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_chat_rejected_total[5m])) * 60`},
	{Code: "reports_pending", Unit: "reports", Query: `sum(increase(eduworld_chat_reports_total[1h]))`,
		Warn: func(t KPIThresholds) *float64 { return fp(t.ReportsWarn) }, Incident: func(t KPIThresholds) *float64 { return fp(t.ReportsIncident) }},
	{Code: "http_5xx_rate", Unit: "ratio", Query: `sum by (module) (rate(eduworld_http_server_requests_total{status_class="5xx"}[5m])) / sum by (module) (rate(eduworld_http_server_requests_total[5m]))`,
		Breakdown: "module", Warn: func(t KPIThresholds) *float64 { return fp(t.Error5xxWarn) }, Incident: func(t KPIThresholds) *float64 { return fp(t.Error5xxIncident) }},
	{Code: "api_latency_p95_seconds", Unit: "seconds", Query: `histogram_quantile(0.95, sum by (le) (rate(eduworld_http_server_duration_seconds_bucket[5m])))`},
	{Code: "content_releases_24h", Unit: "releases", Query: `sum(increase(eduworld_content_releases_total{action="publish"}[24h]))`},
	{Code: "content_rollbacks_24h", Unit: "releases", Query: `sum(increase(eduworld_content_releases_total{action="rollback"}[24h]))`,
		Warn: func(t KPIThresholds) *float64 { return fp(t.RollbacksWarn) }},
	{Code: "sis_last_success_age_hours", Unit: "hours", Query: `(time() - min(eduworld_sis_last_success_timestamp)) / 3600`,
		Warn: func(t KPIThresholds) *float64 { return fp(t.SISAgeWarnHours) }, Incident: func(t KPIThresholds) *float64 { return fp(t.SISAgeIncidentHours) }},
	{Code: "attempts_submitted_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_attempt_submitted_total[5m])) * 60`},
	{Code: "exam_attempts_per_minute", Unit: "per_minute", Query: `sum(rate(eduworld_exam_attempts_started_total[5m])) * 60`},
	{Code: "db_wait_seconds_per_minute", Unit: "seconds", Query: `sum(rate(go_sql_wait_duration_seconds_total[5m])) * 60`},
}

// LiveOpsOverview is getLiveOpsOverview (F080-AC-17): the KPIs of the fixed catalog read from Prometheus with their
// threshold state; a KPI without series is null with status unknown (never a fake 0); Prometheus down → 503
// METRICS_UNAVAILABLE (problem UNAVAILABLE).
type LiveOpsOverview struct {
	Source     port.MetricsSource
	Thresholds KPIThresholds
	Clock      func() time.Time
}

// KPIBreakdown is one labelled value.
type KPIBreakdown struct {
	Label string   `json:"label"`
	Value *float64 `json:"value"`
}

// KPIView is LiveOpsKpi.
type KPIView struct {
	Code              string         `json:"code"`
	Value             *float64       `json:"value"`
	Unit              string         `json:"unit"`
	Status            string         `json:"status"`
	ThresholdWarning  *float64       `json:"threshold_warning"`
	ThresholdIncident *float64       `json:"threshold_incident"`
	Breakdown         []KPIBreakdown `json:"breakdown,omitempty"`
}

// OverviewView is LiveOpsOverview.
type OverviewView struct {
	AsOf time.Time `json:"as_of"`
	KPIs []KPIView `json:"kpis"`
}

// Status classifies a value against its thresholds.
func Status(v *float64, warn, incident *float64) string {
	switch {
	case v == nil:
		return "unknown"
	case incident != nil && *v >= *incident:
		return "incident"
	case warn != nil && *v >= *warn:
		return "warning"
	}
	return "normal"
}

// Get reads the catalog (game.liveops.read, platform staff). tenantIDs only narrows the authorization audit: the
// technical metrics carry no tenant label (MONITORING), the overview is platform-wide.
func (h LiveOpsOverview) Get(ctx context.Context, tenantIDs []uuid.UUID) (out OverviewView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetLiveOpsOverview")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if err := authz.Require(ctx, p, authz.PermGameLiveopsRead, authz.Resource{}); err != nil {
		return out, err
	}
	if !p.IsPlatform() || p.TenantID != nil {
		return out, problem.New(problem.PermissionDenied, "")
	}
	if h.Source == nil {
		return out, problem.New(problem.Unavailable, "metrics source not configured (EW_PROMETHEUS_URL)").Retry()
	}
	out = OverviewView{KPIs: make([]KPIView, 0, len(KPICatalog))}
	for _, d := range KPICatalog {
		samples, err := h.Source.Query(ctx, d.Query)
		if err != nil {
			if errors.Is(err, port.ErrMetricsUnavailable) {
				return OverviewView{}, problem.New(problem.Unavailable, "Prometheus did not answer").Retry().Wrap(err)
			}
			return OverviewView{}, err
		}
		k := KPIView{Code: d.Code, Unit: d.Unit}
		if d.Warn != nil {
			k.ThresholdWarning = d.Warn(h.Thresholds)
		}
		if d.Incident != nil {
			k.ThresholdIncident = d.Incident(h.Thresholds)
		}
		var worst *float64
		if d.Breakdown != "" {
			sort.Slice(samples, func(i, j int) bool { return samples[i].Labels[d.Breakdown] < samples[j].Labels[d.Breakdown] })
			for i, s := range samples {
				if i == 50 {
					break
				}
				v := clean(s.Value)
				k.Breakdown = append(k.Breakdown, KPIBreakdown{Label: trunc64(s.Labels[d.Breakdown]), Value: v})
				if v != nil && (worst == nil || *v > *worst) {
					worst = v
				}
			}
			if d.Code == "matchmaking_queue" && len(samples) > 0 {
				var sum float64
				for _, s := range samples {
					if v := clean(s.Value); v != nil {
						sum += *v
					}
				}
				worst = &sum
			}
			k.Value = worst
		} else if len(samples) > 0 {
			k.Value = clean(samples[0].Value)
		}
		k.Status = Status(k.Value, k.ThresholdWarning, k.ThresholdIncident)
		out.KPIs = append(out.KPIs, k)
	}
	out.AsOf = time.Now().UTC()
	if h.Clock != nil {
		out.AsOf = h.Clock().UTC()
	}
	return out, nil
}

func clean(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

func trunc64(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}
