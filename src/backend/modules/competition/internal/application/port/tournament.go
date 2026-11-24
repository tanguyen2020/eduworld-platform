package port

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrScreeningUnavailable: the text screening of module chat did not answer (503, nothing stored — never fail-open).
var ErrScreeningUnavailable = errors.New("tournament: text screening unavailable")

// TextScreener checks the texts a school types (the name of a grade challenge or school tournament) against the
// effective sensitive-word catalogue of module chat (facade screenText, R15): it returns the positions of the flagged
// texts only, never the matched word.
type TextScreener interface {
	Screen(ctx context.Context, tenant uuid.UUID, locale string, texts []string) ([]int, error)
}

// TournamentMetrics are the metrics of the tournaments (plan F107 §Quan sát).
type TournamentMetrics interface {
	MatchScheduled()
	Forfeit(reason string)
	Override()
	SchedulerLag(seconds float64)
}
