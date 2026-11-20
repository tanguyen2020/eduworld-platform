//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	learninghttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// runScheduler runs the job assignment-scheduler once with the test clock.
func (c *curEnv) runScheduler(ctx context.Context, t *testing.T) command.SchedulerReport {
	t.Helper()
	rep, err := c.asg.RunScheduler(ctx, 100)
	if err != nil {
		t.Fatalf("scheduler: %v", err)
	}
	return rep
}

// F041-AC-18…AC-21 (US3): the scheduler opens scheduled assignments and closes the open ones after the due date plus
// late window (missed but excused, assignment.closed, twice → once); after_close releases at close; manual waits for
// lms.result.release (409 before the close or for another policy); the review hides the results until released.
func TestAssignmentCloseAndRelease(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "releaseAssignmentResults", authztest.CrossTenant, authztest.RoleDenied)
	now := c.clk.Now()
	manual := c.openAsg(t, k, map[string]any{"due_at": now.Add(2 * time.Hour).Format(time.RFC3339), "feedback_timing": "after_close",
		"result_release": "manual"})
	auto := c.openAsg(t, k, map[string]any{"mode": "graded", "due_at": now.Add(2 * time.Hour).Format(time.RFC3339)})
	sched := c.openAsg(t, k, map[string]any{"open_at": now.Add(time.Hour).Format(time.RFC3339), "due_at": now.Add(5 * time.Hour).Format(time.RFC3339)})
	// An answered attempt of An on the manual assignment; Bình is excused on it.
	a := must(t, asgStart(c, t, hsAn, k.s.activityID, manual), http.StatusCreated, "start manual")
	c.submitAll(t, hsAn, a)
	c.deliverAttempts(ctx, t, a.str("attempt_id"), false)
	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+manual+"/students/"+c.users[hsBinh].UserID.String()+":excuse", adA, idem(),
		map[string]any{"reason": "Ốm dài ngày"}), http.StatusOK, "excuse")
	// AC-19: no score nor answer before the release.
	if v := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+a.str("attempt_id"), hsAn, nil, nil), http.StatusOK, "attempt"); v.body["evaluation"] != nil {
		t.Fatalf("evaluation before release: %s", v.raw)
	}
	rv := must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+a.str("attempt_id")+"/review", hsAn, nil, nil), http.StatusOK, "review before")
	if len(rv.items("items")) != 0 || rv.body["score"] != nil {
		t.Fatalf("review before release: %s", rv.raw)
	}
	// AC-21: before the close 409; without lms.result.release 403.
	if r := c.do(t, http.MethodPost, assignmentsPath+"/"+manual+":release-results", adA, idem(), nil); r.status != http.StatusConflict {
		t.Fatalf("release before close: %d %s", r.status, r.raw)
	}
	// Scheduled opens after open_at.
	c.clk.Advance(90 * time.Minute)
	if rep := c.runScheduler(ctx, t); rep.Opened != 1 {
		t.Fatalf("open: %+v", rep)
	}
	if st := must(t, c.do(t, http.MethodGet, assignmentsPath+"/"+sched, adA, nil, nil), http.StatusOK, "scheduled").str("status"); st != "open" {
		t.Fatalf("scheduled → %s", st)
	}
	// AC-18, AC-20: close after the due date.
	c.clk.Advance(time.Hour)
	if rep := c.runScheduler(ctx, t); rep.Closed != 2 || rep.Released != 1 {
		t.Fatalf("close: %+v", rep)
	}
	if rep := c.runScheduler(ctx, t); rep.Closed != 0 {
		t.Fatalf("second run closed again: %+v", rep)
	}
	if st := must(t, c.do(t, http.MethodGet, assignmentsPath+"/"+auto, adA, nil, nil), http.StatusOK, "auto").str("status"); st != "results_released" {
		t.Fatalf("after_close → %s", st)
	}
	if st, _, _ := c.submission(ctx, t, auto, c.users[hsAn].UserID); st != "missed" {
		t.Fatalf("missed: %s", st)
	}
	if st, _, _ := c.submission(ctx, t, manual, c.users[hsBinh].UserID); st != "excused" {
		t.Fatalf("excused stays: %s", st)
	}
	closed := c.validPayloads(ctx, t, "assignment.closed", manual)
	if len(closed) != 1 || closed[0]["missed_count"] != float64(0) || closed[0]["status"] != "closed" {
		t.Fatalf("assignment.closed: %v", closed)
	}
	if ps := c.validPayloads(ctx, t, "assignment.results_released", auto); len(ps) != 1 || ps[0]["trigger"] != "after_close" {
		t.Fatalf("auto release: %v", ps)
	}
	// AC-21: manual release needs lms.result.release (tenant admin lacks it), then succeeds once.
	if r := c.do(t, http.MethodPost, assignmentsPath+"/"+manual+":release-results", taA, idem(), nil); r.status != http.StatusForbidden {
		t.Fatalf("release without permission: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, assignmentsPath+"/"+auto+":release-results", adA, idem(), nil); r.status != http.StatusConflict {
		t.Fatalf("release of after_close: %d %s", r.status, r.raw)
	}
	rel := must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+manual+":release-results", qlcmA, idem(), nil), http.StatusOK, "release")
	if rel.str("status") != "results_released" || c.audits(ctx, t, "assignment.results_released", &c.tenantA) != 1 {
		t.Fatalf("release: %s", rel.raw)
	}
	c.validPayloads(ctx, t, "assignment.results_released", manual)
	// AC-19: the review is released now.
	rv = must(t, c.do(t, http.MethodGet, "/learning/activity-attempts/"+a.str("attempt_id")+"/review", hsAn, nil, nil), http.StatusOK, "review after")
	if len(rv.items("items")) == 0 || rv.body["score"] == nil {
		t.Fatalf("review after release: %s", rv.raw)
	}
	my := must(t, c.do(t, http.MethodGet, "/learning/my-assignments?status=graded", hsAn, nil, nil), http.StatusOK, "graded")
	if len(my.items("data")) != 1 || my.items("data")[0]["assignment_id"] != manual {
		t.Fatalf("graded assignments: %s", my.raw)
	}
	must(t, c.do(t, http.MethodPost, assignmentsPath+"/"+manual+":archive", adA, idem(), nil), http.StatusOK, "archive")
}

// F041-AC-22…AC-25 (US4): an extension later than the due date reopens a missed student; an excuse removes the student
// from the completion; the reminder goes to the pending students once per day (429 + Retry-After then); the
// submissions list is paged.
func TestAssignmentStudentExceptions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "extendAssignmentForStudent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "excuseAssignmentForStudent", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "remindAssignment", authztest.CrossTenant, authztest.RoleDenied)
	now := c.clk.Now()
	id := c.openAsg(t, k, map[string]any{"due_at": now.Add(time.Hour).Format(time.RFC3339),
		"late_policy": map[string]any{"allow_late": true, "late_window_hours": 2}})
	path := assignmentsPath + "/" + id
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	// AC-24: remind both, then 429 the same day.
	rm := must(t, c.do(t, http.MethodPost, path+":remind", adA, idem(), nil), http.StatusAccepted, "remind")
	if rm.body["recipients"] != float64(2) || len(c.validPayloads(ctx, t, "assignment.reminded", id)) != 1 {
		t.Fatalf("remind: %s", rm.raw)
	}
	if r := c.do(t, http.MethodPost, path+":remind", adA, idem(), nil); r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("remind twice: %d %s", r.status, r.raw)
	}
	// AC-23: excuse An (reason required).
	if r := c.do(t, http.MethodPost, path+"/students/"+an.String()+":excuse", adA, idem(), map[string]any{"reason": ""}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("excuse without reason: %d %s", r.status, r.raw)
	}
	ex := must(t, c.do(t, http.MethodPost, path+"/students/"+an.String()+":excuse", adA, idem(), map[string]any{"reason": "Chuyển trường"}),
		http.StatusOK, "excuse")
	if ex.str("status") != "excused" || c.audits(ctx, t, "assignment.student_excused", &c.tenantA) != 1 {
		t.Fatalf("excuse: %s", ex.raw)
	}
	// Bình submits late inside the late window.
	c.clk.Advance(90 * time.Minute)
	a := must(t, asgStart(c, t, hsBinh, k.s.activityID, id), http.StatusCreated, "late start")
	c.submitAll(t, hsBinh, a)
	c.deliverAttempts(ctx, t, a.str("attempt_id"), false)
	if st, _, _ := c.submission(ctx, t, id, binh); st != "late" {
		t.Fatalf("late: %s", st)
	}
	// Close: An excused, Bình late; nothing missed.
	c.clk.Advance(2 * time.Hour)
	c.runScheduler(ctx, t)
	g := must(t, c.do(t, http.MethodGet, path, adA, nil, nil), http.StatusOK, "get")
	comp, _ := g.body["completion"].(map[string]any)
	if comp["assigned"] != float64(1) || comp["submitted"] != float64(1) || comp["excused"] != float64(1) || comp["missed"] != float64(0) {
		t.Fatalf("completion: %s", g.raw)
	}
	// AC-22: an extension of a missed student of another assignment reopens it; earlier than the due date 422.
	id2 := c.openAsg(t, k, map[string]any{"due_at": c.clk.Now().Add(time.Hour).Format(time.RFC3339), "feedback_timing": "after_close",
		"result_release": "manual"})
	c.clk.Advance(2 * time.Hour)
	c.runScheduler(ctx, t)
	if st, _, _ := c.submission(ctx, t, id2, binh); st != "missed" {
		t.Fatalf("missed before extension: %s", st)
	}
	p2 := assignmentsPath + "/" + id2 + "/students/" + binh.String() + ":extend"
	if r := c.do(t, http.MethodPost, p2, adA, idem(), map[string]any{"due_at": c.clk.Now().Add(-3 * time.Hour).Format(time.RFC3339), "reason": "Ốm"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("earlier extension: %d %s", r.status, r.raw)
	}
	key := idem()
	body := map[string]any{"due_at": c.clk.Now().Add(48 * time.Hour).Format(time.RFC3339), "reason": "Ốm hai ngày"}
	e := must(t, c.do(t, http.MethodPost, p2, adA, key, body), http.StatusOK, "extend")
	if e.str("status") != "not_started" || e.str("due_at_override") == "" {
		t.Fatalf("extension: %s", e.raw)
	}
	// AC-29: the same key replays, another payload with it is 409/422 IDEMPOTENCY_KEY_REUSED.
	if r := c.do(t, http.MethodPost, p2, adA, key, body); r.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, p2, adA, key, map[string]any{"due_at": body["due_at"], "reason": "Khác"}); r.str("code") != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("reused key: %d %s", r.status, r.raw)
	}
	if c.audits(ctx, t, "assignment.student_extended", &c.tenantA) != 1 {
		t.Fatal("extend audit")
	}
	must(t, asgStart(c, t, hsBinh, k.s.activityID, id2), http.StatusCreated, "start after the extension")
	// AC-25: submissions paged with a cursor.
	s1 := must(t, c.do(t, http.MethodGet, path+"/submissions?limit=1", adA, nil, nil), http.StatusOK, "page 1")
	cursor, _ := s1.body["page"].(map[string]any)["next_cursor"].(string)
	if len(s1.items("data")) != 1 || cursor == "" {
		t.Fatalf("page 1: %s", s1.raw)
	}
	s2 := must(t, c.do(t, http.MethodGet, path+"/submissions?limit=1&cursor="+cursor, adA, nil, nil), http.StatusOK, "page 2")
	if len(s2.items("data")) != 1 || s2.items("data")[0]["student_id"] == s1.items("data")[0]["student_id"] {
		t.Fatalf("page 2: %s", s2.raw)
	}
	if s := must(t, c.do(t, http.MethodGet, path+"/submissions?status=late", adA, nil, nil), http.StatusOK, "late"); len(s.items("data")) != 1 ||
		s.items("data")[0]["rapid_guess_count"] == nil || s.items("data")[0]["attempts_used"] != float64(1) {
		t.Fatalf("late filter: %s", s.raw)
	}
}

// OQ F062-2 (F041 T047): a question set assignment starts through startAdHocAttempt mode assignment with the same checks.
func TestAssignmentQuestionSet(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	set := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(k.s.qc, staticDef(k.s.questions...))), http.StatusCreated, "set").str("content_id")
	c.approve(ctx, t, qlcmA, "question_set", set, 1)
	must(t, c.publishV(ctx, t, qlcmA, "question_set", set, 1, nil, nil), http.StatusOK, "publish set")
	id := c.openAsg(t, k, map[string]any{"max_attempts": 1, "content_ref": map[string]any{"content_type": "question_set", "content_id": set}})
	my := must(t, c.do(t, http.MethodGet, "/learning/my-assignments", hsAn, nil, nil), http.StatusOK, "my")
	if it := my.items("data")[0]; it["content_kind"] != "question_set" || it["content_ref"] != set {
		t.Fatalf("my assignment: %s", my.raw)
	}
	body := map[string]any{"mode": "assignment", "source": map[string]any{"kind": "question_set", "question_set_id": set},
		"context_type": "assignment", "context_id": id}
	a := must(t, c.do(t, http.MethodPost, "/learning/activity-attempts", hsAn, idem(), body), http.StatusCreated, "start set")
	if a.str("mode") != "assignment" || a.str("context_id") != id || len(a.items("items")) != len(k.s.questions) {
		t.Fatalf("set attempt: %s", a.raw)
	}
	c.submitAll(t, hsAn, a)
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts", hsAn, idem(), body); r.status != http.StatusConflict || r.str("code") != "ASSIGNMENT_MAX_ATTEMPTS" {
		t.Fatalf("max attempts: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts", hsDung, idem(), body); r.status != http.StatusNotFound {
		t.Fatalf("other tenant: %d %s", r.status, r.raw)
	}
	noCtx := map[string]any{"mode": "assignment", "source": map[string]any{"kind": "question_set", "question_set_id": set}}
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts", hsBinh, idem(), noCtx); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("without context: %d %s", r.status, r.raw)
	}
	c.clk.Advance(80 * time.Hour)
	if r := c.do(t, http.MethodPost, "/learning/activity-attempts", hsBinh, idem(), body); r.status != http.StatusForbidden || r.str("code") != "ASSIGNMENT_NOT_OPEN" {
		t.Fatalf("after the due date: %d %s", r.status, r.raw)
	}
}

// F041-AC-30 (US6, TC-CMS-ASG-01): the catalog lists the quests and mini-games published and assigned to the tenant by
// module game (empty projection → empty list), filtered by objectives; an item not published for the tenant is 403 when
// assigned; /lms has no route changing a game configuration.
func TestAssignmentGameCatalog(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "listGameActivities", authztest.CrossTenant, authztest.RoleDenied)
	const path = "/lms/catalog/game-activities"
	if r := must(t, c.do(t, http.MethodGet, path, qlcmA, nil, nil), http.StatusOK, "empty"); len(r.items("data")) != 0 {
		t.Fatalf("empty projection: %s", r.raw)
	}
	quest, hidden := ids.New(), ids.New()
	// tenancyguard:ok TEST fixture under learning_platform: the projection module game feeds (F078)
	if _, err := c.platform.ExecContext(ctx, `INSERT INTO ref.game_activities (item_type, item_id, tenant_id, current_version_id, title_i18n,
		objective_ids, grade_levels, status, source_version) VALUES ('quest', $1, $3, $4, '{"vi":"Rừng phép nhân"}', ARRAY[$5::uuid], '{3}', 'published', 1),
		('minigame_config', $2, $3, $4, '{"vi":"Ẩn"}', '{}', '{3}', 'unpublished', 1)`, quest, hidden, c.tenantA, ids.New(), k.s.qc.lo); err != nil {
		t.Fatal(err)
	}
	r := must(t, c.do(t, http.MethodGet, path+"?objective_ids="+k.s.qc.lo+"&grade_level=3", qlcmA, nil, nil), http.StatusOK, "by objective")
	if items := r.items("data"); len(items) != 1 || items[0]["content_id"] != quest.String() || items[0]["title"] != "Rừng phép nhân" {
		t.Fatalf("catalog: %s", r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, path+"?objective_ids="+ids.New().String(), qlcmA, nil, nil), http.StatusOK, "other objective"); len(r.items("data")) != 0 {
		t.Fatalf("other objective: %s", r.raw)
	}
	if r := must(t, c.do(t, http.MethodGet, path, adB, nil, nil), http.StatusOK, "tenant B"); len(r.items("data")) != 0 {
		t.Fatalf("tenant B: %s", r.raw)
	}
	must(t, c.createAsg(t, adA, c.asgBody(k, map[string]any{"content_ref": map[string]any{"content_type": "quest", "content_id": quest.String()}})),
		http.StatusCreated, "assign the quest")
	if r := c.createAsg(t, adA, c.asgBody(k, map[string]any{"content_ref": map[string]any{"content_type": "minigame_config", "content_id": hidden.String()}})); r.status != http.StatusForbidden ||
		r.str("code") != "RESOURCE_NOT_ASSIGNED" {
		t.Fatalf("unpublished mini-game: %d %s", r.status, r.raw)
	}
	for id, o := range learninghttp.Operations {
		if strings.HasPrefix(o.Pattern, "/lms/") && strings.Contains(o.Pattern, "game") && o.Method != http.MethodGet {
			t.Fatalf("%s changes the game from /lms", id)
		}
	}
}
