package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/simulation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/videolesson"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Events of the simulation runs (EVENT_DESIGN §4: outbox; §5.2: learning event).
const (
	EventSimulationRunVerified = "simulation.run_verified"
	EventSimulationRunRejected = "simulation.run_rejected"
	EventSimulationGoalMet     = "simulation.goal_met"
	// KindSimulationQuestion is a conclusion question of a simulation (graded like kind question).
	KindSimulationQuestion = "simulation_question"
)

// SimulationMetrics counts the runs (eduworld_simulation_runs_total{template_code,status},
// eduworld_simulation_checker_duration_seconds{template_code}); nil = not counted.
type SimulationMetrics interface {
	Run(templateCode, status string)
	CheckerDuration(templateCode string, d time.Duration)
}

// SimulationRunInput is SimulationRunInput of submitSimulationRun.
type SimulationRunInput struct {
	AttemptID       uuid.UUID
	TemplateCode    string
	TemplateVersion int
	Seed            int64
	Parameters      map[string]any
	ActionLog       json.RawMessage
	ClientOutcome   map[string]any
}

// simConfig returns the simulation config of a version (ok false for the other types).
func simConfig(v activity.Version) (simulation.Config, bool, error) {
	if v.Type != activity.Simulation {
		return simulation.Config{}, false, nil
	}
	sc, err := simulation.Parse(v.Config.Raw)
	if err != nil {
		return sc, true, problem.New(problem.ActivityConfigInvalid, "the published simulation config cannot be read")
	}
	return sc, true, nil
}

func runView(r write.SimulationRun, d simulation.Descriptor, runsUsed int, maxRuns *int) dto.SimulationRunResultView {
	var gm []simulation.GoalMet
	_ = json.Unmarshal(r.GoalsMet, &gm)
	goals := make([]dto.GoalMetView, 0, len(gm))
	for _, g := range gm {
		goals = append(goals, dto.GoalMetView{GoalID: g.GoalID, Met: g.Met})
	}
	var outcome simulation.Outcome
	_ = json.Unmarshal(r.VerifiedOutcome, &outcome)
	out := dto.SimulationRunResultView{RunID: r.ID, Verified: r.VerificationStatus == simulation.Verified,
		VerificationStatus: r.VerificationStatus, GoalsMet: goals, RunsUsed: runsUsed, MaxRuns: maxRuns}
	out.GoalMet = out.Verified && simulation.AllMet(gm)
	if out.Verified {
		out.VerifiedOutcome = simulation.Display(d, outcome)
	}
	return out
}

// SubmitSimulationRun verifies one run of a simulation attempt on the server (F066-AC-11…AC-15, ADR-0052): the checker
// of the pinned template replays the pinned parameters, the server seed and the action log; goals are evaluated on the
// verified outcome only (a contradicting client_outcome adds an integrity signal, never a penalty). An invalid log or a
// value other than the pinned one is stored as a rejected run with simulation.run_rejected and an integrity signal, and
// answered 422 SIMULATION_LOG_INVALID; at max_runs stored runs it is 422 SIMULATION_RUN_LIMIT_REACHED. The same
// Idempotency-Key returns the same run and never counts again. The action log is never logged (R14).
func (h Attempts) SubmitSimulationRun(ctx context.Context, in SimulationRunInput) (out dto.SimulationRunResultView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SubmitSimulationRun")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if h.Runs == nil {
		return out, problem.New(problem.Internal, "no simulation run store")
	}
	key := idemKey(ctx)
	var rejected *simulation.LogError
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		_, v, _, err := h.versionOf(ctx, a)
		if err != nil {
			return err
		}
		sc, ok, err := simConfig(v)
		if err != nil {
			return err
		}
		if !ok {
			return problem.Invalid("/", problem.SubInvalid, "runs are only sent for simulation activities")
		}
		d, found := sc.Template()
		runs, err := h.Runs.Runs(ctx, a.ID)
		if err != nil {
			return err
		}
		var maxRuns *int
		if sc.Interaction.MaxRuns != nil {
			m := sc.MaxRuns()
			maxRuns = &m
		}
		for _, r := range runs {
			if r.IdempotencyKey == key {
				if r.VerificationStatus == simulation.Rejected {
					rejected = &simulation.LogError{Code: r.RejectCode, Detail: "the run was rejected"}
					return nil
				}
				out = runView(r, d, len(runs), maxRuns)
				return idempotency.SetResult(ctx, out)
			}
		}
		if err := a.CheckActive(h.now(), h.policy()); err != nil {
			return attemptProblem(err)
		}
		if len(runs) >= sc.MaxRuns() {
			return problem.New(problem.SimulationRunLimitReached, "every run of this simulation is used")
		}
		now := h.now()
		run := write.SimulationRun{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID, AttemptID: a.ID, StudentID: a.StudentID,
			RunNo: len(runs) + 1, TemplateCode: truncate(in.TemplateCode, 64), TemplateVersion: max(in.TemplateVersion, 1), Seed: max(a.Seed, 0),
			ActionLog: in.ActionLog, GoalsMet: json.RawMessage("[]"), IdempotencyKey: key, CreatedAt: now}
		if len(run.ActionLog) == 0 || !json.Valid(run.ActionLog) {
			run.ActionLog = json.RawMessage("[]")
		}
		if in.ClientOutcome != nil {
			run.ClientOutcome, _ = json.Marshal(in.ClientOutcome)
		}
		outcome, verr := h.verifyRun(sc, d, found, a, in)
		var le *simulation.LogError
		switch {
		case verr == nil:
		case errors.As(verr, &le):
			rejected = le
		default:
			return verr
		}
		if rejected != nil {
			if len(run.ActionLog) > simulation.MaxLogBytes {
				run.ActionLog = json.RawMessage("[]") // the CHECK of the table: an oversized log is not kept
			}
			run.VerificationStatus, run.RejectCode = simulation.Rejected, truncate(rejected.Code, 64)
			if err := h.Runs.InsertRun(ctx, run); err != nil {
				return err
			}
			if err := h.integrity(ctx, a, write.SignalSimulationLogInvalid, map[string]any{"run_id": run.ID, "reject_code": run.RejectCode}, now); err != nil {
				return err
			}
			h.countRun(d.TemplateCode, run.VerificationStatus)
			return h.publish(ctx, a, EventSimulationRunRejected, runPayload(a, run, nil))
		}
		var gm []simulation.GoalMet
		if d.CheckerAvailable {
			run.VerificationStatus = simulation.Verified
			gm = simulation.EvaluateGoals(sc.Content.Goals, outcome)
			run.VerifiedOutcome, _ = json.Marshal(outcome)
		} else {
			run.VerificationStatus = simulation.NotVerifiable
			gm = simulation.EvaluateGoals(sc.Content.Goals, nil)
		}
		run.GoalsMet, _ = json.Marshal(gm)
		if err := h.Runs.InsertRun(ctx, run); err != nil {
			return err
		}
		if d.CheckerAvailable && simulation.Mismatch(in.ClientOutcome, outcome, gm) {
			if err := h.integrity(ctx, a, write.SignalSimulationOutcomeMismatch, map[string]any{"run_id": run.ID}, now); err != nil {
				return err
			}
		}
		if run.VerificationStatus == simulation.Verified {
			for _, g := range gm {
				if !g.Met {
					continue
				}
				if err := h.learningEvent(ctx, a, EventSimulationGoalMet, "sg:"+run.ID.String()+":"+g.GoalID, now, a.ObjectiveIDs, "student",
					map[string]any{"attempt_id": a.ID, "run_id": run.ID, "goal_id": g.GoalID}); err != nil {
					return err
				}
			}
		}
		if err := h.publish(ctx, a, EventSimulationRunVerified, runPayload(a, run, gm)); err != nil {
			return err
		}
		h.countRun(d.TemplateCode, run.VerificationStatus)
		out = runView(run, d, len(runs)+1, maxRuns)
		return idempotency.SetResult(ctx, out)
	})
	if err == nil && rejected != nil {
		pe := problem.New(problem.SimulationLogInvalid, "the action log of the run is not valid")
		pe.AddField("/action_log", problem.SubInvalid, rejected.Code)
		return dto.SimulationRunResultView{}, pe
	}
	return out, err
}

// verifyRun checks the run against the pinned values and replays it (AC-13): template, version, seed and parameters
// must be the pinned ones; the generic log limits; then the checker of the template.
func (h Attempts) verifyRun(sc simulation.Config, d simulation.Descriptor, found bool, a attempt.Attempt, in SimulationRunInput) (simulation.Outcome, error) {
	if !found {
		return nil, simulation.Reject(simulation.RejectTemplateNotFound, "no template version for the content")
	}
	if in.TemplateCode != d.TemplateCode || in.TemplateVersion != d.TemplateVersion || in.Seed != a.Seed ||
		!simulation.SameParameters(sc.Content.Parameters, in.Parameters) {
		return nil, simulation.Reject(simulation.RejectPinnedValue, "template, seed or parameters differ from the pinned ones")
	}
	var log []simulation.Action
	if err := json.Unmarshal(in.ActionLog, &log); err != nil {
		return nil, simulation.Reject(simulation.RejectArgument, "the action log cannot be read")
	}
	if err := simulation.ValidateLog(d, len(in.ActionLog), log); err != nil {
		return nil, err
	}
	if !d.CheckerAvailable {
		return nil, nil
	}
	ch, ok := simulation.CheckerFor(d.TemplateCode, d.TemplateVersion)
	if !ok {
		return nil, problem.New(problem.Unavailable, "the checker of the template is not available")
	}
	start := time.Now()
	o, err := ch.Run(sc.Content.Parameters, a.Seed, log)
	if h.SimMetrics != nil {
		h.SimMetrics.CheckerDuration(d.TemplateCode, time.Since(start))
	}
	return o, err
}

func (h Attempts) integrity(ctx context.Context, a attempt.Attempt, signal string, evidence map[string]any, now time.Time) error {
	if h.Progress == nil {
		return nil
	}
	ev, _ := json.Marshal(evidence)
	return h.Progress.InsertIntegritySignal(ctx, write.IntegritySignal{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID,
		StudentID: a.StudentID, AttemptID: a.ID, SignalType: signal, Score: 1, Evidence: ev, CreatedAt: now})
}

func (h Attempts) countRun(code, status string) {
	if h.SimMetrics != nil {
		h.SimMetrics.Run(code, status)
	}
}

// runPayload is the payload of simulation.run_verified / run_rejected (specs/events/payloads/learning/…v1).
func runPayload(a attempt.Attempt, r write.SimulationRun, gm []simulation.GoalMet) map[string]any {
	if gm == nil {
		gm = []simulation.GoalMet{}
	}
	p := map[string]any{"run_id": r.ID, "attempt_id": a.ID, "student_id": a.StudentID, "activity_id": nullableID(a.ActivityID),
		"template_code": r.TemplateCode, "template_version": r.TemplateVersion, "run_no": r.RunNo, "goals_met": gm,
		"verification_status": r.VerificationStatus}
	if r.RejectCode != "" {
		p["reject_code"] = r.RejectCode
	}
	return p
}

// lastRun is the last verified run of a simulation attempt and whether it has any accepted run (AC-15, AC-16).
func (h Attempts) lastRun(ctx context.Context, a attempt.Attempt) (*activity.RunResult, bool, error) {
	if h.Runs == nil {
		return nil, false, nil
	}
	runs, err := h.Runs.Runs(ctx, a.ID)
	if err != nil {
		return nil, false, err
	}
	var last *activity.RunResult
	anyRun := false
	for _, r := range runs {
		switch r.VerificationStatus {
		case simulation.Verified:
			var gm []simulation.GoalMet
			_ = json.Unmarshal(r.GoalsMet, &gm)
			res := &activity.RunResult{Verifiable: true}
			for _, g := range gm {
				res.GoalsMet = append(res.GoalsMet, g.Met)
			}
			last, anyRun = res, true
		case simulation.NotVerifiable:
			anyRun = true
		}
	}
	return last, anyRun, nil
}

func (h Attempts) simulationView(ctx context.Context, a attempt.Attempt, v dto.AttemptView) (dto.AttemptView, error) {
	if a.ActivityType != string(activity.Simulation) || h.Runs == nil {
		return v, nil
	}
	runs, err := h.Runs.Runs(ctx, a.ID)
	if err != nil {
		return v, err
	}
	return attemptview.WithSimulation(v, a, len(runs)), nil
}

// simulationCompleted writes activity.completed once for a graded simulation attempt whose completion rule is met
// (goals_met on the last verified run; any run for a template without checker).
func (h Attempts) simulationCompleted(ctx context.Context, a attempt.Attempt) error {
	plugin, v, _, err := h.versionOf(ctx, a)
	if err != nil {
		return err
	}
	sp, ok := plugin.(activity.Simulated)
	if !ok {
		return nil
	}
	last, anyRun, err := h.lastRun(ctx, a)
	if err != nil {
		return err
	}
	if _, done := sp.EvaluateRuns(v, a.ItemResults(), last, anyRun); !done {
		return nil
	}
	return h.completed(ctx, a, &videolesson.Outcome{Completed: true})
}
