//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
)

// F030 T055 (OQ owner 2026-11-20, receiving side of the platform-ops facade): learning accepts resource_type activity
// for assign/revoke, clone (tenant questions cloned, platform questions kept), trash and restore — like question_set.
func TestActivityResourcePlatformOps(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	qc := c.qbSetup(ctx, t)
	f := c.docFacade(t)
	reg, err := port.NewRegistry(append([]port.ResourceHandler{command.QuestionResource{Store: write.QuestionRepo{}}},
		command.DocResources(write.DocRepo{}, write.QuestionRepo{})...)...)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec([]byte("test-cursor-key-of-32-bytes-0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	deps := command.AssignmentDeps{UoW: c.uow, Assignments: write.AssignmentRepo{}, Tenants: read.TenantDirectory{}, Resources: reg,
		Audit: write.Auditor{R: c.rec}, Metrics: observability.AssignmentCounter{}}
	f.Apply, f.Revoke = command.ApplyTenantAssignments{AssignmentDeps: deps}, command.RevokeTenantAssignments{AssignmentDeps: deps}
	f.Assignments = query.ListResourceAssignments{Session: c.read, Assignments: read.AssignmentReadModel{}, Cursors: codec}
	sa, _ := systemAdmin(ctx)
	const path = "/cms/learning/activities/"

	// Assign / revoke a platform activity.
	pq, _ := c.question(t, cmNoidung, qc, map[string]any{"owner_scope": "platform"})
	pqv := c.publishQuestion(ctx, t, pq, 1)
	pb := activityBody(qc, "quiz", quizConfig(pqv))
	pb["owner_scope"] = "platform"
	plat := uuid.MustParse(c.createActivity(t, cmNoidung, pb))
	must(t, c.do(t, http.MethodGet, path+plat.String(), qlcmA, nil, nil), http.StatusNotFound, "not assigned yet")
	if _, err := asIdentity(sa, "applyTenantAssignments", newKey(), func(ctx context.Context) (learningapi.ApplyTenantAssignmentsResponseObject, error) {
		return f.ApplyTenantAssignments(ctx, learningapi.ApplyTenantAssignmentsRequestObject{Body: &learningapi.ApplyTenantAssignmentsJSONRequestBody{
			ResourceType: "activity", ResourceId: plat, TenantIds: []uuid.UUID{c.tenantA}}})
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, path+plat.String(), qlcmA, nil, nil), http.StatusOK, "assigned to A")
	must(t, c.do(t, http.MethodGet, path+plat.String(), qlcmB, nil, nil), http.StatusNotFound, "tenant B")
	if _, err := asIdentity(sa, "revokeTenantAssignments", newKey(), func(ctx context.Context) (learningapi.RevokeTenantAssignmentsResponseObject, error) {
		return f.RevokeTenantAssignments(ctx, learningapi.RevokeTenantAssignmentsRequestObject{Body: &learningapi.RevokeTenantAssignmentsJSONRequestBody{
			ResourceType: "activity", ResourceId: plat, TenantIds: []uuid.UUID{c.tenantA}, Reason: "hết hợp đồng"}})
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, path+plat.String(), qlcmA, nil, nil), http.StatusNotFound, "revoked")

	// Clone an activity of A (an A question + an assigned platform question) into B.
	c.assignResource(ctx, t, "question", pq, c.tenantA)
	aq, _ := c.question(t, qlcmA, qc, nil)
	aqv := c.publishQuestion(ctx, t, aq, 1)
	act := uuid.MustParse(c.createActivity(t, qlcmA, activityBody(qc, "quiz", quizConfig(aqv, pqv))))
	res, err := cloneStep(sa, f, uuid.New(), c.tenantA, c.tenantB, "activity", act)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cloned) != 1 {
		t.Fatalf("clone %+v", res)
	}
	cb := must(t, c.do(t, http.MethodGet, path+res.Cloned[0].TargetId.String(), qlcmB, nil, nil), http.StatusOK, "clone in B")
	if cb.str("status") != "draft" || cb.str("activity_type") != "quiz" {
		t.Fatalf("clone %s", cb.raw)
	}
	var cfg struct {
		Content struct {
			QuestionSource struct {
				Questions []struct {
					QuestionVersionID string `json:"question_version_id"`
				} `json:"questions"`
			} `json:"question_source"`
		} `json:"content"`
	}
	raw, _ := json.Marshal(cb.body["config"])
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	qs := cfg.Content.QuestionSource.Questions
	if len(qs) != 2 || qs[0].QuestionVersionID == aqv || qs[1].QuestionVersionID != pqv {
		t.Fatalf("config questions %+v (A %s, platform %s)", qs, aqv, pqv)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.question_versions WHERE question_version_id = $1 AND tenant_id = $2`,
		qs[0].QuestionVersionID, c.tenantB); n != 1 {
		t.Fatal("the tenant question was not cloned into B")
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM content.activities WHERE cloned_from_id = $1 AND tenant_id = $2`, act, c.tenantB); n != 1 {
		t.Fatal("clone origin of the activity")
	}

	// Trash and restore a draft activity of A.
	if err := c.uow.Do(authztest.Ctx(ctx, authzPrincipal(c, qlcmA)), func(ctx context.Context) error {
		return command.SoftDeleteResource{TrashDeps: c.trashDeps(reg)}.Handle(ctx, command.SoftDeleteInput{ResourceType: "activity", ID: act,
			Reason: "dọn"})
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, path+act.String(), qlcmA, nil, nil), http.StatusNotFound, "deleted")
	trash, err := listTrash(sa, f, c.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(trash, func(it learningapi.CommonInternalCommonV1TrashItem) bool {
		return it.ResourceType == "activity" && it.ResourceId == act
	}) {
		t.Fatalf("trash lacks the activity: %+v", trash)
	}
	if _, err := asIdentity(sa, "restoreTrashItem", newKey(), func(ctx context.Context) (learningapi.RestoreTrashItemResponseObject, error) {
		return f.RestoreTrashItem(ctx, learningapi.RestoreTrashItemRequestObject{ResourceType: "activity", ResourceId: act,
			Body: &learningapi.RestoreTrashItemJSONRequestBody{Reason: "nhầm"}})
	}); err != nil {
		t.Fatal(err)
	}
	must(t, c.do(t, http.MethodGet, path+act.String(), qlcmA, nil, nil), http.StatusOK, "restored")
}
