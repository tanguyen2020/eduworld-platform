//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func dataOf(r resp) []map[string]any {
	var out []map[string]any
	if d, ok := r.body["data"].([]any); ok {
		for _, x := range d {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// F077-AC-07, AC-08, AC-09, AC-10, AC-12 (T020, T021, T022, TC-EVT-001/012, TC-TEN-001): a grade-3 student sees the
// festival of grade 3 (upcoming 7 days before, then live); a grade-7 student never sees it (404 EVENT_NOT_AVAILABLE, not
// listed, not in its manifest view); joining twice makes one participation; verified server events earn event points
// once per source event under the daily cap; a reached milestone is granted once (same key replays, another key 409),
// a milestone not reached is 409, no key 428; after end_at joining is 409 EVENT_ENDED; a tenant mismatch is 403.
func TestLiveOpsPlayer(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, func(m map[string]any) {
		s := m["schedule"].(map[string]any)
		start := e.clock.Now().Add(5 * 24 * time.Hour) // upcoming for 5 days
		s["start_at"], s["end_at"] = start.Format(time.RFC3339), start.Add(8*24*time.Hour).Format(time.RFC3339)
	}))
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	dung := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.setGrade(ctx, t, dung, 7)
	list := func(s student, q string) resp {
		return e.do(t, http.MethodGet, "/game/events"+q, e.gtoken(s), th(s), nil)
	}
	if r := list(an, "?status=upcoming"); r.status != http.StatusOK || len(dataOf(r)) != 1 || dataOf(r)[0]["status"] != "scheduled" {
		t.Fatalf("upcoming: %d %s", r.status, r.raw)
	}
	if r := list(an, "?status=live"); len(dataOf(r)) != 0 {
		t.Fatalf("not live yet: %s", r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+":join", e.gtoken(an), withTenant(idem(), an.tenant), nil); r.status != http.StatusConflict {
		t.Fatalf("join before start: %d %s", r.status, r.raw)
	}
	e.clock.Add(5 * 24 * time.Hour)
	e.tick(ctx, t)
	if r := list(an, ""); len(dataOf(r)) != 1 || dataOf(r)[0]["status"] != "live" || dataOf(r)[0]["event_id"] != ev.String() {
		t.Fatalf("live: %s", r.raw)
	}
	if r := list(dung, ""); r.status != http.StatusOK || len(dataOf(r)) != 0 {
		t.Fatalf("grade 7 sees nothing: %s", r.raw)
	}
	if r := e.do(t, http.MethodGet, "/game/events/"+ev.String(), e.gtoken(dung), th(dung), nil); r.status != http.StatusNotFound ||
		r.code() != string(problem.EventNotAvailable) {
		t.Fatalf("grade 7 detail: %d %s", r.status, r.raw)
	}
	m := e.do(t, http.MethodGet, "/game/content-manifest", e.gtoken(dung), th(dung), nil)
	if m.status != http.StatusOK {
		t.Fatalf("manifest: %d %s", m.status, m.raw)
	}
	for _, it := range m.body["items"].([]any) {
		if it.(map[string]any)["item_type"] == "event" {
			t.Fatal("the manifest view of a grade-7 student holds no grade-3 event")
		}
	}
	m = e.do(t, http.MethodGet, "/game/content-manifest", e.gtoken(an), th(an), nil)
	found := false
	for _, it := range m.body["items"].([]any) {
		found = found || it.(map[string]any)["item_type"] == "event"
	}
	if !found {
		t.Fatalf("the manifest view of the grade-3 student holds the event: %s", m.raw)
	}
	// AC-08: join twice (two keys) → one participation, one event.participation_started.
	for i := 0; i < 2; i++ {
		if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+":join", e.gtoken(an), withTenant(idem(), an.tenant), nil); r.status != http.StatusOK {
			t.Fatalf("join %d: %d %s", i, r.status, r.raw)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.event_participation WHERE student_id = $1`, an.id); n != 1 ||
		e.events(ctx, t, "event.participation_started") != 1 {
		t.Fatalf("one participation: %d", n)
	}
	// AC-09: points from verified events, once per source event, capped per day (200).
	q := e.questEvent(an, "side")
	e.liveDeliver(ctx, t, q)
	e.liveDeliver(ctx, t, q) // replay
	if n := count(ctx, t, e.platform, `SELECT coalesce(sum(delta), 0) FROM game.event_point_ledger WHERE student_id = $1`, an.id); n != 20 {
		t.Fatalf("points once per source event: %d", n)
	}
	if p := e.live().push.of("event.progress.updated"); len(p) != 1 || p[0].payload["event_points"] != 20 {
		t.Fatalf("event.progress.updated: %+v", p)
	}
	claim := func(s student, milestone string, h map[string]string) resp {
		return e.do(t, http.MethodPost, "/game/events/"+ev.String()+"/milestones/"+milestone+":claim", e.gtoken(s), withTenant(h, s.tenant), nil)
	}
	if r := claim(an, "m1", idem()); r.status != http.StatusConflict || r.code() != string(problem.EventMilestoneNotReached) {
		t.Fatalf("m1 not reached: %d %s", r.status, r.raw)
	}
	for i := 0; i < 12; i++ {
		e.liveDeliver(ctx, t, e.questEvent(an, "side"))
	}
	if n := count(ctx, t, e.platform, `SELECT coalesce(sum(delta), 0) FROM game.event_point_ledger WHERE student_id = $1`, an.id); n != 200 {
		t.Fatalf("daily cap 200: %d", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.event_point_ledger WHERE student_id = $1 AND capped`, an.id); n != 0 {
		t.Fatalf("exactly at the cap, nothing cut: %d", n)
	}
	// AC-10: claim m1 once (same key replays the response), another key 409, no key 428.
	k := idem()
	xu0 := e.balanceOf(ctx, t, an)
	r1 := claim(an, "m1", k)
	r2 := claim(an, "m1", map[string]string{"Idempotency-Key": k["Idempotency-Key"]})
	if r1.status != http.StatusOK || r1.body["claimed"] != true || r2.status != http.StatusOK || r2.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("claim: %d %s / %d %s", r1.status, r1.raw, r2.status, r2.raw)
	}
	if e.grantsOf(ctx, t, an, "event_milestone") != 1 || e.balanceOf(ctx, t, an) != xu0+20 || e.audits(ctx, t, "event_milestone.claimed") != 1 {
		t.Fatalf("one grant of 20 Xu: %d, %d", e.grantsOf(ctx, t, an, "event_milestone"), e.balanceOf(ctx, t, an)-xu0)
	}
	if r := claim(an, "m1", idem()); r.status != http.StatusConflict {
		t.Fatalf("claimed already: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+"/milestones/m2:claim", e.gtoken(an), th(an), nil); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	detail := e.do(t, http.MethodGet, "/game/events/"+ev.String(), e.gtoken(an), map[string]string{"X-Tenant-Id": an.tenant.String(), "Accept-Language": "en-US"}, nil)
	if detail.status != http.StatusOK || detail.header.Get("Content-Language") != "en-US" || detail.body["name"].(map[string]any)["en"] != "Moon Festival" ||
		detail.body["daily_points_remaining"].(float64) != 0 {
		t.Fatalf("detail en: %d %s", detail.status, detail.raw)
	}
	// AC-12: X-Tenant-Id of another tenant is 403 TENANT_MISMATCH.
	if r := e.do(t, http.MethodGet, "/game/events", e.gtoken(an), map[string]string{"X-Tenant-Id": e.tenantB.String()}, nil); r.status != http.StatusForbidden ||
		r.code() != string(problem.TenantMismatch) {
		t.Fatalf("tenant mismatch: %d %s", r.status, r.raw)
	}
	// AC-08: after end_at, joining is 409 EVENT_ENDED; the reached milestone can still be claimed until claim_until.
	bao := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.clock.Add(8 * 24 * time.Hour)
	e.tick(ctx, t)
	if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+":join", e.gtoken(bao), withTenant(idem(), bao.tenant), nil); r.status != http.StatusConflict ||
		r.code() != string(problem.EventEnded) {
		t.Fatalf("join after end: %d %s", r.status, r.raw)
	}
	if r := claim(an, "m2", idem()); r.status != http.StatusOK {
		t.Fatalf("claim m2 during claim_grace: %d %s", r.status, r.raw)
	}
	// Staff tokens on every player route: TestPlayerRoutesStaffDenied (role_denied); the tenant of another session: TENANT_MISMATCH above.
	authztest.Cover(t, "listGameEvents", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getGameEvent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "joinGameEvent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "claimEventMilestone", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getCommunityGoal", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getSeasonTrack", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "claimSeasonTier", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listSeasonTrackHistory", authztest.CrossTenant, authztest.RoleDenied)
	e.checkEventSchemas(ctx, t)
}

// F077-AC-11 (T021, TC-EVT-004): a new version of a live event (raised milestone, later end) applies from 04:00 ICT the
// next day; until then the run keeps its pinned version; after it, the milestone reached under the old version stays
// reached and the points stay; a version ending earlier is refused (only suspend or cancel shortens a run).
func TestLiveOpsRuleChange(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, nil))
	run, _ := e.runOf(ctx, t, ev)
	e.clock.Add(time.Hour)
	e.tick(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	for i := 0; i < 3; i++ {
		e.liveDeliver(ctx, t, e.questEvent(an, "side")) // 60 points: m1 (50) reached
	}
	pinned := func() uuid.UUID {
		var v uuid.UUID
		_ = e.platform.QueryRowContext(ctx, `SELECT event_version_id FROM game.event_runs WHERE event_run_id = $1`, run).Scan(&v)
		return v
	}
	v1 := pinned()
	raised := e.festival(ev, func(m map[string]any) {
		m["milestone_rewards"].([]any)[0].(map[string]any)["event_points"] = 100
		s := m["schedule"].(map[string]any)
		start, _ := time.Parse(time.RFC3339, s["start_at"].(string))
		s["end_at"] = start.Add(10 * 24 * time.Hour).Format(time.RFC3339)
	})
	e.publishEvent(ctx, t, raised)
	var eff time.Time
	_ = e.platform.QueryRowContext(ctx, `SELECT v.effective_from FROM game.events e JOIN game.event_versions v ON v.event_version_id = e.current_version_id
		WHERE e.event_id = $1`, ev).Scan(&eff)
	now := e.clock.Now().In(liveops.ICT)
	if want := time.Date(now.Year(), now.Month(), now.Day()+1, 4, 0, 0, 0, liveops.ICT); !eff.Equal(want) {
		t.Fatalf("effective_from %v, want %v", eff, want)
	}
	e.tick(ctx, t)
	if pinned() != v1 {
		t.Fatal("the run keeps its version until effective_from")
	}
	e.clock.Set(eff.Add(time.Second))
	e.tick(ctx, t)
	if pinned() == v1 {
		t.Fatal("the run is re-pinned at effective_from")
	}
	detail := e.do(t, http.MethodGet, "/game/events/"+ev.String(), e.gtoken(an), th(an), nil)
	ms := detail.body["milestones"].([]any)
	if m1 := ms[0].(map[string]any); m1["points_required"].(float64) != 100 || m1["reached"] != true || detail.body["event_points"].(float64) != 60 {
		t.Fatalf("m1 reached under v1 stays reached: %s", detail.raw)
	}
	if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+"/milestones/m1:claim", e.gtoken(an), withTenant(idem(), an.tenant), nil); r.status != http.StatusOK {
		t.Fatalf("claim m1 after the raise: %d %s", r.status, r.raw)
	}
	shorter := e.festival(ev, func(m map[string]any) {
		s := m["schedule"].(map[string]any)
		start, _ := time.Parse(time.RFC3339, s["start_at"].(string))
		s["end_at"] = start.Add(7*24*time.Hour + time.Hour).Format(time.RFC3339)
		m["name"] = map[string]any{"vi": "Lễ hội rút ngắn", "en": "Shorter"}
	})
	if _, err := e.publish(ctx, t, []command.ContentItem{shorter}, "shorter"); codeOf(err) != problem.ValidationFailed {
		t.Fatalf("a shorter live run is refused: %v", err)
	}
}
