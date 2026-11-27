package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
)

// answerCountsFake is a TEST DOUBLE (R17) of write.AttemptStore: only ScoredAnswerCounts is implemented.
type answerCountsFake struct {
	write.AttemptStore
	scored, rapid int
	err           error
	asked         uuid.UUID
}

func (f *answerCountsFake) ScoredAnswerCounts(_ context.Context, id uuid.UUID) (int, int, error) {
	f.asked = id
	return f.scored, f.rapid, f.err
}

// activity.submitted.rapid_guess_count is the number of scored answers flagged rapid_guess of the attempt (not a constant).
func TestRapidGuessCount(t *testing.T) {
	id := uuid.New()
	f := &answerCountsFake{scored: 3, rapid: 1}
	h := Attempts{Store: f}
	n, err := h.rapidGuessCount(context.Background(), id)
	if err != nil || n != 1 || f.asked != id {
		t.Fatalf("rapidGuessCount = %d, %v (asked %s); want 1 for attempt %s", n, err, f.asked, id)
	}
	f.rapid = 0
	if n, err := h.rapidGuessCount(context.Background(), id); err != nil || n != 0 {
		t.Fatalf("no rapid answer: %d, %v", n, err)
	}
	down := errors.New("database down")
	f.err = down
	if _, err := h.rapidGuessCount(context.Background(), id); !errors.Is(err, down) {
		t.Fatalf("store error not propagated: %v", err)
	}
}
