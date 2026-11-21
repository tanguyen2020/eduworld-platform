package scope

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Portal E2E 2026-11-21: School Admin (school-limited grant) got 403 on every Content Studio list although the role
// holds learning_content.* (ADMIN_PORTAL §2.3). Tenant content has no school: the school grant covers it.
func TestRequireContent(t *testing.T) {
	ctx := context.Background()
	ta, tb, school := uuid.New(), uuid.New(), uuid.New()
	schoolAdmin := authztest.Staff(ta, "school_admin", authztest.AtSchool(school))
	for _, perm := range []authz.Permission{authz.PermLearningContentRead, authz.PermLearningContentWrite,
		authz.PermLearningContentReview, authz.PermLearningContentPublish, authz.PermCurriculumRead} {
		if err := RequireContent(ctx, schoolAdmin, perm, nil); err != nil {
			t.Fatalf("school admin %s on the tenant content: %v", perm, err)
		}
		if err := RequireContent(ctx, schoolAdmin, perm, &ta); err != nil {
			t.Fatalf("school admin %s on content of its tenant: %v", perm, err)
		}
	}
	// another tenant: 404 (no existence leak)
	if err := RequireContent(ctx, schoolAdmin, authz.PermLearningContentRead, &tb); codeOf(err) != problem.NotFound {
		t.Fatalf("content of another tenant: %v (want 404)", err)
	}
	// permissions the role does not hold, and non-content permissions, stay denied
	for _, perm := range []authz.Permission{authz.PermLearningContentReEvaluate, authz.PermCurriculumWrite, authz.PermLearningContentImport} {
		if err := RequireContent(ctx, schoolAdmin, perm, nil); codeOf(err) != problem.PermissionDenied {
			t.Fatalf("school admin %s: %v (want 403)", perm, err)
		}
	}
	// a school-limited academic manager does not gain curriculum writes or re-evaluation tenant-wide
	am := authztest.Staff(ta, "academic_manager", authztest.AtSchool(school))
	for _, perm := range []authz.Permission{authz.PermCurriculumWrite, authz.PermLearningContentReEvaluate} {
		if err := RequireContent(ctx, am, perm, nil); codeOf(err) != problem.PermissionDenied {
			t.Fatalf("school-scoped academic manager %s: %v (want 403)", perm, err)
		}
	}
	// subject-limited grants are unchanged (authz.Require semantics)
	subj := authztest.Staff(ta, "academic_manager", authztest.ForSubject(uuid.New()))
	if err := RequireContent(ctx, subj, authz.PermLearningContentRead, nil); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("subject-limited grant: %v (want 403 as before)", err)
	}
	// a tenant-wide grant and platform staff keep working
	if err := RequireContent(ctx, authztest.Staff(ta, "tenant_admin"), authz.PermLearningContentWrite, &ta); err != nil {
		t.Fatal(err)
	}
	if err := RequireContent(ctx, authztest.Platform("content_admin"), authz.PermLearningContentWrite, nil); err != nil {
		t.Fatal(err)
	}
	// a tenant role without content permissions is denied
	if err := RequireContent(ctx, authztest.Staff(ta, "tenant_content_reviewer", authztest.AtSchool(school)), authz.PermLearningContentWrite, nil); codeOf(err) != problem.PermissionDenied {
		t.Fatalf("reviewer writing: %v (want 403)", err)
	}
}
