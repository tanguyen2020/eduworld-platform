//go:build integration

package integration

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// docFacade wires the learning facade (trash, restore, clone) with the real handlers of F022 and F024.
func (c *curEnv) docFacade(t *testing.T) facade.Facade {
	t.Helper()
	reg, err := port.NewRegistry(append([]port.ResourceHandler{command.QuestionResource{Store: write.QuestionRepo{}}},
		command.DocResources(write.DocRepo{}, write.QuestionRepo{})...)...)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec([]byte("test-cursor-key-of-32-bytes-0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	return facade.Facade{
		Trash:       query.ListTrash{Session: c.read, Resources: reg, Cursors: codec},
		Restore:     command.RestoreResource{TrashDeps: c.trashDeps(reg)},
		Clone:       command.ExecuteCloneStep{CloneDeps: command.CloneDeps{UoW: c.uow, Resources: reg, Tenants: read.TenantDirectory{}, Audit: write.Auditor{R: c.rec}}},
		Idempotency: idempotency.NewStore(c.app, 0, nil),
	}
}

// F024-AC-21, AC-22, AC-23 (TC-TEN-019, TC-TEN-020, TC-CMS-ASG-02, TC-CMS-CLN-01, TC-TEN-015): an assigned platform set
// is read-only for tenant A and invisible to B; a clone copies the set and its tenant questions, keeps platform ones;
// the four types go to the trash and come back.
func TestContentAssignCloneTrash(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	f := c.docFacade(t)
	sa, _ := systemAdmin(ctx)
	// AC-21.
	pq, pqv := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform"})
	ps := must(t, c.do(t, http.MethodPost, setsPath, cmNoidung, idem(), setBody(qc, staticDef(pqv))), http.StatusCreated, "platform set")
	c.assignResource(ctx, t, "question_set", ps.str("content_id"), c.tenantA)
	c.assignResource(ctx, t, "question", pq, c.tenantA)
	got := must(t, c.do(t, http.MethodGet, setsPath+"/"+ps.str("content_id"), qlcmA, nil, nil), http.StatusOK, "assigned read")
	if got.body["assigned"] != true {
		t.Fatalf("assigned flag %s", got.raw)
	}
	must(t, c.do(t, http.MethodPatch, setsPath+"/"+ps.str("content_id")+"/versions/1", qlcmA, ifm(got.header.Get("ETag")),
		map[string]any{"title": map[string]any{"vi": "x"}}), http.StatusForbidden, "assigned is read-only")
	must(t, c.do(t, http.MethodGet, setsPath+"/"+ps.str("content_id"), qlcmB, nil, nil), http.StatusNotFound, "tenant B")
	// AC-22: set of A with an A question and the assigned platform question → clone to B.
	aq, aqv := c.question(t, qlcmA, qc, nil)
	as := must(t, c.do(t, http.MethodPost, setsPath, qlcmA, idem(), setBody(qc, staticDef(aqv, pqv))), http.StatusCreated, "set A")
	job := uuid.New()
	res, err := cloneStep(sa, f, job, c.tenantA, c.tenantB, "question_set", uuid.MustParse(as.str("content_id")))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cloned) != 1 {
		t.Fatalf("clone %+v", res)
	}
	newSet := res.Cloned[0].TargetId.String()
	cb := must(t, c.do(t, http.MethodGet, setsPath+"/"+newSet, qlcmB, nil, nil), http.StatusOK, "clone in B")
	if cb.str("status") != "draft" || cb.str("cloned_from_id") != as.str("content_id") {
		t.Fatalf("clone %s", cb.raw)
	}
	var items []string
	func() {
		rows, err := c.platform.QueryContext(ctx, `SELECT question_version_id FROM content.question_set_items WHERE question_set_id = $1 AND deleted_at IS NULL
		ORDER BY position`, newSet)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			items = append(items, s)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}()
	if len(items) != 2 || items[0] == aqv || items[1] != pqv {
		t.Fatalf("items %v (A question %s, platform %s)", items, aqv, pqv)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = $1 AND tenant_id = $2`,
		items[0], c.tenantB); n != 1 {
		t.Fatal("the tenant question was not cloned into B")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.questions WHERE cloned_from_id = $1 AND tenant_id = $2`, aq, c.tenantB); n != 1 {
		t.Fatal("clone origin of the question")
	}
	// AC-23: delete a draft of each type → trash → restore.
	ids := map[string]string{"question_set": as.str("content_id")}
	l := must(t, c.do(t, http.MethodPost, lessonsPath, qlcmA, idem(), lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("x")})),
		http.StatusCreated, "lesson")
	ids["lesson"] = l.str("content_id")
	mb := lessonBody(qc, map[string]any{"kind": "rich_text", "content": rich("x")})
	delete(mb, "completion_rule")
	mb["material_kind"] = "rich_text"
	ids["learning_material"] = must(t, c.do(t, http.MethodPost, materialsPath, qlcmA, idem(), mb), http.StatusCreated, "material").str("content_id")
	ids["assessment_blueprint"] = must(t, c.do(t, http.MethodPost, blueprintsPath, qlcmA, idem(), blueprintBody(qc, 1,
		[]any{map[string]any{"objective_ids": []string{qc.lo}, "count": 1, "points_each": 1}}, nil)), http.StatusCreated, "blueprint").str("content_id")
	paths := map[string]string{"question_set": setsPath, "lesson": lessonsPath, "learning_material": materialsPath, "assessment_blueprint": blueprintsPath}
	for rt, id := range ids {
		g := must(t, c.do(t, http.MethodGet, paths[rt]+"/"+id, qlcmA, nil, nil), http.StatusOK, "get "+rt)
		must(t, c.do(t, http.MethodDelete, paths[rt]+"/"+id, qlcmA, ifm(g.header.Get("ETag")), map[string]any{"reason": "dọn"}), http.StatusNoContent,
			"delete "+rt)
		if c.audits(ctx, t, command.ActionContentDeleted, &c.tenantA) == 0 {
			t.Fatal("no audit content.deleted")
		}
	}
	trash, err := listTrash(sa, f, c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, it := range trash {
		types = append(types, string(it.ResourceType))
	}
	for rt := range ids {
		if !slices.Contains(types, rt) {
			t.Fatalf("trash lacks %s: %v", rt, types)
		}
	}
	if _, err := asIdentity(sa, "restoreTrashItem", newKey(), func(ctx context.Context) (learningapi.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(ctx, learningapi.RestoreTrashItemRequestObject{ResourceType: "lesson", ResourceId: uuid.MustParse(ids["lesson"]),
			Body: &learningapi.RestoreTrashItemJSONRequestBody{Reason: "nhầm"}})
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, lessonsPath+"/"+ids["lesson"], qlcmA, nil, nil), http.StatusOK, "restored")
	if n := count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'learning_material.deleted'`); n != 1 {
		t.Fatalf("learning_material.deleted %d", n)
	}
}
