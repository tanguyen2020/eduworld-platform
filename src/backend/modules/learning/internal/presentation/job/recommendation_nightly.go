package job

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
)

// RecommendationRunner recomputes the recommendations of the students (command.Recommendations, F032 T051).
type RecommendationRunner interface {
	RunNightly(ctx context.Context, limit int) (command.RecommendationReport, error)
}

// RecommendationNightly is the daily job recommendation-nightly of the worker profile jobs (F032-AC-30): the decay of R4
// and the warm-up of R7 change without any learning event, so every student with recent learning data, an active
// recommendation or an active review entry is recomputed once a day in the scope of its tenant.
type RecommendationNightly struct {
	Runner RecommendationRunner
	Batch  int
}

// RunOnce recomputes up to Batch students and returns how many were recomputed.
func (w RecommendationNightly) RunOnce(ctx context.Context) (int, error) {
	rep, err := w.Runner.RunNightly(ctx, w.Batch)
	return rep.Students, err
}
