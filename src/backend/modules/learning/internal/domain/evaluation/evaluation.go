// Package evaluation holds the evaluation of an attempt and its re-evaluation (ACTIVITY_ENGINE §7.3, ADR-0072): an
// attempt keeps its original answers; a re-evaluation adds a new evaluation and supersedes the effective one. There is
// no manual grading: the new result comes from an action applied by the system to the affected items.
package evaluation

import (
	"errors"
	"math"

	"github.com/google/uuid"
)

// Status of an evaluation.
const (
	StatusFinal      = "final"
	StatusSuperseded = "superseded"
)

// Sources of an evaluation.
const (
	SourceSystem       = "system"
	SourceReEvaluation = "re_evaluation"
)

// Action is the re-evaluation action of a RegradeRequest.
type Action string

// Actions of ACTIVITY_ENGINE §7.3 / EXAM_ENGINE §9.
const (
	AcceptAll Action = "accept_all"
	UseNewKey Action = "use_new_key"
	DropItem  Action = "drop_item"
)

// Valid reports whether a is a known action.
func (a Action) Valid() bool { return a == AcceptAll || a == UseNewKey || a == DropItem }

var (
	// ErrNotEditable is EVALUATION_NOT_EDITABLE: a superseded evaluation never changes (F031-AC-22).
	ErrNotEditable = errors.New("evaluation: superseded evaluation is not editable")
	// ErrUnknownAction is returned for an action outside the three of the contract.
	ErrUnknownAction = errors.New("evaluation: unknown re-evaluation action")
	// ErrMissingRegrade is returned when use_new_key has no new grading for an affected answered item.
	ErrMissingRegrade = errors.New("evaluation: no regrade for an affected item")
)

// Evaluation is the state of an evaluation row that the re-evaluation may change.
type Evaluation struct {
	ID        uuid.UUID
	Status    string
	Effective bool
}

// CheckEditable refuses any write on a superseded evaluation.
func (e Evaluation) CheckEditable() error {
	if e.Status == StatusSuperseded {
		return ErrNotEditable
	}
	return nil
}

// Supersede is the only transition of an evaluation: final and effective → superseded.
func (e Evaluation) Supersede() (Evaluation, error) {
	if err := e.CheckEditable(); err != nil {
		return e, err
	}
	e.Status, e.Effective = StatusSuperseded, false
	return e, nil
}

// Item is the result of one attempt item in an evaluation (details of assessment.evaluations).
type Item struct {
	Position  int     `json:"position"`
	MaxScore  float64 `json:"max_score"`
	Score     float64 `json:"score"`
	IsCorrect bool    `json:"is_correct"`
	Answered  bool    `json:"answered"`
	Dropped   bool    `json:"dropped,omitempty"`
}

// Graded is the result of an answer graded again with a corrected answer key.
type Graded struct {
	Score     float64
	IsCorrect bool
}

// Regrade applies action to the affected positions and returns the new items (the input is not changed):
//   - accept_all: every answered affected item gets its full score;
//   - use_new_key: every answered affected item gets the score of its answer under the corrected key (regraded);
//   - drop_item: the affected items leave score and max_score (and the mastery evidence).
//
// Unanswered items stay unanswered except under drop_item, which removes them as well.
func Regrade(items []Item, affected map[int]bool, action Action, regraded map[int]Graded) ([]Item, error) {
	if !action.Valid() {
		return nil, ErrUnknownAction
	}
	out := make([]Item, len(items))
	copy(out, items)
	for i, it := range out {
		if !affected[it.Position] {
			continue
		}
		switch action {
		case AcceptAll:
			if it.Answered {
				out[i].Score, out[i].IsCorrect = it.MaxScore, true
			}
		case UseNewKey:
			if !it.Answered {
				continue
			}
			g, ok := regraded[it.Position]
			if !ok {
				return nil, ErrMissingRegrade
			}
			out[i].Score, out[i].IsCorrect = math.Min(math.Max(g.Score, 0), it.MaxScore), g.IsCorrect
		case DropItem:
			out[i].Dropped, out[i].Score, out[i].IsCorrect = true, 0, false
		}
	}
	return out, nil
}
