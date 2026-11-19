//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func cloneStep(ctx context.Context, f facade.Facade, job, source, target uuid.UUID, rt string, items ...uuid.UUID) (learningapi.ExecuteTenantCloneStep200JSONResponse, error) {
	res, err := asIdentity(ctx, "executeTenantCloneStep", newKey(), func(ctx context.Context) (learningapi.ExecuteTenantCloneStepResponseObject, error) {
		return f.ExecuteTenantCloneStep(ctx, learningapi.ExecuteTenantCloneStepRequestObject{JobId: job,
			Body: &learningapi.ExecuteTenantCloneStepJSONRequestBody{SourceTenantId: source, TargetTenantId: target,
				ResourceType: learningapi.CommonInternalCommonV1ResourceType(rt), ResourceIds: items}})
	})
	if err != nil {
		return learningapi.ExecuteTenantCloneStep200JSONResponse{}, err
	}
	return res.(learningapi.ExecuteTenantCloneStep200JSONResponse), nil
}

// F020-AC-27 (TC-TEN-020, TC-CMS-CLN-01): the clone step deep-copies items of the source tenant into the target
// (new ids, target tenant, a new draft version, cloned_from_*), returns the id map and replays it for the same
// (job_id, resource_type). F020-AC-28: no student data is copied and platform references are kept.
// F020-AC-29: an element that is not a live item of the source tenant, or a type without cloner, fails with a
// reason and leaves no partial copy. F020-AC-30: audit in both tenants.
func TestCloneStep(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t, true)
	f := e.facadeOf(t, nil)
	sa, saP := systemAdmin(ctx)
	editorA := authztest.Ctx(ctx, authztest.Staff(e.tenantA, "tenant_content_editor"))
	a1 := tenantItem(ctx, t, e, e.tenantA, "a-1")
	a2 := tenantItem(ctx, t, e, e.tenantA, "a-2")
	gone := tenantItem(ctx, t, e, e.tenantA, "a-gone")
	b1 := tenantItem(ctx, t, e, e.tenantB, "b-1")
	curriculum := uuid.New() // platform curriculum version referenced by the content (kept as is)
	student := uuid.New()
	var v1 uuid.UUID
	if err := e.uow.Do(editorA, func(ctx context.Context) error {
		c := fixtures.Client(ctx)
		v, err := c.FixtureContentVersion.Create().SetFixtureContentID(a1).SetVersionNo(1).SetStatus("published").
			SetAuthorID(uuid.New()).SetCurriculumVersionID(curriculum).SetBody("nội dung câu hỏi").Save(ctx)
		if err != nil {
			return err
		}
		v1 = v.ID
		if err := c.FixtureAttempt.Create().SetStudentID(student).SetFixtureContentVersionID(v1).SetScore(9).Exec(ctx); err != nil {
			return err
		}
		return c.FixtureContent.DeleteOneID(gone).Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}

	job := ids.New()
	res, err := cloneStep(sa, f, job, e.tenantA, e.tenantB, "question", a1, a2, b1, gone)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "partially_failed" || len(res.Cloned) != 2 || res.Skipped == nil || len(*res.Skipped) != 2 {
		t.Fatalf("clone step = %+v", res)
	}
	idMap := map[uuid.UUID]uuid.UUID{}
	for _, c := range res.Cloned {
		idMap[c.SourceId] = c.TargetId
	}
	x1, ok := idMap[a1]
	if !ok || x1 == a1 || idMap[a2] == uuid.Nil {
		t.Fatalf("id map = %v", idMap)
	}
	for _, s := range *res.Skipped {
		if s.Reason == "" || (s.SourceId != b1 && s.SourceId != gone) {
			t.Fatalf("skipped = %+v", *res.Skipped)
		}
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE fixture_content_id = $1 AND tenant_id = $2
		AND owner_scope = 'tenant' AND cloned_from_id = $3 AND cloned_from_tenant_id = $4 AND cloned_by = $5 AND cloned_at IS NOT NULL
		AND latest_version_no = 1 AND current_version_id IS NULL`, x1, e.tenantB, a1, e.tenantA, saP.UserID); n != 1 {
		t.Fatal("clone of a1 is not a tenant B item with its origin")
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_content_versions WHERE fixture_content_id = $1 AND tenant_id = $2
		AND status = 'draft' AND version_no = 1 AND curriculum_version_id = $3 AND cloned_from_id = $4 AND body = 'nội dung câu hỏi'`,
		x1, e.tenantB, curriculum, v1); n != 1 {
		t.Fatal("clone of a1 has no draft version copying the content and keeping the platform reference")
	}
	// F020-AC-28: no attempt (student data) in the target; the source attempt is untouched.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_attempts WHERE tenant_id = $1`, e.tenantB); n != 0 {
		t.Fatalf("%d attempts copied to the target", n)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_attempts WHERE tenant_id = $1 AND student_id = $2`, e.tenantA, student); n != 1 {
		t.Fatal("source attempt changed")
	}
	// F020-AC-29: no partial copy of the failed elements.
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE tenant_id = $1`, e.tenantB); n != 3 { // b1 + 2 clones
		t.Fatalf("tenant B items = %d, want 3", n)
	}
	// F020-AC-30: audit in the source and the target tenant.
	for _, tn := range []uuid.UUID{e.tenantA, e.tenantB} {
		if n := e.audits(ctx, t, "tenant_data.cloned", &tn); n != 1 {
			t.Fatalf("audit tenant_data.cloned in %s = %d", tn, n)
		}
	}
	// Tenant B sees and may edit its copy; the source stays in A.
	if !visibleTo(ctx, t, e, e.tenantB, x1) || visibleTo(ctx, t, e, e.tenantB, a1) {
		t.Fatal("visibility after clone")
	}

	// F020-AC-27: same (job_id, resource_type) → the first result, nothing new.
	again, err := cloneStep(sa, f, job, e.tenantA, e.tenantB, "question", a1, a2, b1, gone)
	if err != nil || again.Status != res.Status || len(again.Cloned) != 2 || again.Cloned[0].TargetId != res.Cloned[0].TargetId {
		t.Fatalf("replay = %+v %v", again, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM content.fixture_contents WHERE tenant_id = $1`, e.tenantB); n != 3 {
		t.Fatalf("replay cloned again: %d items", n)
	}
	if n := e.audits(ctx, t, "tenant_data.cloned", nil); n != 2 {
		t.Fatalf("replay audited again: %d", n)
	}

	// F020-AC-29: a learning type without cloner fails every element with a reason.
	none, err := cloneStep(sa, f, ids.New(), e.tenantA, e.tenantB, "lesson", a1)
	if err != nil || none.Status != "failed" || len(none.Cloned) != 0 || none.Skipped == nil || (*none.Skipped)[0].Reason == "" {
		t.Fatalf("type without cloner = %+v %v", none, err)
	}
	// Invalid requests and authorization.
	if _, err := cloneStep(sa, f, ids.New(), e.tenantA, e.tenantA, "question", a1); code(err) != problem.ValidationFailed {
		t.Fatalf("same source and target: %v", err)
	}
	if _, err := cloneStep(sa, f, ids.New(), e.tenantA, e.tenantOff, "question", a1); code(err) != problem.ValidationFailed {
		t.Fatalf("disabled target: %v", err)
	}
	if _, err := cloneStep(authztest.Ctx(ctx, authztest.Platform("content_admin")), f, ids.New(), e.tenantA, e.tenantB, "question", a1); code(err) != problem.PermissionDenied {
		t.Fatalf("content_admin cloning: %v", err)
	}
}
