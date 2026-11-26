//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	ldb "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/leaderboard"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/httpxtest"
)

// points appends a Competition Points entry of k at at (ledger fixture of the boards).
func (r *renv) points(k kid, amount int, at time.Time) {
	r.t.Helper()
	r.exec(`INSERT INTO competition.competition_point_ledger (entry_id, tenant_id, school_id, student_id, grade_level, season_id, subject_id,
		mode, amount, source_type, source_id, reason, breakdown, rule_version_id, day_key, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'battle_1v1', $8, 'match', $9, 'completed', '{}', $10, $11::date, $12)`, uuid.New(), k.tenant,
		k.school, k.id, k.grade, preSeason, r.subject, amount, uuid.New(), rankingRulesV1ID, ldb.DayKey(at).Format("2006-01-02"), at)
}

func (r *renv) board(id uuid.UUID, k kid, query string) presp {
	r.t.Helper()
	path := "/leaderboards/" + id.String()
	if query != "" {
		path += "?" + query
	}
	return r.get(k, path)
}

func entriesOf(p presp) []map[string]any {
	es, _ := p.body["entries"].([]any)
	out := make([]map[string]any, 0, len(es))
	for _, e := range es {
		out = append(out, e.(map[string]any))
	}
	return out
}

func nickOf(e map[string]any) string { return e["player"].(map[string]any)["nickname"].(string) }

// T023 — F106-AC-13, AC-14 (TC-CMP-009, AC-LDB-002, AC-LDB-006): the open period lives in a Valkey sorted set fed after
// the ledger write (processed_events first: a replay adds nothing); equal points rank who got there first above; a lost
// board is computed from the ledger with stale = true, queued, rebuilt identical; the reconciliation repairs a drift.
func TestF106BoardRebuildAndReplay(t *testing.T) {
	r := setupRank(t)
	a, b, c := r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7)
	r.duel(a, b, 5, 1) // a 30, b 11
	res := r.board(lbArenaWeekly, a, "")
	if res.status != http.StatusOK || res.body["stale"] != true || len(entriesOf(res)) != 2 || nickOf(entriesOf(res)[0]) != a.nickname {
		t.Fatalf("first read (board not built): %d %s", res.status, res.raw)
	}
	if n, err := r.boards.RebuildQueued(jobCtx()); err != nil || n < 1 {
		t.Fatalf("rebuild: %d %v", n, err)
	}
	built := r.board(lbArenaWeekly, a, "")
	if built.body["stale"] != false || len(entriesOf(built)) != 2 || entriesOf(built)[0]["value"].(float64) != 30 {
		t.Fatalf("built board: %s", built.raw)
	}
	// c beats a: c 26, a 40, b 11
	env := r.duel(c, a, 1, 0)
	r.clock.Add(time.Minute)
	afterTie := r.board(lbArenaWeekly, a, "")
	es := entriesOf(afterTie)
	if len(es) != 3 || es[0]["value"].(float64) != 40 {
		t.Fatalf("after c: %s", afterTie.raw)
	}
	// replay of the match: values unchanged (AC-LDB-006)
	r.played(env)
	if again := r.board(lbArenaWeekly, a, ""); entriesOf(again)[0]["value"].(float64) != 40 {
		t.Fatalf("replay counted twice: %s", again.raw)
	}
	// all Valkey keys lost (FLUSHALL): stale, then rebuilt identical to the ledger (SC-005)
	if err := r.vk.Do(context.Background(), r.vk.B().Flushall().Build()).Error(); err != nil {
		t.Fatal(err)
	}
	lost := r.board(lbArenaWeekly, a, "")
	if lost.body["stale"] != true || len(entriesOf(lost)) != 3 {
		t.Fatalf("lost board: %s", lost.raw)
	}
	if _, err := r.boards.RebuildQueued(jobCtx()); err != nil {
		t.Fatal(err)
	}
	rebuilt := r.board(lbArenaWeekly, a, "")
	if rebuilt.body["stale"] != false || !sameEntries(entriesOf(lost), entriesOf(rebuilt)) {
		t.Fatalf("rebuilt %s vs %s", rebuilt.raw, lost.raw)
	}
	// drift (an increment applied twice by hand): the hourly reconciliation rebuilds the board from the ledger
	k := port.BoardKey{Tenant: r.tenantA, Board: lbArenaWeekly, Scope: ldb.GradeKey(r.schoolA, 7), Period: ldb.Key("weekly", r.clock.Now(), nil)}
	if err := r.vk.Do(context.Background(), r.vk.B().Zincrby().Key(cache.LeaderboardKey(k)).Increment(1e9).Member(b.id.String()).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if n, err := r.boards.Reconcile(jobCtx()); err != nil || n != 1 {
		t.Fatalf("reconcile: %d %v", n, err)
	}
	if fixed := r.board(lbArenaWeekly, a, ""); !sameEntries(entriesOf(fixed), entriesOf(rebuilt)) {
		t.Fatalf("after reconcile: %s", fixed.raw)
	}
}

func sameEntries(a, b []map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i]["rank"] != b[i]["rank"] || a[i]["value"] != b[i]["value"] || nickOf(a[i]) != nickOf(b[i]) {
			return false
		}
	}
	return true
}

// T024 — F106-AC-15 (AC-LDB-003): at 00:00 Monday ICT (+5 min) the week closes: snapshot of the top 100 and of the rank
// of every member of the school/grade scope, the goal of a grade only as a collective row, leaderboard.period_closed
// once per (leaderboard, scope, period); the history and the period_key of the closed week read the snapshot.
func TestF106PeriodClose(t *testing.T) {
	r := setupRank(t)
	var kids []kid
	for i := 0; i < 6; i++ {
		kids = append(kids, r.kid(r.tenantA, r.schoolA, 7))
	}
	at := r.clock.Now()
	for i, k := range kids {
		r.points(k, 100-i*10, at.Add(time.Duration(i)*time.Minute))
	}
	g3 := []kid{r.kid(r.tenantA, r.schoolA, 3), r.kid(r.tenantA, r.schoolA, 3)}
	for _, k := range g3 {
		r.graded(k, false, false)
	}
	// Sunday 23:59 ICT: W47 still open, nothing closes
	r.clock.set(time.Date(2026, 11, 22, 16, 59, 0, 0, time.UTC))
	if n, err := r.boards.ClosePeriods(jobCtx()); err != nil || n != 0 {
		t.Fatalf("before Monday: %d %v", n, err)
	}
	// Monday 00:06 ICT
	r.clock.set(time.Date(2026, 11, 22, 17, 6, 0, 0, time.UTC))
	n, err := r.boards.ClosePeriods(jobCtx())
	if err != nil || n < 2 {
		t.Fatalf("close: %d %v", n, err)
	}
	if again, err := r.boards.ClosePeriods(jobCtx()); err != nil || again != 0 {
		t.Fatalf("second run: %d %v", again, err)
	}
	scope := ldb.GradeKey(r.schoolA, 7)
	if c := r.count(`SELECT count(*) FROM competition.leaderboard_snapshots WHERE leaderboard_id = $1 AND scope_id = $2 AND period_key = '2026-W47'
		AND rank IS NOT NULL`, lbArenaWeekly, scope); c != 6 {
		t.Fatalf("arena snapshot rows %d", c)
	}
	gg := ldb.GradeKey(r.schoolA, 3)
	if c := r.count(`SELECT count(*) FROM competition.leaderboard_snapshots WHERE leaderboard_id = $1 AND scope_id = $2 AND student_id IS NULL
		AND rank IS NULL AND members = 2`, lbGradeGoal, gg); c != 1 {
		t.Fatalf("grade goal snapshot rows %d", c)
	}
	if c := r.count(`SELECT count(*) FROM competition.leaderboard_snapshots WHERE leaderboard_id = $1 AND student_id IS NOT NULL`, lbGradeGoal); c != 0 {
		t.Fatal("the goal of a grade stored personal rows")
	}
	closed := r.events("leaderboard.period_closed")
	byScope := map[string]int{}
	for _, e := range closed {
		byScope[e["scope_id"].(string)+"|"+e["period_key"].(string)]++
		if e["metric"] == "grade_goal" && len(e["top"].([]any)) != 0 {
			t.Fatalf("grade goal event with a ranking: %+v", e)
		}
	}
	if byScope[scope+"|2026-W47"] != 1 || byScope[gg+"|2026-W47"] != 1 {
		t.Fatalf("period_closed events %+v", byScope)
	}
	if seen := r.checkSchemasOf([]string{"leaderboard.period_closed"}); seen["leaderboard.period_closed"] != len(closed) {
		t.Fatalf("schemas %+v", seen)
	}
	if len(r.audits("leaderboard.period_closed")) < 2 {
		t.Fatal("no audit of the period close")
	}
	// history: the closed week from the snapshot, newest first; the closed period_key reads the snapshot too
	h := r.get(kids[0], "/leaderboards/"+lbArenaWeekly.String()+"/history")
	if h.status != http.StatusOK || len(h.data()) != 1 || h.data()[0]["period_key"] != "2026-W47" || len(h.data()[0]["entries"].([]any)) != 6 {
		t.Fatalf("history: %d %s", h.status, h.raw)
	}
	old := r.board(lbArenaWeekly, kids[5], "period_key=2026-W47")
	me, _ := old.body["me"].(map[string]any)
	if old.status != http.StatusOK || me == nil || me["rank"].(float64) != 6 || len(entriesOf(old)) != 6 {
		t.Fatalf("closed week: %s", old.raw)
	}
	gh := r.get(g3[0], "/leaderboards/"+lbGradeGoal.String()+"/history")
	if gh.status != http.StatusOK || len(gh.data()) != 1 || gh.data()[0]["grade_goal"] == nil || len(gh.data()[0]["entries"].([]any)) != 0 {
		t.Fatalf("grade goal history: %s", gh.raw)
	}
	authztest.Cover(t, "getLeaderboardHistory", authztest.CrossTenant, authztest.RoleDenied)
}

// T021/T025 — F106-AC-16, AC-17 (US-LDB-001, SC-004, AC-LDB-004, AC-LDB-007): grade 3 sees only the goal of its grade:
// collective current/target, its own share, no rank and no nickname of anybody, the contributors cell hidden below 5
// students; effort_points follow the table (rapid guesses earn nothing); every ranked board is closed to grades 1–3.
func TestF106GradeGoal(t *testing.T) {
	r := setupRank(t)
	an, binh, chi := r.kid(r.tenantA, r.schoolA, 3), r.kid(r.tenantA, r.schoolA, 3), r.kid(r.tenantA, r.schoolA, 3)
	// an: 12 activities, 2 rapid guesses, 1 review → 3 + 9·1 + 2 = 14
	for i := 0; i < 12; i++ {
		r.graded(an, i == 4 || i == 7, i == 10)
	}
	r.graded(binh, false, false)
	r.graded(chi, true, false)
	res := r.board(lbGradeGoal, an, "")
	gg, _ := res.body["grade_goal"].(map[string]any)
	if res.status != http.StatusOK || res.body["visibility"] != "grade_goal" || len(entriesOf(res)) != 0 || gg == nil ||
		gg["current"].(float64) != 14+4 || gg["target"].(float64) != 300 || gg["my_contribution"].(float64) != 14 || gg["contributors_count"] != nil {
		t.Fatalf("grade goal: %d %s", res.status, res.raw)
	}
	for _, other := range []kid{binh, chi} {
		if strings.Contains(res.raw, other.nickname) || strings.Contains(res.raw, other.id.String()) {
			t.Fatalf("grade goal shows another student: %s", res.raw)
		}
	}
	if strings.Contains(res.raw, `"rank"`) {
		t.Fatalf("grade goal has a rank: %s", res.raw)
	}
	for i := 0; i < 3; i++ {
		r.graded(r.kid(r.tenantA, r.schoolA, 3), false, false)
	}
	if gg := r.board(lbGradeGoal, an, "").body["grade_goal"].(map[string]any); gg["contributors_count"].(float64) != 5 {
		t.Fatalf("5 contributors: %+v", gg)
	}
	l := r.get(an, "/leaderboards")
	if l.status != http.StatusOK || len(l.data()) != 1 || l.data()[0]["code"] != "grade_goal_weekly" || l.data()[0]["grade_level"].(float64) != 3 {
		t.Fatalf("grade 3 list: %s", l.raw)
	}
	for _, id := range []uuid.UUID{lbArenaWeekly, lbArenaSeason, lbFriendsPoints} {
		if res := r.board(id, an, ""); res.status != http.StatusNotFound {
			t.Fatalf("grade 3 opens a ranked board %s: %d", id, res.status)
		}
	}
	// a grade 3 student never gets a personal rank even if the platform publishes a ranked board for grade 3
	r.exec(`INSERT INTO competition.leaderboard_definitions (leaderboard_id, code, title, metric, scope_type, period, filters, grade_levels, display)
		VALUES ($1, 'g3_points', 'Điểm khối 3', 'competition_points', 'grade', 'weekly', '{}', '[3]', '{}')`, uuid.New())
	if l := r.get(an, "/leaderboards"); len(l.data()) != 1 {
		t.Fatalf("ranked board listed for grade 3: %s", l.raw)
	}
	// a definition the rules forbid is never materialized (XP inside a school, class scope — F106-AC-12)
	r.exec(`UPDATE competition.game_config_versions SET status = 'archived' WHERE config_version_id = '01936a00-0000-7000-8000-000000001071'`)
	v2 := uuid.New()
	r.exec(`INSERT INTO competition.game_config_versions (config_version_id, config_id, version_no, status, title, payload, published_at)
		VALUES ($1, '01936a00-0000-7000-8000-000000000107', 2, 'published', 'v2', $2, now())`, v2,
		`{"schema_version":1,"definitions":[{"leaderboard_id":"`+uuid.NewString()+`","code":"xp_school","title":"XP trường","metric":"xp","scope_type":"school","period":"weekly","filters":{},"grade_levels":[6],"display":{},"sort_order":1,"status":"active"},`+
			`{"leaderboard_id":"`+uuid.NewString()+`","code":"class_x","title":"Lớp","metric":"competition_points","scope_type":"class","period":"weekly","filters":{},"grade_levels":[6],"display":{},"sort_order":2,"status":"active"},`+
			`{"leaderboard_id":"`+uuid.NewString()+`","code":"daily_g7","title":"Ngày","metric":"competition_points","scope_type":"grade","period":"daily","filters":{},"grade_levels":[7],"display":{},"sort_order":3,"status":"active"}]}`)
	// the runtime reads the release timeline (F108, ADR-0043): the publish release of v2
	r.exec(`INSERT INTO competition.content_releases (release_id, created_at, config_id, action, to_version_id, reason, released_at)
		VALUES ($1, clock_timestamp(), '01936a00-0000-7000-8000-000000000107', 'publish', $2, 'test', now())`, uuid.New(), v2)
	if n, err := r.boards.Materialize(jobCtx()); err != nil || n != 1 {
		t.Fatalf("materialize: %d %v", n, err)
	}
	if c := r.count(`SELECT count(*) FROM competition.leaderboard_definitions WHERE code IN ('xp_school','class_x')`); c != 0 {
		t.Fatal("a forbidden definition was materialized")
	}
	authztest.Cover(t, "listLeaderboards", authztest.CrossTenant, authztest.RoleDenied)
}

// T022/T025 — F106-AC-17, AC-18 (AC-LDB-005): grades 4–5 see the top 10 and "nhóm X%" outside it, never the last member;
// grades 6–9 the top 50 and ±3 around them; only nickname/avatar (no real name, no school, no class); a sanctioned student
// is out of the period; a board across schools lists only schools allowing it.
func TestF106VisibilityPrivacy(t *testing.T) {
	r := setupRank(t)
	at := r.clock.Now().Add(-time.Hour)
	var g4 []kid
	for i := 0; i < 12; i++ {
		k := r.kid(r.tenantA, r.schoolA, 4)
		g4 = append(g4, k)
		r.points(k, 200-i*10, at.Add(time.Duration(i)*time.Second))
	}
	last := g4[11]
	res := r.board(lbArenaWeekly, last, "")
	me, _ := res.body["me"].(map[string]any)
	if res.status != http.StatusOK || res.body["visibility"] != "top_n_percentile" || len(entriesOf(res)) != 10 || me == nil || me["rank"] != nil ||
		me["percentile_bucket"].(float64) != 100 {
		t.Fatalf("grade 4 last: %s", res.raw)
	}
	top := r.board(lbArenaWeekly, g4[0], "")
	if m := top.body["me"].(map[string]any); m["rank"].(float64) != 1 {
		t.Fatalf("grade 4 first: %s", top.raw)
	}
	noPII(t, top)
	for _, k := range []string{"student_code", "full_name", "class_name"} {
		if strings.Contains(top.raw, k) {
			t.Fatalf("leaks %s", k)
		}
	}
	var g7 []kid
	for i := 0; i < 60; i++ {
		k := r.kid(r.tenantA, r.schoolA, 7)
		g7 = append(g7, k)
		r.points(k, 1000-i*10, at.Add(time.Duration(i)*time.Second))
	}
	res = r.board(lbArenaWeekly, g7[54], "")
	around := res.body["around_me"].([]any)
	if res.body["visibility"] != "ranked" || len(entriesOf(res)) != 50 || res.body["me"].(map[string]any)["rank"].(float64) != 55 || len(around) != 7 ||
		around[0].(map[string]any)["rank"].(float64) != 52 {
		t.Fatalf("grade 7: entries %d %s", len(entriesOf(res)), res.raw[:min(400, len(res.raw))])
	}
	// a sanctioned student (restrict_competition) is out of the period
	r.exec(`INSERT INTO competition.competition_restrictions (restriction_id, tenant_id, school_id, student_id, kind, reason, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, 'restrict_competition', 'gian lận', $5, $6)`, uuid.New(), r.tenantA, r.schoolA, g7[0].id,
		r.clock.Now().Add(-time.Minute), r.clock.Now().Add(24*time.Hour))
	if res := r.board(lbArenaWeekly, g7[1], ""); nickOf(entriesOf(res)[0]) == g7[0].nickname {
		t.Fatal("a sanctioned student is still listed")
	}
	// tenant board across schools (unit tenant U): only the schools allowing allow_cross_school_listing appear
	r.school(r.tenantU, r.schoolU1, map[string]any{"allow_cross_school_listing": true})
	r.school(r.tenantU, r.schoolU2, map[string]any{"allow_cross_school_listing": false})
	lbTenant := uuid.New()
	r.exec(`INSERT INTO competition.leaderboard_definitions (leaderboard_id, code, title, metric, scope_type, period, filters, grade_levels, display)
		VALUES ($1, 'tenant_monthly', 'Toàn đơn vị', 'competition_points', 'tenant', 'monthly', '{}', '[7]', '{}')`, lbTenant)
	u1, u2 := r.kid(r.tenantU, r.schoolU1, 7), r.kid(r.tenantU, r.schoolU2, 7)
	r.points(u1, 50, at)
	r.points(u2, 80, at)
	res = r.board(lbTenant, u1, "")
	if res.status != http.StatusOK || len(entriesOf(res)) != 1 || nickOf(entriesOf(res)[0]) != u1.nickname {
		t.Fatalf("cross-school: %d %s", res.status, res.raw)
	}
	if res := r.board(lbTenant, u2, ""); res.status != http.StatusNotFound {
		t.Fatalf("a school without cross-school listing opens the tenant board: %d", res.status)
	}
	// the global board stays off (platform: retired; cross-tenant pending — Clarifications 2026-11-22)
	if res := r.board(lbGlobalSeason, g7[1], ""); res.status != http.StatusNotFound {
		t.Fatalf("global board served: %d", res.status)
	}
	authztest.Cover(t, "getLeaderboard", authztest.CrossTenant, authztest.RoleDenied)
}

// T025 — F106-AC-19: a school turns off a board of its school/grade scope (disabled_leaderboard_definitions, also the goal
// of a grade; school_leaderboard_enabled = false turns off all of them): not listed, 404 when called directly.
func TestF106SchoolPolicy(t *testing.T) {
	r := setupRank(t)
	an, dung := r.kid(r.tenantA, r.schoolA, 3), r.kid(r.tenantA, r.schoolA, 7)
	r.school(r.tenantA, r.schoolA, map[string]any{"disabled_leaderboard_definitions": []string{"grade_goal_weekly"}})
	if l := r.get(an, "/leaderboards"); len(l.data()) != 0 {
		t.Fatalf("disabled board listed: %s", l.raw)
	}
	if res := r.board(lbGradeGoal, an, ""); res.status != http.StatusNotFound {
		t.Fatalf("disabled goal: %d", res.status)
	}
	if res := r.board(lbArenaWeekly, dung, ""); res.status != http.StatusOK {
		t.Fatalf("other board still on: %d %s", res.status, res.raw)
	}
	r.school(r.tenantA, r.schoolA, map[string]any{"school_leaderboard_enabled": false})
	for _, c := range r.get(dung, "/leaderboards").data() {
		if c["scope_type"] == "school" || c["scope_type"] == "grade" {
			t.Fatalf("school boards off but listed: %+v", c)
		}
	}
	if res := r.board(lbArenaWeekly, dung, ""); res.status != http.StatusNotFound {
		t.Fatalf("school boards off: %d", res.status)
	}
}

// T026 — F106-AC-20, AC-21 (TC-TEN-001, TC-TEN-003, TC-TEN-011, TC-TEN-024): a student reads only the scope holding it
// (another grade, school or tenant → 404); staff with lms.student.read reads the school/grade boards of the schools of its
// grants; staff with a game token → GAME_ACCESS_STUDENT_ONLY; X-Tenant-Id of another tenant → TENANT_MISMATCH; a
// period_key over maxLength → 422.
func TestF106LeaderboardAuthzTenant(t *testing.T) {
	r := setupRank(t)
	an, em := r.kid(r.tenantA, r.schoolA, 3), r.kid(r.tenantA, r.schoolA, 4)
	a2 := r.kid(r.tenantA, r.schoolA2, 7)
	dung := r.kid(r.tenantA, r.schoolA, 7)
	bk := r.kid(r.tenantB, r.schoolB, 7)
	r.points(dung, 40, r.clock.Now())
	r.graded(an, false, false)
	if res := r.board(lbGradeGoal, em, "grade_level=3"); res.status != http.StatusNotFound {
		t.Fatalf("grade 4 reads the goal of grade 3: %d", res.status)
	}
	if res := r.board(lbArenaWeekly, a2, "scope_id="+r.schoolA.String()); res.status != http.StatusNotFound {
		t.Fatalf("school A2 reads school A: %d", res.status)
	}
	if res := r.board(lbArenaWeekly, bk, "scope_id="+r.schoolA.String()); res.status != http.StatusNotFound {
		t.Fatalf("tenant B reads school A: %d", res.status)
	}
	if res := r.board(lbArenaWeekly, bk, ""); res.status != http.StatusOK || len(entriesOf(res)) != 0 {
		t.Fatalf("tenant B own board: %d %s", res.status, res.raw)
	}
	staff := r.staffAt(r.tenantA, r.schoolA, httpxtest.PortalClient)
	q := "?scope_id=" + r.schoolA.String() + "&grade_level=7"
	res := r.staffGet(staff, r.tenantA, "/leaderboards/"+lbArenaWeekly.String()+q)
	if res.status != http.StatusOK || len(entriesOf(res)) != 1 || nickOf(entriesOf(res)[0]) != dung.nickname || res.body["visibility"] != "ranked" {
		t.Fatalf("staff of school A: %d %s", res.status, res.raw)
	}
	noPII(t, res)
	gg := r.staffGet(staff, r.tenantA, "/leaderboards/"+lbGradeGoal.String()+"?scope_id="+r.schoolA.String()+"&grade_level=3")
	if g, _ := gg.body["grade_goal"].(map[string]any); gg.status != http.StatusOK || g == nil || g["my_contribution"] != nil || g["current"].(float64) != 4 {
		t.Fatalf("staff grade goal: %d %s", gg.status, gg.raw)
	}
	if l := r.staffGet(staff, r.tenantA, "/leaderboards"); l.status != http.StatusOK || len(l.data()) < 3 {
		t.Fatalf("staff list: %d %s", l.status, l.raw)
	}
	if res := r.staffGet(r.staffAt(r.tenantA, r.schoolA2, httpxtest.PortalClient), r.tenantA, "/leaderboards/"+lbArenaWeekly.String()+q); res.status != http.StatusNotFound {
		t.Fatalf("staff of school A2: %d", res.status)
	}
	if res := r.staffGet(r.staffToken(r.tenantB, "school_admin"), r.tenantB, "/leaderboards/"+lbArenaWeekly.String()+q); res.status != http.StatusNotFound {
		t.Fatalf("staff of tenant B: %d %s", res.status, res.raw)
	}
	if res := r.staffGet(staff, r.tenantA, "/leaderboards/"+lbArenaWeekly.String()+"?scope_id="+r.schoolA.String()); res.status != http.StatusUnprocessableEntity {
		t.Fatalf("staff without grade_level: %d", res.status)
	}
	if res := r.staffGet(r.staffAt(r.tenantA, r.schoolA, httpxtest.GameClient), r.tenantA, "/leaderboards"); res.status != http.StatusForbidden ||
		res.code() != "GAME_ACCESS_STUDENT_ONLY" {
		t.Fatalf("staff with a game token: %d %s", res.status, res.raw)
	}
	if res := r.staffGet(staff, r.tenantA, "/leaderboards/"+lbArenaWeekly.String()+"/me"); res.status != http.StatusForbidden {
		t.Fatalf("getMyLeaderboardPosition by staff: %d", res.status)
	}
	if res := r.do(http.MethodGet, "/leaderboards/"+lbArenaWeekly.String(), dung.token, map[string]string{"X-Tenant-Id": r.tenantB.String()}, nil, ""); res.status !=
		http.StatusForbidden || res.code() != "TENANT_MISMATCH" {
		t.Fatalf("tenant mismatch: %d %s", res.status, res.raw)
	}
	if res := r.board(lbArenaWeekly, dung, "period_key="+strings.Repeat("9", 100)); res.status != http.StatusUnprocessableEntity {
		t.Fatalf("period_key 100 chars: %d", res.status)
	}
	if res := r.board(lbArenaWeekly, dung, "period_key=2026-W99"); res.status != http.StatusUnprocessableEntity {
		t.Fatalf("malformed period_key: %d", res.status)
	}
	me := r.get(dung, "/leaderboards/"+lbArenaWeekly.String()+"/me")
	if me.status != http.StatusOK || len(entriesOf(me)) != 0 || me.body["me"].(map[string]any)["rank"].(float64) != 1 {
		t.Fatalf("my position: %d %s", me.status, me.raw)
	}
	if res := r.get(dung, "/leaderboards/"+uuid.NewString()); res.status != http.StatusNotFound {
		t.Fatalf("unknown board: %d", res.status)
	}
	authztest.Cover(t, "getMyLeaderboardPosition", authztest.CrossTenant, authztest.RoleDenied)
}

// Owner rule 2026-11-23 (API_DESIGN §5): the stale path of getLeaderboard (board not built in Valkey) ranks and cuts the
// board in SQL — staff pages of 2 walk the whole board in order without duplicates or gaps, the count comes from the
// same query, a student's own place (getMyLeaderboardPosition) is ranked in SQL too, and tenant B never sees tenant A.
func TestF106StaleBoardWindowPages(t *testing.T) {
	r := setupRank(t)
	now := r.clock.Now()
	ks := []kid{r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7), r.kid(r.tenantA, r.schoolA, 7),
		r.kid(r.tenantA, r.schoolA, 7)}
	// 50, 40 (first), 40 (later), 30, 10 → expected order ks[0..4]
	r.points(ks[0], 50, now.Add(-5*time.Minute))
	r.points(ks[1], 40, now.Add(-4*time.Minute))
	r.points(ks[2], 40, now.Add(-3*time.Minute))
	r.points(ks[3], 30, now.Add(-2*time.Minute))
	r.points(ks[4], 10, now.Add(-1*time.Minute))
	bk := r.kid(r.tenantB, r.schoolB, 7)
	r.points(bk, 99, now)
	staff := r.staffAt(r.tenantA, r.schoolA, httpxtest.PortalClient)
	base := "/leaderboards/" + lbArenaWeekly.String() + "?scope_id=" + r.schoolA.String() + "&grade_level=7&limit=2"
	var got []string
	ranks := []float64{}
	cursor := ""
	for i := 0; i < 5; i++ {
		path := base
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		res := r.staffGet(staff, r.tenantA, path)
		if res.status != http.StatusOK || res.body["stale"] != true {
			t.Fatalf("page %d: %d %s", i+1, res.status, res.raw)
		}
		es := entriesOf(res)
		if len(es) > 2 {
			t.Fatalf("page %d larger than the limit: %s", i+1, res.raw)
		}
		for _, e := range es {
			got = append(got, nickOf(e))
			ranks = append(ranks, e["rank"].(float64))
		}
		pg := res.body["page"].(map[string]any)
		if pg["has_more"] != true {
			if pg["next_cursor"] != nil {
				t.Fatalf("last page with a cursor: %s", res.raw)
			}
			break
		}
		if len(es) != 2 {
			t.Fatalf("short page with has_more: %s", res.raw)
		}
		cursor = pg["next_cursor"].(string)
	}
	want := []string{ks[0].nickname, ks[1].nickname, ks[2].nickname, ks[3].nickname, ks[4].nickname}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pages = %v, want %v", got, want)
	}
	for i, rk := range ranks {
		if int(rk) != i+1 {
			t.Fatalf("ranks %v: gap or duplicate", ranks)
		}
	}
	// the viewer's own place is ranked in SQL (rank 4 of 5), never another tenant's member
	me := r.board(lbArenaWeekly, ks[3], "")
	if m, _ := me.body["me"].(map[string]any); me.status != http.StatusOK || m == nil || m["rank"].(float64) != 4 {
		t.Fatalf("student rank: %d %s", me.status, me.raw)
	}
	if strings.Contains(me.raw, bk.nickname) {
		t.Fatalf("tenant B member on a tenant A board: %s", me.raw)
	}
	pos := r.get(ks[3], "/leaderboards/"+lbArenaWeekly.String()+"/me")
	if m, _ := pos.body["me"].(map[string]any); pos.status != http.StatusOK || m == nil || m["rank"].(float64) != 4 {
		t.Fatalf("getMyLeaderboardPosition: %d %s", pos.status, pos.raw)
	}
	// tenant B: its own board holds its member only
	if res := r.board(lbArenaWeekly, bk, ""); res.status != http.StatusOK || strings.Contains(res.raw, ks[0].nickname) {
		t.Fatalf("tenant B board: %d %s", res.status, res.raw)
	}
}
