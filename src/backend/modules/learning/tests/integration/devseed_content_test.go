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

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// testAssigner records the ADR-0029 assignments of the seed for tenant A (TEST helper: the row the facade
// applyTenantAssignments writes; the HTTP path of cmd/seed-content is covered by devseed unit tests).
type testAssigner struct {
	c      *curEnv
	t      *testing.T
	tenant uuid.UUID
	seen   map[string]bool
}

func (a *testAssigner) Assign(ctx context.Context, resourceType string, id uuid.UUID) error {
	if key := resourceType + id.String(); !a.seen[key] {
		a.seen[key] = true
		a.c.assignResource(ctx, a.t, resourceType, id.String(), a.tenant)
	}
	return nil
}

// E2E Portal v2, finding 5 (LOCAL_DEVELOPMENT §5): the dev content seed publishes Toán khối 3 platform content through
// the public CMS API (author ≠ reviewer, audio exemption acknowledged) over the seeded GDPT2018_SAMPLE; a second run
// publishes nothing; once assigned, the School Admin of tenant A sees the curriculum and the content, tenant B does not.
func TestDevSeedContent(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	if _, err := job.Seed(ctx, c.cmd, seedDir(t)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "seed", "content", "math_g3.dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f devseed.File
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	cl := devseed.Client{BaseURL: c.srv.URL + "/api/v1", HTTP: c.srv.Client(), Author: c.tokens[cmNoidung], Reviewer: c.tokens[cmDuyet],
		ActivityReviewer: c.tokens[saAdmin]}
	as := &testAssigner{c: c, t: t, tenant: c.tenantA, seen: map[string]bool{}}
	first, err := devseed.Run(ctx, cl, as, f)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(f.Questions) + 3; first.Published != want || len(as.seen) != want+1 {
		t.Fatalf("first run: published %d (want %d), assigned %d", first.Published, want, len(as.seen))
	}
	second, err := devseed.Run(ctx, cl, as, f)
	if err != nil {
		t.Fatal(err)
	}
	if second.Published != 0 || second.QuestionSet != first.QuestionSet || second.Lesson != first.Lesson || second.Activity != first.Activity ||
		strings.Join(seedIDs(second.Questions), ",") != strings.Join(seedIDs(first.Questions), ",") {
		t.Fatalf("second run created or published again: %+v vs %+v", second, first)
	}
	// ADR-0029: the School Admin of tenant A reads the assigned curriculum and content; tenant B sees none of it.
	for path, want := range map[string]string{"/cms/learning/curricula": "GDPT2018_SAMPLE", "/cms/learning/question-sets": f.QuestionSet.Title,
		"/cms/learning/lessons": f.Lesson.Title, "/cms/learning/activities": f.Activity.Title} {
		if r := must(t, c.do(t, http.MethodGet, path+"?limit=100", adA, nil, nil), http.StatusOK, "tenant A "+path); !strings.Contains(r.raw, want) {
			t.Fatalf("tenant A %s misses %q: %s", path, want, r.raw)
		}
		if r := must(t, c.do(t, http.MethodGet, path+"?limit=100", qlcmB, nil, nil), http.StatusOK, "tenant B "+path); strings.Contains(r.raw, want) {
			t.Fatalf("tenant B %s sees %q", path, want)
		}
	}
}

func seedIDs(xs []uuid.UUID) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.String()
	}
	return out
}
