package query

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Experiments holds the reads of the A/B experiments (F035 US5): platform.learning_experiment.manage first (R7).
type Experiments struct {
	Session read.Session
	Model   read.ExperimentReadModel
	Cursors *page.Codec
}

// ExperimentView is the view of an experiment row.
func ExperimentView(e read.ExperimentRow) dto.LearningExperimentView {
	var arms []experiment.Arm
	_ = json.Unmarshal(e.Arms, &arms)
	v := dto.LearningExperimentView{ExperimentID: e.ID, Code: e.Code, Kind: e.Kind, Unit: e.Unit, TenantIDs: e.TenantIDs, StudyID: e.StudyID,
		Status: e.Status, StopReason: e.StopReason, StartAt: e.StartAt, EndAt: e.EndAt, Revision: e.Revision, OwnerID: e.OwnerID,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Arms: []dto.ExperimentArmView{}}
	for _, a := range arms {
		v.Arms = append(v.Arms, dto.ExperimentArmView{Code: a.Code, AllocationPct: a.AllocationPct, IsControl: a.IsControl,
			ModelVersion: a.ModelVersion, RuleParams: a.RuleParams})
	}
	if v.TenantIDs == nil {
		v.TenantIDs = []uuid.UUID{}
	}
	return v
}

// ListLearningExperiments is listLearningExperiments.
func (q Experiments) ListLearningExperiments(ctx context.Context, status, kind string, in PageInput) (out Page[dto.LearningExperimentView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListLearningExperiments")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	var views []dto.LearningExperimentView
	if err := q.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Model.Experiments(ctx, status, kind)
		for _, e := range rows {
			views = append(views, ExperimentView(e))
		}
		return err
	}); err != nil {
		return out, err
	}
	return paginate(views, func(v dto.LearningExperimentView) uuid.UUID { return v.ExperimentID }, in, q.Cursors)
}

// GetLearningExperiment is getLearningExperiment.
func (q Experiments) GetLearningExperiment(ctx context.Context, id uuid.UUID) (out dto.LearningExperimentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLearningExperiment")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		e, err := q.Model.Experiment(ctx, id)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "")
		}
		out = ExperimentView(*e)
		return nil
	})
	return out, err
}

// GetLearningExperimentResults is getLearningExperimentResults (F035-AC-27): the latest results of the guardrail job by
// arm, small cells hidden; before the first run every arm is suppressed.
func (q Experiments) GetLearningExperimentResults(ctx context.Context, id uuid.UUID) (out dto.ExperimentResultsView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLearningExperimentResults")
	defer func() { done(err) }()
	if err := efficacyReader(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		e, err := q.Model.Experiment(ctx, id)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "")
		}
		raw, at, breached, err := q.Model.LatestResult(ctx, id)
		if err != nil {
			return err
		}
		out = dto.ExperimentResultsView{ExperimentID: id, ComputedAt: e.UpdatedAt, GuardrailBreached: breached, Arms: []map[string]any{}}
		if raw != nil {
			var r struct {
				Arms []map[string]any `json:"arms"`
			}
			_ = json.Unmarshal(raw, &r)
			out.ComputedAt, out.Arms = *at, r.Arms
			return nil
		}
		var arms []experiment.Arm
		_ = json.Unmarshal(e.Arms, &arms)
		for _, a := range arms {
			out.Arms = append(out.Arms, map[string]any{"arm_code": a.Code, "is_control": a.IsControl, "suppressed": true})
		}
		return nil
	})
	return out, err
}
