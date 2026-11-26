package analyticssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
)

var rollupTables = map[string]bool{"rollup_daily": true, "rollup_weekly": true, "rollup_monthly": true}

type rollupJSON struct {
	Date    string          `json:"metric_date"`
	Code    string          `json:"metric_code"`
	Version int             `json:"metric_version"`
	Hash    string          `json:"dims_hash"`
	Set     string          `json:"dim_set"`
	Dims    analytics.Dims  `json:"dims"`
	Value   *float64        `json:"value"`
	Num     int64           `json:"numerator"`
	Den     int64           `json:"denominator"`
	Players int64           `json:"players"`
	Hist    json.RawMessage `json:"hist"`
	Final   bool            `json:"is_final"`
}

// UpsertRollups implements port.AnalyticsStore (one statement per table and tenant).
func (Store) UpsertRollups(ctx context.Context, table string, tenant uuid.UUID, rows []port.RollupRow) error {
	if !rollupTables[table] {
		return fmt.Errorf("analytics: unknown rollup table %q", table)
	}
	if len(rows) == 0 {
		return nil
	}
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	in := make([]rollupJSON, 0, len(rows))
	for _, r := range rows {
		dims := r.Dims
		if dims == nil {
			dims = analytics.Dims{}
		}
		var hist json.RawMessage
		if len(r.Cell.Hist) > 0 {
			hist, _ = json.Marshal(r.Cell.Hist)
		}
		v := r.Value
		if v != nil && (*v != *v) { // NaN guard
			v = nil
		}
		in = append(in, rollupJSON{Date: day(r.Date), Code: r.Code, Version: max(r.Version, 1), Hash: dims.Hash(), Set: dims.Set(), Dims: dims,
			Value: v, Num: r.Cell.Num, Den: r.Cell.Den, Players: r.Cell.Players, Hist: hist, Final: r.Final})
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named (table from a closed list)
	_, err = t.ExecContext(ctx, `INSERT INTO analytics.`+table+` (metric_date, metric_code, metric_version, tenant_id, dims_hash, dim_set, dims,
			value, numerator, denominator, players, hist, is_final, computed_at)
		SELECT x.metric_date, x.metric_code, x.metric_version, $1, x.dims_hash, x.dim_set, x.dims, x.value, x.numerator, x.denominator,
			least(x.players, 2147483647), x.hist, x.is_final, now()
		FROM jsonb_to_recordset($2::jsonb) AS x(metric_date date, metric_code varchar, metric_version smallint, dims_hash varchar,
			dim_set varchar, dims jsonb, value numeric, numerator bigint, denominator bigint, players bigint, hist jsonb, is_final boolean)
		ON CONFLICT (metric_date, metric_code, metric_version, tenant_id, dims_hash) DO UPDATE SET dims = excluded.dims, dim_set = excluded.dim_set,
			value = excluded.value, numerator = excluded.numerator, denominator = excluded.denominator, players = excluded.players,
			hist = excluded.hist, is_final = excluded.is_final, computed_at = excluded.computed_at`, tenant, string(b))
	if err != nil {
		return fmt.Errorf("analytics upsert %s: %w", table, err)
	}
	return nil
}

// ReadDaily implements port.AnalyticsStore.
func (Store) ReadDaily(ctx context.Context, tenant uuid.UUID, from, to time.Time) ([]port.RollupRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	rows, err := t.QueryContext(ctx, `SELECT metric_date, metric_code, dims::text, coalesce(numerator, 0), coalesce(denominator, 0), players, hist::text
		FROM analytics.rollup_daily WHERE tenant_id = $1 AND metric_date BETWEEN $2 AND $3`, tenant, day(from), day(to))
	if err != nil {
		return nil, fmt.Errorf("analytics read daily: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.RollupRow
	for rows.Next() {
		var r port.RollupRow
		var dims string
		var hist sql.NullString
		if err := rows.Scan(&r.Date, &r.Code, &dims, &r.Cell.Num, &r.Cell.Den, &r.Cell.Players, &hist); err != nil {
			return nil, err
		}
		r.Date, r.Dims, r.Cell.Hist = analytics.DateOf(r.Date), parseDims(dims), parseHist(hist)
		out = append(out, r)
	}
	return out, rows.Err()
}

// daysBounds are the histogram bounds of active_days_per_week (1–7 days).
var daysBounds = []float64{1, 2, 3, 4, 5, 6, 7}

// PeriodRows implements port.AnalyticsStore: per player of [from, to] (dims of its latest day), the period-only metrics and
// the distinct active players of every dims value; active_days_per_week counts player-weeks with ≥ 3 active days.
func (Store) PeriodRows(ctx context.Context, tenant uuid.UUID, from, to time.Time) ([]port.RollupRow, map[string]int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	base := `WITH s0 AS (SELECT * FROM analytics.player_days WHERE tenant_id = $1 AND activity_date BETWEEN $2 AND $3 AND player_key IS NOT NULL),
		p AS (SELECT player_key, count(*) FILTER (WHERE sessions > 0) AS days, bool_or(social) AS soc, sum(matches) > 0 AS m, sum(purchases) > 0 AS pu,
				(array_agg(school_id ORDER BY activity_date DESC))[1] AS school_id, (array_agg(grade_level ORDER BY activity_date DESC))[1] AS grade_level,
				(array_agg(platform ORDER BY activity_date DESC))[1] AS platform, (array_agg(locale ORDER BY activity_date DESC))[1] AS locale,
				(array_agg(device_tier ORDER BY activity_date DESC))[1] AS device_tier
			FROM s0 GROUP BY player_key HAVING count(*) FILTER (WHERE sessions > 0) > 0),
		x AS (SELECT s.*, ` + dimCols + ` FROM p s)`
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	rows, err := t.QueryContext(ctx, base+` SELECT `+dimsSelect+`, count(*), count(*) FILTER (WHERE soc), count(*) FILTER (WHERE m),
		count(*) FILTER (WHERE pu) FROM x GROUP BY `+groupingSets, tenant, day(from), day(to))
	if err != nil {
		return nil, nil, fmt.Errorf("analytics period rows: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.RollupRow
	players := map[string]int64{}
	for rows.Next() {
		var dims string
		var n, soc, m, pu int64
		if err := rows.Scan(&dims, &n, &soc, &m, &pu); err != nil {
			return nil, nil, err
		}
		dm := parseDims(dims)
		players[dm.Key()] = n
		out = append(out, port.RollupRow{Code: "social_play_rate", Dims: dm, Cell: analytics.Cell{Num: soc, Den: n, Players: n}},
			port.RollupRow{Code: "match_participation_rate", Dims: dm, Cell: analytics.Cell{Num: m, Den: n, Players: n}},
			port.RollupRow{Code: "shop_conversion", Dims: dm, Cell: analytics.Cell{Num: pu, Den: n, Players: n}})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	rs1, rerr1 := t.QueryContext(ctx, base+`, pw AS (SELECT player_key, date_trunc('week', activity_date)::date AS wk, count(*) FILTER (WHERE sessions > 0) AS days
			FROM s0 GROUP BY 1, 2 HAVING count(*) FILTER (WHERE sessions > 0) > 0),
		y AS (SELECT x.*, pw.days AS wdays FROM x JOIN pw USING (player_key))
		SELECT `+dimsSelect+`, count(*) FILTER (WHERE wdays >= 3), count(*), `+histExpr("wdays", "true", daysBounds)+`
		FROM y GROUP BY `+groupingSets, tenant, day(from), day(to))
	if err := each(rs1, rerr1, func(rows *sql.Rows) error {
		var dims string
		var three, weeks int64
		var h sql.NullString
		if err := rows.Scan(&dims, &three, &weeks, &h); err != nil {
			return err
		}
		dm := parseDims(dims)
		out = append(out, port.RollupRow{Code: "active_days_per_week", Dims: dm, Cell: analytics.Cell{Num: three, Den: weeks, Players: players[dm.Key()],
			Hist: parseHist(h)}})
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("analytics active days: %w", err)
	}
	return out, players, nil
}

type cohortJSON struct {
	Date     string         `json:"cohort_date"`
	Hash     string         `json:"dims_hash"`
	Set      string         `json:"dim_set"`
	Dims     analytics.Dims `json:"dims"`
	DayN     int            `json:"day_n"`
	Size     int64          `json:"cohort_size"`
	Retained int64          `json:"retained"`
	Final    bool           `json:"is_final"`
}

// Cohorts implements port.AnalyticsStore: for every N, the cohort of day D-N (players whose first active day it is) and
// those of them active on D (retention_dN, AC-ANL-102).
func (Store) Cohorts(ctx context.Context, d port.TenantDay, dayN []int, final bool) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	ns := make([]string, len(dayN))
	for i, n := range dayN {
		ns[i] = strconv.Itoa(n)
	}
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	rows, err := t.QueryContext(ctx, `WITH r AS (SELECT player_key FROM analytics.player_days WHERE tenant_id = $1 AND activity_date = $2 AND sessions > 0
			AND player_key IS NOT NULL),
		c AS (SELECT s.player_key, n.n, `+dimCols+` FROM analytics.player_days s JOIN unnest($3::int[]) AS n(n) ON s.activity_date = $2::date - n.n
			WHERE s.tenant_id = $1 AND s.first_active_date = s.activity_date AND s.player_key IS NOT NULL)
		SELECT n, `+dimsSelect+`, count(*), count(r.player_key) FROM c LEFT JOIN r USING (player_key)
		GROUP BY n, `+groupingSets, d.TenantID, day(d.Day), "{"+joinComma(ns)+"}")
	if err != nil {
		return fmt.Errorf("analytics cohorts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var in []cohortJSON
	for rows.Next() {
		var n int
		var dims string
		var size, kept int64
		if err := rows.Scan(&n, &dims, &size, &kept); err != nil {
			return err
		}
		dm := parseDims(dims)
		in = append(in, cohortJSON{Date: day(d.Day.AddDate(0, 0, -n)), Hash: dm.Hash(), Set: dm.Set(), Dims: dm, DayN: n, Size: size, Retained: kept, Final: final})
	}
	if err := rows.Err(); err != nil || len(in) == 0 {
		return err
	}
	b, _ := json.Marshal(in)
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `INSERT INTO analytics.cohort_retention (cohort_date, tenant_id, dims_hash, dim_set, dims, day_n, cohort_size, retained,
			is_final, computed_at)
		SELECT x.cohort_date, $1, x.dims_hash, x.dim_set, x.dims, x.day_n, x.cohort_size, x.retained, x.is_final, now()
		FROM jsonb_to_recordset($2::jsonb) AS x(cohort_date date, dims_hash varchar, dim_set varchar, dims jsonb, day_n smallint, cohort_size int,
			retained int, is_final boolean)
		ON CONFLICT (cohort_date, tenant_id, dims_hash, day_n) DO UPDATE SET cohort_size = excluded.cohort_size, retained = excluded.retained,
			dims = excluded.dims, dim_set = excluded.dim_set, is_final = excluded.is_final, computed_at = excluded.computed_at`, d.TenantID, string(b))
	if err != nil {
		return fmt.Errorf("analytics upsert cohorts: %w", err)
	}
	return nil
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

type funnelJSON struct {
	Date  string         `json:"bucket_date"`
	Step  int            `json:"step_no"`
	Hash  string         `json:"dims_hash"`
	Set   string         `json:"dim_set"`
	Dims  analytics.Dims `json:"dims"`
	Count int64          `json:"count"`
	Final bool           `json:"is_final"`
}

// Funnels implements port.AnalyticsStore: the onboarding funnel of the cohorts of the last 7 days (steps within 7 days of
// the first active day) and chapter0_completion of each cohort (returned as rollup rows dated by the cohort day).
func (Store) Funnels(ctx context.Context, d port.TenantDay, final bool) ([]port.RollupRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok derived and append-only pseudonymous rows of one tenant (no deleted_at); tenant named
	rows, err := t.QueryContext(ctx, `WITH c AS (SELECT s.player_key, s.activity_date AS cohort, `+dimCols+` FROM analytics.player_days s
			WHERE s.tenant_id = $1 AND s.activity_date BETWEEN $2::date - 6 AND $2::date AND s.first_active_date = s.activity_date
			AND s.player_key IS NOT NULL),
		e AS (SELECT player_key, occurred_on, event_name, props FROM analytics.events_raw WHERE tenant_id = $1
			AND occurred_on BETWEEN $2::date - 6 AND $2::date AND player_key IS NOT NULL
			AND event_name IN ('avatar_create', 'tutorial_step', 'quest_accept', 'quest_complete', 'story_chapter_complete')),
		f AS (SELECT c.player_key, c.cohort, c.d_school, c.d_band, c.d_platform, c.d_locale, c.d_tier,
				coalesce(bool_or(e.event_name = 'avatar_create'), false) AS s2,
				coalesce(bool_or(e.event_name = 'tutorial_step' AND e.props->>'result' = 'done'), false) AS s3,
				coalesce(bool_or(e.event_name = 'quest_accept'), false) AS s4,
				coalesce(bool_or(e.event_name = 'quest_complete'), false) AS s5,
				coalesce(bool_or(e.event_name = 'story_chapter_complete' AND (e.props->>'order' = '0'
					OR e.props->>'chapter_code' LIKE 'chapter.0.%')), false) AS s6
			FROM c LEFT JOIN e ON e.player_key = c.player_key AND e.occurred_on BETWEEN c.cohort AND c.cohort + 6
			GROUP BY c.player_key, c.cohort, c.d_school, c.d_band, c.d_platform, c.d_locale, c.d_tier)
		SELECT cohort, `+dimsSelect+`, count(*), count(*) FILTER (WHERE s2), count(*) FILTER (WHERE s3), count(*) FILTER (WHERE s4),
			count(*) FILTER (WHERE s5), count(*) FILTER (WHERE s6)
		FROM f GROUP BY cohort, `+groupingSets, d.TenantID, day(d.Day))
	if err != nil {
		return nil, fmt.Errorf("analytics funnel: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var in []funnelJSON
	var out []port.RollupRow
	for rows.Next() {
		var cohort time.Time
		var dims string
		var steps [6]int64
		if err := rows.Scan(&cohort, &dims, &steps[0], &steps[1], &steps[2], &steps[3], &steps[4], &steps[5]); err != nil {
			return nil, err
		}
		cohort = analytics.DateOf(cohort)
		dm := parseDims(dims)
		for i, n := range steps {
			in = append(in, funnelJSON{Date: day(cohort), Step: i + 1, Hash: dm.Hash(), Set: dm.Set(), Dims: dm, Count: n, Final: final})
		}
		out = append(out, port.RollupRow{Date: cohort, Code: "chapter0_completion", Dims: dm, Cell: analytics.Cell{Num: steps[5], Den: steps[0], Players: steps[0]}})
	}
	if err := rows.Err(); err != nil || len(in) == 0 {
		return out, err
	}
	b, _ := json.Marshal(in)
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `INSERT INTO analytics.funnel_daily (bucket_date, funnel_code, step_no, tenant_id, dims_hash, dim_set, dims, count,
			is_final, computed_at)
		SELECT x.bucket_date, 'onboarding', x.step_no, $1, x.dims_hash, x.dim_set, x.dims, x.count, x.is_final, now()
		FROM jsonb_to_recordset($2::jsonb) AS x(bucket_date date, step_no smallint, dims_hash varchar, dim_set varchar, dims jsonb, count int, is_final boolean)
		ON CONFLICT (bucket_date, funnel_code, step_no, tenant_id, dims_hash) DO UPDATE SET count = excluded.count, dims = excluded.dims,
			dim_set = excluded.dim_set, is_final = excluded.is_final, computed_at = excluded.computed_at`, d.TenantID, string(b))
	if err != nil {
		return nil, fmt.Errorf("analytics upsert funnel: %w", err)
	}
	return out, nil
}

const uuidRe = `'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'`

// ContentWeek implements port.AnalyticsStore: quest / challenge / boss versions of the ISO week up to the day.
func (Store) ContentWeek(ctx context.Context, tenant uuid.UUID, weekStart, upTo time.Time, final bool) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only pseudonymous rows and derived rows of one tenant (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `WITH e AS (
			SELECT 'quest' AS kind, props->>'quest_version_id' AS v, props->>'quest_id' AS cid, event_name, player_key, props
			FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on BETWEEN $2 AND $3
				AND event_name IN ('quest_accept', 'quest_complete', 'quest_abandon') AND props->>'quest_version_id' ~ `+uuidRe+`
			UNION ALL
			SELECT 'challenge', props->>'challenge_version_id', props->>'challenge_id', event_name, player_key, props
			FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on BETWEEN $2 AND $3
				AND event_name IN ('challenge_start', 'challenge_result') AND props->>'challenge_version_id' ~ `+uuidRe+`
			UNION ALL
			SELECT 'boss', props->>'boss_version_id', props->>'boss_id', event_name, player_key, props
			FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on BETWEEN $2 AND $3
				AND event_name IN ('boss_start', 'boss_result') AND props->>'boss_version_id' ~ `+uuidRe+`)
		INSERT INTO analytics.content_stats_weekly (week_start, tenant_id, content_kind, content_version_id, content_id, started, completed,
			abandoned, succeeded, ended, players, duration_p50_s, is_final, computed_at)
		SELECT $2::date, $1, kind, v::uuid, (max(cid) FILTER (WHERE cid ~ `+uuidRe+`))::uuid,
			count(*) FILTER (WHERE event_name IN ('quest_accept', 'challenge_start', 'boss_start')),
			count(*) FILTER (WHERE event_name IN ('quest_complete', 'challenge_result', 'boss_result')),
			count(*) FILTER (WHERE event_name = 'quest_abandon' OR (event_name = 'challenge_result' AND props->>'status' = 'abandoned')),
			count(*) FILTER (WHERE (event_name = 'challenge_result' AND props->>'status' = 'succeeded')
				OR (event_name = 'boss_result' AND props->>'result' = 'defeated')),
			count(*) FILTER (WHERE event_name IN ('challenge_result', 'boss_result')),
			count(DISTINCT player_key),
			(percentile_cont(0.5) WITHIN GROUP (ORDER BY (props->>'duration_s')::numeric)
				FILTER (WHERE event_name IN ('quest_complete', 'challenge_result') AND props ? 'duration_s'))::int,
			$4, now()
		FROM e GROUP BY kind, v
		ON CONFLICT (week_start, tenant_id, content_kind, content_version_id) DO UPDATE SET content_id = excluded.content_id,
			started = excluded.started, completed = excluded.completed, abandoned = excluded.abandoned, succeeded = excluded.succeeded,
			ended = excluded.ended, players = excluded.players, duration_p50_s = excluded.duration_p50_s, is_final = excluded.is_final,
			computed_at = excluded.computed_at`, tenant, day(weekStart), day(upTo), final)
	if err != nil {
		return fmt.Errorf("analytics content week: %w", err)
	}
	return nil
}

// PerfDay implements port.AnalyticsStore (GAME_ANALYTICS §4.7): FPS medians, budget pass rate (30 FPS mobile/web and preset
// low, 60 FPS desktop — ADR-0057), load times and crash-free sessions per platform × tier × preset × renderer × version.
func (Store) PerfDay(ctx context.Context, d port.TenantDay, final bool) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok append-only pseudonymous rows and derived rows of one tenant day (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `WITH e AS (
			SELECT coalesce(platform, 'unknown') AS pl, coalesce(device_tier, 'unknown') AS dt, coalesce(props->>'preset', 'unknown') AS pr,
				coalesce(props->>'renderer', 'unknown') AS rd, coalesce(client_version, 'unknown') AS cv, event_name, props, session_key, player_key
			FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on = $2
				AND event_name IN ('perf_sample', 'load_time', 'crash', 'client_error'))
		INSERT INTO analytics.perf_daily (bucket_date, tenant_id, platform, device_tier, preset, renderer, client_version, samples, sessions, players,
			fps_p50, fps_p10, budget_pass_rate, load_cold_p50_ms, load_cold_p90_ms, load_area_p90_ms, crash_free_rate, is_final, computed_at)
		SELECT $2::date, $1, left(pl, 16), left(dt, 8), left(pr, 8), left(rd, 16), left(cv, 32),
			count(*) FILTER (WHERE event_name = 'perf_sample'), count(DISTINCT session_key), count(DISTINCT player_key),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY (props->>'fps_p50')::numeric) FILTER (WHERE event_name = 'perf_sample'),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY (props->>'fps_p10')::numeric) FILTER (WHERE event_name = 'perf_sample'),
			avg(CASE WHEN (props->>'fps_p50')::numeric >= CASE WHEN pr = 'low' OR pl IN ('android', 'ios', 'web') THEN 30 ELSE 60 END
				THEN 1 ELSE 0 END) FILTER (WHERE event_name = 'perf_sample'),
			percentile_cont(0.5) WITHIN GROUP (ORDER BY (props->>'ms')::numeric) FILTER (WHERE event_name = 'load_time' AND props->>'kind' = 'cold_start'),
			percentile_cont(0.9) WITHIN GROUP (ORDER BY (props->>'ms')::numeric) FILTER (WHERE event_name = 'load_time' AND props->>'kind' = 'cold_start'),
			percentile_cont(0.9) WITHIN GROUP (ORDER BY (props->>'ms')::numeric) FILTER (WHERE event_name = 'load_time' AND props->>'kind' = 'area_transition'),
			1 - count(DISTINCT session_key) FILTER (WHERE event_name = 'crash')::numeric / nullif(count(DISTINCT session_key), 0),
			$3, now()
		FROM e GROUP BY pl, dt, pr, rd, cv
		ON CONFLICT (bucket_date, tenant_id, platform, device_tier, preset, renderer, client_version) DO UPDATE SET samples = excluded.samples,
			sessions = excluded.sessions, players = excluded.players, fps_p50 = excluded.fps_p50, fps_p10 = excluded.fps_p10,
			budget_pass_rate = excluded.budget_pass_rate, load_cold_p50_ms = excluded.load_cold_p50_ms, load_cold_p90_ms = excluded.load_cold_p90_ms,
			load_area_p90_ms = excluded.load_area_p90_ms, crash_free_rate = excluded.crash_free_rate, is_final = excluded.is_final,
			computed_at = excluded.computed_at`, d.TenantID, day(d.Day), final)
	if err != nil {
		return fmt.Errorf("analytics perf day: %w", err)
	}
	return nil
}

// CloseDay implements port.AnalyticsStore: the closing state of the tenant day (final clears the reclose mark).
func (Store) CloseDay(ctx context.Context, d port.TenantDay, final bool, at time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived closing state of one tenant day (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `INSERT INTO analytics.rollup_days (tenant_id, bucket_date, is_final, needs_reclose, closed_at, computed_at)
		VALUES ($1, $2, $3, false, CASE WHEN $3 THEN $4::timestamptz END, $4)
		ON CONFLICT (tenant_id, bucket_date) DO UPDATE SET is_final = analytics.rollup_days.is_final OR excluded.is_final,
			needs_reclose = CASE WHEN excluded.is_final THEN false ELSE analytics.rollup_days.needs_reclose END,
			closed_at = coalesce(excluded.closed_at, analytics.rollup_days.closed_at), computed_at = excluded.computed_at`,
		d.TenantID, day(d.Day), final, at)
	if err != nil {
		return fmt.Errorf("analytics close day: %w", err)
	}
	return nil
}
