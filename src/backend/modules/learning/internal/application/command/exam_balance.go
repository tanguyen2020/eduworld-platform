package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/blueprint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam/paper"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
)

// pValueBalancer is a balancer that takes the measured p_values of the pool (examquality.Balancer, F044).
type pValueBalancer interface {
	WithPValues(map[uuid.UUID]float64) paper.VariantBalancer
}

// balancer returns the balancer of an independent_draw session with the p_values of its pool (F044-AC-22): the measured
// p_value of every version that has enough first answers, p̂ of its difficulty otherwise.
func (h Exams) balancer(ctx context.Context, strategy string, pool []selection.Candidate) (paper.VariantBalancer, error) {
	pb, ok := h.Balancer.(pValueBalancer)
	if !ok || h.PValues == nil || strategy != blueprint.StrategyIndependentDraw {
		return h.Balancer, nil
	}
	ids := make([]uuid.UUID, len(pool))
	for i, c := range pool {
		ids[i] = c.VersionID
	}
	pv, err := h.PValues.PValues(ctx, ids)
	if err != nil {
		return nil, err
	}
	return pb.WithPValues(pv), nil
}

// balanceView is the balancer result returned to the creator of the session: expected_p of every variant and the warning
// with the figures when the variants could not be balanced.
func balanceView(def blueprint.Definition, ps []paper.Paper) dto.ExamBalanceView {
	v := dto.ExamBalanceView{MaxMeanPDiff: blueprint.DefaultMaxMeanPDiff, Variants: make([]dto.ExamBalanceVariantView, 0, len(ps))}
	if def.Variants != nil && def.Variants.Balance != nil && def.Variants.Balance.MaxMeanPDiff != nil {
		v.MaxMeanPDiff = *def.Variants.Balance.MaxMeanPDiff
	}
	for _, p := range ps {
		v.Warning = v.Warning || p.BalanceWarning
		v.Variants = append(v.Variants, dto.ExamBalanceVariantView{VariantNo: p.VariantNo, ExpectedP: p.ExpectedP})
	}
	return v
}
