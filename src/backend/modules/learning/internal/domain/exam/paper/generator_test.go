package paper

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

var (
	loA = uuid.MustParse("0193a1f0-0000-7000-8000-00000000a001")
	loB = uuid.MustParse("0193a1f0-0000-7000-8000-00000000a002")
)

func pool(n int, lo uuid.UUID, typ string, diff int) []selection.Candidate {
	out := make([]selection.Candidate, n)
	for i := range out {
		out[i] = selection.Candidate{QuestionID: uuid.New(), VersionID: uuid.New(), Type: typ, Difficulty: diff, ObjectiveIDs: []uuid.UUID{lo}}
	}
	return out
}

func def() blueprint.Definition {
	return blueprint.Definition{TotalPoints: 10, Sections: []blueprint.Section{
		{ID: "sec_1", Rules: []blueprint.Rule{
			{ObjectiveIDs: []uuid.UUID{loA}, Difficulty: &blueprint.Range{Min: 1, Max: 2}, Types: []string{"single_choice"}, Count: 4, PointsEach: 0.5},
			{ObjectiveIDs: []uuid.UUID{loB}, Types: []string{"numeric"}, Count: 2, PointsEach: 4},
		}},
	}}
}

func candidates() []selection.Candidate {
	p := append(pool(12, loA, "single_choice", 1), pool(6, loA, "single_choice", 4)...) // difficulty 4 never matches rule 1
	p = append(p, pool(6, loA, "true_false", 1)...)                                     // type never matches rule 1
	return append(p, pool(8, loB, "numeric", 3)...)
}

// F042-AC-02 (T008, SC-001): the matrix objective × difficulty × type of every rule, the same paper for the same seed,
// independent_draw gives each variant its own seed.
func TestGenerateMatrixAndSeed(t *testing.T) {
	d, cs := def(), candidates()
	byVersion := map[uuid.UUID]selection.Candidate{}
	for _, c := range cs {
		byVersion[c.VersionID] = c
	}
	papers, err := SessionPapers(d, cs, blueprint.StrategyIndependentDraw, 4, 42, true, nil)
	if err != nil || len(papers) != 4 {
		t.Fatalf("papers: %d %v", len(papers), err)
	}
	for _, p := range papers {
		if len(p.Items) != 6 || p.TotalPoints != 10 {
			t.Fatalf("variant %d: %d items, %.1f points", p.VariantNo, len(p.Items), p.TotalPoints)
		}
		seen := map[uuid.UUID]bool{}
		for i, it := range p.Items {
			c := byVersion[it.QuestionVersionID]
			if seen[c.QuestionID] {
				t.Fatal("a question twice in a paper")
			}
			seen[c.QuestionID] = true
			if it.Position != i+1 {
				t.Fatalf("position %d at %d", it.Position, i)
			}
			if i < 4 && (c.Type != "single_choice" || c.Difficulty > 2 || c.ObjectiveIDs[0] != loA || it.Points != 0.5) {
				t.Fatalf("rule 1 item %+v", c)
			}
			if i >= 4 && (c.Type != "numeric" || c.ObjectiveIDs[0] != loB || it.Points != 4) {
				t.Fatalf("rule 2 item %+v", c)
			}
		}
	}
	again, _ := SessionPapers(d, cs, blueprint.StrategyIndependentDraw, 4, 42, true, nil)
	for i := range papers {
		if !slices.Equal(ids(papers[i]), ids(again[i])) || papers[i].Seed != again[i].Seed {
			t.Fatal("the same seed must give the same papers")
		}
	}
	if slices.Equal(ids(papers[0]), ids(papers[1])) && slices.Equal(ids(papers[1]), ids(papers[2])) {
		t.Fatal("independent variants all equal")
	}
	fixed, _ := SessionPapers(d, cs, blueprint.StrategyFixed, 4, 42, true, nil)
	if len(fixed) != 1 || fixed[0].VariantNo != 1 {
		t.Fatalf("fixed: %d papers", len(fixed))
	}
	if per, _ := SessionPapers(d, cs, blueprint.StrategyPerStudent, 4, 42, false, nil); per != nil {
		t.Fatal("per_student draws at start")
	}
}

// F042-AC-03 (T009): a rule the pool cannot fill names the rule (QUESTION_POOL_INSUFFICIENT).
func TestGeneratePoolInsufficient(t *testing.T) {
	d := def()
	d.Sections[0].Rules[1].Count = 9
	_, err := Generate(d, candidates(), 1, false)
	var pe *PoolError
	if !errors.As(err, &pe) || !errors.Is(err, ErrPoolInsufficient) || pe.Pointer() != "/definition/sections/0/rules/1" || pe.Available != 8 {
		t.Fatalf("err %v", err)
	}
}

// Fixed questions are taken as they are; exam_only questions come first for an official exam.
func TestGenerateFixedAndExamOnly(t *testing.T) {
	cs := pool(6, loA, "single_choice", 1)
	cs[5].ExamOnly = true
	cs[4].ExamOnly = true
	d := blueprint.Definition{Sections: []blueprint.Section{{ID: "sec_1", Rules: []blueprint.Rule{
		{FixedQuestionVersionIDs: []uuid.UUID{cs[0].VersionID}, Count: 1, PointsEach: 1},
		{ObjectiveIDs: []uuid.UUID{loA}, Count: 2, PointsEach: 1},
	}}}}
	p, err := Generate(d, cs, 7, true)
	if err != nil || p.Items[0].QuestionVersionID != cs[0].VersionID {
		t.Fatalf("fixed: %v", err)
	}
	got := []uuid.UUID{p.Items[1].QuestionVersionID, p.Items[2].QuestionVersionID}
	if !slices.Contains(got, cs[4].VersionID) || !slices.Contains(got, cs[5].VersionID) {
		t.Fatal("exam_only first for an official exam")
	}
}

// EXAM_ENGINE §4: per attempt, the items of a shuffled section change order by seed; shuffle_items false keeps it.
func TestOrder(t *testing.T) {
	no := false
	d := blueprint.Definition{Sections: []blueprint.Section{{ID: "sec_1"}, {ID: "sec_2", ShuffleItems: &no}}}
	var items []Item
	for i := 1; i <= 8; i++ {
		s := "sec_1"
		if i > 5 {
			s = "sec_2"
		}
		items = append(items, Item{SectionID: s, Position: i, QuestionVersionID: uuid.New()})
	}
	a, b := Order(d, items, 1), Order(d, items, 2)
	if slices.Equal(ids(Paper{Items: a[:5]}), ids(Paper{Items: b[:5]})) {
		t.Fatal("seeds 1 and 2 give the same order")
	}
	for i := 5; i < 8; i++ {
		if a[i].QuestionVersionID != items[i].QuestionVersionID || a[i].Position != i+1 {
			t.Fatal("sec_2 keeps its order")
		}
	}
}

func ids(p Paper) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Items))
	for i, it := range p.Items {
		out[i] = it.QuestionVersionID
	}
	return out
}
