package selection

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func pool(n int, mk func(i int) Candidate) []Candidate {
	out := make([]Candidate, n)
	for i := range out {
		c := mk(i)
		c.QuestionID, c.VersionID = uuid.New(), uuid.New()
		out[i] = c
	}
	return out
}

func versions(cs []Candidate) []uuid.UUID {
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.VersionID
	}
	return out
}

// F022-AC-26, SC-003: filters, distribution, exclusion, exam_only, same seed → same result.
func TestSelect(t *testing.T) {
	subject, topic := uuid.New(), uuid.New()
	p := pool(30, func(i int) Candidate {
		return Candidate{Type: []string{"single_choice", "numeric"}[i%2], Difficulty: 1 + i%5, SubjectID: subject, TopicID: topic, GradeLevels: []int{3},
			ExamOnly: i == 0}
	})
	rule := Rule{Filters: Filters{SubjectID: &subject, GradeLevels: []int{3}, Difficulty: &Range{Min: 2, Max: 3}}, Count: 5,
		Distribution: []Bucket{{Difficulty: 2, Count: 3}, {Difficulty: 3, Count: 2}}}
	got, err := Select(p, Request{Rule: rule, Seed: 7})
	if err != nil || len(got) != 5 {
		t.Fatalf("%v %d", err, len(got))
	}
	d2, d3 := 0, 0
	for _, c := range got {
		switch c.Difficulty {
		case 2:
			d2++
		case 3:
			d3++
		default:
			t.Fatalf("difficulty %d out of the rule", c.Difficulty)
		}
	}
	if d2 != 3 || d3 != 2 {
		t.Fatalf("distribution %d/%d", d2, d3)
	}
	for i := 0; i < 100; i++ {
		again, _ := Select(p, Request{Rule: rule, Seed: 7})
		if !slices.Equal(versions(again), versions(got)) {
			t.Fatal("same seed, different selection")
		}
	}
	other, _ := Select(p, Request{Rule: rule, Seed: 8})
	if slices.Equal(versions(other), versions(got)) {
		t.Log("seed 8 picked the same set (possible but unlikely)")
	}
	// Exclusion list and types filter; exam_only never chosen unless allowed.
	all := Rule{Count: 29}
	got, err = Select(p, Request{Rule: all, Seed: 1})
	if err != nil || slices.ContainsFunc(got, func(c Candidate) bool { return c.ExamOnly }) {
		t.Fatalf("exam_only chosen: %v", err)
	}
	if _, err := Select(p, Request{Rule: Rule{Count: 30}, Seed: 1}); !errors.Is(err, ErrPoolInsufficient) {
		t.Fatal("30 needs the exam_only question")
	}
	if got, err := Select(p, Request{Rule: Rule{Count: 30, AllowExamOnly: true}, Seed: 1}); err != nil || len(got) != 30 {
		t.Fatalf("exam: %v", err)
	}
	ex := []uuid.UUID{p[1].QuestionID, p[3].QuestionID}
	got, _ = Select(p, Request{Rule: Rule{Count: 10, Filters: Filters{ExcludeQuestionIDs: ex, Types: []string{"numeric"}}}, Seed: 3})
	for _, c := range got {
		if slices.Contains(ex, c.QuestionID) || c.Type != "numeric" {
			t.Fatalf("filter broken: %+v", c)
		}
	}
}

// F022-AC-27: relaxation avoid_recent → difficulty ±1 → types, then QUESTION_POOL_INSUFFICIENT.
func TestRelaxation(t *testing.T) {
	p := pool(6, func(i int) Candidate { return Candidate{Type: "single_choice", Difficulty: 3} })
	p = append(p, pool(2, func(int) Candidate { return Candidate{Type: "numeric", Difficulty: 4} })...)
	recent := []uuid.UUID{p[0].QuestionID, p[1].QuestionID, p[2].QuestionID}
	// Enough without recent questions: recent avoided.
	got, err := Select(p, Request{Rule: Rule{Count: 3, Filters: Filters{Types: []string{"single_choice"}}}, Recent: recent, Seed: 1})
	if err != nil || slices.ContainsFunc(got, func(c Candidate) bool { return slices.Contains(recent, c.QuestionID) }) {
		t.Fatalf("recent not avoided: %v", err)
	}
	// Not enough: recent questions come back.
	if got, err := Select(p, Request{Rule: Rule{Count: 6, Filters: Filters{Types: []string{"single_choice"}}}, Recent: recent, Seed: 1}); err != nil || len(got) != 6 {
		t.Fatalf("avoid_recent relaxation: %v", err)
	}
	// Difficulty ±1: 7 questions with difficulty 3 asked, 6 + one of difficulty 4.
	if got, err := Select(p, Request{Rule: Rule{Count: 7, Filters: Filters{Difficulty: &Range{Min: 3, Max: 3}}}, Seed: 1}); err != nil || len(got) != 7 {
		t.Fatalf("difficulty relaxation: %v", err)
	}
	if got, err := Select(p, Request{Rule: Rule{Count: 7, Distribution: []Bucket{{Difficulty: 3, Count: 7}}}, Seed: 1}); err != nil || len(got) != 7 {
		t.Fatalf("distribution relaxation: %v", err)
	}
	// Types: 8 single_choice asked → numeric ones join.
	if got, err := Select(p, Request{Rule: Rule{Count: 8, Filters: Filters{Types: []string{"single_choice"}}}, Seed: 1}); err != nil || len(got) != 8 {
		t.Fatalf("types relaxation: %v", err)
	}
	if _, err := Select(p, Request{Rule: Rule{Count: 9}, Seed: 1}); !errors.Is(err, ErrPoolInsufficient) {
		t.Fatalf("insufficient: %v", err)
	}
	if got, _ := Select(nil, Request{Rule: Rule{Count: 0}}); got != nil {
		t.Fatal("count 0")
	}
}

// F076-AC-09, SC-002: a strict request (challenge of a difficulty level) never widens the band nor the types; only the
// recently seen questions come back.
func TestStrictSelection(t *testing.T) {
	p := pool(6, func(i int) Candidate { return Candidate{Type: "single_choice", Difficulty: 3} })
	p = append(p, pool(2, func(int) Candidate { return Candidate{Type: "numeric", Difficulty: 4} })...)
	recent := []uuid.UUID{p[0].QuestionID}
	band := Filters{Types: []string{"single_choice"}, Difficulty: &Range{Min: 3, Max: 3}}
	got, err := Select(p, Request{Rule: Rule{Count: 6, Filters: band}, Recent: recent, Seed: 1, Strict: true})
	if err != nil || len(got) != 6 {
		t.Fatalf("recent relaxed: %v", err)
	}
	if _, err := Select(p, Request{Rule: Rule{Count: 7, Filters: band}, Seed: 1, Strict: true}); !errors.Is(err, ErrPoolInsufficient) {
		t.Fatalf("band widened: %v", err)
	}
	if _, err := Select(p, Request{Rule: Rule{Count: 7, Filters: Filters{Types: []string{"single_choice"}}}, Seed: 1, Strict: true}); !errors.Is(err,
		ErrPoolInsufficient) {
		t.Fatalf("types relaxed: %v", err)
	}
}
