//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	readrepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// dashReady delivers the golden dataset, seeds the state sources and runs the refresh and close jobs.
func (c *curEnv) dashReady(ctx context.Context, t *testing.T) *dashKit {
	t.Helper()
	k := c.dashKit(ctx, t)
	k.golden(ctx, t)
	k.seedState(ctx, t)
	if _, err := k.jobs.CloseDays(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := k.jobs.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	return k
}

func (r resp) obj(path ...string) map[string]any {
	var cur any = r.body
	for _, p := range path {
		m, _ := cur.(map[string]any)
		cur = m[p]
	}
	m, _ := cur.(map[string]any)
	return m
}

func list(m map[string]any, key string) []map[string]any {
	arr, _ := m[key].([]any)
	out := []map[string]any{}
	for _, x := range arr {
		if o, ok := x.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

func num(m map[string]any, key string) float64 {
	f, _ := m[key].(float64)
	return f
}

// forbiddenFields are never part of a learning dashboard answer (F043-AC-11, F043-AC-24, ADR-0018, R16).
var forbiddenFields = regexp.MustCompile(`"(xp|game_rating|rating|rank|ranking|chat|messages|friends|position)"\s*:`)

func noGameOrChat(t *testing.T, r resp) {
	t.Helper()
	if m := forbiddenFields.FindString(r.raw); m != "" {
		t.Fatalf("dashboard answer carries %s: %s", m, r.raw)
	}
}

const dashRange = "from=2026-12-04&to=2026-12-10"

// F043-AC-07, F043-AC-08, F043-AC-09, F043-AC-11 (TC-LMS-003): one call returns the assignment, weak objective, attention,
// self-study and safety blocks with grain/data_as_of/is_partial; numbers match the reconciliation of the golden dataset;
// no XP, rating, ranking nor chat.
func TestSchoolDashboardWidgets(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	r := must(t, c.do(t, http.MethodGet, "/lms/dashboards/school?school_id="+k.school.String()+"&"+dashRange, adS, nil, nil), http.StatusOK, "school dashboard")
	noGameOrChat(t, r)
	if r.str("grain") != "day" || r.str("is_partial") != "true" || r.str("data_as_of") == "" {
		t.Fatalf("meta: %s %s %s", r.str("grain"), r.str("is_partial"), r.str("data_as_of"))
	}
	w := r.obj("widgets")
	// Reconciliation: school S days 12-07, 12-08: accuracy (12+3+6+7)/(14+6+7+10) = 28/37, rapid 6/43.
	if num(w, "accuracy") != 0.757 || num(w, "rapid_guess_ratio") != 0.14 || num(w, "active_students") != 3 {
		t.Fatalf("totals: %v %v %v", w["accuracy"], w["rapid_guess_ratio"], w["active_students"])
	}
	if num(w, "assignments_open") != 1 || num(w, "assignments_due_soon") != 1 || len(list(w, "assignments")) != 3 {
		t.Fatalf("assignments: %v %v %v", w["assignments_open"], w["assignments_due_soon"], len(list(w, "assignments")))
	}
	open := list(w, "assignments")[2]
	if open["status"] != "open" || num(open, "submitted") != 1 || num(open, "total") != 2 || len(list(open, "by_class")) != 1 {
		t.Fatalf("open assignment: %v", open)
	}
	if w["safety_notifications_unacknowledged"] != nil {
		t.Fatal("safety count must be null until module chat provides it (R17)")
	}
	if len(list(w, "classes")) != 2 || num(w, "assignment_completion_rate") != 0.25 {
		t.Fatalf("classes %d completion %v", len(list(w, "classes")), w["assignment_completion_rate"])
	}
	weak := list(w, "weak_objectives")
	if len(weak) != 3 || weak[0]["code"] != "DMATH.G3.NUM.SUB" || num(weak[0], "below_proficient_ratio") != 1 {
		t.Fatalf("weak objectives %v", weak)
	}
	// F043-AC-08: a1 (3 objectives beginning), a2 (rapid guesses, shown as "làm quá nhanh"), b1 (2 missed in 14 days).
	att := list(w, "students_needing_attention")
	got := map[string]string{}
	for _, a := range att {
		got[a["student_id"].(string)] = fmt.Sprint(a["reasons"])
	}
	if num(w, "students_needing_attention_total") != 3 || got[k.a1.String()] != "[beginning_objectives]" ||
		got[k.a2.String()] != "[rapid_guess]" || got[k.b1.String()] != "[missed_assignments]" {
		t.Fatalf("attention %v", got)
	}
	// F043-AC-09: 2220 s of learning / 3 students = 12.3 min; no recommendation in the dataset → null rate.
	self := r.obj("widgets", "self_study")
	if num(self, "avg_active_minutes_per_student") != 12.3 || self["recommendation_started_rate"] != nil ||
		num(self, "objectives_improved_after_recommendation") != 0 {
		t.Fatalf("self study %v", self)
	}
	if len(list(w, "active_minutes_series")) != 7 {
		t.Fatalf("series of 7 days: %d", len(list(w, "active_minutes_series")))
	}
	// A closed range is final.
	r = must(t, c.do(t, http.MethodGet, "/lms/dashboards/school?school_id="+k.school.String()+"&from=2026-12-01&to=2026-12-08", adS, nil, nil),
		http.StatusOK, "closed range")
	if r.str("is_partial") != "false" {
		t.Fatalf("closed range partial: %s", r.raw)
	}
}

// F043-AC-10, F043-AC-12, F043-AC-16, F043-AC-17 (TC-LMS-003, TC-DASH-002, TC-DASH-003): class heatmap and cursor pages
// limited to dashboard_page_size_max; series of at most 120 points with the grain of the range; range and grain errors.
func TestClassDashboardAndSeries(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	base := "/lms/classes/" + k.class3A.String() + "/dashboard?" + dashRange
	r := must(t, c.do(t, http.MethodGet, base+"&page_size=1", adS, nil, nil), http.StatusOK, "class page 1")
	noGameOrChat(t, r)
	page := r.obj("students", "page")
	data := list(r.obj("students"), "data")
	if len(data) != 1 || page["has_more"] != true || page["next_cursor"] == nil {
		t.Fatalf("page 1: %v", r.obj("students"))
	}
	first := data[0]["student_id"].(string)
	r2 := must(t, c.do(t, http.MethodGet, base+"&page_size=1&cursor="+page["next_cursor"].(string), adS, nil, nil), http.StatusOK, "page 2")
	data2 := list(r2.obj("students"), "data")
	if len(data2) != 1 || data2[0]["student_id"] == first || r2.obj("students", "page")["has_more"] != false {
		t.Fatalf("page 2: %v", r2.obj("students"))
	}
	byID := map[string]map[string]any{data[0]["student_id"].(string): data[0], data2[0]["student_id"].(string): data2[0]}
	if a2 := byID[k.a2.String()]; a2["rapid_guess_flag"] != true || fmt.Sprint(a2["reasons"]) != "[rapid_guess]" {
		t.Fatalf("a2 row: %v", a2)
	}
	heat := list(r.body, "objective_distribution")
	if len(heat) != 3 {
		t.Fatalf("heatmap columns: %d", len(heat))
	}
	for _, h := range heat {
		if h["code"] == "DMATH.G3.NUM.ADD" && fmt.Sprint(h["level_counts"]) != "map[beginning:2 developing:0 mastered:0 not_started:0 proficient:0]" {
			t.Fatalf("heat O1: %v", h)
		}
	}
	// page_size above dashboard_page_size_max is limited, not refused (F043-AC-16).
	r = must(t, c.do(t, http.MethodGet, base+"&page_size=500", adS, nil, nil), http.StatusOK, "page size 500")
	if num(r.obj("students", "page"), "limit") != 100 {
		t.Fatalf("limit %v", r.obj("students", "page"))
	}
	series := "/lms/schools/" + k.school.String() + "/analytics/series?metric="
	for _, cs := range []struct {
		from, to, grain string
		points          int
	}{{"2026-11-10", "2026-12-10", "day", 31}, {"2026-09-12", "2026-12-10", "week", 14}, {"2024-12-11", "2026-12-10", "month", 25}} {
		r = must(t, c.do(t, http.MethodGet, series+"accuracy&from="+cs.from+"&to="+cs.to, adS, nil, nil), http.StatusOK, "series "+cs.grain)
		if r.str("grain") != cs.grain || len(r.items("points")) != cs.points || len(r.items("points")) > 120 {
			t.Fatalf("series %s: grain %s, %d points", cs.grain, r.str("grain"), len(r.items("points")))
		}
	}
	r = must(t, c.do(t, http.MethodGet, series+"active_students&from=2026-12-07&to=2026-12-08", adS, nil, nil), http.StatusOK, "active series")
	if pts := r.items("points"); num(pts[0], "value") != 3 || num(pts[1], "value") != 2 || pts[0]["is_final"] != true {
		t.Fatalf("active students per day: %v", pts)
	}
	// Coarser grain hint accepted, finer refused; 731 days refused (F043-AC-16, AC-DASH-002).
	must(t, c.do(t, http.MethodGet, series+"accuracy&from=2026-12-01&to=2026-12-10&grain=month", adS, nil, nil), http.StatusOK, "coarser")
	for _, q := range []string{"accuracy&from=2026-09-12&to=2026-12-10&grain=day", "accuracy&from=2024-12-10&to=2026-12-10",
		"accuracy&from=2026-12-10&to=2026-12-01"} {
		r = c.do(t, http.MethodGet, series+q, adS, nil, nil)
		if r.status != http.StatusUnprocessableEntity || r.str("code") != "DASHBOARD_RANGE_TOO_LARGE" {
			t.Fatalf("%s: %d %s", q, r.status, r.raw)
		}
	}
	// A missing bound is refused by the contract (required parameter) before the use case [Proposed].
	if r = c.do(t, http.MethodGet, series+"accuracy&to=2026-12-10", adS, nil, nil); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("missing from: %d", r.status)
	}
	if _, err := c.dashq.GetSchoolSeries(k.principalCtx(ctx, adS), query.SeriesInput{SchoolID: k.school, Metric: "accuracy", To: date("2026-12-10")}); code(err) != "DASHBOARD_RANGE_TOO_LARGE" {
		t.Fatalf("missing from at the use case: %v", err)
	}
}

// F043-AC-13, F043-AC-14, F043-AC-15 (TC-TEN-002, TC-TEN-003, TC-DASH-009): School Admin of another school 404, Academic
// Manager outside its subject 403 PERMISSION_DENIED, Safety Moderator / student / no role 403, another tenant 404.
func TestDashboardAuthorizationAndTenant(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	school := "/lms/dashboards/school?school_id=" + k.school.String() + "&" + dashRange
	class := "/lms/classes/" + k.class3A.String() + "/dashboard?" + dashRange
	series := "/lms/schools/" + k.school.String() + "/analytics/series?metric=accuracy&" + dashRange
	profile := "/lms/students/" + k.a1.String() + "/analytics?" + dashRange
	for _, p := range []string{school, class, series, profile} {
		for _, cs := range []struct {
			sub    string
			status int
			code   string
		}{{adS2, 404, "NOT_FOUND"}, {adSB, 404, "NOT_FOUND"}, {smMod, 403, ""}, {hsA1, 403, ""}, {noRoleA, 403, "PERMISSION_DENIED"}} {
			var h hdr
			if cs.sub == smMod { // platform staff naming a tenant: TENANT_MISMATCH (403)
				h = hdr{"X-Tenant-Id": c.tenantA.String()}
			}
			r := c.do(t, http.MethodGet, p, cs.sub, h, nil)
			if r.status != cs.status || (cs.code != "" && r.str("code") != cs.code) {
				t.Fatalf("%s as %s: %d %s", p, cs.sub, r.status, r.raw)
			}
		}
	}
	// Academic Manager limited to subject M: subject V → 403 (dashboards), 404 (profile); M or no subject → math only.
	for _, p := range []string{school, class, series} {
		if r := c.do(t, http.MethodGet, p+"&subject_id="+k.subjV.String(), amM, nil, nil); r.status != 403 || r.str("code") != "PERMISSION_DENIED" {
			t.Fatalf("%s subject V: %d %s", p, r.status, r.raw)
		}
		must(t, c.do(t, http.MethodGet, p+"&subject_id="+k.subjM.String(), amM, nil, nil), http.StatusOK, p+" subject M")
	}
	if r := c.do(t, http.MethodGet, profile+"&subject_id="+k.subjV.String(), amM, nil, nil); r.status != 404 {
		t.Fatalf("profile subject V: %d", r.status)
	}
	r := must(t, c.do(t, http.MethodGet, school, amM, nil, nil), http.StatusOK, "academic manager without subject")
	if num(r.obj("widgets"), "accuracy") != 0.792 { // math only: (12+7)/(14+10) = 19/24
		t.Fatalf("subject-limited totals: %v", r.obj("widgets")["accuracy"])
	}
	// A class of another tenant is not found even for a tenant-wide role.
	if r := c.do(t, http.MethodGet, "/lms/classes/"+k.class3X.String()+"/dashboard?"+dashRange, taDash, nil, nil); r.status != 404 && r.status != 403 {
		t.Fatalf("class of tenant B: %d", r.status)
	}
	authztest.Cover(t, "getSchoolDashboard", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getClassDashboard", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getSchoolAnalyticsSeries", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getStudentLearningProfile", authztest.CrossTenant, authztest.RoleDenied)
	// Safety Moderator (platform role) at the use case: no tenant staff → 403 PERMISSION_DENIED (F043-AC-14).
	if _, err := c.dashq.GetSchoolDashboard(k.principalCtx(ctx, smMod), query.SchoolDashboardInput{SchoolID: k.school,
		From: date("2026-12-04"), To: date("2026-12-10")}); code(err) != "PERMISSION_DENIED" {
		t.Fatalf("safety moderator at the use case: %v", err)
	}
}

// F043-AC-19 (TC-DASH-006, AC-DASH-004/005): the 5th concurrent query of a user and a tenant over its limit answer 429
// DASHBOARD_BUSY with Retry-After; a statement over dashboard_statement_timeout is cancelled by the database: 504.
func TestDashboardLimits(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	lim := cache.NewLimiter(nil)
	c.dashq.Limiter = lim
	user := c.users[adS].UserID
	var held []func()
	for i := 0; i < 4; i++ {
		release, ok := lim.Acquire(ctx, user, c.tenantA, 4, 30)
		if !ok {
			t.Fatalf("slot %d refused", i)
		}
		held = append(held, release)
	}
	school := "/lms/dashboards/school?school_id=" + k.school.String() + "&" + dashRange
	r := c.do(t, http.MethodGet, school, adS, nil, nil)
	if r.status != http.StatusTooManyRequests || r.str("code") != "DASHBOARD_BUSY" || r.header.Get("Retry-After") != "1" {
		t.Fatalf("5th query: %d %s %q", r.status, r.raw, r.header.Get("Retry-After"))
	}
	held[0]()
	must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "after release")
	for _, f := range held[1:] {
		f()
	}
	// Tenant limit: 30 queries of other users of the tenant in flight.
	for i := 0; i < 30; i++ {
		release, _ := lim.Acquire(ctx, uuid.New(), c.tenantA, 4, 30)
		defer release()
	}
	if r := c.do(t, http.MethodGet, school, adS, nil, nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("tenant over its limit: %d", r.status)
	}
	c.dashq.Limiter = nil
	// Timeout: a catalog query that sleeps beyond the statement timeout is cancelled at the database.
	c.dashq.Model = slowModel{DashboardModel: readrepo.DashboardModel{}}
	c.dashq.Config.StatementTimeout = 300 * time.Millisecond
	start := time.Now()
	r = c.do(t, http.MethodGet, school, adS, nil, nil)
	if r.status != http.StatusGatewayTimeout || r.str("code") != "DASHBOARD_QUERY_TIMEOUT" || time.Since(start) > 5*time.Second {
		t.Fatalf("timeout: %d %s after %v", r.status, r.raw, time.Since(start))
	}
}

// slowModel is a TEST DOUBLE of the catalog whose school lookup sleeps 2 s in the database (statement_timeout check).
type slowModel struct{ readrepo.DashboardModel }

func (slowModel) SchoolExists(ctx context.Context, _, _ uuid.UUID) (bool, error) {
	tx, _ := entx.Tx(ctx)
	_, err := tx.ExecContext(ctx, `SELECT pg_sleep(2)`)
	return false, readrepo.MapError("slow", err)
}

// F043-AC-20, F043-AC-21 (TC-DASH-007, AC-DASH-008, AC-DASH-012): answers are cached under a key with the tenant and its data
// version; after the day close the old version is never served; without Valkey the numbers come from the database; with
// the replica down the answer is 503 and the primary is never used.
func TestDashboardCacheAndResilience(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	container, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	dc := cache.Dashboard{C: vk}
	c.dashq.Cache = dc
	k.jobs.Versions = dc
	school := "/lms/dashboards/school?school_id=" + k.school.String() + "&from=2026-12-01&to=2026-12-10"
	r := must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "first")
	keys, err := vk.Do(ctx, vk.B().Keys().Pattern("ew:learning:dash:res:*").Build()).AsStrSlice()
	if err != nil || len(keys) != 1 || !strings.HasPrefix(keys[0], "ew:learning:dash:res:"+c.tenantA.String()+":0:") {
		t.Fatalf("cache keys %v %v", keys, err)
	}
	before := num(r.obj("widgets"), "accuracy")
	// A late event within the window changes the closed day 2026-12-09: the cached answer is still served …
	k.run(ctx, t, c.tenantA, k.a1, k.school, k.subjM, ict(2026, 12, 9, 6), "practice", 60, write.ItemCounts{Answered: 10})
	if _, err := k.jobs.CloseDays(ctx); err != nil { // nothing to reclose yet (2026-12-09 had no row): closes it
		t.Fatal(err)
	}
	if v, _ := dc.Version(ctx, c.tenantA); v != 1 {
		t.Fatalf("version after close: %d", v)
	}
	// … until the version bump of the day close: then the new numbers.
	r = must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "after close")
	if after := num(r.obj("widgets"), "accuracy"); after == before {
		t.Fatalf("stale cache after the day close: %v", after)
	}
	// Valkey down: same numbers from the database.
	if err := container.Stop(ctx, nil); err != nil {
		t.Fatal(err)
	}
	r2 := must(t, c.do(t, http.MethodGet, school, adS, nil, nil), http.StatusOK, "without Valkey")
	if num(r2.obj("widgets"), "accuracy") != num(r.obj("widgets"), "accuracy") {
		t.Fatal("numbers differ without Valkey")
	}
	// Replica down: 503, never the business pool.
	dead, err := sql.Open("pgx", "postgres://learning_dashboard:x@127.0.0.1:1/eduworld_learning?sslmode=disable&connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dead.Close() })
	down, err := entx.NewReadSession("learning", entx.DBs{App: dead, Platform: dead})
	if err != nil {
		t.Fatal(err)
	}
	c.dashq.Session, c.dashq.Cache = down, nil
	if r := c.do(t, http.MethodGet, school, adS, nil, nil); r.status != http.StatusServiceUnavailable {
		t.Fatalf("replica down: %d %s", r.status, r.raw)
	}
}

// F043-AC-23, F043-AC-24 (TC-LMS-003, TC-TEN-003): the profile of a student of the school with subjects, topics, 8-week
// trend, assignment history (no answer) and recommendations; audit learning.profile_viewed; 404 outside the scope.
func TestStudentLearningProfile(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	p := "/lms/students/" + k.a1.String() + "/analytics?" + dashRange
	r := must(t, c.do(t, http.MethodGet, p, adS, nil, nil), http.StatusOK, "profile")
	noGameOrChat(t, r)
	subjects := r.items("subjects")
	if len(subjects) != 1 || num(subjects[0], "activities_completed") != 2 || num(subjects[0], "active_minutes") != 15 {
		t.Fatalf("subjects %v", subjects)
	}
	topics := list(subjects[0], "mastery_by_topic")
	if len(topics) != 1 || fmt.Sprint(topics[0]["level_dist"]) != "[0 3 0 0 0]" {
		t.Fatalf("topics %v", topics)
	}
	if len(r.items("trend_weeks")) != 1 || len(r.items("recent_attempts")) != 1 || r.items("recent_attempts")[0]["status"] != "submitted" {
		t.Fatalf("trend %v history %v", r.items("trend_weeks"), r.items("recent_attempts"))
	}
	if strings.Contains(r.raw, "response") || strings.Contains(r.raw, "answer") {
		t.Fatalf("profile leaks answers: %s", r.raw)
	}
	if n := c.audits(ctx, t, "learning.profile_viewed", &c.tenantA); n != 1 {
		t.Fatalf("profile_viewed audits: %d", n)
	}
	for _, sub := range []string{adS2, adSB} {
		if r := c.do(t, http.MethodGet, p, sub, nil, nil); r.status != 404 {
			t.Fatalf("profile as %s: %d", sub, r.status)
		}
	}
	if n := c.audits(ctx, t, "learning.profile_viewed", &c.tenantA); n != 1 {
		t.Fatalf("a refused read was audited as viewed: %d", n)
	}
}

// F043-AC-25, F043-AC-26, F043-AC-27 (TC-DASH-009, TC-TEN-004, TC-TEN-011, TC-TEN-018, AC-DASH-011): tenant summary of the
// session tenant with cells < 5 students hidden and totals over visible cells; tenant_ids of a tenant user ignored; System
// Admin cross-tenant summary with tenant_id per row, audit cross_tenant.accessed, QA tenants excluded; tenant roles refused on
// /admin/analytics/*.
func TestTenantAndPlatformSummary(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashKit(ctx, t)
	k.golden(ctx, t)
	// Three more grade-3 students in 3A so that school S × grade 3 has 6 students (visible), S2 × grade 4 keeps 1 (hidden).
	for i := 0; i < 3; i++ {
		id := uuid.New()
		if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.students (student_id, tenant_id, school_id, grade_level, source_version)
			VALUES ($1, $2, $3, 3, 1)`, id, c.tenantA, k.school); err != nil {
			t.Fatal(err)
		}
		if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.enrollments (student_id, class_id, start_date, tenant_id, status, source_version)
			VALUES ($1, $2, date '2026-09-01', $3, 'active', 1)`, id, k.class3A, c.tenantA); err != nil {
			t.Fatal(err)
		}
		k.run(ctx, t, c.tenantA, id, k.school, k.subjM, ict(2026, 12, 8, 9), "practice", 120, write.ItemCounts{Answered: 2, Correct: 2})
	}
	if _, err := k.jobs.CloseDays(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := k.jobs.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	q := "/tenant/analytics/learning/summary?from=2026-12-01&to=2026-12-10&tenant_ids=" + c.tenantB.String()
	r := must(t, c.do(t, http.MethodGet, q, taDash, nil, nil), http.StatusOK, "tenant summary")
	cells := r.items("cells")
	hidden, visible := 0, 0
	for _, cell := range cells {
		if cell["school_id"] != k.school.String() && cell["school_id"] != k.school2.String() {
			t.Fatalf("cell of another tenant: %v", cell)
		}
		if cell["suppressed"] == true {
			hidden++
			if cell["accuracy"] != nil || cell["students"] != nil {
				t.Fatalf("hidden cell carries values: %v", cell)
			}
		} else {
			visible++
		}
	}
	if hidden != 1 || visible != 2 || num(r.body, "suppressed_cells") != 1 {
		t.Fatalf("cells hidden %d visible %d: %s", hidden, visible, r.raw)
	}
	// Totals over the visible cells only (school S): M (12+7+6)/(14+10+6) = 25/30, V 9/13 → 34/43.
	if num(r.body, "accuracy") != 0.791 || len(r.items("by_school")) != 1 {
		t.Fatalf("totals %v by_school %v", r.body["accuracy"], r.items("by_school"))
	}
	// A School Admin reads its school only.
	r = must(t, c.do(t, http.MethodGet, q, adS2, nil, nil), http.StatusOK, "school admin summary")
	for _, cell := range r.items("cells") {
		if cell["school_id"] != k.school2.String() {
			t.Fatalf("school admin sees another school: %v", cell)
		}
	}
	// Platform: System Admin with tenant_ids (no X-Tenant-Id), tenant_id on every row, audit cross_tenant.accessed.
	audits := c.audits(ctx, t, "cross_tenant.accessed", nil)
	p := "/admin/analytics/learning/summary?from=2026-12-01&to=2026-12-10&tenant_ids=" + c.tenantA.String()
	r = must(t, c.do(t, http.MethodGet, p, saDsh, nil, nil), http.StatusOK, "platform summary")
	for _, cell := range r.items("cells") {
		if cell["tenant_id"] != c.tenantA.String() {
			t.Fatalf("platform cell without its tenant: %v", cell)
		}
	}
	authztest.Cover(t, "getTenantLearningSummary", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getPlatformLearningSummary", authztest.RoleDenied)
	if len(r.items("by_tenant")) != 1 || c.audits(ctx, t, "cross_tenant.accessed", nil) <= audits {
		t.Fatalf("by_tenant %v audits %d → %d", r.items("by_tenant"), audits, c.audits(ctx, t, "cross_tenant.accessed", nil))
	}
	// Without tenant_ids: every active tenant; tenant B (1 student) is hidden.
	r = must(t, c.do(t, http.MethodGet, "/admin/analytics/learning/summary?from=2026-12-01&to=2026-12-10", saDsh, nil, nil), http.StatusOK, "all tenants")
	tenants := map[string]bool{}
	for _, cell := range r.items("cells") {
		tenants[cell["tenant_id"].(string)] = true
	}
	if !tenants[c.tenantA.String()] || !tenants[c.tenantB.String()] {
		t.Fatalf("tenants %v", tenants)
	}
	// QA tenant excluded (DASHBOARD_DATA §3.4).
	if _, err := c.platform.ExecContext(ctx, `UPDATE ref.tenants SET is_qa = true WHERE tenant_id = $1`, c.tenantB); err != nil {
		t.Fatal(err)
	}
	r = must(t, c.do(t, http.MethodGet, "/admin/analytics/learning/summary?from=2026-12-01&to=2026-12-10&tenant_ids="+c.tenantB.String(), saDsh,
		nil, nil), http.StatusOK, "QA tenant")
	if len(r.items("cells")) != 0 || fmt.Sprint(r.body["excluded_tenant_ids"]) != "["+c.tenantB.String()+"]" {
		t.Fatalf("QA tenant: %s", r.raw)
	}
	// Tenant roles on the platform route → 403.
	for _, sub := range []string{taDash, adS} {
		if r := c.do(t, http.MethodGet, p, sub, hdr{"X-Tenant-Id": ""}, nil); r.status != http.StatusForbidden {
			t.Fatalf("%s on /admin: %d %s", sub, r.status, r.raw)
		}
	}
}

// F043-AC-18, F043-AC-22 (TC-DASH-001, AC-DASH-001, SC-003): the role learning_dashboard reads rollups and read models only
// (no privilege on learning_events, attempts, attempt_answers, mastery_history), has the statement timeout of the
// replica, and every query of the catalog has an index path (no Seq Scan on a daily rollup when sequential scans are
// disabled).
func TestDashboardRoleAndQueryCatalog(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.dashReady(ctx, t)
	dash := open(t, c.cl.ModuleDSN("learning", "dashboard"))
	for _, table := range []string{"learning.learning_events", "assessment.attempts", "assessment.attempt_answers", "learning.mastery_history",
		"learning.processed_events", "lms.courses"} {
		_, err := dash.ExecContext(ctx, "SELECT 1 FROM "+table+" LIMIT 1")
		if err == nil || (!strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "does not exist")) {
			t.Fatalf("learning_dashboard reads %s: %v", table, err)
		}
	}
	var timeout string
	if err := dash.QueryRowContext(ctx, `SHOW statement_timeout`).Scan(&timeout); err != nil || timeout != "5s" {
		t.Fatalf("statement_timeout of learning_dashboard: %q %v", timeout, err)
	}
	if _, err := dash.ExecContext(ctx, `INSERT INTO learning.rollup_days (tenant_id, bucket_date) VALUES ($1, current_date)`, c.tenantA); err == nil {
		t.Fatal("learning_dashboard can write")
	}
	session, err := entx.NewReadSession("learning", entx.DBs{App: dash, Platform: dash})
	if err != nil {
		t.Fatal(err)
	}
	plans := &readrepo.Plans{}
	pctx := readrepo.WithPlans(k.principalCtx(ctx, adS), plans)
	m := readrepo.DashboardModel{}
	grade := 3
	err = session.Read(pctx, func(ctx context.Context) error {
		tx, _ := entx.Tx(ctx)
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		sc := read.DashScope{TenantID: c.tenantA, SchoolIDs: []uuid.UUID{k.school}, GradeLevel: &grade}
		cls := sc
		cls.ClassID = &k.class3A
		for _, g := range []struct{ from, to string }{{"2026-12-01", "2026-12-10"}, {"2026-09-01", "2026-12-10"}, {"2025-01-01", "2026-12-10"}} {
			rg, err := analytics.NewRange(date(g.from), date(g.to), "", analytics.DefaultConfig().Range)
			if err != nil {
				return err
			}
			if _, err := m.SchoolTotals(ctx, sc, rg); err != nil {
				return err
			}
			if _, _, err := m.ClassTotals(ctx, sc, rg); err != nil {
				return err
			}
			if _, _, err := m.SchoolSeries(ctx, sc, rg); err != nil {
				return err
			}
			if _, _, err := m.SchoolSeries(ctx, cls, rg); err != nil {
				return err
			}
			if _, _, err := m.SchoolCells(ctx, sc, rg); err != nil {
				return err
			}
			if _, _, err := m.ProfileSubjects(ctx, sc, k.a1, rg); err != nil {
				return err
			}
			if _, err := m.ClassStudents(ctx, cls, rg, nil, 50); err != nil {
				return err
			}
		}
		if _, err := m.ActiveStudents(ctx, sc, date("2026-12-04"), date("2026-12-10")); err != nil {
			return err
		}
		if _, err := m.ActiveStudents(ctx, cls, date("2026-12-04"), date("2026-12-10")); err != nil {
			return err
		}
		if _, err := m.Classes(ctx, sc); err != nil {
			return err
		}
		if _, err := m.ObjectiveStats(ctx, sc); err != nil {
			return err
		}
		if _, _, err := m.Attention(ctx, sc, 50); err != nil {
			return err
		}
		if _, err := m.Assignments(ctx, sc, dashNow, 20); err != nil {
			return err
		}
		if _, err := m.SelfStudy(ctx, sc); err != nil {
			return err
		}
		if _, err := m.ProfileTopics(ctx, sc, k.a1); err != nil {
			return err
		}
		if _, err := m.ProfileTrend(ctx, sc, k.a1, date("2026-10-19"), date("2026-12-07")); err != nil {
			return err
		}
		if _, err := m.ProfileAssignments(ctx, sc, k.a1, 20); err != nil {
			return err
		}
		if _, err := m.AvailableFrom(ctx, sc); err != nil {
			return err
		}
		if _, err := m.AvailableFrom(ctx, cls); err != nil {
			return err
		}
		_, err := m.ProfileRecommendations(ctx, sc, k.a1, 3)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	seq := regexp.MustCompile(`Seq Scan on (rollup_(class|school|tenant)_daily\w*|progress_daily\w*)`)
	seen := 0
	for id, ps := range plans.ByQuery {
		for _, p := range ps {
			seen++
			if strings.HasPrefix(p, "EXPLAIN failed") {
				t.Fatalf("%s: %s", id, p)
			}
			if s := seq.FindString(p); s != "" {
				t.Errorf("query %s: %s\n%s", id, s, p)
			}
		}
	}
	if seen < 20 {
		t.Fatalf("only %d plans collected", seen)
	}
}
