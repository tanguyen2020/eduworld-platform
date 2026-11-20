package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ActionMasteryModelActivated is the audit of a model activation (plan F031 [Proposed]).
const ActionMasteryModelActivated = "mastery_model.activated"

// Recompute is the operations side of the mastery model (F031 US5, LEARNING_ANALYTICS §3.4): add a model version,
// recompute every (student, objective) in shadow with it, then activate it. It runs from the operations CLI
// (modules/learning/cmd/mastery) on the platform role; no public API (LEARNING_ANALYTICS §3.4).
type Recompute struct {
	UoW     write.UnitOfWork
	Store   write.RecomputeStore
	Mastery Mastery
	Audit   write.Auditor
	Clock   func() time.Time
}

func (h Recompute) now() time.Time { return now(h.Clock).UTC() }

func (h Recompute) platform(ctx context.Context) (context.Context, error) {
	return tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonPlatformJob)
}

// AddModel registers a draft model version with its parameters (checked against the model ranges).
func (h Recompute) AddModel(ctx context.Context, version string, params json.RawMessage) (err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.AddMasteryModel")
	defer func() { done(err) }()
	c := write.ModelConfig{ID: ids.New(), ModelVersion: version, Algorithm: "weighted_evidence_ewma", Params: params}
	if version == "" || len(version) > 64 {
		return errors.New("model version must have 1 to 64 characters")
	}
	if _, err := ModelOf(c); err != nil {
		return err
	}
	bctx, err := h.platform(ctx)
	if err != nil {
		return err
	}
	return h.UoW.Do(bctx, func(ctx context.Context) error {
		ok, err := h.Store.InsertModel(ctx, c)
		if err == nil && !ok {
			err = fmt.Errorf("model version %s already exists", version)
		}
		return err
	})
}

// RecomputeReport is the comparison of a shadow run with the mastery in use (F031-AC-23).
type RecomputeReport struct {
	RunID             uuid.UUID      `json:"run_id"`
	ModelVersion      string         `json:"model_version"`
	CurrentVersion    string         `json:"current_model_version"`
	Pairs             int            `json:"pairs"`
	LevelsCurrent     map[string]int `json:"levels_current"`
	LevelsShadow      map[string]int `json:"levels_shadow"`
	ChangedLevel      int            `json:"changed_level"`
	MeanAbsValueDelta float64        `json:"mean_abs_value_delta"`
}

func groupByTenant(pairs []write.MasteryPair) ([]uuid.UUID, map[uuid.UUID][]write.MasteryPair) {
	by := map[uuid.UUID][]write.MasteryPair{}
	var order []uuid.UUID
	for _, p := range pairs {
		if _, ok := by[p.TenantID]; !ok {
			order = append(order, p.TenantID)
		}
		by[p.TenantID] = append(by[p.TenantID], p)
	}
	return order, by
}

// Shadow recomputes every live (student, objective) with the model version into learning.objective_mastery_shadows
// (F031-AC-23): the mastery in use never changes; the report compares the level distributions at now.
func (h Recompute) Shadow(ctx context.Context, version string, by *uuid.UUID) (rep RecomputeReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.MasteryRecompute")
	defer func() { done(err) }()
	bctx, err := h.platform(ctx)
	if err != nil {
		return rep, err
	}
	var target *write.ModelConfig
	var current write.ModelConfig
	var pairs []write.MasteryPair
	rep = RecomputeReport{RunID: ids.New(), ModelVersion: version, LevelsCurrent: map[string]int{}, LevelsShadow: map[string]int{}}
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		if target, err = h.Store.ModelByVersion(ctx, version); err != nil {
			return err
		}
		if target == nil || (target.Status != "draft" && target.Status != "shadow") {
			return fmt.Errorf("model version %s must exist as draft or shadow", version)
		}
		if current, err = h.Mastery.Store.ActiveModel(ctx); err != nil {
			return err
		}
		if err := h.Store.SetModelStatus(ctx, version, "shadow"); err != nil {
			return err
		}
		if err := h.Store.InsertRun(ctx, write.RecomputeRun{ID: rep.RunID, ModelVersion: version, Status: "running", StartedBy: by,
			StartedAt: h.now()}); err != nil {
			return err
		}
		pairs, err = h.Store.Pairs(ctx)
		return err
	}); err != nil {
		return rep, err
	}
	m, err := ModelOf(*target)
	if err != nil {
		return rep, err
	}
	cur, err := ModelOf(current)
	if err != nil {
		return rep, err
	}
	rep.CurrentVersion = current.ModelVersion
	at := h.now()
	order, byTenant := groupByTenant(pairs)
	var delta float64
	for _, t := range order {
		sc, err := authz.EventScope(t)
		if err != nil {
			return rep, err
		}
		if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
			for _, p := range byTenant[t] {
				row, err := h.Mastery.Store.Mastery(ctx, p.StudentID, p.ObjectiveID)
				if err != nil || row == nil {
					return err
				}
				rows, err := h.Mastery.Evidence.ObjectiveEvidence(ctx, p.StudentID, p.ObjectiveID)
				if err != nil {
					return err
				}
				after := mastery.Replay(m, row.State, EvidenceOf(rows))
				curValue, curLevel := cur.Effective(row.State, at)
				newValue, newLevel := m.Effective(after, at)
				rep.LevelsCurrent[string(curLevel)]++
				rep.LevelsShadow[string(newLevel)]++
				if curLevel != newLevel {
					rep.ChangedLevel++
				}
				delta += math.Abs(newValue - curValue)
				rep.Pairs++
				if err := h.Store.InsertShadow(ctx, write.ShadowRow{RunID: rep.RunID, Pair: p, State: after, Level: newLevel,
					CurrentLevel: curLevel, ModelVersion: version}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			_ = h.finish(bctx, rep.RunID, "failed", nil)
			return rep, err
		}
	}
	if rep.Pairs > 0 {
		rep.MeanAbsValueDelta = math.Round(delta/float64(rep.Pairs)*1000) / 1000
	}
	raw, _ := json.Marshal(rep)
	return rep, h.finish(bctx, rep.RunID, "completed", raw)
}

func (h Recompute) finish(bctx context.Context, id uuid.UUID, status string, report json.RawMessage) error {
	return h.UoW.Do(bctx, func(ctx context.Context) error { return h.Store.FinishRun(ctx, id, status, report, h.now()) })
}

// ActivateReport counts what an activation changed.
type ActivateReport struct {
	ModelVersion string `json:"model_version"`
	Pairs        int    `json:"pairs"`
}

// Activate makes the model of a completed shadow run the active one (F031-AC-24) and applies it to the pairs of the run,
// tenant by tenant: each pair is recomputed with the new model (source recompute), so learning_objective.mastered is
// emitted only for an objective never mastered before. The operator (System Admin) is recorded and audited.
func (h Recompute) Activate(ctx context.Context, runID, by uuid.UUID) (rep ActivateReport, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.ActivateMasteryModel")
	defer func() { done(err) }()
	if by == uuid.Nil {
		return rep, errors.New("the operator activating the model is required")
	}
	bctx, err := h.platform(ctx)
	if err != nil {
		return rep, err
	}
	var cfg *write.ModelConfig
	var pairs []write.MasteryPair
	at := h.now()
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		run, err := h.Store.Run(ctx, runID)
		if err != nil {
			return err
		}
		if run == nil || run.Status != "completed" {
			return fmt.Errorf("recompute run %s is not a completed shadow run", runID)
		}
		if cfg, err = h.Store.ModelByVersion(ctx, run.ModelVersion); err != nil {
			return err
		}
		if cfg == nil {
			return fmt.Errorf("model version %s is gone", run.ModelVersion)
		}
		if err := h.Store.ActivateModel(ctx, run.ModelVersion, by, at); err != nil {
			return err
		}
		if err := h.Store.MarkRunActivated(ctx, runID, by, at); err != nil {
			return err
		}
		if pairs, err = h.Store.ShadowPairs(ctx, runID); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: ActionMasteryModelActivated, TargetType: "mastery_model", TargetID: run.ModelVersion,
			Result: audit.ResultSuccess, Metadata: map[string]any{"run_id": runID.String(), "activated_by": by.String(), "pairs": len(pairs)}})
	}); err != nil {
		return rep, err
	}
	m, err := ModelOf(*cfg)
	if err != nil {
		return rep, err
	}
	rep.ModelVersion = cfg.ModelVersion
	order, byTenant := groupByTenant(pairs)
	for _, t := range order {
		sc, err := authz.EventScope(t)
		if err != nil {
			return rep, err
		}
		if err := h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
			for _, p := range byTenant[t] {
				src := Source{EventID: runID, TenantID: t, StudentID: p.StudentID, SchoolID: p.SchoolID, Mode: "practice",
					Kind: MasterySourceRecompute, OccurredAt: at}
				if err := h.Mastery.Recompute(ctx, m, src, p.ObjectiveID); err != nil {
					return err
				}
				rep.Pairs++
			}
			return nil
		}); err != nil {
			return rep, err
		}
	}
	return rep, nil
}
