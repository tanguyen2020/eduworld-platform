//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// payloadSchema compiles a payload schema of module learning with the common enums it references.
func payloadSchema(t *testing.T, typ string) *jsonschema.Schema {
	t.Helper()
	root := testkit.RepoRoot(t)
	comp := jsonschema.NewCompiler()
	for url, path := range map[string]string{
		"https://schemas.eduworld.vn/common/enums.schema.json":                            filepath.Join(root, "specs", "schemas", "common", "enums.schema.json"),
		"https://schemas.eduworld.vn/events/payloads/learning/" + typ + ".v1.schema.json": filepath.Join(root, "specs", "events", "payloads", "learning", typ+".v1.schema.json"),
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := comp.AddResource(url, doc); err != nil {
			t.Fatal(err)
		}
	}
	sch, err := comp.Compile("https://schemas.eduworld.vn/events/payloads/learning/" + typ + ".v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

// validPayloads checks every outbox payload of a type about an assignment against its schema and returns them.
func (c *curEnv) validPayloads(ctx context.Context, t *testing.T, typ, assignment string) []map[string]any {
	t.Helper()
	schema := typ
	if typ == "assignment.published" {
		schema = "assignment.created" // the payload of assignment.published (EVENT_DESIGN §4.3)
	}
	sch := payloadSchema(t, schema)
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>'assignment_id' = $2 ORDER BY created_at, event_id`, typ, assignment)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		_ = rows.Scan(&raw)
		var v any
		_ = json.Unmarshal(raw, &v)
		if err := sch.Validate(v); err != nil {
			t.Fatalf("%s payload: %v (%s)", typ, err, raw)
		}
		out = append(out, v.(map[string]any))
	}
	return out
}

func asgStart(c *curEnv, t *testing.T, sub, activity, assignment string) resp {
	t.Helper()
	return c.start(t, sub, activity, nil, map[string]any{"mode": "assignment", "context_type": "assignment", "context_id": assignment})
}

// submitAll answers every item right and submits the attempt.
func (c *curEnv) submitAll(t *testing.T, sub string, r resp) {
	t.Helper()
	id := r.str("attempt_id")
	right, _ := correctOption()
	for pos := range r.items("items") {
		c.clk.Advance(answerTime)
		must(t, c.answer(t, sub, id, pos+1, map[string]any{"option_id": right}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+id+":submit", sub, idem(), nil), http.StatusOK, "submit")
}

// F041-AC-12, AC-13, AC-15, AC-17 (US2, TC-LMS-002): the student sees the assignments of the targets with the
// submission, starts an attempt mode assignment (resumed while in progress), at most max_attempts; the events update the
// submission once each, assignment.completed once; outside the targets 404, outside the window 403.
func TestAssignmentStudentFlow(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	authztest.Cover(t, "listAssignmentSubmissions", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMyAssignments", authztest.CrossTenant, authztest.RoleDenied)
	id := c.openAsg(t, k, nil)
	// AC-17: assignment.published follows assignment.created.v1.
	if ps := c.validPayloads(ctx, t, "assignment.published", id); len(ps) != 1 || ps[0]["status"] != "open" {
		t.Fatalf("published payload: %v", ps)
	}
	// AC-12: the student of 3A sees it with the subject name (OQ F062-1); a student of another tenant does not.
	my := must(t, c.do(t, http.MethodGet, "/learning/my-assignments", hsAn, nil, nil), http.StatusOK, "my assignments")
	items := my.items("data")
	if len(items) != 1 || items[0]["assignment_id"] != id || items[0]["status"] != "not_started" || items[0]["attempts_left"] != float64(2) ||
		items[0]["content_kind"] != "activity" || items[0]["subject_name"] == nil {
		t.Fatalf("my assignments: %s", my.raw)
	}
	if d := must(t, c.do(t, http.MethodGet, "/learning/my-assignments", hsDung, nil, nil), http.StatusOK, "other tenant"); len(d.items("data")) != 0 {
		t.Fatalf("tenant B: %s", d.raw)
	}
	if r := c.do(t, http.MethodGet, "/learning/my-assignments", adA, nil, nil); r.status != http.StatusForbidden {
		t.Fatalf("staff on the student route: %d %s", r.status, r.raw)
	}
	// AC-13: an assignment for Bình only is 404 for An.
	binhOnly := c.openAsg(t, k, map[string]any{"targets": []any{map[string]any{"target_type": "students",
		"student_ids": []string{c.users[hsBinh].UserID.String()}}}})
	if r := asgStart(c, t, hsAn, k.s.activityID, binhOnly); r.status != http.StatusNotFound {
		t.Fatalf("not targeted: %d %s", r.status, r.raw)
	}
	// Start, resume, submit.
	a1 := must(t, asgStart(c, t, hsAn, k.s.activityID, id), http.StatusCreated, "start")
	if a1.str("mode") != "assignment" || a1.str("context_type") != "assignment" || a1.str("context_id") != id ||
		a1.str("activity_version_id") != c.activityVersion(ctx, t, k.s.activityID) {
		t.Fatalf("attempt: %s", a1.raw)
	}
	if again := must(t, asgStart(c, t, hsAn, k.s.activityID, id), http.StatusCreated, "resume"); again.str("attempt_id") != a1.str("attempt_id") {
		t.Fatalf("resume: %s", again.raw)
	}
	c.deliverAttempts(ctx, t, a1.str("attempt_id"), false)
	if st, used, _ := c.submission(ctx, t, id, c.users[hsAn].UserID); st != "in_progress" || used != 1 {
		t.Fatalf("in progress: %s %d", st, used)
	}
	if my := must(t, c.do(t, http.MethodGet, "/learning/my-assignments", hsAn, nil, nil), http.StatusOK, "my"); my.items("data")[0]["attempt_id"] != a1.str("attempt_id") {
		t.Fatalf("attempt in progress: %s", my.raw)
	}
	c.submitAll(t, hsAn, a1)
	// AC-15: each event applied twice changes the submission once; assignment.completed once.
	c.deliverAttempts(ctx, t, a1.str("attempt_id"), true)
	st, used, score := c.submission(ctx, t, id, c.users[hsAn].UserID)
	if st != "submitted" || used != 1 || score == nil || *score <= 0 {
		t.Fatalf("submitted: %s %d %v", st, used, score)
	}
	if ps := c.validPayloads(ctx, t, "assignment.completed", id); len(ps) != 1 || ps[0]["late"] != false {
		t.Fatalf("completed: %v", ps)
	}
	c.validPayloads(ctx, t, "assignment.submission_updated", id)
	// Second attempt allowed, third is 409.
	a2 := must(t, asgStart(c, t, hsAn, k.s.activityID, id), http.StatusCreated, "second attempt")
	c.submitAll(t, hsAn, a2)
	if r := asgStart(c, t, hsAn, k.s.activityID, id); r.status != http.StatusConflict || r.str("code") != "ASSIGNMENT_MAX_ATTEMPTS" {
		t.Fatalf("max attempts: %d %s", r.status, r.raw)
	}
	c.deliverAttempts(ctx, t, a2.str("attempt_id"), false)
	if _, used, _ := c.submission(ctx, t, id, c.users[hsAn].UserID); used != 2 {
		t.Fatalf("attempts used = %d", used)
	}
	// AC-25: the submissions of the assignment.
	subs := must(t, c.do(t, http.MethodGet, assignmentsPath+"/"+id+"/submissions", adA, nil, nil), http.StatusOK, "submissions")
	if len(subs.items("data")) != 2 {
		t.Fatalf("submissions: %s", subs.raw)
	}
	if r := c.do(t, http.MethodGet, assignmentsPath+"/"+id+"/submissions", adB, nil, nil); r.status != http.StatusNotFound {
		t.Fatalf("submissions of tenant B: %d", r.status)
	}
	// AC-13: a scheduled assignment is not open yet.
	later := c.clk.Now().Add(24 * time.Hour)
	sched := c.openAsg(t, k, map[string]any{"open_at": later.Format(time.RFC3339), "due_at": later.Add(48 * time.Hour).Format(time.RFC3339)})
	if r := asgStart(c, t, hsBinh, k.s.activityID, sched); r.status != http.StatusForbidden || r.str("code") != "ASSIGNMENT_NOT_OPEN" {
		t.Fatalf("scheduled: %d %s", r.status, r.raw)
	}
	// Another activity than the one of the assignment.
	other := c.quizOf(ctx, t, k.s, k.s.questions[0])
	if r := asgStart(c, t, hsBinh, other, id); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("other activity: %d %s", r.status, r.raw)
	}
}

// F041-AC-14: an assignment keeps the version pinned at creation when the activity is published again; follow_latest gives
// the new version to new attempts while the attempt in progress keeps its version.
func TestAssignmentVersionPinning(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	v1 := c.activityVersion(ctx, t, k.s.activityID)
	pinned := c.openAsg(t, k, nil)
	latest := c.openAsg(t, k, map[string]any{"follow_latest": true})
	inProgress := must(t, asgStart(c, t, hsBinh, k.s.activityID, latest), http.StatusCreated, "start before the new version")
	must(t, c.do(t, http.MethodPost, "/cms/learning/activities/"+k.s.activityID+"/versions", qlcmA, idem(), map[string]any{"from_version": 1}),
		http.StatusCreated, "new version")
	c.publishActivity(ctx, t, k.s.activityID, 2)
	v2 := c.activityVersion(ctx, t, k.s.activityID)
	if v2 == v1 {
		t.Fatal("version 2 not current")
	}
	if a := must(t, asgStart(c, t, hsAn, k.s.activityID, pinned), http.StatusCreated, "pinned"); a.str("activity_version_id") != v1 {
		t.Fatalf("pinned version: %s", a.raw)
	}
	if a := must(t, asgStart(c, t, hsAn, k.s.activityID, latest), http.StatusCreated, "latest"); a.str("activity_version_id") != v2 {
		t.Fatalf("latest version: %s", a.raw)
	}
	if a := must(t, asgStart(c, t, hsBinh, k.s.activityID, latest), http.StatusCreated, "resume"); a.str("attempt_id") != inProgress.str("attempt_id") ||
		a.str("activity_version_id") != v1 {
		t.Fatalf("in progress keeps v1: %s", a.raw)
	}
}

// F041-AC-16: a student joining the class while the assignment is open is added; a locked account is excused before the
// due date.
func TestAssignmentEnrollmentChanges(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	id := c.openAsg(t, k, nil)
	c.user("hs.moi.3a", authztest.Student(c.tenantA).PrincipalData, false)
	c.enrollIn(ctx, t, "hs.moi.3a", k.school, k.class, 3)
	newcomer := c.users["hs.moi.3a"].UserID
	payload, _ := json.Marshal(map[string]any{"student_id": newcomer, "class_id": k.class, "op": "added", "status": "active"})
	c.enrollmentEvent(ctx, t, "enrollment.changed", payload)
	if st, _, _ := c.submission(ctx, t, id, newcomer); st != "not_started" {
		t.Fatalf("newcomer: %s", st)
	}
	payload, _ = json.Marshal(map[string]any{"user_id": c.users[hsBinh].UserID, "reason": "locked"})
	c.enrollmentEvent(ctx, t, "user.access_revoked", payload)
	if st, _, _ := c.submission(ctx, t, id, c.users[hsBinh].UserID); st != "excused" {
		t.Fatalf("locked: %s", st)
	}
	if r := asgStart(c, t, "hs.moi.3a", k.s.activityID, id); r.status != http.StatusCreated {
		t.Fatalf("newcomer starts: %d %s", r.status, r.raw)
	}
}

// F041-AC-15 (edge: re-evaluation): attempt.re_evaluated updates the score of the submission from the new effective
// evaluation, the attempt kept by the score policy.
func TestAssignmentReEvaluated(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	id := c.openAsg(t, k, map[string]any{"feedback_timing": "after_submit"})
	a := must(t, asgStart(c, t, hsAn, k.s.activityID, id), http.StatusCreated, "start")
	_, wrong := correctOption()
	for pos := range a.items("items") {
		c.clk.Advance(answerTime)
		must(t, c.answer(t, hsAn, a.str("attempt_id"), pos+1, map[string]any{"option_id": wrong}, nil), http.StatusOK, "answer")
	}
	must(t, c.do(t, http.MethodPost, "/learning/activity-attempts/"+a.str("attempt_id")+":submit", hsAn, idem(), nil), http.StatusOK, "submit")
	c.deliverAttempts(ctx, t, a.str("attempt_id"), false)
	if st, _, score := c.submission(ctx, t, id, c.users[hsAn].UserID); st != "submitted" || score == nil || *score != 0 {
		t.Fatalf("before: %s %v", st, score)
	}
	must(t, c.do(t, http.MethodPost, "/cms/learning/regrade-requests", qlcmA, idem(), map[string]any{"scope": "question_version",
		"question_version_id": k.s.questions[0], "action": "accept_all", "reason": "Câu 1 có hai đáp án đúng"}), http.StatusAccepted, "regrade")
	if n, err := c.rgd.RunOnce(ctx); err != nil || n != 1 {
		t.Fatalf("regrade run: %d %v", n, err)
	}
	c.deliverAttempts(ctx, t, a.str("attempt_id"), false)
	if _, _, score := c.submission(ctx, t, id, c.users[hsAn].UserID); score == nil || *score <= 0 {
		t.Fatalf("after re-evaluation: %v", score)
	}
}
