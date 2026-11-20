package command

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/recommendation"
)

// ExperienceArm is what the live experiment arms of a student change in the student experience (F035-AC-24): the
// mastery model of a live mastery_model arm with the states computed with it, and the rule_params of a live
// recommendation_rule arm. The zero value is the control experience (active model, default engine). School reports,
// XP, learning_objective.mastered and exams never read it.
type ExperienceArm struct {
	// Model is the model of a live mastery_model arm that is not the control (nil = the active model).
	Model *mastery.WeightedEvidence
	// States are the mastery states of the student under Model (an objective without one keeps the active state).
	States map[uuid.UUID]mastery.State
	// Rules are the parameters of the live recommendation_rule arms (first started wins per parameter).
	Rules recommendation.Params
}

// Mastery returns the model and the state of an objective for the student experience: the arm model and its state
// when the student is in a live mastery_model arm, otherwise m and s unchanged.
func (x ExperienceArm) Mastery(m mastery.WeightedEvidence, objective uuid.UUID, s mastery.State) (mastery.WeightedEvidence, mastery.State) {
	if x.Model == nil {
		return m, s
	}
	if st, ok := x.States[objective]; ok {
		s = st
	}
	return *x.Model, s
}

// ExperienceResolver is Experiments.Experience.
type ExperienceResolver interface {
	Experience(ctx context.Context, tenant, student uuid.UUID) (ExperienceArm, error)
}

// Experience resolves the live arms of a student in the write context (F035-AC-22, AC-24): every live experiment
// covering the tenant, in start order; the arm is stored at the first contact like ArmsOf; a control arm, a removed
// assignment, a shadow experiment or a stopped one leaves the control experience. Runs in the transaction of ctx.
func (h Experiments) Experience(ctx context.Context, tenant, student uuid.UUID) (ExperienceArm, error) {
	var out ExperienceArm
	es, err := h.Store.RunningExperiments(ctx, &tenant)
	if err != nil || len(es) == 0 {
		return out, err
	}
	slices.SortStableFunc(es, func(a, b write.ExperimentRow) int { return startedCompare(a, b) })
	for _, e := range es {
		if e.Status != experiment.StatusLive {
			continue
		}
		a, err := h.assign(ctx, e, tenant, student)
		if err != nil {
			return out, err
		}
		if a == nil || a.Status != "active" {
			continue
		}
		var arms []experiment.Arm
		if err := json.Unmarshal(e.Arms, &arms); err != nil {
			return out, err
		}
		i := slices.IndexFunc(arms, func(x experiment.Arm) bool { return x.Code == a.ArmCode })
		if i < 0 || arms[i].IsControl {
			continue
		}
		arm := arms[i]
		switch e.Kind {
		case experiment.KindMastery:
			if out.Model != nil {
				continue // one mastery model per student: the experiment started first
			}
			cfg, err := h.Store.ModelConfig(ctx, arm.ModelVersion)
			if err != nil || cfg == nil {
				return out, err
			}
			m, err := ModelOf(*cfg)
			if err != nil {
				return out, err
			}
			states, err := h.Store.ArmStates(ctx, tenant, e.ID, arm.Code, student)
			if err != nil {
				return out, err
			}
			out.Model, out.States = &m, states
		case experiment.KindRecommendation:
			p, err := recommendation.ParseParams(arm.RuleParams)
			if err != nil {
				return out, err
			}
			out.Rules = out.Rules.Merge(p)
		}
	}
	return out, nil
}

func startedCompare(a, b write.ExperimentRow) int {
	switch {
	case a.StartedAt == nil && b.StartedAt == nil:
	case a.StartedAt == nil:
		return 1
	case b.StartedAt == nil:
		return -1
	case !a.StartedAt.Equal(*b.StartedAt):
		return a.StartedAt.Compare(*b.StartedAt)
	}
	return compareID(a.ID, b.ID)
}
