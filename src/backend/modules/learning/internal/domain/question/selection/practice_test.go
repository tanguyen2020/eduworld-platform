package selection

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func cand(q, lo, topic uuid.UUID, typ string, d int) Candidate {
	return Candidate{QuestionID: q, VersionID: uuid.New(), Type: typ, Difficulty: d, TopicID: topic, ObjectiveIDs: []uuid.UUID{lo}}
}

// F032-AC-05 (T012): difficulty by the current level.
func TestDifficultyForLevel(t *testing.T) {
	for level, want := range map[string]Range{"beginning": {1, 2}, "developing": {2, 3}, "proficient": {3, 4}, "mastered": {4, 5},
		"not_started": {1, 2}} {
		if got := DifficultyForLevel(level); got != want {
			t.Fatalf("%s: %+v", level, got)
		}
	}
}

// F032-AC-22 (T033): a sibling of the same objective, type and difficulty; then ±1; then a recent one; last the
// original question; reproducible by seed.
func TestSiblings(t *testing.T) {
	lo, other, topic := uuid.New(), uuid.New(), uuid.New()
	orig := uuid.New()
	w := WrongItem{QuestionID: orig, ObjectiveIDs: []uuid.UUID{lo}, Type: "single_choice", Difficulty: 2}
	original := cand(orig, lo, topic, "single_choice", 2)
	same := cand(uuid.New(), lo, topic, "single_choice", 2)
	near := cand(uuid.New(), lo, topic, "single_choice", 3)
	otherType := cand(uuid.New(), lo, topic, "numeric", 2)
	otherLO := cand(uuid.New(), other, topic, "single_choice", 2)
	pool := []Candidate{original, same, near, otherType, otherLO}
	got, err := Siblings(pool, []WrongItem{w}, nil, 7)
	if err != nil || len(got) != 1 || got[0].Candidate.QuestionID != same.QuestionID || got[0].Original {
		t.Fatalf("same difficulty: %+v %v", got, err)
	}
	again, _ := Siblings(pool, []WrongItem{w}, nil, 7)
	if !slices.Equal(pickIDs(got), pickIDs(again)) {
		t.Fatal("not reproducible")
	}
	got, _ = Siblings(pool, []WrongItem{w}, []uuid.UUID{same.QuestionID}, 7)
	if got[0].Candidate.QuestionID != near.QuestionID {
		t.Fatalf("±1 before a recent one: %+v", got)
	}
	got, _ = Siblings([]Candidate{original, same}, []WrongItem{w}, []uuid.UUID{same.QuestionID}, 7)
	if got[0].Candidate.QuestionID != same.QuestionID {
		t.Fatalf("recent sibling before the original: %+v", got)
	}
	got, _ = Siblings([]Candidate{original, otherType, otherLO}, []WrongItem{w}, nil, 7)
	if !got[0].Original || got[0].Candidate.QuestionID != orig {
		t.Fatalf("original last: %+v", got)
	}
	if _, err := Siblings([]Candidate{otherLO}, []WrongItem{w}, nil, 7); !errors.Is(err, ErrPoolInsufficient) {
		t.Fatalf("nothing: %v", err)
	}
	// Two wrong items never get the same sibling, nor another wrong original.
	orig2 := uuid.New()
	w2 := WrongItem{QuestionID: orig2, ObjectiveIDs: []uuid.UUID{lo}, Type: "single_choice", Difficulty: 2}
	got, _ = Siblings([]Candidate{original, same, cand(orig2, lo, topic, "single_choice", 2)}, []WrongItem{w, w2}, nil, 3)
	if len(got) != 2 || got[0].Candidate.QuestionID == got[1].Candidate.QuestionID {
		t.Fatalf("distinct picks: %+v", got)
	}
}

func pickIDs(ps []Pick) []uuid.UUID {
	out := make([]uuid.UUID, len(ps))
	for i, p := range ps {
		out[i] = p.Candidate.VersionID
	}
	return out
}

// F032-AC-24 (T035): split by interleave_mix and no more than two questions in a row from one topic.
func TestInterleave(t *testing.T) {
	m := DefaultMix()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		count  int
		topics []uuid.UUID
		want   []int
	}{{10, []uuid.UUID{a, b}, []int{5, 5}}, {10, []uuid.UUID{a, b, c}, []int{5, 3, 2}}, {3, []uuid.UUID{a, b}, []int{2, 1}},
		{5, []uuid.UUID{a, b, c}, []int{3, 1, 1}}} {
		if got := m.Split(tc.count, tc.topics); !slices.Equal(got, tc.want) {
			t.Fatalf("split %d/%d: %v", tc.count, len(tc.topics), got)
		}
	}
	var cs []Candidate
	for i := 0; i < 5; i++ {
		cs = append(cs, cand(uuid.New(), uuid.New(), a, "numeric", 2))
	}
	for i := 0; i < 3; i++ {
		cs = append(cs, cand(uuid.New(), uuid.New(), b, "numeric", 2))
	}
	for i := 0; i < 2; i++ {
		cs = append(cs, cand(uuid.New(), uuid.New(), c, "numeric", 2))
	}
	for seed := uint64(1); seed < 20; seed++ {
		out := Interleave(cs, m.MaxConsecutive, seed)
		if len(out) != len(cs) {
			t.Fatalf("lost questions: %d", len(out))
		}
		run := 1
		for i := 1; i < len(out); i++ {
			if out[i].TopicID == out[i-1].TopicID {
				run++
			} else {
				run = 1
			}
			if run > m.MaxConsecutive {
				t.Fatalf("seed %d: %d in a row", seed, run)
			}
		}
	}
}

// F032-AC-25 (T040): two questions per objective first (prerequisites listed first), a third only when every objective
// has two, never above the maximum; nothing → ErrPoolInsufficient.
func TestDiagnosticSelection(t *testing.T) {
	topic := uuid.New()
	a, b := uuid.New(), uuid.New()
	var pool []Candidate
	for i := 0; i < 4; i++ {
		pool = append(pool, cand(uuid.New(), a, topic, "numeric", 2), cand(uuid.New(), b, topic, "numeric", 2))
	}
	count := func(cs []Candidate, lo uuid.UUID) int {
		n := 0
		for _, c := range cs {
			if c.ObjectiveIDs[0] == lo {
				n++
			}
		}
		return n
	}
	got, err := Diagnostic(pool, []uuid.UUID{a, b}, 3, 5, nil, 1)
	if err != nil || len(got) != 5 || count(got[:4], a) != 2 || count(got[:4], b) != 2 {
		t.Fatalf("5 of 2×3: %d %v", len(got), err)
	}
	if got, _ := Diagnostic(pool, []uuid.UUID{a, b}, 3, 20, nil, 1); len(got) != 6 {
		t.Fatalf("at most 3 per objective: %d", len(got))
	}
	if _, err := Diagnostic(pool, []uuid.UUID{uuid.New()}, 3, 20, nil, 1); !errors.Is(err, ErrPoolInsufficient) {
		t.Fatalf("nothing: %v", err)
	}
}

// F035-AC-24 (T051): practice difficulty of an experiment arm.
func TestRangeShift(t *testing.T) {
	if got := DifficultyForLevel("developing").Shift(1); got != (Range{Min: 3, Max: 4}) {
		t.Fatalf("+1: %+v", got)
	}
	if got := DifficultyForLevel("beginning").Shift(-1); got != (Range{Min: 1, Max: 1}) {
		t.Fatalf("-1 clamped: %+v", got)
	}
	if got := DifficultyForLevel("mastered").Shift(1); got != (Range{Min: 5, Max: 5}) {
		t.Fatalf("+1 clamped: %+v", got)
	}
}
