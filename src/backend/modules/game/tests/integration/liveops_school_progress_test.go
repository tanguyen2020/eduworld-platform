//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// wrongCounter is a TEST DOUBLE of port.GoalCounter (R17): it answers a value PostgreSQL never holds, so a read that
// used Valkey would show it (F081-AC-10: the school progress is read from PostgreSQL only).
type wrongCounter struct{}

func (wrongCounter) Get(context.Context, uuid.UUID, uuid.UUID) (int64, bool, error) {
	return 987654, true, nil
}
func (wrongCounter) Raise(context.Context, uuid.UUID, uuid.UUID, int64) error { return nil }
func (wrongCounter) Set(context.Context, uuid.UUID, uuid.UUID, int64) error   { return nil }
func (wrongCounter) Throttle(context.Context, uuid.UUID, uuid.UUID, time.Duration) (bool, error) {
	return false, nil
}

// assignTemplate assigns the Maths Week template of the seed to the tenants (as the assignment saga would).
func (e *env) assignTemplate(ctx context.Context, t *testing.T, tenants ...string) {
	t.Helper()
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	if _, skipped, err := (command.AssignSeedQuests{UoW: e.uow, Assignments: writerepo.QuestRepo{}, Audit: writerepo.Auditor{R: e.rec},
		ItemType: write.ItemEventTemplate}).Assign(tenancy.WithScope(ctx, sc), []command.SeedAssignment{{QuestID: templateID, Tenants: tenants}}); err != nil ||
		len(skipped) > 0 {
		t.Fatalf("assign template: %v %v", skipped, err)
	}
}

// schoolEventBody is a valid SchoolEventCreate of the Maths Week template starting in one hour for seven days.
func (e *env) schoolEventBody(grades []int, edit func(m map[string]any)) map[string]any {
	start := e.clock.Now().Add(time.Hour)
	m := map[string]any{"template_id": templateID.String(), "name": map[string]any{"vi": "Tuần lễ Toán học"},
		"starts_at": start.Format(time.RFC3339), "ends_at": start.Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"audience": map[string]any{"grade_levels": grades}, "community_goal_target": 300}
	if edit != nil {
		edit(m)
	}
	return m
}

// F081-AC-10, AC-12 (T016, T017, T019; TC-TEN-003): getSchoolEventProgress answers the run status, the participants, the
// community goal total and milestones of the school scope read from PostgreSQL (a Valkey counter is never read), the
// contributions by grade only for grades 4–9 with ≥ analytics_min_cell_size contributing students, data_as_of — and
// never a student id, name, nickname or rank. Without lms.school_event.write 403 PERMISSION_DENIED; another school or
// tenant 404; X-Tenant-Id of another tenant 403 TENANT_MISMATCH. A school of the event is required when it has several.
func TestSchoolEventProgress(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	e.publishEvent(ctx, t, templateItem(t))
	e.assignTemplate(ctx, t, "t-a")
	adminA := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA)))
	adminA2 := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA2)))
	tenantAdminA := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	adminB := e.staffToken(e.staff(e.tenantB, "tenant_admin"))
	academic := e.staffToken(e.staff(e.tenantA, "academic_manager"))
	ha := func() map[string]string { return withTenant(idem(), e.tenantA) }
	r := e.do(t, http.MethodPost, "/lms/school-events", adminA, ha(), e.schoolEventBody([]int{3, 4, 5}, nil))
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	event := r.body["event_id"].(string)
	progress := func(tok string, h map[string]string, query string) resp {
		return e.do(t, http.MethodGet, "/lms/school-events/"+event+"/progress"+query, tok, h, nil)
	}
	// Scheduled: no participant, a goal at 0 with its milestones, no grade.
	p := progress(adminA, ha(), "")
	if p.status != http.StatusOK || p.body["status"] != "scheduled" || p.body["participants_count"].(float64) != 0 ||
		p.body["school_id"] != e.schoolA.String() || p.body["data_as_of"] == nil || len(p.body["by_grade"].([]any)) != 0 {
		t.Fatalf("scheduled progress: %d %s", p.status, p.raw)
	}
	goal := p.body["community_goal"].(map[string]any)
	if goal["target"].(float64) != 300 || goal["total"].(float64) != 0 || goal["metric"] != "correct_answers" || len(goal["milestones"].([]any)) != 2 {
		t.Fatalf("goal: %v", goal)
	}
	e.clock.Add(2 * time.Hour)
	e.tick(ctx, t)
	newAt := func(school uuid.UUID, grade int) student {
		s := e.newStudent(ctx, t, e.tenantA, school, "active")
		e.setGrade(ctx, t, s, grade)
		return s
	}
	// Grade 3: 5 contributors but outside 4–9; grade 4: 5 contributors (shown); grade 5: 4 contributors (hidden).
	var all []student
	for i := 0; i < 5; i++ {
		all = append(all, newAt(e.schoolA, 3), newAt(e.schoolA, 4))
	}
	for i := 0; i < 4; i++ {
		all = append(all, newAt(e.schoolA, 5))
	}
	other := newAt(e.schoolA2, 4) // another school of the tenant (outside the audience): never counted in school A
	cv := uuid.New()
	for _, s := range append(append([]student{}, all...), other) {
		e.liveDeliver(ctx, t, e.gradedEvent(s, cv, 2, 4, nil))
	}
	var pgTotal int64
	if err := e.platform.QueryRowContext(ctx, `SELECT coalesce(sum(p.total), 0) FROM game.community_goal_progress p JOIN game.community_goals g
		ON g.goal_id = p.goal_id JOIN game.event_runs r ON r.event_run_id = g.event_run_id WHERE r.event_id = $1 AND p.scope_id = $2`,
		event, e.schoolA).Scan(&pgTotal); err != nil || pgTotal != 28 {
		t.Fatalf("progress of school A in PostgreSQL: %d %v", pgTotal, err)
	}
	e.live().uc.Counter = wrongCounter{} // a Valkey counter that disagrees: the staff progress never reads it
	p = progress(adminA, ha(), "")
	if p.status != http.StatusOK || p.body["status"] != "live" || p.body["participants_count"].(float64) != 14 {
		t.Fatalf("live progress: %d %s", p.status, p.raw)
	}
	goal = p.body["community_goal"].(map[string]any)
	if goal["total"].(float64) != 28 {
		t.Fatalf("total from PostgreSQL (not the counter): %v", goal)
	}
	grades := p.body["by_grade"].([]any)
	if len(grades) != 1 {
		t.Fatalf("only grade 4 (grade 3 outside 4–9, grade 5 < 5 contributors): %v", grades)
	}
	g4 := grades[0].(map[string]any)
	if g4["grade_level"].(float64) != 4 || g4["contributors_count"].(float64) != 5 || g4["contribution_total"].(float64) != 10 {
		t.Fatalf("grade 4: %v", g4)
	}
	for _, s := range append(append([]student{}, all...), other) {
		if strings.Contains(p.raw, s.id.String()) {
			t.Fatal("no student id in the progress")
		}
	}
	for _, k := range []string{"student_id", "nickname", "display_name", "rank", "full_name"} {
		if strings.Contains(p.raw, `"`+k+`"`) {
			t.Fatalf("no %s in the progress: %s", k, p.raw)
		}
	}
	// An event that does not show the grades (show_grade_contributions = false): no by_grade for the school either.
	tx, err := e.migrator.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, e.tenantA.String()); err != nil { // RLS of the tenant row
		t.Fatal(err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE game.community_goals SET show_grade_contributions = false WHERE goal_id = $1`, goal["goal_id"])
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("show_grade_contributions off: %d rows", n)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if q := progress(adminA, ha(), ""); q.status != http.StatusOK || len(q.body["by_grade"].([]any)) != 0 || q.body["community_goal"] == nil {
		t.Fatalf("grades hidden by the event: %d %s", q.status, q.raw)
	}
	// A tenant admin sees the same event; the school is the event's school.
	if q := progress(tenantAdminA, ha(), ""); q.status != http.StatusOK || q.body["school_id"] != e.schoolA.String() {
		t.Fatalf("tenant admin: %d %s", q.status, q.raw)
	}
	// AC-12: another school 404, another tenant 404, academic_manager 403, X-Tenant-Id of another tenant 403.
	if q := progress(adminA2, ha(), ""); q.status != http.StatusNotFound {
		t.Fatalf("another school: %d %s", q.status, q.raw)
	}
	if q := progress(adminB, withTenant(idem(), e.tenantB), ""); q.status != http.StatusNotFound {
		t.Fatalf("another tenant: %d %s", q.status, q.raw)
	}
	if q := progress(adminA, ha(), "?school_id="+e.schoolA2.String()); q.status != http.StatusNotFound {
		t.Fatalf("school outside the audience/scope: %d %s", q.status, q.raw)
	}
	denied := e.audits(ctx, t, "authz.denied")
	if q := progress(academic, ha(), ""); q.status != http.StatusForbidden || q.code() != string(problem.PermissionDenied) {
		t.Fatalf("academic_manager: %d %s", q.status, q.raw)
	}
	if e.audits(ctx, t, "authz.denied") <= denied {
		t.Fatal("authz.denied of academic_manager")
	}
	if q := progress(adminA, withTenant(idem(), e.tenantB), ""); q.status != http.StatusForbidden || q.code() != string(problem.TenantMismatch) {
		t.Fatalf("X-Tenant-Id of another tenant: %d %s", q.status, q.raw)
	}
	st := all[0]
	if q := progress(e.gtoken(st), ha(), ""); q.status != http.StatusForbidden {
		t.Fatalf("student token: %d %s", q.status, q.raw)
	}
	// A tenant-wide event of the tenant admin (every school): the school is required, then counted for that school only.
	r = e.do(t, http.MethodPost, "/lms/school-events", tenantAdminA, ha(), e.schoolEventBody([]int{4}, nil))
	if r.status != http.StatusCreated {
		t.Fatalf("tenant-wide event: %d %s", r.status, r.raw)
	}
	wide := r.body["event_id"].(string)
	if q := e.do(t, http.MethodGet, "/lms/school-events/"+wide+"/progress", tenantAdminA, ha(), nil); q.status != http.StatusUnprocessableEntity ||
		!strings.Contains(q.raw, "school_id") {
		t.Fatalf("school required: %d %s", q.status, q.raw)
	}
	if q := e.do(t, http.MethodGet, "/lms/school-events/"+wide+"/progress?school_id="+e.schoolA2.String(), tenantAdminA, ha(), nil); q.status != http.StatusOK ||
		q.body["school_id"] != e.schoolA2.String() {
		t.Fatalf("school A2 of the tenant-wide event: %d %s", q.status, q.raw)
	}
	if q := e.do(t, http.MethodGet, "/lms/school-events/"+wide+"/progress?school_id="+e.schoolB.String(), tenantAdminA, ha(), nil); q.status != http.StatusNotFound {
		t.Fatalf("school of another tenant: %d %s", q.status, q.raw)
	}
	if q := e.do(t, http.MethodGet, "/lms/school-events/"+wide+"/progress?school_id="+e.schoolA.String(), adminA, ha(), nil); q.status != http.StatusNotFound {
		t.Fatalf("a school admin never sees a tenant-wide event: %d %s", q.status, q.raw)
	}
	authztest.Cover(t, "getSchoolEventProgress", authztest.CrossTenant, authztest.RoleDenied)
}

// F081-AC-06, AC-02 and the suspended edge case: System Admin lists the school events of several tenants without
// X-Tenant-Id (tenant_ids, audited cross_tenant.accessed, every row with its tenant_id; absent = every tenant) and creates
// one in one target tenant (X-Tenant-Id); a tenant user never reads another tenant through tenant_ids. The templates
// carry the title and grades of their quest options; a run suspended by EduWorld shows `suspended`.
func TestSchoolEventsCrossTenant(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	quest := uuid.NewSHA1(templateID, []byte("quest"))
	e.publishWorld(ctx, t)
	e.publishQuests(ctx, t, []command.ContentItem{learningQuest(quest.String(), "quest.f081_option", nil)})
	e.publishEvent(ctx, t, templateItem(t))
	e.assignTemplate(ctx, t, "t-a", "t-b")
	sa := e.platformToken("system_admin")
	adminA := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	// AC-02: quest_option_items with the title and grades of the published quest version.
	r := e.do(t, http.MethodGet, "/lms/event-templates", adminA, withTenant(idem(), e.tenantA), nil)
	items, _ := dataOf(r)[0]["quest_option_items"].([]any)
	if r.status != http.StatusOK || len(items) != 1 {
		t.Fatalf("quest option items: %d %s", r.status, r.raw)
	}
	it := items[0].(map[string]any)
	if it["id"] != quest.String() || it["title"].(map[string]any)["vi"] != "Bài toán phiên chợ" || len(it["grade_levels"].([]any)) != 1 {
		t.Fatalf("quest option item: %v", it)
	}
	if c, _ := dataOf(r)[0]["challenge_option_items"].([]any); c == nil || len(c) != 0 {
		t.Fatalf("no challenge option: %s", r.raw)
	}
	// System Admin creates in one target tenant (X-Tenant-Id).
	for _, tenant := range []uuid.UUID{e.tenantA, e.tenantB} {
		if c := e.do(t, http.MethodPost, "/lms/school-events", sa, withTenant(idem(), tenant), e.schoolEventBody([]int{4}, nil)); c.status != http.StatusCreated ||
			c.body["tenant_id"] != tenant.String() {
			t.Fatalf("system admin create in %s: %d %s", tenant, c.status, c.raw)
		}
	}
	audited := e.audits(ctx, t, "cross_tenant.accessed")
	l := e.do(t, http.MethodGet, "/lms/school-events?tenant_ids="+e.tenantB.String(), sa, nil, nil)
	if l.status != http.StatusOK || len(dataOf(l)) != 1 || dataOf(l)[0]["tenant_id"] != e.tenantB.String() {
		t.Fatalf("tenant_ids of B: %d %s", l.status, l.raw)
	}
	l = e.do(t, http.MethodGet, "/lms/school-events?tenant_ids="+e.tenantA.String()+"&tenant_ids="+e.tenantB.String(), sa, nil, nil)
	if l.status != http.StatusOK || len(dataOf(l)) != 2 {
		t.Fatalf("tenant_ids of A and B (repeated): %d %s", l.status, l.raw)
	}
	l = e.do(t, http.MethodGet, "/lms/school-events?limit=1", sa, nil, nil)
	if l.status != http.StatusOK || len(dataOf(l)) != 1 || l.body["page"].(map[string]any)["has_more"] != true {
		t.Fatalf("every tenant, keyset page of 1: %d %s", l.status, l.raw)
	}
	next := l.body["page"].(map[string]any)["next_cursor"].(string)
	l2 := e.do(t, http.MethodGet, "/lms/school-events?limit=1&cursor="+next, sa, nil, nil)
	if l2.status != http.StatusOK || len(dataOf(l2)) != 1 || dataOf(l2)[0]["event_id"] == dataOf(l)[0]["event_id"] {
		t.Fatalf("second page: %d %s", l2.status, l2.raw)
	}
	if e.audits(ctx, t, "cross_tenant.accessed") <= audited {
		t.Fatal("cross-tenant read audited")
	}
	// A tenant user: tenant_ids ignored, only the session tenant.
	l = e.do(t, http.MethodGet, "/lms/school-events?tenant_ids="+e.tenantB.String(), adminA, withTenant(map[string]string{}, e.tenantA), nil)
	if l.status != http.StatusOK || len(dataOf(l)) != 1 || dataOf(l)[0]["tenant_id"] != e.tenantA.String() {
		t.Fatalf("tenant user with tenant_ids: %d %s", l.status, l.raw)
	}
	// Another platform role without platform.cross_tenant.read: 403.
	if l := e.do(t, http.MethodGet, "/lms/school-events", e.platformToken("content_admin"), nil, nil); l.status != http.StatusForbidden {
		t.Fatalf("content_admin across tenants: %d %s", l.status, l.raw)
	}
	// Suspended by EduWorld → status suspended (no edit/cancel button for the school), filterable.
	e.clock.Add(2 * time.Hour)
	e.tick(ctx, t)
	var ev, run uuid.UUID
	if err := e.platform.QueryRowContext(ctx, `SELECT r.event_id, r.event_run_id FROM game.event_runs r JOIN game.events e ON e.event_id = r.event_id
		WHERE e.tenant_id = $1 AND e.owner_scope = 'tenant'`, e.tenantA).Scan(&ev, &run); err != nil {
		t.Fatal(err)
	}
	if o := e.operate(t, e.platformToken("system_admin"), ev, run, "suspend", "Kiểm tra nội dung"); o.status != http.StatusOK {
		t.Fatalf("suspend: %d %s", o.status, o.raw)
	}
	g := e.do(t, http.MethodGet, "/lms/school-events/"+ev.String(), adminA, withTenant(map[string]string{}, e.tenantA), nil)
	if g.status != http.StatusOK || g.body["status"] != "suspended" {
		t.Fatalf("suspended: %d %s", g.status, g.raw)
	}
	if l := e.do(t, http.MethodGet, "/lms/school-events?status=suspended", adminA, withTenant(map[string]string{}, e.tenantA), nil); len(dataOf(l)) != 1 {
		t.Fatalf("filter suspended: %s", l.raw)
	}
	if l := e.do(t, http.MethodGet, "/lms/school-events?status=live", adminA, withTenant(map[string]string{}, e.tenantA), nil); len(dataOf(l)) != 0 {
		t.Fatalf("a suspended event is not live: %s", l.raw)
	}
}
