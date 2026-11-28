//go:build integration

package integration

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// LOCAL_DEVELOPMENT §5 (make seed-attempts): the golden lesson file publishes its questions, its two quizzes and — after
// the quizzes — version 2 of the regrade question through the public CMS workflow (R11: the faulty version 1 is never
// edited, the regrade quiz keeps pinning it); a second run publishes nothing and returns the same ids.
func TestDevSeedGoldenLessons(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	if _, err := job.Seed(ctx, c.cmd, seedDir(t)); err != nil {
		t.Fatal(err)
	}
	var f devseed.LessonFile
	if err := devseed.LoadJSON(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "learning", "seed", "content", "golden_g3.dev.json"), &f); err != nil {
		t.Fatal(err)
	}
	cl := devseed.Client{BaseURL: c.srv.URL + "/api/v1", HTTP: c.srv.Client(), Author: c.tokens[cmNoidung], Reviewer: c.tokens[cmDuyet],
		ActivityReviewer: c.tokens[saAdmin]}
	as := &testAssigner{c: c, t: t, tenant: c.tenantA, seen: map[string]bool{}}
	first, err := devseed.RunLessons(ctx, cl, nil, as, f)
	if err != nil {
		t.Fatal(err)
	}
	// 5 questions v1 + 2 quizzes + version 2 of the regrade question; every question and activity assigned.
	if first.Published != 8 || len(first.Activities) != 2 || len(as.seen) != 7 {
		t.Fatalf("first run: published %d, activities %d, assigned %d", first.Published, len(first.Activities), len(as.seen))
	}
	rg := first.Versions["devseed-rg-q1"]
	if len(rg) != 2 || rg[0] == "" || rg[1] == "" || rg[0] == rg[1] || first.QuestionKeyOf(rg[0]) != "devseed-rg-q1" {
		t.Fatalf("regrade question versions: %v", rg)
	}
	// The regrade quiz pins version 1 (the faulty key), not the corrected version 2.
	v := must(t, c.do(t, http.MethodGet, "/cms/learning/content/activity/"+first.Activities["regrade"].String()+"/versions/1", cmNoidung, nil, nil),
		http.StatusOK, "regrade quiz version")
	if !strings.Contains(v.raw, rg[0]) || strings.Contains(v.raw, rg[1]) {
		t.Fatalf("regrade quiz must pin version 1 %s, not version 2 %s: %s", rg[0], rg[1], v.raw)
	}
	second, err := devseed.RunLessons(ctx, cl, nil, as, f)
	if err != nil {
		t.Fatal(err)
	}
	if second.Published != 0 || second.Activities["golden"] != first.Activities["golden"] || second.Activities["regrade"] != first.Activities["regrade"] ||
		second.Versions["devseed-rg-q1"][1] != rg[1] {
		t.Fatalf("second run created or published again: %+v vs %+v", second, first)
	}
}
