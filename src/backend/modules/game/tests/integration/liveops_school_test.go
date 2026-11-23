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

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	writerepo "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

var templateID = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-0000000f7710")

// templateItem is the template of the seed (Maths Week: description, schools, grades, target, reward option allowed;
// quests not allowed; 14 days).
func templateItem(t *testing.T) command.ContentItem {
	t.Helper()
	m := map[string]any{"schema_version": 1, "template_id": templateID.String(), "code": "template.math_week", "owner_scope": "platform",
		"tenant_id": nil, "event_kind": "school", "name": map[string]any{"vi": "Tuần lễ Toán học", "en": "Maths Week"},
		"description":       map[string]any{"vi": "Cả trường cùng giải toán", "en": "The whole school solves maths"},
		"allowed_params":    []string{"description", "audience.school_ids", "audience.grade_levels", "community_goal_target", "reward_option_id"},
		"max_duration_days": 14, "quest_options": []string{uuid.NewSHA1(templateID, []byte("quest")).String()},
		"reward_options": []any{map[string]any{"reward_option_id": "school_badge", "label": map[string]any{"vi": "Huy hiệu", "en": "Badge"},
			"rewards": []any{map[string]any{"type": "badge", "badge_id": "badge_explorer_01"}}}},
		"community_goal": map[string]any{"goal_code": "math_week", "metric": "correct_answers", "scope": "school", "default_target": 500,
			"per_student_daily_cap": 50, "milestones": []any{map[string]any{"percent": 50, "rewards": []any{xuGrant(10)}},
				map[string]any{"percent": 100, "rewards": []any{}}}}}
	b, _ := json.Marshal(m)
	return command.ContentItem{Type: write.ItemEventTemplate, Payload: b}
}

// F077-AC-21…AC-25 (T030, T031, T032, TC-EVT-008, TC-TEN-003/019): templates assigned to the tenant only; a school event
// takes only the allowed parameters and options within the duration (422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED); its texts are
// screened (422 naming the field, never the word; module chat down: 503, nothing stored); at most 3 active events per
// tenant (409); If-Match on update (412 stale, 409 once started); cancel needs a reason (audit); another school or tenant
// sees 404, academic_manager 403 (authz.denied), a student token 403 PORTAL_ACCESS_STAFF_ONLY.
func TestSchoolEvents(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	e.publishEvent(ctx, t, templateItem(t))
	sc, err := authz.SystemPlatformScope("game-seed")
	if err != nil {
		t.Fatal(err)
	}
	if _, skipped, err := (command.AssignSeedQuests{UoW: e.uow, Assignments: writerepo.QuestRepo{}, Audit: writerepo.Auditor{R: e.rec},
		ItemType: write.ItemEventTemplate}).Assign(tenancy.WithScope(ctx, sc), []command.SeedAssignment{{QuestID: templateID, Tenants: []string{"t-a"}}}); err != nil ||
		len(skipped) > 0 {
		t.Fatalf("assign template: %v %v", skipped, err)
	}
	adminA := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA)))
	adminA2 := e.staffToken(e.staff(e.tenantA, "school_admin", authztest.AtSchool(e.schoolA2)))
	tenantAdminA := e.staffToken(e.staff(e.tenantA, "tenant_admin"))
	adminB := e.staffToken(e.staff(e.tenantB, "tenant_admin"))
	academic := e.staffToken(e.staff(e.tenantA, "academic_manager"))
	ha, hb := func() map[string]string { return withTenant(idem(), e.tenantA) }, func() map[string]string { return withTenant(idem(), e.tenantB) }
	// AC-21: templates of the tenant only.
	if r := e.do(t, http.MethodGet, "/lms/event-templates", adminA, ha(), nil); r.status != http.StatusOK || len(dataOf(r)) != 1 ||
		dataOf(r)[0]["code"] != "template.math_week" || strings.Contains(r.raw, "badge_explorer_01") {
		t.Fatalf("templates of A: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/lms/event-templates", adminB, hb(), nil); r.status != http.StatusOK || len(dataOf(r)) != 0 {
		t.Fatalf("templates of B: %d %s", r.status, r.raw)
	}
	start := e.clock.Now().Add(24 * time.Hour)
	body := func(edit func(m map[string]any)) map[string]any {
		m := map[string]any{"template_id": templateID.String(), "name": map[string]any{"vi": "Tuần lễ Toán học trường A"},
			"starts_at": start.Format(time.RFC3339), "ends_at": start.Add(7 * 24 * time.Hour).Format(time.RFC3339),
			"audience": map[string]any{"grade_levels": []int{3, 4}}, "community_goal_target": 300, "reward_option_id": "school_badge"}
		if edit != nil {
			edit(m)
		}
		return m
	}
	create := func(tok string, h map[string]string, b map[string]any) resp {
		return e.do(t, http.MethodPost, "/lms/school-events", tok, h, b)
	}
	// AC-22: outside the template.
	for name, edit := range map[string]func(m map[string]any){
		"quest param":  func(m map[string]any) { m["quest_ids"] = []string{uuid.NewSHA1(templateID, []byte("quest")).String()} },
		"other reward": func(m map[string]any) { m["reward_option_id"] = "gold" },
		"too long":     func(m map[string]any) { m["ends_at"] = start.Add(15 * 24 * time.Hour).Format(time.RFC3339) },
		"other school": func(m map[string]any) { m["audience"] = map[string]any{"school_ids": []string{e.schoolB.String()}} },
		"no template":  func(m map[string]any) { m["template_id"] = uuid.NewString() },
	} {
		r := create(tenantAdminA, ha(), body(edit))
		if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.EventTemplateParamNotAllowed) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// AC-23: screening (placeholder only, R15), chat down, maxLength, vi required.
	calls := e.live().screen.calls
	r := create(adminA, ha(), body(func(m map[string]any) { m["description"] = map[string]any{"vi": "Mô tả " + flagged} }))
	if r.status != http.StatusUnprocessableEntity || r.code() != string(problem.ValidationFailed) || strings.Contains(r.raw, flagged) ||
		!strings.Contains(r.raw, "description.vi") {
		t.Fatalf("flagged text: %d %s", r.status, r.raw)
	}
	if e.live().screen.calls != calls+1 {
		t.Fatal("texts screened by module chat")
	}
	e.live().screen.down = true
	if r := create(adminA, ha(), body(nil)); r.status != http.StatusServiceUnavailable || r.code() != string(problem.Unavailable) {
		t.Fatalf("chat down: %d %s", r.status, r.raw)
	}
	e.live().screen.down = false
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.events WHERE owner_scope = 'tenant'`); n != 0 {
		t.Fatalf("nothing stored: %d", n)
	}
	if r := create(adminA, ha(), body(func(m map[string]any) { m["name"] = map[string]any{"vi": strings.Repeat("a", 256)} })); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("name too long: %d %s", r.status, r.raw)
	}
	if r := create(adminA, ha(), body(func(m map[string]any) { m["name"] = map[string]any{"en": "Maths week"} })); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("vi required: %d %s", r.status, r.raw)
	}
	if r := create(adminA, withTenant(map[string]string{}, e.tenantA), body(nil)); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d", r.status)
	}
	// Created by the school admin of school A: its school by default, run scheduled, audit.
	r = create(adminA, ha(), body(nil))
	if r.status != http.StatusCreated || r.body["status"] != "scheduled" || r.header.Get("ETag") == "" {
		t.Fatalf("create: %d %s", r.status, r.raw)
	}
	eventA := r.body["event_id"].(string)
	if aud := r.body["audience"].(map[string]any); len(aud["school_ids"].([]any)) != 1 || aud["school_ids"].([]any)[0] != e.schoolA.String() {
		t.Fatalf("audience of school A: %v", aud)
	}
	if e.audits(ctx, t, "school_event.created") != 1 || e.events(ctx, t, "event.scheduled") != 1 {
		t.Fatal("audit and event.scheduled")
	}
	// AC-25: other school, other tenant → 404; academic_manager → 403; student → PORTAL_ACCESS_STAFF_ONLY.
	if r := e.do(t, http.MethodGet, "/lms/school-events/"+eventA, adminA2, ha(), nil); r.status != http.StatusNotFound {
		t.Fatalf("other school: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/lms/school-events/"+eventA, adminB, hb(), nil); r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/lms/school-events", adminA2, ha(), nil); len(dataOf(r)) != 0 {
		t.Fatalf("list of school A2: %s", r.raw)
	}
	denied := e.audits(ctx, t, "authz.denied")
	for _, op := range []func() resp{
		func() resp { return e.do(t, http.MethodGet, "/lms/school-events", academic, ha(), nil) },
		func() resp { return create(academic, ha(), body(nil)) },
		func() resp { return e.do(t, http.MethodGet, "/lms/event-templates", academic, ha(), nil) },
	} {
		if r := op(); r.status != http.StatusForbidden || r.code() != string(problem.PermissionDenied) {
			t.Fatalf("academic_manager: %d %s", r.status, r.raw)
		}
	}
	if e.audits(ctx, t, "authz.denied") < denied+3 {
		t.Fatal("authz.denied of academic_manager")
	}
	st := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	if r := e.do(t, http.MethodGet, "/lms/school-events", e.gtoken(st), ha(), nil); r.status != http.StatusForbidden ||
		r.code() != string(problem.PortalAccessStaffOnly) {
		t.Fatalf("student: %d %s", r.status, r.raw)
	}
	// Update: stale If-Match → 412, then a valid update.
	get := e.do(t, http.MethodGet, "/lms/school-events/"+eventA, adminA, ha(), nil)
	etag := get.header.Get("ETag")
	h := ha()
	h["If-Match"] = `"999"`
	if r := e.do(t, http.MethodPatch, "/lms/school-events/"+eventA, adminA, h, map[string]any{"community_goal_target": 400}); r.status != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: %d %s", r.status, r.raw)
	}
	h = ha()
	h["If-Match"] = etag
	up := e.do(t, http.MethodPatch, "/lms/school-events/"+eventA, adminA, h, map[string]any{"community_goal_target": 400})
	if up.status != http.StatusOK || up.body["community_goal_target"].(float64) != 400 || up.header.Get("ETag") == etag {
		t.Fatalf("update: %d %s", up.status, up.raw)
	}
	// AC-24: at most 3 active events per tenant.
	for i := 0; i < 2; i++ {
		if r := create(tenantAdminA, ha(), body(nil)); r.status != http.StatusCreated {
			t.Fatalf("event %d: %d %s", i+2, r.status, r.raw)
		}
	}
	if r := create(tenantAdminA, ha(), body(nil)); r.status != http.StatusConflict || r.code() != string(problem.Conflict) {
		t.Fatalf("fourth event: %d %s", r.status, r.raw)
	}
	// Cancel: reason required, audit, frees a slot.
	if r := e.do(t, http.MethodPost, "/lms/school-events/"+eventA+":cancel", adminA, ha(), map[string]any{"reason": "x"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("short reason: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/lms/school-events/"+eventA+":cancel", adminA2, ha(), map[string]any{"reason": "Trùng lịch thi"}); r.status != http.StatusNotFound {
		t.Fatalf("cancel by another school: %d", r.status)
	}
	if r := e.do(t, http.MethodPost, "/lms/school-events/"+eventA+":cancel", adminA, ha(), map[string]any{"reason": "Trùng lịch thi"}); r.status != http.StatusOK ||
		r.body["status"] != "cancelled" {
		t.Fatalf("cancel: %d %s", r.status, r.raw)
	}
	if e.audits(ctx, t, "school_event.cancelled") != 1 {
		t.Fatal("cancel audit")
	}
	if r := create(tenantAdminA, ha(), body(nil)); r.status != http.StatusCreated {
		t.Fatalf("a slot again after cancel: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/lms/school-events?status=cancelled", tenantAdminA, ha(), nil); len(dataOf(r)) != 1 {
		t.Fatalf("filter cancelled: %s", r.raw)
	}
	// Started: no more update.
	e.clock.Add(25 * time.Hour)
	e.tick(ctx, t)
	var running string
	_ = e.platform.QueryRowContext(ctx, `SELECT e.event_id FROM game.events e JOIN game.event_runs r ON r.event_id = e.event_id
		WHERE e.owner_scope = 'tenant' AND r.status = 'live' LIMIT 1`).Scan(&running)
	got := e.do(t, http.MethodGet, "/lms/school-events/"+running, tenantAdminA, ha(), nil)
	h = ha()
	h["If-Match"] = got.header.Get("ETag")
	if r := e.do(t, http.MethodPatch, "/lms/school-events/"+running, tenantAdminA, h, map[string]any{"community_goal_target": 600}); r.status != http.StatusConflict {
		t.Fatalf("update once live: %d %s", r.status, r.raw)
	}
	authztest.Cover(t, "listEventTemplates", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSchoolEvents", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createSchoolEvent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getSchoolEvent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateSchoolEvent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "cancelSchoolEvent", authztest.CrossTenant, authztest.RoleDenied)
	e.checkEventSchemas(ctx, t)
}
