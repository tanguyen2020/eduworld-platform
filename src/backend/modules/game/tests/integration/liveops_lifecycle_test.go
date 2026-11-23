//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// manifestEvents returns the event entries of the latest manifest of tenant.
func (e *env) manifestEvents(ctx context.Context, t *testing.T, tenant uuid.UUID) []map[string]any {
	t.Helper()
	var raw []byte
	err := e.platform.QueryRowContext(ctx, `SELECT entries FROM game.content_manifests WHERE tenant_id = $1 ORDER BY manifest_version DESC LIMIT 1`,
		tenant).Scan(&raw)
	if err != nil {
		return nil
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(raw, &doc)
	var out []map[string]any
	for _, it := range doc.Items {
		if it["item_type"] == "event" {
			out = append(out, it)
		}
	}
	return out
}

// F077-AC-01, AC-02, AC-05 (T012, T014, TC-EVT-002/003/012): publishing a scheduled event creates one scheduled run
// (event.scheduled); at starts_at two scheduler instances running together make one transition (event.started once, ±1
// minute), the manifest of the audience tenants gains the event and online students get event.status.changed; at ends_at
// the run ends and leaves the manifest; past claim_until the reached milestones not claimed are granted once and the run
// is archived; ticks after that change nothing.
func TestLiveOpsScheduler(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, nil))
	run, status := e.runOf(ctx, t, ev)
	if status != "scheduled" || e.events(ctx, t, "event.scheduled") != 1 {
		t.Fatalf("published: %s, %d event.scheduled", status, e.events(ctx, t, "event.scheduled"))
	}
	// Re-publishing the same payload changes nothing (the seed can run again).
	e.publishEvent(ctx, t, e.festival(ev, nil))
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.event_runs WHERE event_id = $1`, ev); n != 1 {
		t.Fatalf("runs after re-publish: %d", n)
	}
	// F075 integration point: a shop listing limited to the event is sold only while its run is live.
	e.publishEvent(ctx, t, economyItem(t, shopFirework, func(m map[string]any) { m["event_id"] = ev.String() }))
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	listed := func() bool {
		r := e.do(t, http.MethodGet, "/game/shop/items", e.gtoken(an), th(an), nil)
		for _, it := range dataOf(r) {
			if it["shop_item_id"] == shopFirework {
				return true
			}
		}
		return false
	}
	if listed() {
		t.Fatal("an event listing is not sold before its run is live")
	}
	d := e.device(an)
	if r := e.start(t, an, d, newKey(), nil); r.status != http.StatusCreated {
		t.Fatalf("start session: %d %s", r.status, r.raw)
	}
	if e.tick(ctx, t) != 0 {
		t.Fatal("nothing is due before starts_at")
	}
	e.clock.Add(time.Hour + 30*time.Second) // 30 s after starts_at: within the ±1 minute window
	var wg sync.WaitGroup
	moved := make([]int, 2)
	for i := range moved {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n, err := e.live().uc.RunScheduler(ctx, 50)
			if err != nil {
				t.Errorf("scheduler %d: %v", i, err)
			}
			moved[i] = n
		}(i)
	}
	wg.Wait()
	if _, status := e.runOf(ctx, t, ev); status != "live" || moved[0]+moved[1] != 1 || e.events(ctx, t, "event.started") != 1 {
		t.Fatalf("two instances: status %s, moved %v, %d event.started", status, moved, e.events(ctx, t, "event.started"))
	}
	if evs := e.manifestEvents(ctx, t, e.tenantA); len(evs) != 1 || evs[0]["event_run_id"] != run.String() {
		t.Fatalf("manifest of tenant A: %v", evs)
	}
	if len(e.manifestEvents(ctx, t, e.tenantB)) != 1 {
		t.Fatal("a platform event of every tenant is in the manifest of tenant B too")
	}
	if p := e.live().push.of("event.status.changed"); len(p) != 1 || p[0].student != an.id || p[0].payload["status"] != "live" {
		t.Fatalf("event.status.changed: %+v", p)
	}
	if !listed() {
		t.Fatal("the event listing is sold while the run is live")
	}
	// A student earns 60 points (m1 reached, not claimed) before the end.
	for i := 0; i < 3; i++ {
		e.liveDeliver(ctx, t, e.questEvent(an, "side"))
	}
	e.clock.Add(8 * 24 * time.Hour)
	e.tick(ctx, t)
	if _, status := e.runOf(ctx, t, ev); status != "ended" || e.events(ctx, t, "event.ended") != 1 || len(e.manifestEvents(ctx, t, e.tenantA)) != 0 {
		t.Fatalf("ended: %s, manifest %v", status, e.manifestEvents(ctx, t, e.tenantA))
	}
	if listed() {
		t.Fatal("the event listing is no longer sold after the run ended")
	}
	e.clock.Add(72*time.Hour + time.Minute)
	e.tick(ctx, t)
	if _, status := e.runOf(ctx, t, ev); status != "archived" {
		t.Fatalf("archived: %s", status)
	}
	if n := e.grantsOf(ctx, t, an, "event_milestone"); n != 1 {
		t.Fatalf("m1 auto-granted once: %d", n)
	}
	auto := e.payloads(ctx, t, "event.milestone_claimed")
	if len(auto) != 1 || auto[0]["auto_granted"] != true || auto[0]["milestone_id"] != "m1" {
		t.Fatalf("event.milestone_claimed: %v", auto)
	}
	if e.tick(ctx, t) != 0 || e.grantsOf(ctx, t, an, "event_milestone") != 1 {
		t.Fatal("ticks after archive change nothing")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM game.event_point_ledger WHERE delta <= 0`); n != 0 {
		t.Fatalf("no negative or zero event points: %d", n)
	}
	seen := e.checkEventSchemas(ctx, t)
	for _, typ := range []string{"event.scheduled", "event.started", "event.ended", "event.milestone_claimed", "event.participation_started"} {
		if seen[typ] == 0 {
			t.Fatalf("no %s checked", typ)
		}
	}
}

// F077-AC-03, AC-04, AC-06 (T013, T015, TC-EVT-003, TC-TEN-004): a liveops operator suspends a live run (reason in the
// audit, event.suspended): its components leave the manifest and the event detail, no event points or new reward while
// suspended, the pinned version stays; resume brings it back; an invalid transition is 422 INVALID_STATE_TRANSITION; a
// tenant token, a platform role without game.event.operate or a game token is 403 (authz.denied); cancel then archive.
func TestLiveOpsOperate(t *testing.T) {
	ctx := context.Background()
	e := setupEconomy(ctx, t)
	ev := uuid.New()
	e.publishEvent(ctx, t, e.festival(ev, nil))
	run, _ := e.runOf(ctx, t, ev)
	e.clock.Add(time.Hour)
	e.tick(ctx, t)
	an := e.newStudent(ctx, t, e.tenantA, e.schoolA, "active")
	e.liveDeliver(ctx, t, e.questEvent(an, "side"))
	var pinned uuid.UUID
	_ = e.platform.QueryRowContext(ctx, `SELECT event_version_id FROM game.event_runs WHERE event_run_id = $1`, run).Scan(&pinned)
	questRows := count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress`)

	ops := e.platformToken("liveops")
	if r := e.operate(t, ops, ev, run, "suspend", "Sự cố hiển thị trang trí"); r.status != http.StatusOK || r.body["status"] != "suspended" {
		t.Fatalf("suspend: %d %s", r.status, r.raw)
	}
	if e.audits(ctx, t, "event_run.suspend") != 1 || e.events(ctx, t, "event.suspended") != 1 {
		t.Fatal("suspend audit and event")
	}
	sus := e.payloads(ctx, t, "event.suspended")[0]
	if sus["reason"] != "Sự cố hiển thị trang trí" {
		t.Fatalf("reason in event.suspended: %v", sus)
	}
	if len(e.manifestEvents(ctx, t, e.tenantA)) != 0 {
		t.Fatal("suspended: components leave the manifest")
	}
	detail := e.do(t, http.MethodGet, "/game/events/"+ev.String(), e.gtoken(an), th(an), nil)
	if detail.status != http.StatusOK || detail.body["status"] != "suspended" || len(detail.body["quest_ids"].([]any)) != 0 || detail.body["community_goal"] != nil {
		t.Fatalf("suspended detail: %d %s", detail.status, detail.raw)
	}
	e.liveDeliver(ctx, t, e.questEvent(an, "side")) // no event points while suspended
	if n := count(ctx, t, e.platform, `SELECT coalesce(sum(delta), 0) FROM game.event_point_ledger WHERE student_id = $1`, an.id); n != 20 {
		t.Fatalf("points while suspended: %d", n)
	}
	if r := e.do(t, http.MethodPost, "/game/events/"+ev.String()+"/milestones/m1:claim", e.gtoken(an), withTenant(idem(), an.tenant), nil); r.status != http.StatusConflict {
		t.Fatalf("claim while suspended: %d %s", r.status, r.raw)
	}
	var after uuid.UUID
	_ = e.platform.QueryRowContext(ctx, `SELECT event_version_id FROM game.event_runs WHERE event_run_id = $1`, run).Scan(&after)
	if after != pinned || count(ctx, t, e.platform, `SELECT count(*) FROM game.quest_progress`) != questRows {
		t.Fatal("suspension keeps the pinned version and never touches the plays of the students")
	}
	if r := e.operate(t, ops, ev, run, "suspend", "Lần nữa"); r.status != http.StatusUnprocessableEntity || r.code() != string(problem.InvalidStateTransition) {
		t.Fatalf("suspend twice: %d %s", r.status, r.raw)
	}
	if r := e.operate(t, ops, ev, run, "resume", "ab"); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("short reason: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodPost, "/game-admin/events/"+ev.String()+"/runs/"+run.String()+":resume", ops, nil, map[string]any{"reason": "Đã sửa"}); r.status != http.StatusPreconditionRequired {
		t.Fatalf("no key: %d %s", r.status, r.raw)
	}
	if r := e.operate(t, ops, ev, run, "resume", "Đã sửa xong"); r.status != http.StatusOK || r.body["status"] != "live" {
		t.Fatalf("resume: %d %s", r.status, r.raw)
	}
	if len(e.manifestEvents(ctx, t, e.tenantA)) != 1 || e.events(ctx, t, "event.resumed") != 1 {
		t.Fatal("resumed: back in the manifest")
	}
	// F077-AC-06: tenant token, platform role without the permission, a game token, a wrong event id.
	denied0 := e.audits(ctx, t, "authz.denied")
	for name, tok := range map[string]string{
		"tenant_admin":  e.staffToken(e.staff(e.tenantA, "tenant_admin")),
		"content_admin": e.platformToken("content_admin"),
	} {
		if r := e.operate(t, tok, ev, run, "suspend", "Không có quyền"); r.status != http.StatusForbidden || r.code() != string(problem.PermissionDenied) {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
	}
	if e.audits(ctx, t, "authz.denied") < denied0+2 {
		t.Fatal("authz.denied recorded")
	}
	if r := e.operate(t, e.gtoken(an), ev, run, "suspend", "Không có quyền"); r.status != http.StatusForbidden {
		t.Fatalf("game token: %d %s", r.status, r.raw)
	}
	if r := e.do(t, http.MethodGet, "/game-admin/events/"+ev.String()+"/runs/"+run.String()+"/stats", e.platformToken("content_admin"), nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("stats without game.liveops.read: %d", r.status)
	}
	if r := e.operate(t, ops, uuid.New(), run, "suspend", "Sai sự kiện"); r.status != http.StatusNotFound {
		t.Fatalf("other event: %d", r.status)
	}
	stats := e.do(t, http.MethodGet, "/game-admin/events/"+ev.String()+"/runs/"+run.String()+"/stats", ops, nil, nil)
	if stats.status != http.StatusOK || stats.body["participants"].(float64) != 1 || strings.Contains(stats.raw, an.id.String()) {
		t.Fatalf("stats: %d %s", stats.status, stats.raw)
	}
	authztest.Cover(t, "suspendEventRun", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "resumeEventRun", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "cancelEventRun", authztest.RoleDenied, authztest.CrossTenant)
	authztest.Cover(t, "getEventRunStats", authztest.RoleDenied, authztest.CrossTenant)
	if r := e.operate(t, ops, ev, run, "cancel", "Hủy do sự cố kéo dài"); r.status != http.StatusOK || r.body["status"] != "cancelled" {
		t.Fatalf("cancel: %d %s", r.status, r.raw)
	}
	e.tick(ctx, t)
	if _, status := e.runOf(ctx, t, ev); status != "archived" || e.events(ctx, t, "event.cancelled") != 1 {
		t.Fatalf("cancelled → archived: %s", status)
	}
	if r := e.do(t, http.MethodGet, "/game/events/"+ev.String(), e.gtoken(an), th(an), nil); r.status != http.StatusNotFound ||
		r.code() != string(problem.EventNotAvailable) {
		t.Fatalf("archived event: %d %s", r.status, r.raw)
	}
	e.checkEventSchemas(ctx, t)
}
