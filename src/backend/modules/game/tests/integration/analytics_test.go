//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// golden is the golden dataset of the rollup tests (T003): tenant A, school A (grade 3: p1…p6) and school A2 (grade 7:
// q1…q3), days D0 = 2026-11-16 and D1 = 2026-11-17 (ICT). Expected values are computed by hand in the test.
type golden struct {
	p  [6]uuid.UUID
	q  [3]uuid.UUID
	d0 string
	d1 string
}

func (e *env) seedGolden(ctx context.Context, t *testing.T) golden {
	t.Helper()
	g := golden{d0: "2026-11-16", d1: "2026-11-17"}
	a, a2 := e.schoolA, e.schoolA2
	var rows []port.RawEvent
	for i := range g.p {
		g.p[i] = e.newStudent(ctx, t, e.tenantA, a, "active").id
	}
	for i := range g.q {
		g.q[i] = uuid.New()
		if _, err := e.migrator.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, status, source_version)
			VALUES ($1, $2, $3, 7, 'active', 1)`, g.q[i], e.tenantA, a2); err != nil {
			t.Fatal(err)
		}
	}
	d0 := g.d0
	rows = append(rows, session(e.tenantA, &a, 3, g.p[0], "p1-s1", ictAt(d0, "08:00"), 120, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[0], "p1-s2", ictAt(d0, "15:00"), 200, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[1], "p2-s1", ictAt(d0, "08:30"), 30, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[2], "p3-s1", ictAt(d0, "09:00"), 600, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[3], "p4-s1", ictAt(d0, "09:30"), 600, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[4], "p5-s1", ictAt(d0, "10:00"), 600, false)...)
	rows = append(rows, session(e.tenantA, &a, 3, g.p[5], "p6-s1", ictAt(d0, "23:00"), 600, true)...)
	for i := range g.q {
		rows = append(rows, session(e.tenantA, &a2, 7, g.q[i], "q-s1-"+g.q[i].String(), ictAt(d0, "14:00"), 900, false)...)
	}
	// Break reminders (F080-AC-35): p3 once, p4 twice; the second reminder of p4 is delivered twice (same event id).
	rem := func(st uuid.UUID, key string, no int, at string) port.RawEvent {
		r := rawEvent(e.tenantA, &a, 3, st, key, "break_reminder_shown", ictAt(d0, at), map[string]any{"reminder_no": float64(no),
			"minutes_played": float64(45 * no)})
		r.Source = "client"
		return r
	}
	dup := rem(g.p[3], "p4-s1", 2, "09:38")
	rows = append(rows, rem(g.p[2], "p3-s1", 1, "09:05"), rem(g.p[3], "p4-s1", 1, "09:34"), dup, dup)
	// Quests: p1 accepts and completes Q1, p3 accepts and abandons it.
	qv, qid := uuid.New(), uuid.New()
	qp := func(extra map[string]any) map[string]any {
		m := map[string]any{"quest_id": qid.String(), "quest_version_id": qv.String(), "quest_type": "main"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	rows = append(rows, rawEvent(e.tenantA, &a, 3, g.p[0], "p1-s1", "quest_accept", ictAt(d0, "08:01"), qp(nil)),
		rawEvent(e.tenantA, &a, 3, g.p[0], "p1-s1", "quest_complete", ictAt(d0, "08:06"), qp(map[string]any{"duration_s": 300.0})),
		rawEvent(e.tenantA, &a, 3, g.p[2], "p3-s1", "quest_accept", ictAt(d0, "09:01"), qp(nil)),
		rawEvent(e.tenantA, &a, 3, g.p[2], "p3-s1", "quest_abandon", ictAt(d0, "09:04"), qp(nil)))
	// Onboarding: p1…p5 created their avatar; performance samples of five players (web, low preset: budget 30 FPS).
	for i := 0; i < 5; i++ {
		rows = append(rows, rawEvent(e.tenantA, &a, 3, g.p[i], "", "avatar_create", ictAt(d0, "07:59"), map[string]any{"preset_code": "a"}))
		pi := i
		if pi == 1 {
			pi = 5
		}
		pr := rawEvent(e.tenantA, &a, 3, g.p[pi], "perf-"+g.p[pi].String(), "perf_sample", ictAt(d0, "11:00"),
			map[string]any{"preset": "low", "renderer": "mobile", "fps_p50": 45.0, "fps_p10": 31.0})
		pr.Source = "client"
		rows = append(rows, pr)
	}
	// D1: p1, p3, p4, p5 and q1 come back (5 players: the day is not a small cell).
	for _, i := range []int{0, 2, 3, 4} {
		rows = append(rows, session(e.tenantA, &a, 3, g.p[i], "d1-"+g.p[i].String(), ictAt(g.d1, "08:00"), 300, false)...)
	}
	rows = append(rows, session(e.tenantA, &a2, 7, g.q[0], "q1-d1", ictAt(g.d1, "09:00"), 300, false)...)
	e.insertRaw(ctx, t, e.tenantA, rows...)
	return g
}

// closeGolden sets the clock to 2026-11-18 10:00 ICT and runs the day close (D0, D1 final).
func (e *env) closeGolden(ctx context.Context, t *testing.T, k *anaKit) {
	t.Helper()
	e.clock.Set(ictAt("2026-11-18", "10:00"))
	if _, err := k.rollup.CloseDays(ctx); err != nil {
		t.Fatal(err)
	}
}

func approx(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// F080-AC-08, AC-10, AC-35 (T022, T024, T065; TC-ANL-001, AC-ANL-101, AC-ANL-102): the golden dataset rolled up and
// closed matches the hand computation — dau counts a player once (p1, two sessions) and not a 30 s session (p2), the
// wellbeing metrics count sessions with reminders once per client event, retention_d1 of the D0 cohort, the onboarding
// funnel, weekly rows, content and performance rows; every closed row is is_final.
func TestAnalyticsRollupGolden(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	// The hourly job writes provisional rows of the open days first.
	e.clock.Set(ictAt(g.d1, "12:00"))
	if _, err := k.rollup.RunHourly(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, _, final := e.rollupRow(ctx, t, "rollup_daily", e.tenantA, "dau", g.d1); final {
		t.Fatal("hourly rows are provisional")
	}
	e.closeGolden(ctx, t, k)
	check := func(code string, want float64) {
		t.Helper()
		v, _, _, final := e.rollupRow(ctx, t, "rollup_daily", e.tenantA, code, g.d0)
		if !approx(v, want) || !final {
			t.Fatalf("%s = %v (final %v), want %v", code, v, final, want)
		}
	}
	check("dau", 8)
	check("sessions_per_player_day", 9.0/8)
	check("sessions_over_reminder", 2.0/10)
	check("sessions_over_second_reminder", 1.0/10)
	check("late_night_play_rate", 1.0/8)
	check("quest_completion_rate", 0.5)
	check("session_duration_p50", 450)
	check("wau", 8)
	if v, _, _, _ := e.rollupRow(ctx, t, "rollup_daily", e.tenantA, "dau", g.d1); v != 5 {
		t.Fatalf("dau D1 = %v", v)
	}
	if v, _, _, _ := e.rollupRow(ctx, t, "rollup_daily", e.tenantA, "mau", g.d1); v != 8 {
		t.Fatalf("mau D1 = %v", v)
	}
	if n := count(ctx, t, e.platform, `SELECT reminders_shown FROM analytics.player_days WHERE activity_date = $1 AND player_key = $2`, g.d0,
		analytics.Pseudonym(anaKey, g.p[3])); n != 2 {
		t.Fatalf("reminders_shown of p4 = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT retained FROM analytics.cohort_retention WHERE tenant_id = $1 AND cohort_date = $2 AND day_n = 1
		AND dim_set = ''`, e.tenantA, g.d0); n != 5 {
		t.Fatalf("retention d1 retained = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT cohort_size FROM analytics.cohort_retention WHERE tenant_id = $1 AND cohort_date = $2 AND day_n = 1
		AND dim_set = ''`, e.tenantA, g.d0); n != 8 {
		t.Fatalf("cohort size = %d", n)
	}
	for step, want := range map[int]int{1: 8, 2: 4, 4: 2, 5: 1} {
		if n := count(ctx, t, e.platform, `SELECT count FROM analytics.funnel_daily WHERE tenant_id = $1 AND bucket_date = $2 AND step_no = $3
			AND dim_set = ''`, e.tenantA, g.d0, step); n != want {
			t.Fatalf("funnel step %d = %d, want %d", step, n, want)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.rollup_weekly WHERE tenant_id = $1 AND metric_date = '2026-11-16'
		AND metric_code IN ('dau', 'active_days_per_week', 'social_play_rate')`, e.tenantA); n < 3 {
		t.Fatalf("weekly rows = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT started FROM analytics.content_stats_weekly WHERE tenant_id = $1 AND content_kind = 'quest'`, e.tenantA); n != 2 {
		t.Fatalf("content started = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT samples FROM analytics.perf_daily WHERE tenant_id = $1 AND bucket_date = $2 AND preset = 'low'`,
		e.tenantA, g.d0); n != 5 {
		t.Fatalf("perf samples = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.rollup_days WHERE tenant_id = $1 AND is_final AND NOT needs_reclose`,
		e.tenantA); n != 2 {
		t.Fatalf("final days = %d", n)
	}
	if k.versions.bumped(e.tenantA) == 0 {
		t.Fatal("closing bumps the data version of the tenant")
	}
}

// F080-AC-09 (T023; TC-DASH-005): an event 30 hours late marks its closed day, which the close job computes again (and
// bumps the cache version); an event later than analytics_late_event_window is dropped (rollups unchanged).
func TestAnalyticsLateEvents(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	e.closeGolden(ctx, t, k)
	before := k.versions.bumped(e.tenantA)
	late := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	ectx := tenancy.WithScope(ctx, mustEventScope(t, e.tenantA))
	end := ictAt(g.d1, "04:00") // 30 hours before 2026-11-18 10:00 ICT
	ended := func(at time.Time) command.DomainEvent {
		return command.DomainEvent{EventID: uuid.New(), EventType: "game_session.ended", OccurredAt: at, TenantID: e.tenantA,
			Payload: map[string]any{"student_id": late.id.String(), "game_session_id": uuid.NewString(), "platform": "web", "duration_ms": 600000.0,
				"end_reason": "logout", "ended_at": at.Format(time.RFC3339)}}
	}
	if err := k.ingest.IngestDomainEvent(ectx, ended(end)); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.rollup_days WHERE tenant_id = $1 AND bucket_date = $2 AND needs_reclose`,
		e.tenantA, g.d1); n != 1 {
		t.Fatal("the closed day is marked for reclose")
	}
	if _, err := k.rollup.CloseDays(ctx); err != nil {
		t.Fatal(err)
	}
	if v, _, _, final := e.rollupRow(ctx, t, "rollup_daily", e.tenantA, "dau", g.d1); v != 6 || !final {
		t.Fatalf("dau D1 after reclose = %v final %v", v, final)
	}
	if k.versions.bumped(e.tenantA) <= before {
		t.Fatal("reclose bumps the cache version")
	}
	// 50 hours late: dropped.
	if err := k.ingest.IngestDomainEvent(ectx, ended(ictAt("2026-11-16", "08:00"))); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE occurred_on = '2026-11-16' AND player_key = $1`,
		analytics.Pseudonym(anaKey, late.id)); n != 0 {
		t.Fatal("an event later than the window is not stored")
	}
}

// F080-AC-05, AC-06, AC-07 (T018–T020; TC-ANL-002, TC-ANL-003, TC-ANL-010): the consumer maps domain events to
// pseudonymous rows (HMAC player_key/session_key, dims from the projections, catalog props only), idempotent per event id,
// one row per student of a match; the analytics schema and props carry no identity; anonymising a student clears its keys
// (rows kept, aggregates unchanged, nothing deleted).
func TestAnalyticsConsumerAndAnonymize(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	s1 := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	s2 := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.clock.Set(ictAt("2026-11-18", "10:00"))
	c := consumer.Analytics{Ingest: k.ingest}
	ectx := tenancy.WithScope(ctx, mustEventScope(t, e.tenantA))
	tenant := e.tenantA
	env := func(id uuid.UUID, typ string, payload map[string]any) outbox.Envelope {
		b, _ := json.Marshal(payload)
		return outbox.Envelope{EventID: id, EventType: typ, OccurredAt: ictAt("2026-11-18", "09:00"), TenantID: &tenant, Payload: b}
	}
	gs := uuid.New()
	quest := uuid.New()
	for _, ev := range []outbox.Envelope{
		env(uuid.New(), "game_session.started", map[string]any{"student_id": s1.id.String(), "game_session_id": gs.String(), "platform": "android",
			"client_version": "1.3.0", "started_at": "2026-11-18T01:00:00Z"}),
		env(quest, "quest.completed", map[string]any{"student_id": s1.id.String(), "quest_id": uuid.NewString(), "quest_version_id": uuid.NewString(),
			"quest_type": "main", "rewards": []any{}, "objective_ids": []any{}, "completion_no": 1.0, "completed_at": "2026-11-18T02:00:00Z"}),
		env(quest, "quest.completed", map[string]any{"student_id": s1.id.String(), "quest_id": uuid.NewString(), "quest_version_id": uuid.NewString(),
			"quest_type": "main", "rewards": []any{}, "objective_ids": []any{}, "completion_no": 1.0, "completed_at": "2026-11-18T02:00:00Z"}),
		env(uuid.New(), "match.completed", map[string]any{"match_id": uuid.NewString(), "mode": "duel", "matchmaking_type": "solo", "status": "completed",
			"ended_at": "2026-11-18T02:00:00Z", "results": []any{map[string]any{"student_id": s1.id.String(), "outcome": "win", "score": 9.0},
				map[string]any{"student_id": s2.id.String(), "outcome": "lose", "score": 3.0}}}),
		env(uuid.New(), "quest.accepted", map[string]any{"student_id": uuid.NewString(), "quest_id": uuid.NewString(), "quest_version_id": uuid.NewString()}),
	} {
		if err := c.Handle(ectx, ev); err != nil {
			t.Fatal(err)
		}
	}
	pk := analytics.Pseudonym(anaKey, s1.id)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1`, pk); n != 3 {
		t.Fatalf("rows of s1 = %d, want 3 (session, quest once, match)", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE event_name = 'match_result'`); n != 2 {
		t.Fatalf("match rows = %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1 AND session_key = $2 AND school_id = $3
		AND grade_level = 3 AND platform = 'android' AND event_name = 'session_start'`, pk, analytics.Pseudonym(anaKey, gs), e.schoolA); n != 1 {
		t.Fatal("session_start carries the HMAC session key and the dims of the projections")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE props::text LIKE '%'||$1||'%' OR player_key = $1
		OR props ? 'student_id' OR props ? 'score'`, s1.id.String()); n != 0 {
		t.Fatal("no student id, no score in analytics (AC-ANL-103)")
	}
	// F080-AC-06: schema scan.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'analytics'
		AND column_name ~ '(student|user_id|^name$|nickname|full_name|display_name|content_text|message|chat|email|phone|ip_address|^ip$)'`); n != 0 {
		t.Fatalf("%d identifying columns in schema analytics", n)
	}
	// F080-AC-07: anonymise s1 (rollup first so that aggregates exist).
	if _, err := k.rollup.RunHourly(ctx); err != nil {
		t.Fatal(err)
	}
	before := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.rollup_daily WHERE tenant_id = $1`, e.tenantA)
	raw := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw`)
	if _, _, err := k.ingest.AnonymizeStudent(ectx, s1.id); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1`, pk); n != 0 {
		t.Fatal("raw keys cleared")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.player_days WHERE player_key = $1`, pk); n != 0 {
		t.Fatal("player_days keys cleared")
	}
	if count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw`) != raw ||
		count(ctx, t, e.platform, `SELECT count(*) FROM analytics.rollup_daily WHERE tenant_id = $1`, e.tenantA) != before {
		t.Fatal("anonymisation deletes nothing and leaves the aggregates")
	}
}

// F080-AC-01…04, AC-34 (T013–T016, T064; TC-ANL-003, TC-ANL-007): postTelemetry validates the closed schema per type,
// stores each client event once, attaches the session_key of the game session, drops product events of a tenant that
// switched product telemetry off (perf/crash/break reminders kept), drops QA tenants and future events; 429 + Retry-After
// beyond the rate limit; staff tokens 403; crash details go to the error log only, without PII.
func TestAnalyticsTelemetry(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, func(c *analytics.Config) { c.TelemetryRatePerMin = 6 })
	s := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	d := e.device(s)
	sessions0 := testutil.ToFloat64(observability.ClientSessions().WithLabelValues("web", "1.2.3"))
	if r := e.start(t, s, d, newKey(), map[string]any{"client_platform": "web", "client_version": "1.2.3"}); r.status != http.StatusCreated {
		t.Fatalf("start: %d %s", r.status, r.raw)
	}
	// MONITORING §3/§5: the created game session is the client session of ClientCrashRateHigh.
	if d := testutil.ToFloat64(observability.ClientSessions().WithLabelValues("web", "1.2.3")) - sessions0; d != 1 {
		t.Fatalf("client_sessions_total delta %v", d)
	}
	crashes0 := testutil.ToFloat64(observability.ClientCrashes().WithLabelValues("web", "1.2.3"))
	gaps0 := testutil.ToFloat64(observability.ChatSeqGaps().WithLabelValues("party"))
	bytes0 := testutil.ToFloat64(observability.ContentDownloadBytes().WithLabelValues("lan_cache"))
	now := e.clock.Now()
	ts := now.Add(-time.Minute).Format(time.RFC3339)
	rem := uuid.NewString()
	batch := func(events ...map[string]any) map[string]any {
		return map[string]any{"platform": "web", "client_version": "1.2.3", "device_tier": "mid", "preset": "low", "renderer": "mobile",
			"locale": "vi-VN", "events": events}
	}
	h := map[string]string{"X-Tenant-Id": s.tenant.String()}
	post := func(b map[string]any) resp { return e.do(t, http.MethodPost, "/game/telemetry", d.token, h, b) }
	good := batch(map[string]any{"type": "perf_sample", "occurred_at": ts, "client_event_id": uuid.NewString(), "fps_p50": 40.0, "fps_p10": 28.0},
		map[string]any{"type": "screen_view", "occurred_at": ts, "client_event_id": uuid.NewString(), "screen_code": "home", "duration_ms": 1200},
		map[string]any{"type": "break_reminder_shown", "occurred_at": ts, "client_event_id": rem, "reminder_no": 1, "minutes_played": 45},
		map[string]any{"type": "crash", "occurred_at": ts, "client_event_id": uuid.NewString(), "error_code": "E42", "fatal": true,
			"crash_signature": "sig-abc"},
		map[string]any{"type": "chat_seq_gap", "occurred_at": ts, "client_event_id": uuid.NewString(), "channel_type": "party", "gap_count": 3},
		map[string]any{"type": "content_download", "occurred_at": ts, "client_event_id": uuid.NewString(), "source": "lan_cache", "bytes": 4096})
	if r := post(good); r.status != http.StatusAccepted {
		t.Fatalf("telemetry: %d %s", r.status, r.raw)
	}
	pk := analytics.Pseudonym(anaKey, s.id)
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1 AND source = 'client'`, pk); n != 4 {
		t.Fatalf("stored %d client events", n)
	}
	// MONITORING §3: crash, chat_seq_gap and content_download feed the client metrics; the last two are metric-only (not stored).
	for name, c := range map[string]struct{ got, want float64 }{
		"client_crashes_total":                           {testutil.ToFloat64(observability.ClientCrashes().WithLabelValues("web", "1.2.3")) - crashes0, 1},
		"chat_seq_gaps_total":                            {testutil.ToFloat64(observability.ChatSeqGaps().WithLabelValues("party")) - gaps0, 3},
		"client_content_download_bytes_total{lan_cache}": {testutil.ToFloat64(observability.ContentDownloadBytes().WithLabelValues("lan_cache")) - bytes0, 4096},
	} {
		if c.got != c.want {
			t.Fatalf("%s delta %v, want %v", name, c.got, c.want)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1 AND event_name = 'break_reminder_shown'
		AND session_key IS NOT NULL AND props->>'reminder_no' = '1'`, pk); n != 1 {
		t.Fatal("break_reminder_shown carries the session_key of the game session (F080-AC-34)")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE props ? 'crash_signature'`); n != 0 {
		t.Fatal("crash_signature never goes to analytics (F080-AC-04)")
	}
	logs := k.logs.String()
	if !strings.Contains(logs, "sig-abc") || strings.Contains(logs, s.id.String()) || strings.Contains(logs, d.token) {
		t.Fatalf("error log: crash signature yes, no student id or token: %s", logs)
	}
	// Resent batch: stored once (client_event_id).
	if r := post(good); r.status != http.StatusAccepted {
		t.Fatalf("resend: %d", r.status)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1`, pk); n != 4 {
		t.Fatalf("dedup: %d rows", n)
	}
	// Closed schema: unknown attribute, attribute of another type, reminder out of range → 422.
	for _, bad := range []map[string]any{
		batch(map[string]any{"type": "screen_view", "occurred_at": ts, "screen_code": "home", "student_name": "An"}),
		batch(map[string]any{"type": "screen_view", "occurred_at": ts, "screen_code": "home", "fps_p50": 3}),
		batch(map[string]any{"type": "break_reminder_shown", "occurred_at": ts, "reminder_no": 21, "minutes_played": 10}),
		batch(map[string]any{"type": "break_reminder_shown", "occurred_at": ts, "minutes_played": 10}),
	} {
		if r := post(bad); r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
			t.Fatalf("invalid batch: %d %s", r.status, r.raw)
		}
	}
	// F080-AC-03: tenant switches product telemetry off.
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.tenants SET settings_subset = '{"analytics_product_telemetry_enabled": false}'
		WHERE tenant_id = $1`, s.tenant); err != nil {
		t.Fatal(err)
	}
	off := batch(map[string]any{"type": "screen_view", "occurred_at": ts, "client_event_id": uuid.NewString(), "screen_code": "map"},
		map[string]any{"type": "tutorial_step", "occurred_at": ts, "client_event_id": uuid.NewString(), "step_code": "move", "result": "done"},
		map[string]any{"type": "perf_sample", "occurred_at": ts, "client_event_id": uuid.NewString(), "fps_p50": 33.0},
		map[string]any{"type": "break_reminder_shown", "occurred_at": ts, "client_event_id": uuid.NewString(), "reminder_no": 2, "minutes_played": 90},
		map[string]any{"type": "perf_sample", "occurred_at": now.Add(10 * time.Minute).Format(time.RFC3339), "client_event_id": uuid.NewString(),
			"fps_p50": 33.0})
	if r := post(off); r.status != http.StatusAccepted {
		t.Fatalf("telemetry off: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1 AND event_name IN ('screen_view', 'tutorial_step')`,
		pk); n != 1 {
		t.Fatalf("product events after switch-off: %d (only the earlier screen_view)", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE player_key = $1 AND event_name IN ('perf_sample', 'break_reminder_shown')`,
		pk); n != 4 {
		t.Fatalf("perf and reminders kept, future dropped: %d", n)
	}
	cfg := e.do(t, http.MethodGet, "/game/client-config?platform=web&version=1.2.3&tenant_id="+s.tenant.String(), "", nil, nil)
	if cfg.status != http.StatusOK || cfg.body["product_telemetry_enabled"] != false {
		t.Fatalf("client-config of the tenant: %d %s", cfg.status, cfg.raw)
	}
	if cfg := e.do(t, http.MethodGet, "/game/client-config?platform=web&version=1.2.3", "", nil, nil); cfg.body["product_telemetry_enabled"] != true {
		t.Fatalf("client-config of the platform: %s", cfg.raw)
	}
	// F080-AC-02: rate limit (6 per minute in this test: 4 batches so far) and staff tokens.
	post(off)
	post(off)
	r := post(off)
	if r.status != http.StatusTooManyRequests || r.code() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("rate limit: %d %s %v", r.status, r.raw, r.header)
	}
	staff := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	if r := e.do(t, http.MethodPost, "/game/telemetry", staff, h, good); r.status != http.StatusForbidden || r.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/telemetry", d.token, map[string]string{"X-Tenant-Id": e.tenantB.String()}, good); r.status != http.StatusForbidden {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "postTelemetry", authztest.RoleDenied, authztest.CrossTenant)
	// QA tenant (GAME_ANALYTICS A-8): nothing stored.
	sb := e.newStudent(ctx, t, e.tenantB, e.schoolB, "active")
	if _, err := e.migrator.ExecContext(ctx, `UPDATE ref.tenants SET is_qa = true WHERE tenant_id = $1`, e.tenantB); err != nil {
		t.Fatal(err)
	}
	db := e.device(sb)
	if r := e.do(t, http.MethodPost, "/game/telemetry", db.token, map[string]string{"X-Tenant-Id": e.tenantB.String()}, good); r.status != http.StatusAccepted {
		t.Fatalf("qa: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM analytics.events_raw WHERE tenant_id = $1`, e.tenantB); n != 0 {
		t.Fatal("QA tenant events are never stored")
	}
}

// F080-AC-11, AC-12, AC-14, AC-26 (T026, T027, T029, T047; TC-DASH-002, TC-DASH-003, TC-ANL-006, TC-TEN-024): the platform
// queries read the rollups on the dashboard session with grain/data_as_of/is_partial, suppress small cells, audit the
// cross-tenant read; range, busy, timeout and parameter limits; tenant roles and the Safety Moderator get 403.
func TestAnalyticsAdminQueries(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	e.closeGolden(ctx, t, k)
	lo := e.staffToken(e.platformStaff("liveops"))
	get := func(token, path string) resp { return e.do(t, http.MethodGet, path, token, nil, nil) }
	base := "/admin/analytics/game/metrics?metric_codes=dau&metric_codes=sessions_over_reminder&tenant_ids=" + e.tenantA.String()
	audits := e.audits(ctx, t, "cross_tenant.accessed")
	r := get(lo, base+"&from="+g.d0+"&to="+g.d1)
	if r.status != http.StatusOK || r.body["grain"] != "day" || r.body["data_as_of"] == nil || r.body["is_partial"] != false {
		t.Fatalf("metrics: %d %s", r.status, r.raw)
	}
	series := r.body["data"].([]any)
	dau := series[0].(map[string]any)["points"].([]any)
	if p0 := dau[0].(map[string]any); p0["value"].(float64) != 8 {
		t.Fatalf("dau D0: %v", p0)
	}
	if p1 := dau[1].(map[string]any); p1["value"].(float64) != 5 {
		t.Fatalf("dau D1: %v", p1)
	}
	if e.audits(ctx, t, "cross_tenant.accessed") <= audits {
		t.Fatal("platform reads are audited (cross_tenant.accessed)")
	}
	for days, grain := range map[int]string{7: "day", 90: "week", 730: "month"} {
		to := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
		from := to.AddDate(0, 0, -(days - 1)).Format(time.DateOnly)
		if r := get(lo, base+"&from="+from+"&to="+to.Format(time.DateOnly)); r.status != http.StatusOK || r.body["grain"] != grain ||
			len(r.body["data"].([]any)[0].(map[string]any)["points"].([]any)) > 120 {
			t.Fatalf("%d days: %d %v", days, r.status, r.body["grain"])
		}
	}
	if r := get(lo, base+"&from=2024-11-16&to=2026-11-17"); r.status != http.StatusUnprocessableEntity || r.code() != "DASHBOARD_RANGE_TOO_LARGE" {
		t.Fatalf("731 days: %d %s", r.status, r.raw)
	}
	if r := get(lo, base+"&to=2026-11-17"); r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("missing from: %d %s", r.status, r.raw)
	}
	if r := get(lo, base+"&from="+g.d0+"&to="+g.d1+"&platforms=web&locales=vi-VN"); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("two dimensions: %d %s", r.status, r.raw)
	}
	cv := strings.Repeat("&client_versions=1.0", 21)
	if r := get(lo, base+"&from="+g.d0+"&to="+g.d1+cv); r.status != http.StatusUnprocessableEntity || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("21 client versions: %d %s", r.status, r.raw)
	}
	// Dimension series and their suppression (school A: 5 players, A2: 3 → both hidden, complementary).
	r = get(lo, base+"&from="+g.d0+"&to="+g.d0+"&school_ids="+e.schoolA.String()+"&school_ids="+e.schoolA2.String())
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 4 {
		t.Fatalf("by school: %d %s", r.status, r.raw)
	}
	for _, s := range r.body["data"].([]any) {
		if pt := s.(map[string]any)["points"].([]any)[0].(map[string]any); s.(map[string]any)["metric_code"] == "dau" && pt["suppressed"] != true {
			t.Fatalf("small school cells are hidden with their complement: %v", s)
		}
	}
	// Retention, funnel, content stats, performance.
	if r := get(lo, "/admin/analytics/game/retention?cohort_from="+g.d0+"&cohort_to="+g.d0); r.status != http.StatusOK ||
		r.body["cohorts"].([]any)[0].(map[string]any)["cohort_size"].(float64) != 8 {
		t.Fatalf("retention: %d %s", r.status, r.raw)
	}
	if r := get(lo, "/admin/analytics/game/funnels/onboarding?from="+g.d0+"&to="+g.d0); r.status != http.StatusOK ||
		r.body["steps"].([]any)[0].(map[string]any)["count"].(float64) != 8 || r.body["steps"].([]any)[1].(map[string]any)["count"] != nil {
		t.Fatalf("funnel (step 2 has 4 players: hidden): %d %s", r.status, r.raw)
	}
	if r := get(lo, "/admin/analytics/game/funnels/unknown?from="+g.d0+"&to="+g.d0); r.status != http.StatusNotFound {
		t.Fatalf("unknown funnel: %d", r.status)
	}
	r = get(lo, "/admin/analytics/game/content-stats?kind=quest&limit=1&from="+g.d0+"&to="+g.d1)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 1 || r.body["data"].([]any)[0].(map[string]any)["abandon_rate"] != nil {
		t.Fatalf("content stats (2 players: rates hidden): %d %s", r.status, r.raw)
	}
	r = get(lo, "/admin/analytics/game/performance?from="+g.d0+"&to="+g.d0)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) == 0 {
		t.Fatalf("performance: %d %s", r.status, r.raw)
	}
	if row := r.body["data"].([]any)[0].(map[string]any); row["fps_p50"].(float64) != 45 || row["fps_budget_pass_rate"].(float64) != 1 {
		t.Fatalf("performance row: %v", row)
	}
	// F080-AC-12: busy and timeout.
	k.queries.Limiter = busyLimiter{}
	if r := get(lo, base+"&from="+g.d0+"&to="+g.d1); r.status != http.StatusTooManyRequests || r.code() != "DASHBOARD_BUSY" ||
		r.header.Get("Retry-After") == "" {
		t.Fatalf("busy: %d %s", r.status, r.raw)
	}
	k.queries.Limiter = nil
	k.queries.Model = slowModel{}
	k.queries.Config.StatementTimeout = 50 * time.Millisecond
	if r := get(lo, base+"&from="+g.d0+"&to="+g.d1); r.status != http.StatusGatewayTimeout || r.code() != "DASHBOARD_QUERY_TIMEOUT" {
		t.Fatalf("timeout: %d %s", r.status, r.raw)
	}
	// F080-AC-14: tenant roles and the Safety Moderator.
	for _, tok := range []string{e.staffToken(e.staff(e.tenantA, "tenant_admin")), e.staffToken(e.platformStaff("safety_moderator"))} {
		if r := get(tok, base+"&from="+g.d0+"&to="+g.d1); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
			t.Fatalf("forbidden: %d %s", r.status, r.raw)
		}
	}
	// The dashboard role never reads the raw events (DASHBOARD_DATA §7).
	dash := open(t, e.cl.ModuleDSN("game", "dashboard"))
	if _, err := dash.ExecContext(ctx, `SELECT 1 FROM analytics.events_raw LIMIT 1`); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("game_dashboard reads events_raw: %v", err)
	}
	tenantTok := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	for _, p := range []string{"/admin/analytics/game/retention?cohort_from=" + g.d0 + "&cohort_to=" + g.d0, "/admin/analytics/game/funnels/onboarding?from=" + g.d0 + "&to=" + g.d0,
		"/admin/analytics/game/content-stats?from=" + g.d0 + "&to=" + g.d0, "/admin/analytics/game/performance?from=" + g.d0 + "&to=" + g.d0} {
		if r := get(tenantTok, p); r.status != http.StatusForbidden {
			t.Fatalf("tenant role %s: %d", p, r.status)
		}
	}
	authztest.Cover(t, "getGameMetrics", authztest.RoleDenied)
	authztest.Cover(t, "getGameRetention", authztest.RoleDenied)
	authztest.Cover(t, "getGameFunnel", authztest.RoleDenied)
	authztest.Cover(t, "getGameContentStats", authztest.RoleDenied)
	authztest.Cover(t, "getGamePerformance", authztest.RoleDenied)
}

// F080-AC-13 (T028; TC-DASH-008): an export is a job (202), the worker writes the CSV and the link is signed for
// dashboard_export_link_ttl_minutes; at most two running per user (429 EXPORT_LIMIT_REACHED); another user cannot read
// the job (404); report.exported is audited.
func TestAnalyticsExport(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	e.closeGolden(ctx, t, k)
	lo := e.staffToken(e.platformStaff("liveops"))
	body := map[string]any{"report": "metrics", "params": map[string]any{"from": g.d0, "to": g.d1, "metric_codes": []any{"dau"},
		"tenant_ids": []any{e.tenantA.String()}}}
	r := e.do(t, http.MethodPost, "/admin/analytics/game/exports", lo, idem(), body)
	if r.status != http.StatusAccepted || r.body["status"] != "queued" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	id := r.body["job_id"].(string)
	if n, err := k.exports.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("worker: %d %v", n, err)
	}
	r = e.do(t, http.MethodGet, "/admin/analytics/game/exports/"+id, lo, nil, nil)
	if r.status != http.StatusOK || r.body["status"] != "succeeded" || !strings.Contains(r.body["download_url"].(string), "expires=900") {
		t.Fatalf("get: %d %s", r.status, r.raw)
	}
	csv := string(k.files.files["game-analytics/"+id+".csv"])
	if !strings.HasPrefix(csv, "metric_code,") || !strings.Contains(csv, "dau,") {
		t.Fatalf("csv: %q", csv)
	}
	if e.audits(ctx, t, "report.exported") != 1 {
		t.Fatal("report.exported")
	}
	other := e.staffToken(e.platformStaff("liveops"))
	if r := e.do(t, http.MethodGet, "/admin/analytics/game/exports/"+id, other, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("other user: %d", r.status)
	}
	for i := 0; i < 2; i++ {
		if r := e.do(t, http.MethodPost, "/admin/analytics/game/exports", lo, idem(), body); r.status != http.StatusAccepted {
			t.Fatalf("queued %d: %d", i, r.status)
		}
	}
	if r := e.do(t, http.MethodPost, "/admin/analytics/game/exports", lo, idem(), body); r.status != http.StatusTooManyRequests ||
		r.code() != "EXPORT_LIMIT_REACHED" {
		t.Fatalf("limit: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/admin/analytics/game/exports", e.staffToken(e.staff(e.tenantA, "tenant_admin")), idem(), body); r.status != http.StatusForbidden {
		t.Fatalf("tenant role: %d", r.status)
	}
	if r := e.do(t, http.MethodGet, "/admin/analytics/game/exports/"+id, e.staffToken(e.staff(e.tenantA, "tenant_admin")), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant role get: %d", r.status)
	}
	authztest.Cover(t, "createGameAnalyticsExport", authztest.RoleDenied)
	authztest.Cover(t, "getGameAnalyticsExport", authztest.RoleDenied, authztest.CrossTenant)
}

// F080-AC-15, AC-16, AC-31, AC-32 (T032, T033, T056, T057; TC-ANL-004, TC-ANL-005, TC-DASH-001): the tenant summary reads
// the session tenant only, from the rollups (raw events added later change nothing), hides small cells with their
// complement (suppressed_cells), keeps a School Admin on its school (another school 403, a school of another tenant 404),
// rejects another tenant (TENANT_MISMATCH), a subject-scoped grant and a role without analytics permission.
func TestTenantGameSummary(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	g := e.seedGolden(ctx, t)
	e.closeGolden(ctx, t, k)
	path := "/tenant/analytics/game/summary?from=" + g.d0 + "&to=" + g.d1
	hA := map[string]string{"X-Tenant-Id": e.tenantA.String()}
	admin := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	r := e.do(t, http.MethodGet, path, admin, hA, nil)
	if r.status != http.StatusOK || r.body["grain"] != "day" {
		t.Fatalf("summary: %d %s", r.status, r.raw)
	}
	if r.body["active_students"].(float64) != 8 || r.body["dau_avg"].(float64) != 6.5 || r.body["quest_completion_rate"].(float64) != 0.5 {
		t.Fatalf("totals: %s", r.raw)
	}
	bySchool := r.body["by_school"].([]any)
	if len(bySchool) != 2 || r.body["suppressed_cells"].(float64) < 4 {
		t.Fatalf("by_school / suppressed: %s", r.raw)
	}
	for _, s := range bySchool {
		if s.(map[string]any)["active_students"] != nil {
			t.Fatalf("school A2 has 3 students: hidden with its complement: %v", bySchool)
		}
	}
	for _, f := range []string{"student_id", "nickname", "currency", "xu_", "total_play"} {
		if strings.Contains(r.raw, f) {
			t.Fatalf("no student list, no economy, no total play time (%s): %s", f, r.raw)
		}
	}
	// AC-31: raw events not rolled up change nothing (the summary never reads events_raw).
	extra := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	a := e.schoolA
	e.insertRaw(ctx, t, e.tenantA, session(e.tenantA, &a, 3, extra.id, "x", ictAt(g.d0, "12:00"), 900, false)...)
	if r2 := e.do(t, http.MethodGet, path, admin, hA, nil); r2.body["active_students"].(float64) != 8 {
		t.Fatalf("summary read raw events: %s", r2.raw)
	}
	// AC-15, AC-32: scopes.
	school := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA)))
	if r := e.do(t, http.MethodGet, path, school, hA, nil); r.status != http.StatusOK || len(r.body["by_school"].([]any)) != 1 {
		t.Fatalf("school admin: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, path+"&school_ids="+e.schoolA2.String(), school, hA, nil); r.status != http.StatusForbidden ||
		r.code() != "PERMISSION_DENIED" {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, path+"&school_ids="+e.schoolB.String(), admin, hA, nil); r.status != http.StatusNotFound {
		t.Fatalf("school of tenant B: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, path, admin, map[string]string{"X-Tenant-Id": e.tenantB.String()}, nil); r.status != http.StatusForbidden ||
		r.code() != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
	subject := e.staffToken(e.staff(e.tenantA, "academic_manager", authztest.ForSubject(uuid.New())))
	if r := e.do(t, http.MethodGet, path, subject, hA, nil); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("subject scope: %d %s", r.status, r.raw)
	}
	editor := e.staffToken(e.staff(e.tenantA, "tenant_content_editor"))
	if r := e.do(t, http.MethodGet, path, editor, hA, nil); r.status != http.StatusForbidden || r.code() != "PERMISSION_DENIED" {
		t.Fatalf("no permission: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "getTenantGameSummary", authztest.RoleDenied, authztest.CrossTenant)
	if r := e.do(t, http.MethodGet, "/tenant/analytics/game/summary?from=2024-11-16&to=2026-11-17", admin, hA, nil); r.status != http.StatusUnprocessableEntity ||
		r.code() != "DASHBOARD_RANGE_TOO_LARGE" {
		t.Fatalf("range: %d %s", r.status, r.raw)
	}
}

// F080-AC-17 (T035; TC-UI-004): the Live dashboard reads the fixed catalog from Prometheus with threshold states; a KPI
// without series is null (never a fake 0); Prometheus down → 503; only game.liveops.read.
func TestLiveOpsOverview(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, envOpts{})
	k := e.setupAnalytics(t, nil)
	k.prom.vals["eduworld_players_online"] = []map[string]any{promSample(nil, "9500")}
	k.prom.vals["eduworld_mm_queue_size"] = []map[string]any{promSample(map[string]string{"mode": "duel"}, "12"),
		promSample(map[string]string{"mode": "team_battle"}, "30")}
	lo := e.staffToken(e.platformStaff("liveops"))
	r := e.do(t, http.MethodGet, "/admin/liveops/overview", lo, nil, nil)
	if r.status != http.StatusOK || r.body["as_of"] == nil {
		t.Fatalf("overview: %d %s", r.status, r.raw)
	}
	kpis := map[string]map[string]any{}
	for _, x := range r.body["kpis"].([]any) {
		m := x.(map[string]any)
		kpis[m["code"].(string)] = m
	}
	if c := kpis["ccu"]; c["value"].(float64) != 9500 || c["status"] != "warning" {
		t.Fatalf("ccu: %v", c)
	}
	if q := kpis["matchmaking_queue"]; q["value"].(float64) != 42 || len(q["breakdown"].([]any)) != 2 {
		t.Fatalf("queue: %v", q)
	}
	if s := kpis["chat_messages_per_minute"]; s["value"] != nil || s["status"] != "unknown" {
		t.Fatalf("no series → null/unknown: %v", s)
	}
	k.prom.down = true
	if r := e.do(t, http.MethodGet, "/admin/liveops/overview", lo, nil, nil); r.status != http.StatusServiceUnavailable || r.code() == "" {
		t.Fatalf("prometheus down: %d %s", r.status, r.raw)
	}
	for _, tok := range []string{e.staffToken(e.staff(e.tenantA, "tenant_admin")), e.staffToken(e.platformStaff("safety_moderator"))} {
		if r := e.do(t, http.MethodGet, "/admin/liveops/overview", tok, nil, nil); r.status != http.StatusForbidden {
			t.Fatalf("forbidden: %d", r.status)
		}
	}
	authztest.Cover(t, "getLiveOpsOverview", authztest.RoleDenied)
	_ = ctx
}

// F080-AC-19 (backend of the event monitor): listEventRuns lists the platform runs for game.liveops.read (keyset).
func TestListEventRuns(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, nil))
	run, _ := e.runOf(ctx, t, ev)
	lo := e.staffToken(e.platformStaff("liveops"))
	r := e.do(t, http.MethodGet, "/game-admin/event-runs?limit=1&status=scheduled&status=live", lo, nil, nil)
	if r.status != http.StatusOK || len(r.body["data"].([]any)) != 1 || r.body["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("list: %d %s", r.status, r.raw)
	}
	if got := r.body["data"].([]any)[0].(map[string]any); got["event_run_id"] != run.String() || got["event_id"] != ev.String() || got["name"] == nil {
		t.Fatalf("run summary: %v", got)
	}
	if r := e.do(t, http.MethodGet, "/game-admin/event-runs?status=ended", lo, nil, nil); r.status != http.StatusOK || len(r.body["data"].([]any)) != 0 {
		t.Fatalf("status filter: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/game-admin/event-runs", e.staffToken(e.staff(e.tenantA, "tenant_admin")), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("tenant role: %d", r.status)
	}
	if r := e.do(t, http.MethodGet, "/game-admin/event-runs", e.staffToken(e.platformStaff("safety_moderator")), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("safety moderator: %d", r.status)
	}
	authztest.Cover(t, "listEventRuns", authztest.RoleDenied)
	if r := e.do(t, http.MethodGet, "/game-admin/event-runs?status=bogus", lo, nil, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("bad status: %d", r.status)
	}
}
