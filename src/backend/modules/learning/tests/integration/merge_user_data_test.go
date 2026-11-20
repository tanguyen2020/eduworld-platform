//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// mergeStep calls mergeUserData the way the merge saga of module identity does: on behalf of the principal of ctx, in
// the tenant scope of the merge, Idempotency-Key = merge_id.
func mergeStep(ctx context.Context, t *testing.T, c *curEnv, tenant uuid.UUID, merge, from, into uuid.UUID) learningapi.MergeUserData200JSONResponse {
	t.Helper()
	f := c.docFacade(t)
	f.Merge = command.MergeUserData{UoW: c.uow, Store: write.MergeRepo{}, Audit: write.Auditor{R: c.rec}}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		t.Fatal(err)
	}
	res, err := asIdentity(tenancy.WithScope(ctx, sc), "mergeUserData", merge.String(), func(ctx context.Context) (learningapi.MergeUserDataResponseObject, error) {
		return f.MergeUserData(ctx, learningapi.MergeUserDataRequestObject{UserId: from,
			Body: &learningapi.MergeUserDataJSONRequestBody{MergeId: merge, TargetUserId: into, Reason: "cùng một học sinh"}})
	})
	if err != nil {
		t.Fatalf("mergeUserData: %v", err)
	}
	return res.(learningapi.MergeUserData200JSONResponse)
}

// F030 T056 (OQ F016-2, F045-3, owner 2026-11-20; receiving side of the facade): the merge step keeps the learning data
// under the original student_id and links it to the target user once per merge_id; nothing_to_do without learning
// data, blocked when the original is already linked to another user; the tenant of the scope only.
func TestMergeUserDataLinksStudent(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil)
	sa, _ := systemAdmin(ctx)
	an, binh := c.users[hsAn].UserID, c.users[hsBinh].UserID
	attempt := must(t, c.start(t, hsAn, s.activityID, nil, nil), http.StatusCreated, "start").str("attempt_id")

	// No learning data for Binh: nothing to link.
	if r := mergeStep(sa, t, c, c.tenantA, ids.New(), binh, an); r.Status != "nothing_to_do" {
		t.Fatalf("no data: %+v", r)
	}
	// An has an attempt: linked to Binh, the attempt stays under An.
	merge := ids.New()
	if r := mergeStep(sa, t, c, c.tenantA, merge, an, binh); r.Status != "done" || r.UserId != an {
		t.Fatalf("merge: %+v", r)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.student_merge_links WHERE merge_id = $1 AND tenant_id = $2
		AND source_student_id = $3 AND target_student_id = $4 AND deleted_at IS NULL`, merge, c.tenantA, an, binh); n != 1 {
		t.Fatalf("links = %d", n)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempts WHERE attempt_id = $1 AND student_id = $2`, attempt, an); n != 1 {
		t.Fatal("the attempt left the original student_id")
	}
	if c.audits(ctx, t, command.ActionStudentMergeLinked, &c.tenantA) != 1 {
		t.Fatal("no audit student.merge_linked")
	}
	// Idempotent by merge_id (the saga retries the step).
	if r := mergeStep(sa, t, c, c.tenantA, merge, an, binh); r.Status != "done" {
		t.Fatalf("replay: %+v", r)
	}
	if n := count(ctx, t, c.platform, `SELECT count(*) FROM learning.student_merge_links WHERE source_student_id = $1`, an); n != 1 {
		t.Fatalf("links after replay = %d", n)
	}
	// Another merge of the same original into someone else is blocked.
	if r := mergeStep(sa, t, c, c.tenantA, ids.New(), an, uuid.New()); r.Status != "blocked" || r.BlockedReason == nil {
		t.Fatalf("second target: %+v", r)
	}
	// Tenant B sees no learning data of An (scope of the merge only).
	if r := mergeStep(sa, t, c, c.tenantB, ids.New(), an, binh); r.Status != "nothing_to_do" {
		t.Fatalf("tenant B: %+v", r)
	}
}
