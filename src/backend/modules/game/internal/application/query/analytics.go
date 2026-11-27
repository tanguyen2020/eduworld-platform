package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// DashSession is the dashboard read session (entx.ReadSession on game_dashboard / game_platform, replica).
type DashSession interface {
	Read(ctx context.Context, fn func(ctx context.Context) error) error
}

// GameAnalytics are the Game Analytics reads (F080 US4, US5): getGameMetrics, getGameRetention, getGameFunnel,
// getGameContentStats, getGamePerformance and getTenantGameSummary. They read the rollup tables only (never events_raw nor
// player_days, ADR-0062) through the dashboard session, with the range and grain rules, statement timeout, per user /
// tenant concurrency limits, small-cell suppression (analytics_min_cell_size, complementary) and the meta of every
// dashboard answer (grain, data_as_of, is_partial, available_from).
type GameAnalytics struct {
	Session DashSession // nil = not configured → 503
	Model   port.AnalyticsDashboard
	Cache   port.DashboardCache
	Limiter port.DashboardLimiter
	Metrics port.AnalyticsMetrics
	Config  analytics.Config
	Cursors *page.Codec
	Clock   func() time.Time
}

func (q GameAnalytics) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func (q GameAnalytics) rejected(reason string) {
	if q.Metrics != nil {
		q.Metrics.Rejected(reason)
	}
}

// AnalyticsFilter is the common filter of the platform analytics operations.
type AnalyticsFilter struct {
	From, To       time.Time
	Grain          string
	TenantIDs      []uuid.UUID
	Schools        []string
	GradeBands     []string
	Platforms      []string
	Locales        []string
	DeviceTiers    []string
	ClientVersions []string
}

// BusyError is 429 DASHBOARD_BUSY with Retry-After.
type BusyError struct {
	After time.Duration
	Err   *problem.Error
}

func (e *BusyError) Error() string { return e.Err.Error() }

// Unwrap exposes the problem.
func (e *BusyError) Unwrap() error { return e.Err }

// Seconds is the Retry-After value (≥ 1).
func (e *BusyError) Seconds() string {
	return strconv.Itoa(max(int((e.After+time.Second-1)/time.Second), 1))
}

// Meta is the dashboard meta of every answer (DASHBOARD_DATA §8).
type Meta struct {
	Grain         string    `json:"grain"`
	DataAsOf      time.Time `json:"data_as_of"`
	IsPartial     bool      `json:"is_partial"`
	AvailableFrom *string   `json:"available_from"`
	// ExcludedTenantIDs are the QA tenants left out of a cross-tenant answer, named in tenant_ids or not (F134-AC-09).
	ExcludedTenantIDs []uuid.UUID `json:"excluded_tenant_ids,omitempty"`
}

type metaAcc struct {
	oldest  time.Time
	partial bool
}

func (m *metaAcc) add(computed time.Time, final bool) {
	if !computed.IsZero() && (m.oldest.IsZero() || computed.Before(m.oldest)) {
		m.oldest = computed
	}
	if !final {
		m.partial = true
	}
}

func (q GameAnalytics) meta(rg analytics.Range, m metaAcc, from *time.Time) Meta {
	asOf := q.now()
	if !m.oldest.IsZero() {
		asOf = m.oldest.UTC()
	}
	out := Meta{Grain: string(rg.Grain), DataAsOf: asOf, IsPartial: m.partial || !rg.To.Before(analytics.DayOf(q.now()))}
	if from != nil {
		s := from.Format(time.DateOnly)
		out.AvailableFrom = &s
	}
	return out
}

// scopeTenants returns the non-QA tenants of the scope (every non-QA tenant when none is named; [uuid.Nil] when only QA
// tenants are named, so the answer is empty, never every tenant) and the QA tenants left out (F134-AC-09).
func (q GameAnalytics) scopeTenants(ctx context.Context, named []uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	tenants, err := q.Model.NonQATenants(ctx, named)
	if err != nil {
		return nil, nil, err
	}
	qa, err := q.Model.QATenants(ctx, named)
	if err != nil {
		return nil, nil, err
	}
	return tenants, qa, nil
}

func (q GameAnalytics) dashRange(from, to time.Time, grain string) (analytics.Range, error) {
	rg, err := analytics.NewRange(from, to, analytics.Grain(grain), q.Config.Range)
	if err != nil {
		q.rejected("range")
		if errors.Is(err, analytics.ErrRangeMissing) {
			return rg, problem.Invalid("from", problem.SubRequired, "from and to are required")
		}
		return rg, problem.New(problem.DashboardRangeTooLarge, err.Error())
	}
	return rg, nil
}

// platformCtx authorizes a platform analytics read (game.analytics.read, platform staff — R20) and lifts the tenant filter
// with the closed reason platform_analytics (audit cross_tenant.accessed + metric — ADR-0089).
func platformCtx(ctx context.Context, perm authz.Permission, tenants []uuid.UUID) (context.Context, authz.Principal, error) {
	if _, ok := ctx.Value(exportKey{}).(bool); ok {
		// Export worker: authorization was checked when the job was created (createGameAnalyticsExport); the read is still an
		// audited platform_analytics bypass.
		bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonPlatformAnalytics, tenancy.WithTenantIDs(tenants...),
			tenancy.WithMode(tenancy.ModeRead))
		return bctx, authz.Principal{}, err
	}
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, err
	}
	if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
		return ctx, p, err
	}
	if !p.IsPlatform() || p.TenantID != nil {
		_ = authz.Require(ctx, p, authz.PermPlatformCrossTenantRead, authz.Resource{})
		return ctx, p, problem.New(problem.PermissionDenied, "")
	}
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "game"), tenancy.ReasonPlatformAnalytics, tenancy.WithTenantIDs(tenants...),
		tenancy.WithMode(tenancy.ModeRead))
	return bctx, p, err
}

// exportKey marks the context of the export worker (unexported: only RenderCSV sets it).
type exportKey struct{}

// run executes fn under the limits and the dashboard session (statement timeout, 504 mapping by the model).
func (q GameAnalytics) run(ctx context.Context, queryID string, user, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	if q.Session == nil || q.Model == nil {
		return problem.New(problem.Unavailable, "dashboard database not configured").Retry()
	}
	start := time.Now()
	if _, export := ctx.Value(exportKey{}).(bool); q.Limiter != nil && !export {
		release, ok := q.Limiter.Acquire(ctx, user, tenant, q.Config.MaxConcurrentPerUser, q.Config.MaxConcurrentTenant)
		if !ok {
			q.rejected("busy")
			return &BusyError{After: 2 * time.Second, Err: problem.New(problem.DashboardBusy, "")}
		}
		defer release()
	}
	err := q.Session.Read(ctx, func(ctx context.Context) error {
		if err := q.Model.Prepare(ctx, q.Config.StatementTimeout); err != nil {
			return err
		}
		return fn(ctx)
	})
	if err != nil {
		var pe *problem.Error
		if errors.As(err, &pe) {
			if pe.Code == problem.DashboardQueryTimeout {
				q.rejected("timeout")
			}
			return err
		}
		slog.ErrorContext(ctx, "analytics dashboard database unavailable", "query", queryID, "error", err)
		return problem.New(problem.Unavailable, "dashboard database unavailable").Retry().Wrap(err)
	}
	if q.Metrics != nil {
		q.Metrics.Query(queryID, time.Since(start).Seconds(), "miss")
	}
	return nil
}

// ---------- getGameMetrics ----------

// MetricPoint is one point of GameMetricSeries.
type MetricPoint struct {
	PeriodStart string   `json:"period_start"`
	Value       *float64 `json:"value"`
	Suppressed  bool     `json:"suppressed"`
}

// MetricSeries is GameMetricSeries.
type MetricSeries struct {
	MetricCode  string            `json:"metric_code"`
	Unit        string            `json:"unit"`
	Granularity string            `json:"granularity"`
	Dims        map[string]string `json:"dims,omitempty"`
	Points      []MetricPoint     `json:"points"`
}

// MetricsView is GameMetricsResponse.
type MetricsView struct {
	Data                 []MetricSeries `json:"data"`
	GeneratedAt          time.Time      `json:"generated_at"`
	SuppressionThreshold int            `json:"suppression_threshold"`
	Meta
}

// maxSeries is the contract bound of GameMetricsResponse.data.
const maxSeries = 50

func filterOf(f AnalyticsFilter) (analytics.Filter, error) {
	flt, err := analytics.NewFilter(map[string][]string{analytics.DimSchool: f.Schools, analytics.DimGradeBand: f.GradeBands,
		analytics.DimPlatform: f.Platforms, analytics.DimLocale: f.Locales, analytics.DimDeviceTier: f.DeviceTiers})
	if err != nil {
		return flt, problem.Invalid("dimension", problem.SubInvalid, err.Error())
	}
	return flt, nil
}

func dimSetOf(f analytics.Filter) string { return f.Key }

// GetMetrics is getGameMetrics (F080-AC-11, AC-12): series of the requested metrics over the range, one per metric (or
// per value of the single dimension filter), merged across the tenants of the scope (every non-QA tenant by default),
// small cells suppressed.
func (q GameAnalytics) GetMetrics(ctx context.Context, codes []string, f AnalyticsFilter) (out MetricsView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameMetrics")
	defer func() { done(err) }()
	ctx, p, err := platformCtx(ctx, authz.PermGameAnalyticsRead, f.TenantIDs)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(f.From, f.To, f.Grain)
	if err != nil {
		return out, err
	}
	if len(codes) == 0 {
		codes = []string{"dau"}
	}
	for i, c := range codes {
		if _, ok := analytics.Lookup(c); !ok {
			return out, problem.Invalid(fmt.Sprintf("metric_codes/%d", i), problem.SubInvalid, "unknown metric code")
		}
	}
	flt, err := filterOf(f)
	if err != nil {
		return out, err
	}
	if n := len(codes) * max(len(flt.Values), 1); n > maxSeries {
		return out, problem.Invalid("metric_codes", problem.SubInvalid, "too many series: narrow the metrics or the dimension values")
	}
	err = q.run(ctx, "game_metrics", p.UserID, uuid.Nil, func(ctx context.Context) error {
		tenants, excluded, err := q.scopeTenants(ctx, f.TenantIDs)
		if err != nil {
			return err
		}
		var macc metaAcc
		series, err := q.series(ctx, tenants, rg, codes, flt, &macc)
		if err != nil {
			return err
		}
		af, err := q.Model.AvailableFrom(ctx, tenants)
		if err != nil {
			return err
		}
		out = MetricsView{Data: series, GeneratedAt: q.now(), SuppressionThreshold: q.Config.MinCellSize, Meta: q.meta(rg, macc, af)}
		out.Meta.ExcludedTenantIDs = excluded
		return nil
	})
	return out, err
}

type cellKey struct {
	date time.Time
	code string
	dims string
}

// merged reads the rows of one grain and merges them across tenants per (date, code, dims).
func (q GameAnalytics) merged(ctx context.Context, tenants []uuid.UUID, rg analytics.Range, g analytics.Grain, codes []string, dimSet string,
	macc *metaAcc) (map[cellKey]analytics.Cell, map[string]analytics.Dims, error) {
	from := analytics.BucketStart(rg.From, g)
	rows, err := q.Model.Rollups(ctx, port.DashboardQuery{Tenants: tenants, From: from, To: rg.To, Grain: g}, codes, dimSet)
	if err != nil {
		return nil, nil, err
	}
	cells := map[cellKey]analytics.Cell{}
	dims := map[string]analytics.Dims{}
	for _, r := range rows {
		m, _ := analytics.Lookup(r.Code)
		k := cellKey{date: r.Date, code: r.Code, dims: r.Dims.Key()}
		cells[k] = m.Merge(cells[k], r.Cell)
		dims[k.dims] = r.Dims
		macc.add(r.Computed, r.Final)
	}
	return cells, dims, nil
}

// hiddenCells applies Suppress per (date, code) over every dims value of the set (the whole dimension, so different
// filters cannot be differenced) or Small for the whole scope.
func (q GameAnalytics) hiddenCells(cells map[cellKey]analytics.Cell) map[cellKey]bool {
	groups := map[[2]string][]analytics.Population{}
	for k, c := range cells {
		g := [2]string{k.date.Format(time.DateOnly), k.code}
		groups[g] = append(groups[g], analytics.Population{Key: k.dims, Players: c.Players})
	}
	out := map[cellKey]bool{}
	for g, pops := range groups {
		d, _ := time.Parse(time.DateOnly, g[0])
		for key := range analytics.Suppress(pops, q.Config.MinCellSize) {
			out[cellKey{date: d, code: g[1], dims: key}] = true
		}
	}
	return out
}

func (q GameAnalytics) series(ctx context.Context, tenants []uuid.UUID, rg analytics.Range, codes []string, flt analytics.Filter,
	macc *metaAcc) ([]MetricSeries, error) {
	byGrain := map[analytics.Grain][]string{}
	for _, c := range codes {
		m, _ := analytics.Lookup(c)
		g := rg.Grain
		if m.Kind == analytics.KindWeekly && g == analytics.Day {
			g = analytics.Week
		}
		byGrain[g] = append(byGrain[g], c)
	}
	var out []MetricSeries
	for _, g := range []analytics.Grain{analytics.Day, analytics.Week, analytics.Month} {
		cs := byGrain[g]
		if len(cs) == 0 {
			continue
		}
		cells, dims, err := q.merged(ctx, tenants, rg, g, cs, dimSetOf(flt), macc)
		if err != nil {
			return nil, err
		}
		hidden := q.hiddenCells(cells)
		buckets := analytics.Range{From: rg.From, To: rg.To, Grain: g}.Buckets()
		values := flt.Values
		if flt.Key == "" {
			values = []string{""}
		}
		for _, c := range codes {
			if !hasString(cs, c) {
				continue
			}
			m, _ := analytics.Lookup(c)
			for _, v := range values {
				s := MetricSeries{MetricCode: c, Unit: m.Unit, Granularity: string(g), Points: make([]MetricPoint, 0, len(buckets))}
				dk := ""
				if flt.Key != "" {
					for key, d := range dims {
						if flt.ValueOf(d) == v {
							dk = key
							s.Dims = d
							break
						}
					}
					if s.Dims == nil {
						s.Dims = dimsOfValue(flt, v)
						dk = analytics.Dims(s.Dims).Key()
					}
				}
				for _, b := range buckets {
					pt := MetricPoint{PeriodStart: b.Format(time.DateOnly)}
					k := cellKey{date: b, code: c, dims: dk}
					if cell, ok := cells[k]; ok {
						if hidden[k] {
							pt.Suppressed = true
						} else {
							pt.Value = m.Value(cell)
						}
					}
					s.Points = append(s.Points, pt)
				}
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func dimsOfValue(f analytics.Filter, v string) analytics.Dims {
	if f.Key == analytics.DimSchool+","+analytics.DimGradeBand {
		for i := 0; i < len(v); i++ {
			if v[i] == '|' {
				return analytics.Dims{analytics.DimSchool: v[:i], analytics.DimGradeBand: v[i+1:]}
			}
		}
	}
	return analytics.Dims{f.Key: v}
}

func hasString(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// ---------- getGameRetention ----------

// RetentionPoint is one day offset of a cohort.
type RetentionPoint struct {
	DayOffset int      `json:"day_offset"`
	Rate      *float64 `json:"rate"`
}

// RetentionCohort is one cohort bucket of RetentionMatrix.
type RetentionCohort struct {
	CohortDate string           `json:"cohort_date"`
	CohortSize *int64           `json:"cohort_size"`
	Retention  []RetentionPoint `json:"retention"`
}

// RetentionView is RetentionMatrix.
type RetentionView struct {
	Cohorts []RetentionCohort `json:"cohorts"`
	Meta
}

// GetRetention is getGameRetention: cohorts of the first active day bucketed by the grain of the cohort range, merged
// across tenants and the selected dimension values; a bucket whose cohort, or one of whose selected values, is small is
// hidden (null).
func (q GameAnalytics) GetRetention(ctx context.Context, f AnalyticsFilter) (out RetentionView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameRetention")
	defer func() { done(err) }()
	ctx, p, err := platformCtx(ctx, authz.PermGameAnalyticsRead, f.TenantIDs)
	if err != nil {
		return out, err
	}
	rg, err := q.dashRange(f.From, f.To, f.Grain)
	if err != nil {
		return out, err
	}
	flt, err := filterOf(f)
	if err != nil {
		return out, err
	}
	err = q.run(ctx, "game_retention", p.UserID, uuid.Nil, func(ctx context.Context) error {
		tenants, excluded, err := q.scopeTenants(ctx, f.TenantIDs)
		if err != nil {
			return err
		}
		rows, err := q.Model.Retention(ctx, port.DashboardQuery{Tenants: tenants, From: rg.From, To: rg.To, Grain: rg.Grain}, flt.Key, flt.Values)
		if err != nil {
			return err
		}
		var macc metaAcc
		type acc struct{ size, ret map[int]int64 }
		buckets := map[time.Time]*acc{}
		for _, r := range rows {
			b := analytics.BucketStart(r.Cohort, rg.Grain)
			a := buckets[b]
			if a == nil {
				a = &acc{size: map[int]int64{}, ret: map[int]int64{}}
				buckets[b] = a
			}
			a.size[r.DayN] += r.Size
			a.ret[r.DayN] += r.Retained
			macc.add(r.Computed, r.Final)
		}
		out.Cohorts = []RetentionCohort{}
		for _, b := range rg.Buckets() {
			c := RetentionCohort{CohortDate: b.Format(time.DateOnly), Retention: []RetentionPoint{}}
			a := buckets[b]
			if a != nil {
				var size int64
				for _, s := range a.size {
					size = max(size, s)
				}
				if !analytics.Small(size, q.Config.MinCellSize) && size > 0 {
					c.CohortSize = &size
					days := make([]int, 0, len(a.size))
					for d := range a.size {
						days = append(days, d)
					}
					sort.Ints(days)
					for _, d := range days {
						pt := RetentionPoint{DayOffset: d}
						if a.size[d] > 0 && !analytics.Small(a.size[d], q.Config.MinCellSize) {
							v := float64(a.ret[d]) / float64(a.size[d])
							pt.Rate = &v
						}
						c.Retention = append(c.Retention, pt)
					}
				}
			}
			out.Cohorts = append(out.Cohorts, c)
		}
		out.Meta = q.meta(rg, macc, nil)
		out.Meta.ExcludedTenantIDs = excluded
		return nil
	})
	return out, err
}

// ---------- getGameFunnel ----------

// FunnelSteps are the steps of the onboarding funnel (GAME_ANALYTICS §4.2).
var FunnelSteps = map[string][]string{"onboarding": {"first_login", "avatar_created", "tutorial_completed", "first_quest_accepted",
	"first_quest_completed", "chapter0_completed"}}

// FunnelStepView is one step of FunnelReport.
type FunnelStepView struct {
	StepCode       string   `json:"step_code"`
	Count          *int64   `json:"count"`
	ConversionRate *float64 `json:"conversion_rate"`
}

// FunnelView is FunnelReport.
type FunnelView struct {
	FunnelCode string           `json:"funnel_code"`
	Steps      []FunnelStepView `json:"steps"`
	Meta
}

// GetFunnel is getGameFunnel: players of the cohorts of the range reaching each step within 7 days; a step below the
// cell size is hidden (count and conversion null).
func (q GameAnalytics) GetFunnel(ctx context.Context, code string, f AnalyticsFilter) (out FunnelView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameFunnel")
	defer func() { done(err) }()
	ctx, p, err := platformCtx(ctx, authz.PermGameAnalyticsRead, f.TenantIDs)
	if err != nil {
		return out, err
	}
	steps, ok := FunnelSteps[code]
	if !ok {
		return out, problem.New(problem.NotFound, "")
	}
	rg, err := q.dashRange(f.From, f.To, f.Grain)
	if err != nil {
		return out, err
	}
	flt, err := filterOf(f)
	if err != nil {
		return out, err
	}
	err = q.run(ctx, "game_funnel", p.UserID, uuid.Nil, func(ctx context.Context) error {
		tenants, excluded, err := q.scopeTenants(ctx, f.TenantIDs)
		if err != nil {
			return err
		}
		rows, err := q.Model.Funnel(ctx, port.DashboardQuery{Tenants: tenants, From: rg.From, To: rg.To, Grain: rg.Grain}, code, flt.Key, flt.Values)
		if err != nil {
			return err
		}
		var macc metaAcc
		counts := map[int]int64{}
		for _, r := range rows {
			counts[r.Step] += r.Count
			macc.add(r.Computed, r.Final)
		}
		out = FunnelView{FunnelCode: code, Steps: make([]FunnelStepView, 0, len(steps))}
		first := counts[1]
		for i, s := range steps {
			v := FunnelStepView{StepCode: s}
			if n := counts[i+1]; !analytics.Small(n, q.Config.MinCellSize) && !analytics.Small(first, q.Config.MinCellSize) {
				v.Count = &n
				r := float64(n) / float64(first)
				v.ConversionRate = &r
			}
			out.Steps = append(out.Steps, v)
		}
		out.Meta = q.meta(rg, macc, nil)
		out.Meta.ExcludedTenantIDs = excluded
		return nil
	})
	return out, err
}

// ---------- getGameContentStats ----------

// ContentItem is ContentStatsItem (no title: the portal links to the item in workspace A).
type ContentItem struct {
	Kind             string   `json:"kind"`
	ContentID        string   `json:"content_id"`
	Starts           *int64   `json:"starts,omitempty"`
	Completions      *int64   `json:"completions,omitempty"`
	CompletionRate   *float64 `json:"completion_rate"`
	AbandonRate      *float64 `json:"abandon_rate"`
	SuccessRate      *float64 `json:"success_rate"`
	MedianDurationMS *int64   `json:"median_duration_ms"`
	FlaggedForReview bool     `json:"flagged_for_review"`
}

// ContentPage is the getGameContentStats answer.
type ContentPage struct {
	Data []ContentItem `json:"data"`
	Page PageInfo      `json:"page"`
	Meta
}

// ContentSorts are the sort keys of getGameContentStats (descending, keyset with the content id).
var ContentSorts = map[string]bool{"abandon_rate": true, "starts": true, "success_rate": true, "completion_rate": true}

// GetContentStats is getGameContentStats: content of a kind over the weeks of the range, merged across tenants, keyset page
// in the database (LIMIT n+1) on the sort value; a content below the cell size keeps only its id (rates null).
func (q GameAnalytics) GetContentStats(ctx context.Context, kind, sortKey string, limit int, cursor string, f AnalyticsFilter) (out ContentPage, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGameContentStats")
	defer func() { done(err) }()
	ctx, p, err := platformCtx(ctx, authz.PermGameAnalyticsRead, f.TenantIDs)
	if err != nil {
		return out, err
	}
	if kind == "" {
		kind = "quest"
	}
	if sortKey == "" {
		sortKey = "abandon_rate"
	}
	if !ContentSorts[sortKey] {
		return out, problem.Invalid("sort", problem.SubInvalid, "abandon_rate, starts, success_rate or completion_rate")
	}
	if len(f.Schools)+len(f.GradeBands)+len(f.Platforms)+len(f.Locales)+len(f.DeviceTiers)+len(f.ClientVersions) > 0 {
		return out, problem.Invalid("dimension", problem.SubInvalid, "content statistics are not cut by dimension")
	}
	rg, err := q.dashRange(f.From, f.To, f.Grain)
	if err != nil {
		return out, err
	}
	if limit, err = limitOf(limit); err != nil {
		return out, err
	}
	var after *port.ContentCursor
	if cursor != "" {
		if after, err = q.decodeContent(cursor); err != nil {
			return out, err
		}
	}
	err = q.run(ctx, "game_content_stats", p.UserID, uuid.Nil, func(ctx context.Context) error {
		tenants, excluded, err := q.scopeTenants(ctx, f.TenantIDs)
		if err != nil {
			return err
		}
		rows, err := q.Model.Content(ctx, port.DashboardQuery{Tenants: tenants, From: analytics.BucketStart(rg.From, analytics.Week), To: rg.To,
			Grain: analytics.Week}, kind, sortKey, limit+1, after)
		if err != nil {
			return err
		}
		var macc metaAcc
		out.Data = []ContentItem{}
		for i, r := range rows {
			if i == limit {
				break
			}
			macc.add(r.Computed, r.Final)
			out.Data = append(out.Data, q.contentItem(r))
		}
		out.Page = PageInfo{Limit: limit, HasMore: len(rows) > limit}
		if out.Page.HasMore && q.Cursors != nil {
			last := rows[limit-1]
			s := q.encodeContent(port.ContentCursor{Value: last.SortValue, ID: last.ContentID})
			out.Page.NextCursor = &s
		}
		out.Meta = q.meta(rg, macc, nil)
		out.Meta.ExcludedTenantIDs = excluded
		out.Grain = string(analytics.Week)
		return nil
	})
	return out, err
}

func (q GameAnalytics) contentItem(r port.ContentRow) ContentItem {
	it := ContentItem{Kind: r.Kind, ContentID: r.ContentID.String()}
	if analytics.Small(r.Players, q.Config.MinCellSize) {
		return it
	}
	it.Starts, it.Completions = &r.Started, &r.Completed
	ratio := func(n, d int64) *float64 {
		if d <= 0 {
			return nil
		}
		v := float64(n) / float64(d)
		return &v
	}
	it.CompletionRate = ratio(r.Completed, r.Started)
	it.AbandonRate = ratio(r.Abandoned, r.Started)
	if r.Kind != "quest" {
		it.SuccessRate = ratio(r.Succeeded, r.Ended)
	}
	if r.DurationP50 != nil {
		ms := int64(*r.DurationP50 * 1000)
		it.MedianDurationMS = &ms
	}
	it.FlaggedForReview = r.Kind == "quest" && it.AbandonRate != nil && *it.AbandonRate > q.Config.QuestAbandonReview
	return it
}

func (q GameAnalytics) encodeContent(c port.ContentCursor) string {
	return q.Cursors.Encode(page.Cursor{Key: strconv.FormatFloat(c.Value, 'g', -1, 64), ID: c.ID})
}

func (q GameAnalytics) decodeContent(s string) (*port.ContentCursor, error) {
	if q.Cursors == nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "invalid cursor")
	}
	c, err := q.Cursors.Decode(s)
	if err != nil || c == nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "invalid cursor")
	}
	v, err := strconv.ParseFloat(c.Key, 64)
	if err != nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "invalid cursor")
	}
	return &port.ContentCursor{Value: v, ID: c.ID}, nil
}

// ---------- getGamePerformance ----------

// PerfView is one PerformanceReportRow.
type PerfView struct {
	Platform          string   `json:"platform"`
	DeviceTier        string   `json:"device_tier"`
	Preset            string   `json:"preset,omitempty"`
	Renderer          string   `json:"renderer,omitempty"`
	ClientVersion     string   `json:"client_version,omitempty"`
	Sessions          int64    `json:"sessions"`
	FPSP50            *float64 `json:"fps_p50"`
	FPSP10            *float64 `json:"fps_p10"`
	FPSBudgetPassRate *float64 `json:"fps_budget_pass_rate"`
	LoadTimeP50MS     *int64   `json:"load_time_p50_ms"`
	CrashFreeSessions *float64 `json:"crash_free_sessions"`
}

// PerformanceView is the getGamePerformance answer.
type PerformanceView struct {
	Data []PerfView `json:"data"`
	Meta
}

var deviceTiers = map[string]bool{"minimum": true, "mid": true, "high": true}
var presets = map[string]bool{"low": true, "medium": true, "high": true}
var renderers = map[string]bool{"mobile": true, "compatibility": true, "forward_plus": true}

// GetPerformance is getGamePerformance (ADR-0057, AC-ANL-108): real performance per platform × device tier × preset ×
// renderer × client version over the range, merged across tenants; groups below the cell size keep only their keys.
func (q GameAnalytics) GetPerformance(ctx context.Context, f AnalyticsFilter) (out PerformanceView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.GetGamePerformance")
	defer func() { done(err) }()
	ctx, p, err := platformCtx(ctx, authz.PermGameAnalyticsRead, f.TenantIDs)
	if err != nil {
		return out, err
	}
	if len(f.Schools)+len(f.GradeBands)+len(f.Locales) > 0 {
		return out, problem.Invalid("dimension", problem.SubInvalid, "performance is cut by platform, device tier and client version only")
	}
	rg, err := q.dashRange(f.From, f.To, f.Grain)
	if err != nil {
		return out, err
	}
	err = q.run(ctx, "game_performance", p.UserID, uuid.Nil, func(ctx context.Context) error {
		tenants, excluded, err := q.scopeTenants(ctx, f.TenantIDs)
		if err != nil {
			return err
		}
		rows, err := q.Model.Performance(ctx, port.DashboardQuery{Tenants: tenants, From: rg.From, To: rg.To, Grain: analytics.Day},
			port.PerfFilter{Platforms: f.Platforms, DeviceTiers: f.DeviceTiers, ClientVersions: f.ClientVersions})
		if err != nil {
			return err
		}
		var macc metaAcc
		out.Data = []PerfView{}
		for _, r := range rows {
			macc.add(r.Computed, r.Final)
			v := PerfView{Platform: r.Platform, DeviceTier: r.DeviceTier, Sessions: r.Sessions, ClientVersion: r.ClientVersion}
			if !deviceTiers[v.DeviceTier] {
				continue // unknown tier: not representable in the contract (DeviceTier enum)
			}
			if presets[r.Preset] {
				v.Preset = r.Preset
			}
			if renderers[r.Renderer] {
				v.Renderer = r.Renderer
			}
			if !analytics.Small(r.Players, q.Config.MinCellSize) {
				v.FPSP50, v.FPSP10, v.FPSBudgetPassRate, v.CrashFreeSessions = r.FPSP50, r.FPSP10, r.BudgetPass, r.CrashFree
				if r.LoadP50 != nil {
					ms := int64(*r.LoadP50)
					v.LoadTimeP50MS = &ms
				}
			}
			out.Data = append(out.Data, v)
		}
		out.Meta = q.meta(rg, macc, nil)
		out.Meta.ExcludedTenantIDs = excluded
		return nil
	})
	return out, err
}

// cacheKey is the Valkey key of a cached tenant answer (DASHBOARD_DATA §9).
func cacheKey(tenant uuid.UUID, ver int64, queryID string, parts ...any) string {
	b, _ := json.Marshal(append([]any{queryID}, parts...))
	h := sha256.Sum256(b)
	return fmt.Sprintf("ew:game:dash:res:%s:%d:%s", tenant, ver, hex.EncodeToString(h[:16]))
}
