//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// outboxPayloads returns the payloads of the outbox events of a type about a subject id, checked against the payload
// schema of the type.
func (c *curEnv) outboxPayloads(ctx context.Context, t *testing.T, typ, field, id string) []map[string]any {
	t.Helper()
	path := filepath.Join(testkit.RepoRoot(t), "specs", "events", "payloads", "learning", typ+".v1.schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(path, doc); err != nil {
		t.Fatal(err)
	}
	sch, err := comp.Compile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.relay.QueryContext(ctx, `SELECT envelope->'payload' FROM infra.outbox_events WHERE event_type = $1
		AND envelope->'payload'->>`+"'"+field+"'"+` = $2 ORDER BY created_at`, typ, id)
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

// F040-AC-13…AC-23, AC-28 (US3, US4; quickstart 10–13; TC-SOC-008, TC-TEN-015): schedule items in scope, soft-deleted
// with schedule_item.deleted, due dates from assignment.published once, the student's schedule; study groups with
// members of the school (and class), unique names, If-Match, soft-delete, study_group.* in the outbox without names.
func TestLMSScheduleAndStudyGroups(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.lmsSetup(ctx, t)
	authztest.Cover(t, "listScheduleItems", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createScheduleItem", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateScheduleItem", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteScheduleItem", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listMySchedule", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "listStudyGroups", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "createStudyGroup", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "getStudyGroup", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "updateStudyGroup", authztest.CrossTenant, authztest.RoleDenied)
	authztest.Cover(t, "deleteStudyGroup", authztest.CrossTenant, authztest.RoleDenied)
	courseID := k.courses[k.s.qc.subject]
	day := func(d int) string { return c.clk.Now().Add(time.Duration(d) * 24 * time.Hour).Format(time.DateOnly) }

	// US3: create, list, update, the student's schedule, delete = soft-delete.
	item := must(t, c.do(t, http.MethodPost, "/lms/schedules", qlcmA, idem(), map[string]any{"course_id": courseID, "date": day(7), "period": 2,
		"kind": "lesson", "note": "Bài 12"}), http.StatusCreated, "schedule item")
	id := item.str("schedule_item_id")
	if r := c.do(t, http.MethodPost, "/lms/schedules", adA2, idem(), map[string]any{"course_id": courseID, "date": day(7), "kind": "event"}); r.status != 404 {
		t.Fatalf("other school: %d", r.status)
	}
	rng := "?from=" + day(0) + "&to=" + day(30)
	if l := must(t, c.do(t, http.MethodGet, "/lms/schedules"+rng, adA, nil, nil), 200, "list").items("data"); len(l) != 1 || l[0]["schedule_item_id"] != id {
		t.Fatalf("list: %v", l)
	}
	if l := must(t, c.do(t, http.MethodGet, "/lms/schedules"+rng, adA2, nil, nil), 200, "list other school").items("data"); len(l) != 0 {
		t.Fatalf("other school sees: %v", l)
	}
	if r := c.do(t, http.MethodPatch, "/lms/schedules/"+id, adA, ifm(`W/"1"`), map[string]any{"course_id": courseID, "date": day(8), "kind": "lesson"}); r.status != 200 ||
		r.str("date") != day(8) {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}
	if r := c.do(t, http.MethodPatch, "/lms/schedules/"+id, adA, ifm(`W/"1"`), map[string]any{"course_id": courseID, "date": day(9), "kind": "lesson"}); r.status != 412 {
		t.Fatalf("stale: %d", r.status)
	}
	mine := func() []map[string]any {
		return must(t, c.do(t, http.MethodGet, "/learning/my-schedule"+rng, hsAn, nil, nil), 200, "my schedule").items("items")
	}
	if m := mine(); len(m) != 1 || m[0]["subject_id"] != k.s.qc.subject {
		t.Fatalf("my schedule: %v", m)
	}
	if r := c.do(t, http.MethodGet, "/learning/my-schedule?from="+day(0)+"&to="+day(70), hsAn, nil, nil); r.status != 422 {
		t.Fatalf("63 days: %d", r.status)
	}
	if m := must(t, c.do(t, http.MethodGet, "/learning/my-schedule"+rng, hsDung, nil, nil), 200, "dung").items("items"); len(m) != 0 {
		t.Fatalf("tenant B student sees: %v", m)
	}
	// AC-15: assignment.published of the course twice (same event) → one item.
	eventID, assignment := ids.New(), ids.New()
	payload, _ := json.Marshal(map[string]any{"assignment_id": assignment, "course_id": courseID, "due_at": c.clk.Now().Add(48 * time.Hour)})
	sc, _ := authz.EventScope(c.tenantA)
	tenant := c.tenantA
	for i := 0; i < 2; i++ {
		if err := c.uow.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
			return consumer.LMSSchedule{Schedule: c.lms}.Handle(ctx, outbox.Envelope{EventID: eventID, EventType: "assignment.published",
				TenantID: &tenant, Payload: payload})
		}); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.schedule_items WHERE ref_id = $1 AND kind = 'assignment_due'
		AND deleted_at IS NULL`, assignment); n != 1 {
		t.Fatalf("items of the assignment: %d", n)
	}
	// AC-14: delete = soft-delete, event, audit, gone from the reads.
	if r := c.do(t, http.MethodDelete, "/lms/schedules/"+id, adA, nil, nil); r.status != 204 {
		t.Fatalf("delete: %d %s", r.status, r.raw)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.schedule_items WHERE schedule_item_id = $1 AND deleted_at IS NOT NULL
		AND deleted_by IS NOT NULL`, id); n != 1 {
		t.Fatal("soft-delete")
	}
	if ev := c.outboxPayloads(ctx, t, "schedule_item.deleted", "schedule_item_id", id); len(ev) != 1 {
		t.Fatal("schedule_item.deleted")
	}
	for _, it := range mine() {
		if it["schedule_item_id"] == id {
			t.Fatal("deleted item listed")
		}
	}
	if r := c.do(t, http.MethodDelete, "/lms/schedules/"+id, adA, nil, nil); r.status != 404 {
		t.Fatalf("delete twice: %d", r.status)
	}

	// US4: study groups.
	groups := "/lms/schools/" + k.school.String() + "/study-groups"
	an, binh := c.users[hsAn].UserID.String(), c.users[hsBinh].UserID.String()
	key := idem()
	g := must(t, c.do(t, http.MethodPost, groups, adA, key, map[string]any{"name": "Ôn chia có dư", "class_id": k.class, "member_ids": []string{an}}),
		http.StatusCreated, "create group")
	gid := g.str("study_group_id")
	if g.str("member_count") != "1" || g.str("grade_level") != "3" {
		t.Fatalf("group: %s", g.raw)
	}
	if again := must(t, c.do(t, http.MethodPost, groups, adA, key, map[string]any{"name": "Ôn chia có dư", "class_id": k.class,
		"member_ids": []string{an}}), http.StatusCreated, "replay"); again.str("study_group_id") != gid {
		t.Fatal("replay")
	}
	if r := c.do(t, http.MethodPost, groups, qlcmA, idem(), map[string]any{"name": "ÔN CHIA CÓ DƯ", "grade_level": 3, "member_ids": []string{binh}}); r.status != 409 {
		t.Fatalf("duplicate name: %d %s", r.status, r.raw)
	}
	for _, bad := range []map[string]any{
		{"name": "B", "grade_level": 3, "member_ids": []string{c.users[hsDung].UserID.String()}},
		{"name": "C", "grade_level": 3, "member_ids": []string{uuid.NewString()}},
		{"name": strings.Repeat("x", 65), "grade_level": 3, "member_ids": []string{an}},
		{"name": "D", "grade_level": 3, "member_ids": []string{}},
		{"name": "E", "member_ids": []string{an}},
	} {
		if r := c.do(t, http.MethodPost, groups, adA, idem(), bad); r.status != 422 || strings.Contains(r.raw, c.users[hsDung].UserID.String()) {
			t.Fatalf("invalid %v: %d %s", bad["name"], r.status, r.raw)
		}
	}
	got := must(t, c.do(t, http.MethodGet, groups+"/"+gid, adA, nil, nil), 200, "get group")
	if got.header.Get("ETag") != `W/"1"` || len(got.items("member_ids")) != 0 && got.body["member_ids"].([]any)[0] != an {
		t.Fatalf("get: %s", got.raw)
	}
	if l := must(t, c.do(t, http.MethodGet, groups+"?class_id="+k.class.String(), adA, nil, nil), 200, "list").items("data"); len(l) != 1 {
		t.Fatalf("list groups: %v", l)
	}
	if r := c.do(t, http.MethodGet, groups+"/"+gid, adA2, nil, nil); r.status != 404 {
		t.Fatalf("other school: %d", r.status)
	}
	if r := c.do(t, http.MethodPatch, groups+"/"+gid, adA, nil, map[string]any{"name": "x", "member_ids": []string{an}}); r.status != 428 {
		t.Fatalf("no If-Match: %d", r.status)
	}
	up := must(t, c.do(t, http.MethodPatch, groups+"/"+gid, adA, ifm(`W/"1"`), map[string]any{"name": "Ôn chia", "class_id": k.class,
		"member_ids": []string{binh}}), 200, "update group")
	if up.str("member_count") != "1" || up.str("name") != "Ôn chia" {
		t.Fatalf("update: %s", up.raw)
	}
	if r := c.do(t, http.MethodPatch, groups+"/"+gid, adA, ifm(`W/"1"`), map[string]any{"name": "y", "class_id": k.class, "member_ids": []string{an}}); r.status != 412 {
		t.Fatalf("stale group: %d", r.status)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM lms.study_group_members WHERE study_group_id = $1 AND student_id = $2
		AND deleted_at IS NOT NULL`, gid, an); n != 1 {
		t.Fatal("removed member soft-deleted")
	}
	if r := c.do(t, http.MethodDelete, groups+"/"+gid, adA, nil, nil); r.status != 204 {
		t.Fatalf("delete group: %d", r.status)
	}
	if l := must(t, c.do(t, http.MethodGet, groups, adA, nil, nil), 200, "after delete").items("data"); len(l) != 0 {
		t.Fatalf("deleted group listed: %v", l)
	}
	// AC-23: one event per change, schema-valid, ids only.
	created := c.outboxPayloads(ctx, t, "study_group.created", "study_group_id", gid)
	updated := c.outboxPayloads(ctx, t, "study_group.updated", "study_group_id", gid)
	changed := c.outboxPayloads(ctx, t, "study_group.member_changed", "study_group_id", gid)
	deleted := c.outboxPayloads(ctx, t, "study_group.deleted", "study_group_id", gid)
	if len(created) != 1 || len(updated) != 1 || len(changed) != 1 || len(deleted) != 1 {
		t.Fatalf("events: %d %d %d %d", len(created), len(updated), len(changed), len(deleted))
	}
	if a := changed[0]["added_member_ids"].([]any); len(a) != 1 || a[0] != binh {
		t.Fatalf("member_changed: %v", changed[0])
	}
	for _, action := range []string{"study_group.created", "study_group.updated", "study_group.deleted", "schedule_item.created",
		"schedule_item.updated", "schedule_item.deleted"} {
		if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
			AND envelope->'payload'->>'action' = $1`, action); n < 1 {
			t.Fatalf("audit %s", action)
		}
	}
}
