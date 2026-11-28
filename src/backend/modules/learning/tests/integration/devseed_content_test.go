//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
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
	if want := len(f.Questions) + len(f.PoolQuestions) + 3; first.Published != want || len(as.seen) != want+1 {
		t.Fatalf("first run: published %d (want %d), assigned %d", first.Published, want, len(as.seen))
	}
	second, err := devseed.Run(ctx, cl, as, f)
	if err != nil {
		t.Fatal(err)
	}
	if second.Published != 0 || second.QuestionSet != first.QuestionSet || second.Lesson != first.Lesson || second.Activity != first.Activity ||
		strings.Join(seedIDs(second.Questions), ",") != strings.Join(seedIDs(first.Questions), ",") ||
		strings.Join(seedIDs(second.Pool), ",") != strings.Join(seedIDs(first.Pool), ",") {
		t.Fatalf("second run created or published again: %+v vs %+v", second, first)
	}
	checkGameMinigameSeeds(ctx, t, c, f, first)
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

// checkGameMinigameSeeds: the DEV mini-games of module game (seed/minigames, F073) are compatible with the seeded pool once
// their placeholder learning ids are resolved by code as cmd/seed-content of module game does (learning_refs.json): the
// same validateMinigameQuestionSource the Game Operations Console calls on create and publish.
func checkGameMinigameSeeds(ctx context.Context, t *testing.T, c *curEnv, f devseed.File, r devseed.Result) {
	t.Helper()
	dir := filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "game", "seed", "minigames")
	raw, err := os.ReadFile(filepath.Join(dir, "learning_refs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var refs struct {
		CurriculumCode string            `json:"curriculum_code"`
		Subjects       map[string]string `json:"subjects"`
		Objectives     map[string]string `json:"objectives"`
	}
	if err := json.Unmarshal(raw, &refs); err != nil || refs.CurriculumCode != f.CurriculumCode {
		t.Fatalf("learning_refs.json: %v %q", err, refs.CurriculumCode)
	}
	resolve := map[string]string{}
	for id, code := range refs.Subjects {
		if code != f.SubjectCode {
			t.Fatalf("subject %s of learning_refs.json is not seeded", code)
		}
		resolve[id] = r.SubjectID
	}
	for id, code := range refs.Objectives {
		i := slices.Index(f.ObjectiveCodes, code)
		if i < 0 {
			t.Fatalf("objective %s of learning_refs.json is not seeded", code)
		}
		resolve[id] = r.ObjectiveIDs[i]
	}
	fc := c.mgFacade()
	pctx := authztest.Ctx(ctx, authztest.Platform("game_designer"))
	for _, name := range []string{"math.g3.true_false_race.v1.json", "math.g3.multiplication_card_match.v1.json"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for from, to := range resolve {
			s = strings.ReplaceAll(s, from, to)
		}
		var cfg struct {
			MinigameType   string          `json:"minigame_type"`
			QuestionCount  int             `json:"question_count"`
			QuestionSource json.RawMessage `json:"question_source"`
		}
		if err := json.Unmarshal([]byte(s), &cfg); err != nil {
			t.Fatal(err)
		}
		in, _ := json.Marshal(map[string]any{"minigame_type": cfg.MinigameType, "question_count": cfg.QuestionCount, "question_source": cfg.QuestionSource})
		var body learningapi.MinigameQuestionSourceCheck
		if err := json.Unmarshal(in, &body); err != nil {
			t.Fatal(err)
		}
		res, err := asModule(pctx, "game", "validateMinigameQuestionSource", "", func(ctx context.Context) (learningapi.ValidateMinigameQuestionSourceResponseObject, error) {
			return fc.ValidateMinigameQuestionSource(ctx, learningapi.ValidateMinigameQuestionSourceRequestObject{Body: &body})
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if out := res.(learningapi.ValidateMinigameQuestionSource200JSONResponse); !out.Compatible || out.AvailableCount < cfg.QuestionCount {
			t.Fatalf("%s not compatible with the seeded pool: %+v", name, out)
		}
	}
}
