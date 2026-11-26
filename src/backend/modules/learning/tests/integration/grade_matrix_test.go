//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/facade"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Grade matrix 1–9 of module learning on PostgreSQL (TEST_STRATEGY §6.10).

// TestGradeMatrixChallengeQuestions (F076-AC-08/09, CHALLENGE_BOSS_SYSTEM §2.2 "không lấy nội dung khối khác"): one
// published question of each grade 1–9 in the same band; a challenge attempt of a student of grade g draws the question
// of grade g and nothing else (two questions asked → QUESTION_POOL_INSUFFICIENT: no neighbour grade fills the band);
// a request for the neighbour grade of the student (3↔4, 5↔6, 7↔8…) is 422.
func TestGradeMatrixChallengeQuestions(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	s := c.attemptSetup(ctx, t, nil) // the activity's own items: grade 3, difficulty 2 (outside the band 3–3)
	byGrade := map[int]string{}
	for g := 1; g <= 9; g++ {
		byGrade[g] = c.published(ctx, t, qlcmA, s.qc, map[string]any{"difficulty": 3, "grade_levels": []int{g}})
	}
	f := facade.Facade{AttemptOpener: c.att, AttemptResults: c.attq, Idempotency: idempotency.NewStore(c.app, 0, nil)}
	an := authz.NewPrincipal(c.users[hsAn])
	sctx := authztest.Ctx(ctx, an)
	objective := uuid.MustParse(s.qc.lo)
	create := func(grade, n int) (learningapi.CreateAttemptResponseObject, error) {
		lo, hi := 3, 3
		b := learningapi.CreateAttemptJSONRequestBody{StudentId: an.UserID, Mode: learningapi.AttemptModeChallenge,
			Activity: learningapi.ActivityRef{ContentType: learningapi.ActivityRefContentTypeActivity, ContentId: uuid.MustParse(s.activityID),
				DifficultyMin: &lo, DifficultyMax: &hi, GradeLevel: &grade, QuestionCount: &n, ObjectiveIds: &[]uuid.UUID{objective}}}
		b.Context.ContextType, b.Context.ContextId = "challenge_attempt", uuid.New()
		return asModule(sctx, "game", "createAttempt", newKey(), func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
			return f.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: &b})
		})
	}
	for g := 1; g <= 9; g++ {
		if _, err := c.platform.ExecContext(ctx, `UPDATE ref.students SET grade_level = $2 WHERE student_id = $1`, an.UserID, g); err != nil {
			t.Fatal(err)
		}
		res, err := create(g, 1)
		if err != nil {
			t.Fatalf("grade %d: %v", g, err)
		}
		created := res.(learningapi.CreateAttempt201JSONResponse)
		for h, q := range byGrade {
			n := count(ctx, t, c.platform, `SELECT count(*) FROM assessment.attempt_items WHERE attempt_id = $1 AND question_version_id = $2
				AND deleted_at IS NULL`, created.AttemptId, q)
			if (n == 1) != (h == g) {
				t.Fatalf("grade %d drew the question of grade %d: %d", g, h, n)
			}
		}
		if _, err := create(g, 2); code(err) != problem.QuestionPoolInsufficient {
			t.Fatalf("grade %d, two questions: %v (a neighbour grade filled the band?)", g, err)
		}
		for _, h := range []int{g - 1, g + 1} {
			if h < 1 || h > 9 {
				continue
			}
			if _, err := create(h, 1); code(err) != problem.ValidationFailed {
				t.Fatalf("student of grade %d asks grade %d: %v", g, h, err)
			}
		}
	}
}

// TestGradeMatrixAssignmentTargets (F041-AC-32, ADR-0095 mục 3): one student of each grade 1–9 in the school; an
// assignment for grade g reaches the students of grade g only (never 3→4, 5→6, 7→8), with their grade in the snapshot.
func TestGradeMatrixAssignmentTargets(t *testing.T) {
	ctx := context.Background()
	c := newCurEnv(ctx, t)
	k := c.asgSetup(ctx, t)
	placed := map[int]uuid.UUID{}
	for g := 1; g <= 9; g++ {
		placed[g] = c.placeStudent(ctx, t, fmt.Sprintf("hs.matrix.%d", g), k.school, g)
	}
	for g := 1; g <= 9; g++ {
		body := c.asgBody(k, map[string]any{"targets": []any{map[string]any{"target_type": "grade", "grade_level": g}}})
		r := must(t, c.createAsg(t, adA, body), http.StatusCreated, fmt.Sprintf("create grade %d", g))
		must(t, c.publishAsg(t, adA, r), http.StatusOK, fmt.Sprintf("publish grade %d", g))
		snap := c.targeted(ctx, t, r.str("assignment_id"))
		for id, v := range snap {
			if gr := v[1].(*int); gr == nil || *gr != g {
				t.Fatalf("assignment for grade %d targets %s of grade %v", g, id, gr)
			}
		}
		for h, id := range placed {
			if _, ok := snap[id]; ok != (h == g) {
				t.Fatalf("assignment for grade %d targets the student of grade %d: %v", g, h, ok)
			}
		}
	}
}
