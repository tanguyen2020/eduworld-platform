package command

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// F042-AC-03 (T009): at creation the matrix is checked again — total points, immediate feedback, a fixed question twice,
// a type not graded automatically and a rule the pool cannot fill (× variants) are each named by their pointer.
func TestCheckMatrix(t *testing.T) {
	lo, q := uuid.New(), uuid.New()
	pool := []selection.Candidate{{QuestionID: uuid.New(), VersionID: q, Type: "single_choice", ObjectiveIDs: []uuid.UUID{lo}}}
	b := loadedBlueprint{Def: blueprint.Definition{TotalPoints: 10, Variants: &blueprint.Variants{Strategy: "independent_draw", Count: 2},
		Sections: []blueprint.Section{{ID: "sec_1", Rules: []blueprint.Rule{
			{FixedQuestionVersionIDs: []uuid.UUID{q}, Count: 1, PointsEach: 1},
			{FixedQuestionVersionIDs: []uuid.UUID{q}, Count: 1, PointsEach: 1},
			{ObjectiveIDs: []uuid.UUID{lo}, Types: []string{"essay"}, Count: 1, PointsEach: 1},
		}}}}}
	err := Exams{}.checkMatrix(b, "immediate", pool)
	var pe *problem.Error
	if !errors.As(err, &pe) || pe.Code != problem.ValidationFailed {
		t.Fatalf("err %v", err)
	}
	var got []string
	for _, f := range pe.Fields {
		got = append(got, f.Field)
	}
	for _, want := range []string{"/definition/total_points", "/feedback_timing", "/definition/sections/0/rules/1/fixed_question_version_ids",
		"/definition/sections/0/rules/2/types", "/definition/sections/0/rules/2"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
}
