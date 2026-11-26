// Package analyticssql is the PostgreSQL side of Game Analytics (F080, DATABASE_DESIGN §2.8b, DASHBOARD_DATA §3–§10): raw
// ingestion, anonymisation, the rollup steps of the jobs (Store), the dashboard query catalog (Dashboard) and the export
// jobs (Exports), on the transaction of ctx. The analytics tables are derived and pseudonymous (no deleted_at, DASHBOARD_DATA
// §3.1): every statement names its tenant(s) explicitly; RLS applies to game_app / game_dashboard on top.
package analyticssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Store implements port.AnalyticsStore.
type Store struct{}

var _ port.AnalyticsStore = Store{}

func tx(ctx context.Context) (*sql.Tx, error) {
	t, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	return t, nil
}

func day(t time.Time) string { return t.Format(time.DateOnly) }

// TenantFlags implements port.AnalyticsStore (ref.tenants: is_qa, settings_subset.analytics_product_telemetry_enabled).
func (Store) TenantFlags(ctx context.Context, tenant uuid.UUID) (port.TenantFlags, error) {
	t, err := tx(ctx)
	if err != nil {
		return port.TenantFlags{}, err
	}
	var qa bool
	var product sql.NullBool
	// tenancyguard:ok ref.tenants is the tenant registry projection (G, no deleted_at)
	err = t.QueryRowContext(ctx, `SELECT is_qa, (settings_subset->>'analytics_product_telemetry_enabled')::boolean
		FROM ref.tenants WHERE tenant_id = $1`, tenant).Scan(&qa, &product)
	if errors.Is(err, sql.ErrNoRows) {
		return port.TenantFlags{ProductTelemetry: true}, nil
	}
	if err != nil {
		return port.TenantFlags{}, fmt.Errorf("analytics tenant flags: %w", err)
	}
	return port.TenantFlags{Known: true, QA: qa, ProductTelemetry: !product.Valid || product.Bool}, nil
}

// StudentDims implements port.AnalyticsStore: school and grade from ref.students, the active (else latest) game session.
func (Store) StudentDims(ctx context.Context, tenant, student uuid.UUID) (port.StudentDims, error) {
	t, err := tx(ctx)
	if err != nil {
		return port.StudentDims{}, err
	}
	var out port.StudentDims
	var school uuid.NullUUID
	var grade sql.NullInt64
	// tenancyguard:ok projection row of the tenant, deleted students included (their events stay pseudonymous until anonymised)
	err = t.QueryRowContext(ctx, `SELECT school_id, grade_level FROM ref.students WHERE tenant_id = $1 AND student_id = $2`, tenant, student).Scan(&school, &grade)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("analytics student dims: %w", err)
	}
	out.Known = true
	if school.Valid {
		s := school.UUID
		out.SchoolID = &s
	}
	if grade.Valid {
		g := int(grade.Int64)
		out.GradeLevel = &g
	}
	var sess uuid.UUID
	var platform, version string
	err = t.QueryRowContext(ctx, `SELECT game_session_id, client_platform, client_version FROM game.game_sessions
		WHERE tenant_id = $1 AND student_id = $2 AND deleted_at IS NULL ORDER BY (ended_at IS NULL) DESC, started_at DESC LIMIT 1`,
		tenant, student).Scan(&sess, &platform, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("analytics student session: %w", err)
	}
	out.SessionID, out.Platform, out.ClientVersion = &sess, platform, version
	return out, nil
}

// MatchStart implements port.AnalyticsStore.
func (Store) MatchStart(ctx context.Context, tenant uuid.UUID, matchID string, end time.Time) (time.Time, bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return time.Time{}, false, err
	}
	var at time.Time
	// tenancyguard:ok analytics.events_raw is append-only and pseudonymous (no deleted_at); tenant named
	err = t.QueryRowContext(ctx, `SELECT occurred_at FROM analytics.events_raw WHERE tenant_id = $1 AND event_name = 'match_start'
		AND occurred_on BETWEEN $2::date - 2 AND $2::date AND props->>'match_id' = $3 ORDER BY occurred_at LIMIT 1`,
		tenant, day(analytics.DayOf(end)), matchID).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return at, false, nil
	}
	return at, err == nil, err
}

// DayFinal implements port.AnalyticsStore.
func (Store) DayFinal(ctx context.Context, tenant uuid.UUID, d time.Time) (bool, error) {
	t, err := tx(ctx)
	if err != nil {
		return false, err
	}
	var final bool
	// tenancyguard:ok derived closing state (no deleted_at); tenant named
	err = t.QueryRowContext(ctx, `SELECT is_final FROM analytics.rollup_days WHERE tenant_id = $1 AND bucket_date = $2`, tenant, day(d)).Scan(&final)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return final, err
}

// MarkReclose implements port.AnalyticsStore.
func (Store) MarkReclose(ctx context.Context, tenant uuid.UUID, d time.Time) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived closing state (no deleted_at); tenant named
	_, err = t.ExecContext(ctx, `UPDATE analytics.rollup_days SET needs_reclose = true WHERE tenant_id = $1 AND bucket_date = $2`, tenant, day(d))
	return err
}

type rawJSON struct {
	OccurredOn    string         `json:"occurred_on"`
	EventID       uuid.UUID      `json:"event_id"`
	OccurredAt    time.Time      `json:"occurred_at"`
	TenantID      uuid.UUID      `json:"tenant_id"`
	SchoolID      *uuid.UUID     `json:"school_id"`
	GradeLevel    *int           `json:"grade_level"`
	Platform      *string        `json:"platform"`
	Locale        *string        `json:"locale"`
	DeviceTier    *string        `json:"device_tier"`
	ClientVersion *string        `json:"client_version"`
	PlayerKey     *string        `json:"player_key"`
	SessionKey    *string        `json:"session_key"`
	KeyVersion    int            `json:"key_version"`
	EventName     string         `json:"event_name"`
	Source        string         `json:"source"`
	Props         map[string]any `json:"props"`
}

func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// InsertRaw implements port.AnalyticsStore: one statement, ON CONFLICT DO NOTHING on (occurred_on, event_id).
func (Store) InsertRaw(ctx context.Context, rows []port.RawEvent) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	in := make([]rawJSON, len(rows))
	for i, r := range rows {
		props := r.Props
		if props == nil {
			props = map[string]any{}
		}
		in[i] = rawJSON{OccurredOn: day(analytics.DayOf(r.OccurredAt)), EventID: r.EventID, OccurredAt: r.OccurredAt.UTC(), TenantID: r.TenantID,
			SchoolID: r.SchoolID, GradeLevel: r.GradeLevel, Platform: opt(r.Platform), Locale: opt(r.Locale), DeviceTier: opt(r.DeviceTier),
			ClientVersion: opt(r.ClientVersion), PlayerKey: opt(r.PlayerKey), SessionKey: opt(r.SessionKey), KeyVersion: max(r.KeyVersion, 1),
			EventName: r.EventName, Source: r.Source, Props: props}
	}
	b, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	res, err := t.ExecContext(ctx, `INSERT INTO analytics.events_raw (occurred_on, event_id, occurred_at, tenant_id, school_id, grade_level,
		platform, locale, device_tier, client_version, player_key, session_key, key_version, event_name, source, props)
		SELECT x.occurred_on, x.event_id, x.occurred_at, x.tenant_id, x.school_id, x.grade_level, x.platform, x.locale, x.device_tier,
			x.client_version, x.player_key, x.session_key, x.key_version, x.event_name, x.source, coalesce(x.props, '{}')
		FROM jsonb_to_recordset($1::jsonb) AS x(occurred_on date, event_id uuid, occurred_at timestamptz, tenant_id uuid, school_id uuid,
			grade_level smallint, platform varchar, locale varchar, device_tier varchar, client_version varchar, player_key varchar,
			session_key varchar, key_version smallint, event_name varchar, source varchar, props jsonb)
		ON CONFLICT (occurred_on, event_id) DO NOTHING`, string(b))
	if err != nil {
		return 0, fmt.Errorf("analytics insert raw: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Anonymize implements port.AnalyticsStore (F080-AC-07): UPDATE only, the aggregate tables are untouched.
func (Store) Anonymize(ctx context.Context, keys []string) (int64, int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, 0, err
	}
	// tenancyguard:ok pseudonymous append-only rows located by their key (tenant scope of the transaction applies through RLS)
	r1, err := t.ExecContext(ctx, `UPDATE analytics.events_raw SET player_key = NULL, session_key = NULL WHERE player_key = ANY($1::text[])`, keys)
	if err != nil {
		return 0, 0, fmt.Errorf("analytics anonymize raw: %w", err)
	}
	// tenancyguard:ok pseudonymous derived rows located by their key (RLS of the transaction applies)
	r2, err := t.ExecContext(ctx, `UPDATE analytics.player_days SET player_key = NULL WHERE player_key = ANY($1::text[])`, keys)
	if err != nil {
		return 0, 0, fmt.Errorf("analytics anonymize player days: %w", err)
	}
	a, _ := r1.RowsAffected()
	b, _ := r2.RowsAffected()
	return a, b, nil
}

// AnonymizeBefore implements port.AnalyticsStore (retention, platform pool; batched).
func (Store) AnonymizeBefore(ctx context.Context, rawBefore, daysBefore time.Time, batch int) (int64, int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, 0, err
	}
	// tenancyguard:ok retention job of every tenant (platform pool): pseudonymous rows past their retention lose their keys
	r1, err := t.ExecContext(ctx, `UPDATE analytics.events_raw SET player_key = NULL, session_key = NULL WHERE (occurred_on, event_id) IN (
		SELECT occurred_on, event_id FROM analytics.events_raw WHERE occurred_on < $1 AND player_key IS NOT NULL LIMIT $2)`, day(rawBefore), batch)
	if err != nil {
		return 0, 0, fmt.Errorf("analytics retention raw: %w", err)
	}
	// tenancyguard:ok retention job of every tenant (platform pool)
	r2, err := t.ExecContext(ctx, `UPDATE analytics.player_days SET player_key = NULL WHERE (activity_date, day_id) IN (
		SELECT activity_date, day_id FROM analytics.player_days WHERE activity_date < $1 AND player_key IS NOT NULL LIMIT $2)`, day(daysBefore), batch)
	if err != nil {
		return 0, 0, fmt.Errorf("analytics retention player days: %w", err)
	}
	a, _ := r1.RowsAffected()
	b, _ := r2.RowsAffected()
	return a, b, nil
}

// PendingDays implements port.AnalyticsStore (platform pool).
func (Store) PendingDays(ctx context.Context, from, to time.Time) ([]port.TenantDay, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok job of every tenant (platform pool): tenant days with raw events not closed, or marked for reclose
	rows, err := t.QueryContext(ctx, `SELECT d.tenant_id, d.occurred_on FROM (
			SELECT DISTINCT tenant_id, occurred_on FROM analytics.events_raw WHERE occurred_on BETWEEN $1 AND $2) d
		LEFT JOIN analytics.rollup_days r ON r.tenant_id = d.tenant_id AND r.bucket_date = d.occurred_on
		LEFT JOIN ref.tenants t ON t.tenant_id = d.tenant_id
		WHERE (r.tenant_id IS NULL OR NOT r.is_final OR r.needs_reclose) AND NOT coalesce(t.is_qa, false)
		UNION
		SELECT tenant_id, bucket_date FROM analytics.rollup_days WHERE needs_reclose AND bucket_date BETWEEN $1 AND $2`, day(from), day(to))
	if err != nil {
		return nil, fmt.Errorf("analytics pending days: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []port.TenantDay
	for rows.Next() {
		var d port.TenantDay
		if err := rows.Scan(&d.TenantID, &d.Day); err != nil {
			return nil, err
		}
		d.Day = analytics.DateOf(d.Day)
		out = append(out, d)
	}
	return out, rows.Err()
}

// RebuildPlayerDays implements port.AnalyticsStore: one row per player of the day recomputed from the raw events (upsert
// on (activity_date, player_key)); a session counts as activity from analytics_active_session_min_seconds; level and the
// first active day carry over from earlier days.
func (Store) RebuildPlayerDays(ctx context.Context, d port.TenantDay, activeMin time.Duration) error {
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	// tenancyguard:ok derived rows of one tenant day (no deleted_at); tenant named in every source
	_, err = t.ExecContext(ctx, `WITH e AS (
			SELECT * FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on = $2 AND player_key IS NOT NULL),
		agg AS (
			SELECT player_key,
				(array_agg(school_id ORDER BY occurred_at DESC) FILTER (WHERE school_id IS NOT NULL))[1] AS school_id,
				(array_agg(grade_level ORDER BY occurred_at DESC) FILTER (WHERE grade_level IS NOT NULL))[1] AS grade_level,
				(array_agg(platform ORDER BY occurred_at DESC) FILTER (WHERE platform IS NOT NULL))[1] AS platform,
				(array_agg(locale ORDER BY occurred_at DESC) FILTER (WHERE locale IS NOT NULL))[1] AS locale,
				(array_agg(device_tier ORDER BY occurred_at DESC) FILTER (WHERE device_tier IS NOT NULL))[1] AS device_tier,
				(array_agg(client_version ORDER BY occurred_at DESC) FILTER (WHERE client_version IS NOT NULL))[1] AS client_version,
				count(*) FILTER (WHERE event_name = 'session_end' AND coalesce((props->>'active_seconds')::numeric, 0) >= $3) AS sessions,
				coalesce(sum((props->>'active_seconds')::numeric) FILTER (WHERE event_name = 'session_end'), 0)::int AS active_seconds,
				bool_or(event_name = 'xp_award' AND coalesce((props->>'capped')::boolean, false)) AS capped_xp,
				bool_or(event_name = 'currency_credit' AND coalesce((props->>'capped')::boolean, false)) AS capped_currency,
				count(DISTINCT props->>'reminder_no') FILTER (WHERE event_name = 'break_reminder_shown') AS reminders,
				bool_or(event_name = 'session_end' AND coalesce((props->>'late_night')::boolean, false)
					AND coalesce((props->>'active_seconds')::numeric, 0) >= $3) AS late_night,
				bool_or(coalesce((props->>'social')::boolean, false)) AS social,
				count(*) FILTER (WHERE event_name = 'match_result' AND props->>'outcome' <> 'abandoned') AS matches,
				count(*) FILTER (WHERE event_name = 'shop_purchase') AS purchases,
				max((props->>'to_level')::int) FILTER (WHERE event_name = 'level_up') AS level
			FROM e GROUP BY player_key)
		INSERT INTO analytics.player_days (activity_date, player_key, tenant_id, school_id, grade_level, platform, locale, device_tier,
			client_version, sessions, active_seconds, first_active_date, capped_xp, capped_currency, reminders_shown, late_night, social,
			matches, purchases, level, computed_at)
		SELECT $2::date, a.player_key, $1, a.school_id, a.grade_level, a.platform, a.locale, a.device_tier, a.client_version, a.sessions,
			a.active_seconds,
			CASE WHEN a.sessions > 0 THEN coalesce((SELECT min(p.first_active_date) FROM analytics.player_days p WHERE p.tenant_id = $1
				AND p.player_key = a.player_key AND p.activity_date < $2 AND p.first_active_date IS NOT NULL), $2::date)
				ELSE (SELECT min(p.first_active_date) FROM analytics.player_days p WHERE p.tenant_id = $1 AND p.player_key = a.player_key
				AND p.activity_date < $2 AND p.first_active_date IS NOT NULL) END,
			coalesce(a.capped_xp, false), coalesce(a.capped_currency, false), least(a.reminders, 32767), coalesce(a.late_night, false),
			coalesce(a.social, false), a.matches, a.purchases,
			coalesce(a.level, (SELECT p.level FROM analytics.player_days p WHERE p.tenant_id = $1 AND p.player_key = a.player_key
				AND p.activity_date < $2 AND p.level IS NOT NULL ORDER BY p.activity_date DESC LIMIT 1)),
			now()
		FROM agg a
		ON CONFLICT (activity_date, player_key) DO UPDATE SET school_id = excluded.school_id, grade_level = excluded.grade_level,
			platform = excluded.platform, locale = excluded.locale, device_tier = excluded.device_tier, client_version = excluded.client_version,
			sessions = excluded.sessions, active_seconds = excluded.active_seconds, first_active_date = excluded.first_active_date,
			capped_xp = excluded.capped_xp, capped_currency = excluded.capped_currency, reminders_shown = excluded.reminders_shown,
			late_night = excluded.late_night, social = excluded.social, matches = excluded.matches, purchases = excluded.purchases,
			level = excluded.level, computed_at = excluded.computed_at`, d.TenantID, day(d.Day), activeMin.Seconds())
	if err != nil {
		return fmt.Errorf("analytics player days: %w", err)
	}
	return nil
}

// dimExpr are the dimension expressions of a source aliased s (school, band, platform, locale, tier) — unknown values are
// the literal 'unknown' so that they never fall in the whole-tenant set.
const dimCols = `coalesce(s.school_id::text, 'unknown') AS d_school,
	CASE WHEN s.grade_level BETWEEN 1 AND 3 THEN 'g1_3' WHEN s.grade_level IN (4, 5) THEN 'g4_5' WHEN s.grade_level IN (6, 7) THEN 'g6_7'
		WHEN s.grade_level IN (8, 9) THEN 'g8_9' ELSE 'unknown' END AS d_band,
	coalesce(s.platform, 'unknown') AS d_platform, coalesce(s.locale, 'unknown') AS d_locale, coalesce(s.device_tier, 'unknown') AS d_tier`

const groupingSets = `GROUPING SETS ((), (d_school), (d_band), (d_platform), (d_locale), (d_tier), (d_school, d_band), (d_school, d_platform),
	(d_school, d_tier))`

const dimsSelect = `jsonb_strip_nulls(jsonb_build_object(
	'school_id', CASE WHEN GROUPING(d_school) = 0 THEN d_school END,
	'grade_band', CASE WHEN GROUPING(d_band) = 0 THEN d_band END,
	'platform', CASE WHEN GROUPING(d_platform) = 0 THEN d_platform END,
	'locale', CASE WHEN GROUPING(d_locale) = 0 THEN d_locale END,
	'device_tier', CASE WHEN GROUPING(d_tier) = 0 THEN d_tier END))::text AS dims`

// histExpr builds the histogram of expr (rows matching cond) on bounds as a jsonb array (bucket i: ≤ bounds[i]).
func histExpr(expr, cond string, bounds []float64) string {
	parts := make([]string, 0, len(bounds)+1)
	lo := ""
	for _, b := range bounds {
		bs := strconv.FormatFloat(b, 'f', -1, 64)
		c := fmt.Sprintf("%s AND %s <= %s", cond, expr, bs)
		if lo != "" {
			c += " AND " + expr + " > " + lo
		}
		parts = append(parts, "count(*) FILTER (WHERE "+c+")")
		lo = bs
	}
	parts = append(parts, fmt.Sprintf("count(*) FILTER (WHERE %s AND %s > %s)", cond, expr, lo))
	return "jsonb_build_array(" + strings.Join(parts, ", ") + ")::text"
}

func parseDims(s string) analytics.Dims {
	d := analytics.Dims{}
	_ = json.Unmarshal([]byte(s), &d)
	return d
}

func parseHist(s sql.NullString) []int64 {
	if !s.Valid {
		return nil
	}
	var h []int64
	_ = json.Unmarshal([]byte(s.String), &h)
	return h
}

func metricBounds(code string) []float64 {
	m, _ := analytics.Lookup(code)
	return m.Bounds
}

// DailyRows implements port.AnalyticsStore: the daily metrics of GAME_ANALYTICS §4 of one tenant day for every fixed
// dimension set (GROUPING SETS), from player_days and the raw events of the day (job only — dashboards never read them).
func (Store) DailyRows(ctx context.Context, d port.TenantDay) ([]port.RollupRow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var out []port.RollupRow
	add := func(code string, dims analytics.Dims, c analytics.Cell) {
		out = append(out, port.RollupRow{Date: d.Day, Code: code, Dims: dims, Cell: c})
	}
	// 1. player_days of the day.
	// tenancyguard:ok derived rows of one tenant day (no deleted_at); tenant named
	q1 := `WITH s AS (SELECT * FROM analytics.player_days WHERE tenant_id = $1 AND activity_date = $2 AND player_key IS NOT NULL),
		x AS (SELECT s.*, ` + dimCols + ` FROM s)
		SELECT ` + dimsSelect + `,
			count(*) FILTER (WHERE sessions > 0), count(*), coalesce(sum(sessions) FILTER (WHERE sessions > 0), 0),
			count(*) FILTER (WHERE sessions > 0 AND late_night), count(*) FILTER (WHERE capped_xp), count(*) FILTER (WHERE capped_currency),
			` + histExpr("active_seconds", "sessions > 0", metricBounds("daily_playtime_p90")) + `,
			` + histExpr("level", "sessions > 0 AND level IS NOT NULL", metricBounds("level_distribution")) + `
		FROM x GROUP BY ` + groupingSets
	// tenancyguard:ok derived rows of one tenant day (no deleted_at); tenant named
	rs1, rerr1 := t.QueryContext(ctx, q1, d.TenantID, day(d.Day))
	if err := each(rs1, rerr1, func(rows *sql.Rows) error {
		var dims string
		var active, all, sessions, late, cxp, ccur int64
		var hp, hl sql.NullString
		if err := rows.Scan(&dims, &active, &all, &sessions, &late, &cxp, &ccur, &hp, &hl); err != nil {
			return err
		}
		dm := parseDims(dims)
		add("dau", dm, analytics.Cell{Num: active, Den: 1, Players: active})
		add("sessions_per_player_day", dm, analytics.Cell{Num: sessions, Den: active, Players: active})
		add("late_night_play_rate", dm, analytics.Cell{Num: late, Den: active, Players: active})
		add("xp_cap_hit_rate", dm, analytics.Cell{Num: cxp, Den: all, Players: all})
		add("currency_cap_hit_rate", dm, analytics.Cell{Num: ccur, Den: all, Players: all})
		add("daily_playtime_p90", dm, analytics.Cell{Players: active, Hist: parseHist(hp)})
		add("level_distribution", dm, analytics.Cell{Players: active, Hist: parseHist(hl)})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("analytics daily player metrics: %w", err)
	}
	// 2. rolling windows (wau, mau, stickiness) from the 30 days of player_days ending at the day.
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	q2 := `WITH s AS (SELECT * FROM analytics.player_days WHERE tenant_id = $1 AND activity_date BETWEEN $2::date - 29 AND $2::date
			AND sessions > 0 AND player_key IS NOT NULL),
		x AS (SELECT s.*, ` + dimCols + ` FROM s)
		SELECT ` + dimsSelect + `, count(DISTINCT player_key) FILTER (WHERE activity_date > $2::date - 7), count(DISTINCT player_key), count(*)
		FROM x GROUP BY ` + groupingSets
	// tenancyguard:ok derived rows of one tenant (no deleted_at); tenant named
	rs2, rerr2 := t.QueryContext(ctx, q2, d.TenantID, day(d.Day))
	if err := each(rs2, rerr2, func(rows *sql.Rows) error {
		var dims string
		var wau, mau, pdays int64
		if err := rows.Scan(&dims, &wau, &mau, &pdays); err != nil {
			return err
		}
		dm := parseDims(dims)
		add("wau", dm, analytics.Cell{Num: wau, Players: wau})
		add("mau", dm, analytics.Cell{Num: mau, Players: mau})
		add("stickiness", dm, analytics.Cell{Num: pdays, Den: 30 * mau, Players: mau})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("analytics rolling metrics: %w", err)
	}
	// 3. raw events of the day.
	// tenancyguard:ok append-only pseudonymous rows of one tenant day (no deleted_at); tenant named
	q3 := `WITH s AS (SELECT * FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on = $2),
		x AS (SELECT s.*, ` + dimCols + ` FROM s)
		SELECT ` + dimsSelect + `,
			count(DISTINCT session_key) FILTER (WHERE event_name IN ('session_start', 'break_reminder_shown')),
			count(DISTINCT session_key) FILTER (WHERE event_name = 'break_reminder_shown'),
			count(DISTINCT session_key) FILTER (WHERE event_name = 'break_reminder_shown' AND (props->>'reminder_no')::int >= 2),
			count(*) FILTER (WHERE event_name = 'quest_accept'), count(*) FILTER (WHERE event_name = 'quest_complete'),
			count(*) FILTER (WHERE event_name = 'quest_abandon'), count(*) FILTER (WHERE event_name = 'challenge_start'),
			count(*) FILTER (WHERE event_name = 'challenge_result'),
			count(*) FILTER (WHERE event_name = 'challenge_result' AND props->>'status' = 'succeeded'),
			count(*) FILTER (WHERE event_name = 'challenge_result' AND props->>'status' = 'abandoned'),
			count(*) FILTER (WHERE event_name = 'boss_result'), count(*) FILTER (WHERE event_name = 'boss_result' AND props->>'result' = 'defeated'),
			count(*) FILTER (WHERE event_name = 'match_result'),
			count(*) FILTER (WHERE event_name = 'match_result' AND props->>'outcome' = 'abandoned'),
			count(*) FILTER (WHERE event_name = 'party_start'),
			coalesce(sum((props->>'party_size')::numeric) FILTER (WHERE event_name = 'party_start'), 0)::bigint,
			coalesce(sum((props->>'delta')::numeric) FILTER (WHERE event_name = 'currency_credit'), 0)::bigint,
			coalesce(sum(abs((props->>'delta')::numeric)) FILTER (WHERE event_name = 'currency_debit'), 0)::bigint,
			count(DISTINCT player_key),
			count(DISTINCT player_key) FILTER (WHERE event_name = 'event_join'),
			count(DISTINCT player_key) FILTER (WHERE event_name = 'session_end'),
			` + histExpr("(props->>'active_seconds')::numeric", "event_name = 'session_end'", metricBounds("session_duration_p50")) + `
		FROM x GROUP BY ` + groupingSets
	// tenancyguard:ok append-only pseudonymous rows of one tenant day (no deleted_at); tenant named
	rs3, rerr3 := t.QueryContext(ctx, q3, d.TenantID, day(d.Day))
	if err := each(rs3, rerr3, func(rows *sql.Rows) error {
		var dims string
		var sess, rem, rem2, qa, qc, qab, cs, cr, csu, cab, br, bd, mr, mab, ps, psz, cred, deb, players, joins, ended int64
		var hd sql.NullString
		if err := rows.Scan(&dims, &sess, &rem, &rem2, &qa, &qc, &qab, &cs, &cr, &csu, &cab, &br, &bd, &mr, &mab, &ps, &psz, &cred, &deb,
			&players, &joins, &ended, &hd); err != nil {
			return err
		}
		dm := parseDims(dims)
		add("sessions_over_reminder", dm, analytics.Cell{Num: rem, Den: sess, Players: players})
		add("sessions_over_second_reminder", dm, analytics.Cell{Num: rem2, Den: sess, Players: players})
		add("quest_completion_rate", dm, analytics.Cell{Num: qc, Den: qa, Players: players})
		add("challenge_success_rate", dm, analytics.Cell{Num: csu, Den: cr, Players: players})
		add("boss_defeat_rate", dm, analytics.Cell{Num: bd, Den: br, Players: players})
		add("abandon_rate", dm, analytics.Cell{Num: qab + cab + mab, Den: qa + cs + mr, Players: players})
		add("match_completion_rate", dm, analytics.Cell{Num: mr - mab, Den: mr, Players: players})
		add("party_size_avg", dm, analytics.Cell{Num: psz, Den: ps, Players: players})
		add("currency_sources", dm, analytics.Cell{Num: cred, Players: players})
		add("currency_sinks", dm, analytics.Cell{Num: deb, Players: players})
		add("currency_net_flow", dm, analytics.Cell{Num: cred - deb, Players: players})
		add("event_participation_rate", dm, analytics.Cell{Num: joins, Players: players})
		add("session_duration_p50", dm, analytics.Cell{Players: ended, Hist: parseHist(hd)})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("analytics daily event metrics: %w", err)
	}
	// event_participation_rate: denominator = active players of the same dims; chat_activity: messages / active players.
	active := map[string]int64{}
	for _, r := range out {
		if r.Code == "dau" {
			active[r.Dims.Key()] = r.Cell.Num
		}
	}
	for i := range out {
		if out[i].Code == "event_participation_rate" {
			out[i].Cell.Den = active[out[i].Dims.Key()]
		}
	}
	extra, err := extraDims(ctx, t, d, active[""])
	if err != nil {
		return nil, err
	}
	return append(out, extra...), nil
}

// extraDims are the tenant-only cuts of a metric by its extra key (currency reason, event) and chat activity.
func extraDims(ctx context.Context, t *sql.Tx, d port.TenantDay, activeTenant int64) ([]port.RollupRow, error) {
	var out []port.RollupRow
	// tenancyguard:ok append-only pseudonymous rows of one tenant day (no deleted_at); tenant named
	rs4, rerr4 := t.QueryContext(ctx, `SELECT event_name, coalesce(props->>'reason_code', 'unknown'), coalesce(sum(abs((props->>'delta')::numeric)), 0)::bigint,
			count(DISTINCT player_key)
		FROM analytics.events_raw WHERE tenant_id = $1 AND occurred_on = $2 AND event_name IN ('currency_credit', 'currency_debit')
		GROUP BY 1, 2`, d.TenantID, day(d.Day))
	if err := each(rs4, rerr4, func(rows *sql.Rows) error {
		var name, reason string
		var sum, players int64
		if err := rows.Scan(&name, &reason, &sum, &players); err != nil {
			return err
		}
		code := "currency_sources"
		if name == "currency_debit" {
			code = "currency_sinks"
		}
		out = append(out, port.RollupRow{Date: d.Day, Code: code, Dims: analytics.Dims{analytics.DimReason: trunc(reason, 64)},
			Cell: analytics.Cell{Num: sum, Players: players}})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("analytics currency reasons: %w", err)
	}
	// tenancyguard:ok append-only pseudonymous rows of one tenant day (no deleted_at); tenant named
	rs5, rerr5 := t.QueryContext(ctx, `SELECT props->>'event_id', count(DISTINCT player_key) FROM analytics.events_raw
		WHERE tenant_id = $1 AND occurred_on = $2 AND event_name = 'event_join' AND props->>'event_id' IS NOT NULL GROUP BY 1`,
		d.TenantID, day(d.Day))
	if err := each(rs5, rerr5, func(rows *sql.Rows) error {
		var ev string
		var n int64
		if err := rows.Scan(&ev, &n); err != nil {
			return err
		}
		out = append(out, port.RollupRow{Date: d.Day, Code: "event_participation_rate", Dims: analytics.Dims{analytics.DimEvent: trunc(ev, 64)},
			Cell: analytics.Cell{Num: n, Den: activeTenant, Players: activeTenant}})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("analytics event participation: %w", err)
	}
	var msgs sql.NullInt64
	// tenancyguard:ok append-only pseudonymous rows of one tenant day (no deleted_at); tenant named
	if err := t.QueryRowContext(ctx, `SELECT sum((props->>'messages')::numeric)::bigint FROM analytics.events_raw
		WHERE tenant_id = $1 AND occurred_on = $2 AND event_name = 'chat_daily_volume'`, d.TenantID, day(d.Day)).Scan(&msgs); err != nil {
		return nil, fmt.Errorf("analytics chat activity: %w", err)
	}
	if msgs.Valid {
		out = append(out, port.RollupRow{Date: d.Day, Code: "chat_activity", Dims: analytics.Dims{},
			Cell: analytics.Cell{Num: msgs.Int64, Den: activeTenant, Players: activeTenant}})
	}
	return out, nil
}

// each scans every row of a query and closes the rows (the error of the query is returned first).
func each(rows *sql.Rows, err error, scan func(rows *sql.Rows) error) error {
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if cerr := rows.Close(); err == nil {
		err = cerr
	}
	return err
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
