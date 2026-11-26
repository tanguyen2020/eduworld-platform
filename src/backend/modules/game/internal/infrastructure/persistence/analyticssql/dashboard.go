package analyticssql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Dashboard is the query catalog of Game Analytics (F080-AC-11, AC-31; DASHBOARD_DATA §8 rule 6): every statement reads
// analytics.rollup_*, cohort_retention, funnel_daily, content_stats_weekly or perf_daily — never events_raw nor player_days —
// names the tenants explicitly and a bounded range.
type Dashboard struct{}

var _ port.AnalyticsDashboard = Dashboard{}

// Catalog lists the query ids of the catalog with their tables (EXPLAIN / static test, TC-DASH-001).
var Catalog = map[string][]string{
	"rollups":        {"analytics.rollup_daily", "analytics.rollup_weekly", "analytics.rollup_monthly"},
	"retention":      {"analytics.cohort_retention"},
	"funnel":         {"analytics.funnel_daily"},
	"content":        {"analytics.content_stats_weekly"},
	"performance":    {"analytics.perf_daily"},
	"available_from": {"analytics.rollup_daily"},
	"tenants":        {"ref.tenants"},
	"schools":        {"ref.schools"},
}

// MapError is dashErr for test doubles of the catalog (TEST hook).
func MapError(what string, err error) error { return dashErr(what, err) }

// dashErr maps a statement cancelled by statement_timeout to 504 DASHBOARD_QUERY_TIMEOUT (F080-AC-12).
func dashErr(what string, err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "57014" {
		return problem.New(problem.DashboardQueryTimeout, "narrow the range or the scope").Wrap(err)
	}
	return fmt.Errorf("analytics dashboard %s: %w", what, err)
}

// Prepare implements port.AnalyticsDashboard: statement_timeout of the read transaction (dashboard_statement_timeout).
func (Dashboard) Prepare(ctx context.Context, timeout time.Duration) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if timeout <= 0 {
		return nil
	}
	_, err = t.ExecContext(ctx, "SELECT set_config('statement_timeout', $1, true)", strconv.FormatInt(timeout.Milliseconds(), 10))
	return err
}

func ids(xs []uuid.UUID) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.String()
	}
	return out
}

var grainTable = map[analytics.Grain]string{analytics.Day: "rollup_daily", analytics.Week: "rollup_weekly", analytics.Month: "rollup_monthly"}

// Rollups implements port.AnalyticsDashboard.
func (Dashboard) Rollups(ctx context.Context, q port.DashboardQuery, codes []string, dimSet string) ([]port.DashboardRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	table, ok := grainTable[q.Grain]
	if !ok {
		table = "rollup_daily"
	}
	versions := make([]string, len(codes))
	for i, c := range codes {
		m, _ := analytics.Lookup(c)
		versions[i] = strconv.Itoa(max(m.Version, 1))
	}
	// tenancyguard:ok rollup rows (derived, no deleted_at); tenants named explicitly (table from a closed list)
	rows, err := t.QueryContext(ctx, `SELECT metric_date, tenant_id, metric_code, dims::text, coalesce(numerator, 0), coalesce(denominator, 0),
			players, hist::text, is_final, computed_at
		FROM analytics.`+table+` WHERE tenant_id = ANY($1::uuid[]) AND (metric_code, metric_version) IN (SELECT * FROM unnest($2::text[], $3::int[]))
			AND dim_set = $4 AND metric_date BETWEEN $5 AND $6`,
		"{"+strings.Join(ids(q.Tenants), ",")+"}", "{"+strings.Join(codes, ",")+"}", "{"+strings.Join(versions, ",")+"}", dimSet,
		day(q.From), day(q.To))
	if err != nil {
		return nil, dashErr("rollups", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.DashboardRow
	for rows.Next() {
		var r port.DashboardRow
		var dims string
		var hist sql.NullString
		if err := rows.Scan(&r.Date, &r.TenantID, &r.Code, &dims, &r.Cell.Num, &r.Cell.Den, &r.Cell.Players, &hist, &r.Final, &r.Computed); err != nil {
			return nil, dashErr("rollups", err)
		}
		r.Date, r.Dims, r.Cell.Hist = analytics.DateOf(r.Date), parseDims(dims), parseHist(hist)
		out = append(out, r)
	}
	return out, dashErr("rollups", closeRows(rows))
}

// dimFilter is the SQL of the dims values selected (dims->>key = value, or the school|band pair).
func dimFilter(set string, values []string, arg int) (string, []any) {
	if len(values) == 0 {
		return "", nil
	}
	if set == analytics.DimSchool+","+analytics.DimGradeBand {
		return fmt.Sprintf(" AND (dims->>'school_id') || '|' || (dims->>'grade_band') = ANY($%d::text[])", arg), []any{"{" + strings.Join(values, ",") + "}"}
	}
	return fmt.Sprintf(" AND dims->>'%s' = ANY($%d::text[])", strings.ReplaceAll(set, "'", ""), arg), []any{"{" + strings.Join(values, ",") + "}"}
}

var dimSets = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range analytics.DimSets {
		m[analytics.Dims(dimsOf(s)).Set()] = true
	}
	return m
}()

func dimsOf(keys []string) map[string]string {
	m := map[string]string{}
	for _, k := range keys {
		m[k] = "x"
	}
	return m
}

// Retention implements port.AnalyticsDashboard.
func (Dashboard) Retention(ctx context.Context, q port.DashboardQuery, set string, values []string) ([]port.RetentionRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	if !dimSets[set] {
		return nil, fmt.Errorf("analytics: unknown dimension set %q", set)
	}
	where, args := dimFilter(set, values, 5)
	// tenancyguard:ok cohort rows (derived, no deleted_at); tenants named explicitly
	rows, err := t.QueryContext(ctx, `SELECT cohort_date, day_n, sum(cohort_size), sum(retained), bool_and(is_final), min(computed_at)
		FROM analytics.cohort_retention WHERE tenant_id = ANY($1::uuid[]) AND dim_set = $2 AND cohort_date BETWEEN $3 AND $4`+where+`
		GROUP BY cohort_date, day_n ORDER BY cohort_date, day_n`,
		append([]any{"{" + strings.Join(ids(q.Tenants), ",") + "}", set, day(q.From), day(q.To)}, args...)...)
	if err != nil {
		return nil, dashErr("retention", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.RetentionRow
	for rows.Next() {
		var r port.RetentionRow
		if err := rows.Scan(&r.Cohort, &r.DayN, &r.Size, &r.Retained, &r.Final, &r.Computed); err != nil {
			return nil, dashErr("retention", err)
		}
		r.Cohort = analytics.DateOf(r.Cohort)
		out = append(out, r)
	}
	return out, dashErr("retention", closeRows(rows))
}

// Funnel implements port.AnalyticsDashboard.
func (Dashboard) Funnel(ctx context.Context, q port.DashboardQuery, code, set string, values []string) ([]port.FunnelRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	if !dimSets[set] {
		return nil, fmt.Errorf("analytics: unknown dimension set %q", set)
	}
	where, args := dimFilter(set, values, 6)
	// tenancyguard:ok funnel rows (derived, no deleted_at); tenants named explicitly
	rows, err := t.QueryContext(ctx, `SELECT step_no, sum(count), bool_and(is_final), min(computed_at)
		FROM analytics.funnel_daily WHERE tenant_id = ANY($1::uuid[]) AND funnel_code = $2 AND dim_set = $3 AND bucket_date BETWEEN $4 AND $5`+where+`
		GROUP BY step_no ORDER BY step_no`,
		append([]any{"{" + strings.Join(ids(q.Tenants), ",") + "}", code, set, day(q.From), day(q.To)}, args...)...)
	if err != nil {
		return nil, dashErr("funnel", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.FunnelRow
	for rows.Next() {
		var r port.FunnelRow
		if err := rows.Scan(&r.Step, &r.Count, &r.Final, &r.Computed); err != nil {
			return nil, dashErr("funnel", err)
		}
		out = append(out, r)
	}
	return out, dashErr("funnel", closeRows(rows))
}

var contentSort = map[string]string{
	"abandon_rate":    "coalesce(sum(abandoned)::float8 / nullif(sum(started), 0), 0)",
	"starts":          "sum(started)::float8",
	"success_rate":    "coalesce(sum(succeeded)::float8 / nullif(sum(ended), 0), 0)",
	"completion_rate": "coalesce(sum(completed)::float8 / nullif(sum(started), 0), 0)",
}

// Content implements port.AnalyticsDashboard: aggregated per content id, keyset on (sort value, content id) descending,
// LIMIT in the database.
func (Dashboard) Content(ctx context.Context, q port.DashboardQuery, kind, sortKey string, limit int, after *port.ContentCursor) ([]port.ContentRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	expr, ok := contentSort[sortKey]
	if !ok {
		return nil, fmt.Errorf("analytics: unknown content sort %q", sortKey)
	}
	having := ""
	args := []any{"{" + strings.Join(ids(q.Tenants), ",") + "}", kind, day(q.From), day(q.To), limit}
	if after != nil {
		having = " HAVING (" + expr + ", coalesce(content_id, '00000000-0000-0000-0000-000000000000'::uuid)) < ($6, $7)"
		args = append(args, after.Value, after.ID)
	}
	// tenancyguard:ok content rows (derived, no deleted_at); tenants named explicitly (sort expression from a closed list)
	rows, err := t.QueryContext(ctx, `SELECT coalesce(content_id, '00000000-0000-0000-0000-000000000000'::uuid), sum(started), sum(completed),
			sum(abandoned), sum(succeeded), sum(ended), max(players),
			(sum(duration_p50_s::float8 * completed) / nullif(sum(completed) FILTER (WHERE duration_p50_s IS NOT NULL), 0)),
			bool_and(is_final), min(computed_at), `+expr+`
		FROM analytics.content_stats_weekly WHERE tenant_id = ANY($1::uuid[]) AND content_kind = $2 AND week_start BETWEEN $3 AND $4
		GROUP BY 1`+having+` ORDER BY `+expr+` DESC, 1 DESC LIMIT $5`, args...)
	if err != nil {
		return nil, dashErr("content", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.ContentRow
	for rows.Next() {
		r := port.ContentRow{Kind: kind}
		var dur sql.NullFloat64
		if err := rows.Scan(&r.ContentID, &r.Started, &r.Completed, &r.Abandoned, &r.Succeeded, &r.Ended, &r.Players, &dur, &r.Final, &r.Computed,
			&r.SortValue); err != nil {
			return nil, dashErr("content", err)
		}
		if dur.Valid {
			v := dur.Float64
			r.DurationP50 = &v
		}
		out = append(out, r)
	}
	return out, dashErr("content", closeRows(rows))
}

// Performance implements port.AnalyticsDashboard: weighted by samples / sessions over the days and tenants.
func (Dashboard) Performance(ctx context.Context, q port.DashboardQuery, f port.PerfFilter) ([]port.PerfRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	arr := func(xs []string) string { return "{" + strings.Join(xs, ",") + "}" }
	// tenancyguard:ok perf rows (derived, no deleted_at); tenants named explicitly
	rows, err := t.QueryContext(ctx, `SELECT platform, device_tier, preset, renderer, client_version, sum(samples), sum(sessions), max(players),
			sum(fps_p50 * samples) / nullif(sum(samples) FILTER (WHERE fps_p50 IS NOT NULL), 0),
			sum(fps_p10 * samples) / nullif(sum(samples) FILTER (WHERE fps_p10 IS NOT NULL), 0),
			sum(budget_pass_rate * samples) / nullif(sum(samples) FILTER (WHERE budget_pass_rate IS NOT NULL), 0),
			avg(load_cold_p50_ms), sum(crash_free_rate * sessions) / nullif(sum(sessions) FILTER (WHERE crash_free_rate IS NOT NULL), 0),
			bool_and(is_final), min(computed_at)
		FROM analytics.perf_daily WHERE tenant_id = ANY($1::uuid[]) AND bucket_date BETWEEN $2 AND $3
			AND (cardinality($4::text[]) = 0 OR platform = ANY($4::text[])) AND (cardinality($5::text[]) = 0 OR device_tier = ANY($5::text[]))
			AND (cardinality($6::text[]) = 0 OR client_version = ANY($6::text[]))
		GROUP BY platform, device_tier, preset, renderer, client_version ORDER BY sum(sessions) DESC, 1, 2, 3, 4, 5 LIMIT 1000`,
		"{"+strings.Join(ids(q.Tenants), ",")+"}", day(q.From), day(q.To), arr(f.Platforms), arr(f.DeviceTiers), arr(f.ClientVersions))
	if err != nil {
		return nil, dashErr("performance", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.PerfRow
	for rows.Next() {
		var r port.PerfRow
		var p50, p10, pass, load, crash sql.NullFloat64
		if err := rows.Scan(&r.Platform, &r.DeviceTier, &r.Preset, &r.Renderer, &r.ClientVersion, &r.Samples, &r.Sessions, &r.Players,
			&p50, &p10, &pass, &load, &crash, &r.Final, &r.Computed); err != nil {
			return nil, dashErr("performance", err)
		}
		r.FPSP50, r.FPSP10, r.BudgetPass, r.LoadP50, r.CrashFree = nf(p50), nf(p10), nf(pass), nf(load), nf(crash)
		out = append(out, r)
	}
	return out, dashErr("performance", closeRows(rows))
}

func nf(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	x := v.Float64
	return &x
}

// AvailableFrom implements port.AnalyticsDashboard.
func (Dashboard) AvailableFrom(ctx context.Context, tenants []uuid.UUID) (*time.Time, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var d sql.NullTime
	// tenancyguard:ok rollup rows (derived, no deleted_at); tenants named explicitly
	if err := t.QueryRowContext(ctx, `SELECT min(metric_date) FROM analytics.rollup_daily WHERE tenant_id = ANY($1::uuid[]) AND metric_code = 'dau'
		AND dim_set = ''`, "{"+strings.Join(ids(tenants), ",")+"}").Scan(&d); err != nil {
		return nil, dashErr("available_from", err)
	}
	if !d.Valid {
		return nil, nil
	}
	v := analytics.DateOf(d.Time)
	return &v, nil
}

// SchoolsInTenant implements port.AnalyticsDashboard.
func (Dashboard) SchoolsInTenant(ctx context.Context, tenant uuid.UUID, schools []uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok school projection of the tenant (ref.schools has no deleted_at; status kept)
	rows, err := t.QueryContext(ctx, `SELECT school_id FROM ref.schools WHERE tenant_id = $1 AND school_id = ANY($2::uuid[])`,
		tenant, "{"+strings.Join(ids(schools), ",")+"}")
	if err != nil {
		return nil, dashErr("schools", err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, dashErr("schools", closeRows(rows))
}

// NonQATenants implements port.AnalyticsDashboard (GAME_ANALYTICS A-8: QA tenants never counted, even when named).
func (Dashboard) NonQATenants(ctx context.Context, tenants []uuid.UUID) ([]uuid.UUID, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok tenant registry projection (G, no deleted_at)
	rows, err := t.QueryContext(ctx, `SELECT tenant_id FROM ref.tenants WHERE NOT is_qa AND (cardinality($1::uuid[]) = 0 OR tenant_id = ANY($1::uuid[]))
		ORDER BY tenant_id`, "{"+strings.Join(ids(tenants), ",")+"}")
	if err != nil {
		return nil, dashErr("tenants", err)
	}
	defer func() { _ = rows.Close() }()
	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := closeRows(rows); err != nil {
		return nil, dashErr("tenants", err)
	}
	if len(tenants) > 0 && len(out) == 0 {
		return []uuid.UUID{uuid.Nil}, nil // only QA tenants named: an empty answer, never every tenant
	}
	return out, nil
}
