package command

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/efficacy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/experiment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Audit actions of the experiments.
const (
	ActionExperimentCreated = "learning_experiment.created"
	ActionExperimentUpdated = "learning_experiment.updated"
	ActionExperimentStarted = "learning_experiment.started"
	ActionExperimentStopped = "learning_experiment.stopped"
)

// ExperimentConfig holds experiment_guardrail_drop_pp, experiment_eval_delay_days and analytics_min_cell_size.
type ExperimentConfig struct {
	GuardrailDropPP float64
	EvalDelay       time.Duration
	MinCellSize     int
}

// DefaultExperimentConfig returns the registered defaults (CONFIG_DEFAULTS §13).
func DefaultExperimentConfig() ExperimentConfig {
	return ExperimentConfig{GuardrailDropPP: 5, EvalDelay: 7 * 24 * time.Hour, MinCellSize: 5}
}

// ExperimentMetrics observes the experiments; nil = not observed.
type ExperimentMetrics interface {
	Assigned(kind string)
	GuardrailBreach()
	ShadowUpdates(n int)
}

// Experiments holds the A/B experiments on the mastery model and the recommendation rules (F035 US5, ADR-0080 mục 5):
// definition and transitions (System Admin, platform.learning_experiment.manage), the stable assignment at the first
// contact, the arm labels of the learning events, the arm mastery states computed in parallel and the daily guardrail.
type Experiments struct {
	UoW      write.UnitOfWork
	Store    write.ExperimentStore
	Efficacy write.EfficacyStore // agreements of the tenants
	Mastery  write.MasteryStore
	Evidence write.EvidenceReader
	Audit    write.Auditor
	Clock    func() time.Time
	Config   ExperimentConfig
	Metrics  ExperimentMetrics
}

func (h Experiments) now() time.Time { return now(h.Clock).UTC() }

func (h Experiments) config() ExperimentConfig {
	if h.Config.GuardrailDropPP <= 0 {
		return DefaultExperimentConfig()
	}
	return h.Config
}

// ExperimentInput is LearningExperimentCreate (and the merged result of an update).
type ExperimentInput struct {
	Code           string
	Kind           string
	Unit           string
	Arms           []experiment.Arm
	TenantIDs      []uuid.UUID
	StudyID        *uuid.UUID
	StartAt        time.Time
	EndAt          time.Time
	IdempotencyKey string
}

// ExperimentViewOf is the view of an experiment row.
func ExperimentViewOf(e write.ExperimentRow) dto.LearningExperimentView {
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

func experimentViolations(vs []experiment.Violation) error {
	if len(vs) == 0 {
		return nil
	}
	pe := problem.Invalid(vs[0].Field, problem.SubInvalid, vs[0].Reason)
	for _, v := range vs[1:] {
		pe.AddField(v.Field, problem.SubInvalid, v.Reason)
	}
	return pe
}

// check validates a definition against the model versions, the agreements of the tenants and the study (F035-AC-21).
func (h Experiments) check(ctx context.Context, in ExperimentInput) error {
	known, active, err := h.Store.ModelVersions(ctx)
	if err != nil {
		return err
	}
	d := experiment.Definition{Code: in.Code, Kind: in.Kind, Unit: in.Unit, Arms: in.Arms, TenantIDs: in.TenantIDs, StartAt: in.StartAt,
		EndAt: in.EndAt, ActiveVer: active, KnownVers: known}
	for _, t := range in.TenantIDs {
		as, err := h.Efficacy.Agreements(ctx, t)
		if err != nil {
			return err
		}
		qa, err := h.Efficacy.TenantIsQA(ctx, t)
		if err != nil {
			return err
		}
		if qa || !efficacy.Covers(agreementsOf(as), efficacy.CoversExperiment) {
			d.NoConsent = append(d.NoConsent, t)
		}
	}
	if in.StudyID != nil {
		cohorts, err := h.Store.InterventionCohorts(ctx, *in.StudyID)
		if err != nil {
			return err
		}
		d.Comparison = len(cohorts) == 0
	}
	return experimentViolations(d.Validate())
}

// CreateLearningExperiment creates a draft experiment (F035-AC-21): validated definition, unique code (409), revision 1;
// idempotent; audit learning_experiment.created.
func (h Experiments) CreateLearningExperiment(ctx context.Context, in ExperimentInput) (out dto.LearningExperimentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateLearningExperiment")
	defer func() { done(err) }()
	p, err := efficacyAdmin(ctx, authz.PermPlatformLearningExperimentManage)
	if err != nil {
		return out, err
	}
	if in.Unit == "" {
		in.Unit = experiment.UnitSchoolGrade
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite, in.TenantIDs...)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		if in.IdempotencyKey != "" {
			if prev, err := h.Store.ExperimentByKey(ctx, p.UserID, in.IdempotencyKey); err != nil || prev != nil {
				if err == nil {
					out = ExperimentViewOf(*prev)
				}
				return err
			}
		}
		if err := h.check(ctx, in); err != nil {
			return err
		}
		if taken, err := h.Store.ExperimentCodeTaken(ctx, in.Code); err != nil || taken {
			if err == nil {
				err = problem.New(problem.Conflict, "the experiment code is taken")
			}
			return err
		}
		arms, _ := json.Marshal(in.Arms)
		now := h.now()
		e := write.ExperimentRow{ID: ids.New(), Code: in.Code, Kind: in.Kind, Unit: in.Unit, Arms: arms, TenantIDs: in.TenantIDs, StudyID: in.StudyID,
			Status: experiment.StatusDraft, StartAt: in.StartAt.UTC(), EndAt: in.EndAt.UTC(), Revision: 1, OwnerID: p.UserID,
			IdempotencyKey: truncateRunes(in.IdempotencyKey, 64), Version: 1, CreatedAt: now, UpdatedAt: now}
		if e.IdempotencyKey == "" {
			e.IdempotencyKey = e.ID.String()
		}
		if err := h.Store.InsertExperiment(ctx, e); err != nil {
			return err
		}
		if err := h.auditExperiment(ctx, ActionExperimentCreated, e, ""); err != nil {
			return err
		}
		out = ExperimentViewOf(e)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func (h Experiments) auditExperiment(ctx context.Context, action string, e write.ExperimentRow, reason string) error {
	return h.Audit.Record(ctx, audit.Event{Action: action, TargetType: "learning_experiment", TargetID: e.ID.String(), Result: audit.ResultSuccess,
		Reason: reason, Metadata: map[string]any{"code": e.Code, "kind": e.Kind, "status": e.Status, "revision": e.Revision}})
}

// ExperimentPatch is LearningExperimentUpdate (nil = unchanged; StudyID.Set with nil Value clears it).
type ExperimentPatch struct {
	Unit      *string
	Arms      []experiment.Arm
	TenantIDs []uuid.UUID
	StudyID   Patch[uuid.UUID]
	StartAt   *time.Time
	EndAt     *time.Time
}

// UpdateLearningExperiment edits a draft experiment (If-Match = revision; another status → 422 INVALID_STATE_TRANSITION),
// increments the revision; audit learning_experiment.updated.
func (h Experiments) UpdateLearningExperiment(ctx context.Context, id uuid.UUID, ifMatch int64, in ExperimentPatch) (out dto.LearningExperimentView,
	err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpdateLearningExperiment")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		e, err := h.Store.Experiment(ctx, id, true)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "")
		}
		if int64(e.Revision) != ifMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		if e.Status != experiment.StatusDraft {
			return problem.New(problem.InvalidStateTransition, "only a draft experiment is edited")
		}
		var arms []experiment.Arm
		_ = json.Unmarshal(e.Arms, &arms)
		in2 := ExperimentInput{Code: e.Code, Kind: e.Kind, Unit: e.Unit, Arms: arms, TenantIDs: e.TenantIDs, StudyID: e.StudyID, StartAt: e.StartAt,
			EndAt: e.EndAt}
		if in.Unit != nil {
			in2.Unit = *in.Unit
		}
		if in.Arms != nil {
			in2.Arms = in.Arms
		}
		if in.TenantIDs != nil {
			in2.TenantIDs = in.TenantIDs
		}
		if in.StudyID.Set {
			in2.StudyID = in.StudyID.Value
		}
		if in.StartAt != nil {
			in2.StartAt = *in.StartAt
		}
		if in.EndAt != nil {
			in2.EndAt = *in.EndAt
		}
		if err := h.check(ctx, in2); err != nil {
			return err
		}
		prev := e.Version
		e.Unit, e.TenantIDs, e.StudyID, e.StartAt, e.EndAt = in2.Unit, in2.TenantIDs, in2.StudyID, in2.StartAt.UTC(), in2.EndAt.UTC()
		e.Arms, _ = json.Marshal(in2.Arms)
		e.Revision++
		if ok, err := h.Store.UpdateExperiment(ctx, *e, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		if err := h.auditExperiment(ctx, ActionExperimentUpdated, *e, ""); err != nil {
			return err
		}
		out = ExperimentViewOf(*e)
		return nil
	})
	return out, err
}

// StartLearningExperiment starts an experiment in shadow (from draft) or live (from draft or shadow) (F035-AC-23, AC-24);
// audit learning_experiment.started.
func (h Experiments) StartLearningExperiment(ctx context.Context, id uuid.UUID, exposure string) (out dto.LearningExperimentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StartLearningExperiment")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		e, err := h.Store.Experiment(ctx, id, true)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "")
		}
		next, err := experiment.Start(e.Status, exposure)
		if errors.Is(err, experiment.ErrState) {
			return problem.New(problem.InvalidStateTransition, "the experiment cannot move to "+exposure)
		}
		if err != nil {
			return problem.Invalid("/exposure", problem.SubInvalid, err.Error())
		}
		now, prev := h.now(), e.Version
		e.Status = next
		if e.StartedAt == nil {
			e.StartedAt = &now
		}
		if ok, err := h.Store.UpdateExperiment(ctx, *e, prev); err != nil || !ok {
			if err == nil {
				err = problem.New(problem.VersionMismatch, "")
			}
			return err
		}
		if err := h.auditExperiment(ctx, ActionExperimentStarted, *e, ""); err != nil {
			return err
		}
		out = ExperimentViewOf(*e)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// stop ends a running experiment with a reason: every assignment removed (the students are back on the control
// experience from the next request); nothing is published again (learning_objective.mastered is never replayed).
func (h Experiments) stop(ctx context.Context, e *write.ExperimentRow, status, reason, note string) error {
	now, prev := h.now(), e.Version
	e.Status, e.StopReason, e.StoppedAt = status, &reason, &now
	if note != "" {
		e.StopNote = &note
	}
	if ok, err := h.Store.UpdateExperiment(ctx, *e, prev); err != nil || !ok {
		if err == nil {
			err = problem.New(problem.VersionMismatch, "")
		}
		return err
	}
	if _, err := h.Store.RemoveAssignments(ctx, e.ID, nil); err != nil {
		return err
	}
	return h.auditExperiment(ctx, ActionExperimentStopped, *e, reason)
}

// StopLearningExperiment stops a running experiment by hand (F035-AC-26, stop_reason manual).
func (h Experiments) StopLearningExperiment(ctx context.Context, id uuid.UUID, note string) (out dto.LearningExperimentView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StopLearningExperiment")
	defer func() { done(err) }()
	if _, err := efficacyAdmin(ctx, authz.PermPlatformLearningExperimentManage); err != nil {
		return out, err
	}
	if !efficacy.ValidReason(note) {
		return out, problem.Invalid("/reason", problem.SubInvalid, "must have 10 to 1000 characters")
	}
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		e, err := h.Store.Experiment(ctx, id, true)
		if err != nil {
			return err
		}
		if e == nil {
			return problem.New(problem.NotFound, "")
		}
		if !experiment.Running(e.Status) {
			return problem.New(problem.InvalidStateTransition, "only a running experiment is stopped")
		}
		if err := h.stop(ctx, e, experiment.StatusStopped, experiment.StopManual, note); err != nil {
			return err
		}
		out = ExperimentViewOf(*e)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// LeaveTenant takes the students of a tenant out of every running experiment (F035-AC-20, agreement revoked), in the
// transaction of ctx (TenantScope of the tenant).
func (h Experiments) LeaveTenant(ctx context.Context, tenant uuid.UUID, _ time.Time) (int, error) {
	es, err := h.Store.RunningExperiments(ctx, &tenant)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range es {
		k, err := h.Store.RemoveAssignments(ctx, e.ID, &tenant)
		if err != nil {
			return n, err
		}
		n += k
	}
	return n, nil
}

// ArmsOf is Attempts.Arms (F035-AC-22, AC-25): for every running experiment covering the tenant, the arm of the student —
// stored at the first contact from hash(experiment_id, unit) (unit = the cohort school × grade, or the student) and kept
// afterwards (a change of grade or school keeps it); a study restricts the units to its intervention cohorts; a removed assignment is out.
// Returns "<code>:<arm>" labels; runs in the transaction of ctx.
func (h Experiments) ArmsOf(ctx context.Context, tenant, student uuid.UUID) ([]string, error) {
	es, err := h.Store.RunningExperiments(ctx, &tenant)
	if err != nil || len(es) == 0 {
		return nil, err
	}
	var out []string
	for _, e := range es {
		a, err := h.assign(ctx, e, tenant, student)
		if err != nil {
			return nil, err
		}
		if a != nil && a.Status == "active" {
			out = append(out, experiment.ArmLabel(e.Code, a.ArmCode))
		}
	}
	return out, nil
}

func (h Experiments) assign(ctx context.Context, e write.ExperimentRow, tenant, student uuid.UUID) (*write.ExperimentAssignmentRow, error) {
	a, err := h.Store.Assignment(ctx, tenant, e.ID, student)
	if err != nil || a != nil {
		return a, err
	}
	cohort, err := h.Store.StudentCohort(ctx, tenant, student)
	if err != nil {
		return nil, err
	}
	if e.StudyID != nil {
		cohorts, err := h.Store.InterventionCohorts(ctx, *e.StudyID)
		if err != nil {
			return nil, err
		}
		if cohort == nil || !containsID(cohorts, *cohort) {
			return nil, nil // comparison and validation cohorts never take part (F035-AC-21)
		}
	}
	unit := student
	if e.Unit == experiment.UnitSchoolGrade {
		if cohort == nil {
			return nil, nil
		}
		unit = *cohort
	}
	var arms []experiment.Arm
	if err := json.Unmarshal(e.Arms, &arms); err != nil || len(arms) == 0 {
		return nil, err
	}
	b := experiment.Bucket(e.ID, unit)
	row := write.ExperimentAssignmentRow{ExperimentID: e.ID, StudentID: student, TenantID: tenant, UnitID: unit, Bucket: b,
		ArmCode: experiment.ArmOf(arms, b).Code, Status: "active", AssignedAt: h.now()}
	ok, err := h.Store.InsertAssignment(ctx, row)
	if err != nil {
		return nil, err
	}
	if !ok {
		return h.Store.Assignment(ctx, tenant, e.ID, student)
	}
	if h.Metrics != nil {
		h.Metrics.Assigned(e.Kind)
	}
	return &row, nil
}

// Shadow computes, from the same evidence, the mastery of the student under the model of its arm in every running
// mastery_model experiment (F035-AC-23); called by the mastery consumer after the active model; nothing displayed
// changes. Runs in the transaction and TenantScope of the consumer.
func (h Experiments) Shadow(ctx context.Context, tenant, student uuid.UUID, objectives []uuid.UUID) error {
	es, err := h.Store.RunningExperiments(ctx, &tenant)
	if err != nil {
		return err
	}
	n := 0
	for _, e := range es {
		if e.Kind != experiment.KindMastery {
			continue
		}
		a, err := h.assign(ctx, e, tenant, student)
		if err != nil {
			return err
		}
		if a == nil || a.Status != "active" {
			continue
		}
		var arms []experiment.Arm
		_ = json.Unmarshal(e.Arms, &arms)
		var arm experiment.Arm
		for _, x := range arms {
			if x.Code == a.ArmCode {
				arm = x
			}
		}
		cfg, err := h.Store.ModelConfig(ctx, arm.ModelVersion)
		if err != nil || cfg == nil {
			return err
		}
		m, err := ModelOf(*cfg)
		if err != nil {
			return err
		}
		for _, o := range objectives {
			row, err := h.Mastery.Mastery(ctx, student, o)
			if err != nil {
				return err
			}
			prior := mastery.State{}
			if row != nil {
				prior = row.State
			}
			rows, err := h.Evidence.ObjectiveEvidence(ctx, student, o)
			if err != nil {
				return err
			}
			after := mastery.Replay(m, prior, EvidenceOf(rows))
			at := h.now()
			if after.LastEvidenceAt != nil {
				at = *after.LastEvidenceAt
			}
			_, level := m.Effective(after, at)
			if err := h.Store.UpsertArmState(ctx, tenant, e.ID, a.ArmCode, student, o, after, level, m.Version()); err != nil {
				return err
			}
			n++
		}
	}
	if n > 0 && h.Metrics != nil {
		h.Metrics.ShadowUpdates(n)
	}
	return nil
}

// RunGuardrail is the job experiment-guardrail (daily, F035-AC-26): for every running experiment, the primary metric of
// each arm (first accuracy on new questions of the objective after experiment_eval_delay_days) against the control; an
// arm at least experiment_guardrail_drop_pp points under it stops the experiment (guardrail_breach), the students go
// back to the control, alert metric and audit; the results (small cells hidden) are stored for
// getLearningExperimentResults.
func (h Experiments) RunGuardrail(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ExperimentGuardrail")
	defer func() { done(err) }()
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return 0, err
	}
	var es []write.ExperimentRow
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		es, err = h.Store.RunningExperiments(ctx, nil)
		return err
	}); err != nil {
		return 0, err
	}
	for _, e := range es {
		if err := h.UoW.Do(bctx, func(ctx context.Context) error { return h.evaluate(ctx, e.ID) }); err != nil {
			slog.ErrorContext(ctx, "experiment guardrail", "experiment_id", e.ID, "error", err)
			return n, err
		}
		n++
	}
	return n, nil
}

func (h Experiments) evaluate(ctx context.Context, id uuid.UUID) error {
	e, err := h.Store.Experiment(ctx, id, true)
	if err != nil || e == nil || !experiment.Running(e.Status) {
		return err
	}
	cfg := h.config()
	metrics, err := h.Store.ArmMetrics(ctx, *e, cfg.EvalDelay)
	if err != nil {
		return err
	}
	results, breached := ExperimentResultsOf(*e, metrics, cfg)
	raw, _ := json.Marshal(results)
	if err := h.Store.InsertResult(ctx, e.ID, h.now(), raw, breached); err != nil {
		return err
	}
	if !breached {
		return nil
	}
	if h.Metrics != nil {
		h.Metrics.GuardrailBreach()
	}
	slog.WarnContext(ctx, "experiment stopped by the guardrail", "experiment_id", e.ID, "code", e.Code)
	return h.stop(ctx, e, experiment.StatusStopped, experiment.StopGuardrail, "")
}

// ExperimentResultsOf builds the results of the arms: an arm with fewer than analytics_min_cell_size students (or
// answers) is suppressed; breached reports an arm under the control by experiment_guardrail_drop_pp or more.
func ExperimentResultsOf(e write.ExperimentRow, metrics []write.ArmMetric, cfg ExperimentConfig) (map[string]any, bool) {
	var arms []experiment.Arm
	_ = json.Unmarshal(e.Arms, &arms)
	by := map[string]write.ArmMetric{}
	for _, m := range metrics {
		by[m.ArmCode] = m
	}
	control := experiment.Control(arms)
	rate := func(m write.ArmMetric) (float64, bool) {
		if m.Students < cfg.MinCellSize || m.Answers < cfg.MinCellSize {
			return 0, false
		}
		return float64(m.Correct) / float64(m.Answers), true
	}
	cRate, cOK := rate(by[control.Code])
	breached := false
	var out []map[string]any
	for _, a := range arms {
		m := by[a.Code]
		r, ok := rate(m)
		row := map[string]any{"arm_code": a.Code, "is_control": a.IsControl, "suppressed": !ok}
		if ok {
			row["unit_count"], row["student_count"], row["primary_metric"] = m.Units, m.Students, round3(r)
			if m.LogLoss != nil {
				row["next_item_log_loss"] = round3(*m.LogLoss)
			}
			if m.ActiveStudents > 0 {
				row["active_minutes_avg"] = round3(m.ActiveMinutes / float64(m.ActiveStudents))
			}
			if cOK && !a.IsControl && experiment.Breach(cRate, r, cfg.GuardrailDropPP) {
				breached = true
			}
		}
		out = append(out, row)
	}
	if out == nil {
		out = []map[string]any{}
	}
	return map[string]any{"arms": out}, breached
}

// RunLifecycle is the job experiment-lifecycle: running experiments past end_at complete (stop_reason ended).
func (h Experiments) RunLifecycle(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.ExperimentLifecycle")
	defer func() { done(err) }()
	bctx, err := studyScope(ctx, tenancy.ModeWrite)
	if err != nil {
		return 0, err
	}
	err = h.UoW.Do(bctx, func(ctx context.Context) error {
		es, err := h.Store.DueExperiments(ctx, h.now())
		if err != nil {
			return err
		}
		for i := range es {
			e, err := h.Store.Experiment(ctx, es[i].ID, true)
			if err != nil || e == nil {
				return err
			}
			if err := h.stop(ctx, e, experiment.StatusCompleted, experiment.StopEnded, ""); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}
