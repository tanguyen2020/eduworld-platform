package evaluation

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func items() []Item {
	return []Item{
		{Position: 1, MaxScore: 1, Score: 1, IsCorrect: true, Answered: true},
		{Position: 2, MaxScore: 1, Score: 0, IsCorrect: false, Answered: true},
		{Position: 3, MaxScore: 2, Score: 0, IsCorrect: false, Answered: false},
	}
}

func total(it []Item) (score, max float64) {
	for _, i := range it {
		if i.Dropped {
			continue
		}
		max += i.MaxScore
		score += i.Score
	}
	return
}

// F031-AC-20 (T030): accept_all gives full credit to every answered affected item; use_new_key regrades the affected items
// with the corrected key; drop_item takes them out of score and max_score; unaffected items never change.
func TestRegradeActions(t *testing.T) {
	got, err := Regrade(items(), map[int]bool{2: true, 3: true}, AcceptAll, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s, m := total(got); s != 2 || m != 4 || !got[1].IsCorrect || got[2].Score != 0 {
		t.Fatalf("accept_all: %v %v %+v", s, m, got)
	}
	got, err = Regrade(items(), map[int]bool{1: true, 2: true}, UseNewKey, map[int]Graded{1: {Score: 0}, 2: {Score: 1, IsCorrect: true}})
	if err != nil {
		t.Fatal(err)
	}
	if s, m := total(got); s != 1 || m != 4 || got[0].IsCorrect || !got[1].IsCorrect {
		t.Fatalf("use_new_key: %v %v %+v", s, m, got)
	}
	if _, err := Regrade(items(), map[int]bool{2: true}, UseNewKey, map[int]Graded{}); !errors.Is(err, ErrMissingRegrade) {
		t.Fatalf("missing regrade: %v", err)
	}
	got, err = Regrade(items(), map[int]bool{2: true}, DropItem, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s, m := total(got); s != 1 || m != 3 || !got[1].Dropped {
		t.Fatalf("drop_item: %v %v %+v", s, m, got)
	}
	// Score never exceeds the item max (a corrected key giving more is clamped).
	got, _ = Regrade(items(), map[int]bool{1: true}, UseNewKey, map[int]Graded{1: {Score: 5, IsCorrect: true}})
	if got[0].Score != 1 {
		t.Fatalf("clamp: %+v", got[0])
	}
	if _, err := Regrade(items(), map[int]bool{1: true}, Action("manual"), nil); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("unknown action: %v", err)
	}
	if !items()[0].IsCorrect || items()[1].IsCorrect {
		t.Fatal("input changed")
	}
}

// F031-AC-22: a superseded evaluation is not editable; supersede is the only transition.
func TestSupersede(t *testing.T) {
	e := Evaluation{ID: uuid.New(), Status: StatusFinal, Effective: true}
	s, err := e.Supersede()
	if err != nil || s.Status != StatusSuperseded || s.Effective {
		t.Fatalf("supersede: %+v %v", s, err)
	}
	if _, err := s.Supersede(); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("second supersede: %v", err)
	}
	if err := (Evaluation{Status: StatusSuperseded}).CheckEditable(); !errors.Is(err, ErrNotEditable) {
		t.Fatal("superseded editable")
	}
	for _, a := range []Action{AcceptAll, UseNewKey, DropItem} {
		if !a.Valid() {
			t.Fatalf("%s invalid", a)
		}
	}
}
