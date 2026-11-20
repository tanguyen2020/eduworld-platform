package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/scope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/evaluation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Re-evaluation constants (ACTIVITY_ENGINE §7.3, F031 US4).
const (
	EventAttemptReEvaluated  = "attempt.re_evaluated"
	ActionContentReEvaluated = "content.re_evaluated"
	RegradeQueued            = "queued"
	RegradeRunning           = "running"
	RegradeCompleted         = "completed"
	RegradeFailed            = "failed"
)

// RegradeMetrics counts re-evaluated attempts by action (eduworld_regrade_attempts_total{action}); nil = not counted.
type RegradeMetrics interface {
	ReEvaluated(action string)
}

// Regrades are the write use cases of the re-evaluation (createRegradeRequest, job regrade-runner; F031 US4). The
// attempt engine (F030) loads attempts, versions and grades answers; there is no manual grading (ADR-0072).
type Regrades struct {
	UoW      write.UnitOfWork
	Store    write.RegradeStore
	Attempts Attempts
	Audit    write.Auditor
	Clock    func() time.Time
	Metrics  RegradeMetrics
}

func (h Regrades) now() time.Time { return now(h.Clock).UTC() }

// CreateRegradeInput is RegradeRequestCreate.
type CreateRegradeInput struct {
	Scope              string     `json:"scope"`
	QuestionVersionID  *uuid.UUID `json:"question_version_id"`
	ActivityVersionID  *uuid.UUID `json:"activity_version_id"`
	ExamSessionID      *uuid.UUID `json:"exam_session_id"`
	CorrectedVersionID *uuid.UUID `json:"corrected_version_id"`
	Action             string     `json:"action"`
	Reason             string     `json:"reason"`
}

func holdsPerm(p authz.Principal, perm authz.Permission) bool {
	for _, g := range p.Grants {
		for _, x := range g.Permissions {
			if x == perm {
				return true
			}
		}
	}
	return false
}

// validate checks the body rules the schema cannot express (F031-AC-18, Clarifications 2026-11-20 [Proposed]).
func (in *CreateRegradeInput) validate() error {
	if in.Action == "" {
		in.Action = string(evaluation.UseNewKey)
	}
	if !evaluation.Action(in.Action).Valid() {
		return problem.Invalid("/action", problem.SubInvalid, "must be accept_all, use_new_key or drop_item")
	}
	if n := utf8.RuneCountInString(in.Reason); n < 10 || n > 1000 {
		return problem.Invalid("/reason", problem.SubInvalid, "must have 10 to 1000 characters")
	}
	switch in.Scope {
	case "question_version":
		if in.QuestionVersionID == nil {
			return problem.Invalid("/question_version_id", problem.SubRequired, "is required for scope question_version")
		}
	case "activity_version":
		if in.ActivityVersionID == nil {
			return problem.Invalid("/activity_version_id", problem.SubRequired, "is required for scope activity_version")
		}
		if in.Action != string(evaluation.AcceptAll) {
			return problem.Invalid("/action", problem.SubInvalid, "scope activity_version supports accept_all only; use scope question_version")
		}
	case "exam_session":
		return problem.Invalid("/scope", problem.SubInvalid, "exam_session re-evaluation belongs to the exam engine (F042)")
	default:
		return problem.Invalid("/scope", problem.SubInvalid, "must be question_version or activity_version")
	}
	if in.Action == string(evaluation.UseNewKey) && in.CorrectedVersionID == nil {
		return problem.Invalid("/corrected_version_id", problem.SubRequired, "is required for use_new_key")
	}
	return nil
}

// RegradeView renders a request (RegradeRequest of the API).
func RegradeView(r write.RegradeRequest) dto.RegradeRequestView {
	return dto.RegradeRequestView{RegradeRequestID: r.ID, Scope: r.Scope, Action: string(r.Action), Status: r.Status,
		AffectedAttempts: r.Affected, ReEvaluatedAttempts: r.ReEvaluated, Reason: r.Reason, RequestedBy: r.RequestedBy,
		CreatedAt: r.CreatedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, QuestionVersionID: r.QuestionVersionID,
		ActivityVersionID: r.ActivityVersionID, CorrectedVersionID: r.CorrectedVersionID, TenantID: r.TenantID}
}

// Create is createRegradeRequest (F031-AC-18, AC-19, AC-22): learning_content.re_evaluate; content_admin for platform
// content (no session tenant: every tenant is re-evaluated by the job), academic_manager for content visible to the
// session tenant within its subject scope; one queued/running request per version and tenant (409 CONFLICT).
func (h Regrades) Create(ctx context.Context, in CreateRegradeInput) (out dto.RegradeRequestView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.CreateRegradeRequest")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !holdsPerm(p, authz.PermLearningContentReEvaluate) {
		return out, authz.Require(ctx, p, authz.PermLearningContentReEvaluate, authz.Resource{TenantID: p.TenantID})
	}
	if err := in.validate(); err != nil {
		return out, err
	}
	owner, role := contentmodel.OwnerPlatform, "content_admin"
	if p.TenantID != nil {
		owner, role = contentmodel.OwnerTenant, "academic_manager"
	}
	sc, err := scope.ForWrite(p, owner, nil)
	if err != nil {
		return out, err
	}
	key := idemKey(ctx)
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		if prev, err := h.Store.RequestByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if prev != nil {
				out = RegradeView(*prev)
			}
			return err
		}
		var v *write.VersionInfo
		if in.Scope == "question_version" {
			v, err = h.Store.QuestionVersion(ctx, *in.QuestionVersionID)
		} else {
			v, err = h.Store.ActivityVersion(ctx, *in.ActivityVersionID)
		}
		if err != nil {
			return err
		}
		// Platform staff re-evaluates platform content only; a tenant sees its own and its assigned content (RLS H).
		if v == nil || (p.TenantID == nil && v.TenantID != nil) {
			return problem.New(problem.NotFound, "")
		}
		if err := authz.Require(ctx, p, authz.PermLearningContentReEvaluate, authz.Resource{TenantID: p.TenantID, SubjectID: v.SubjectID}); err != nil {
			return err
		}
		if in.Action == string(evaluation.UseNewKey) {
			c, err := h.Store.QuestionVersion(ctx, *in.CorrectedVersionID)
			if err != nil {
				return err
			}
			if c == nil || c.ItemID != v.ItemID || c.ID == v.ID || c.Status != "published" {
				return problem.Invalid("/corrected_version_id", problem.SubInvalid, "must be another published version of the same question")
			}
		}
		r := write.RegradeRequest{ID: ids.New(), TenantID: p.TenantID, SubjectID: v.SubjectID, Scope: in.Scope,
			QuestionVersionID: in.QuestionVersionID, ActivityVersionID: in.ActivityVersionID, CorrectedVersionID: in.CorrectedVersionID,
			Action: evaluation.Action(in.Action), Reason: in.Reason, Status: RegradeQueued, RequestedBy: p.UserID, RequestedRole: role,
			IdempotencyKey: key, CreatedAt: h.now()}
		ok, err := h.Store.InsertRequest(ctx, r)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.Conflict, "a re-evaluation of this version is already queued or running")
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: ActionContentReEvaluated, TargetType: in.Scope, TargetID: r.Target().String(),
			Result: audit.ResultSuccess, Reason: truncate(in.Reason, 500), Metadata: map[string]any{"regrade_request_id": r.ID.String(),
				"action": in.Action, "status": RegradeQueued}}); err != nil {
			return err
		}
		out = RegradeView(r)
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// RunOnce is one run of the job regrade-runner (F031-AC-20): it claims the oldest queued request under
// tenancy.Bypass(re_evaluation), re-evaluates every affected attempt in the scope of its tenant and its own transaction,
// audits each tenant touched and records the counts. It returns the number of attempts re-evaluated.
func (h Regrades) RunOnce(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "learning.job.RegradeRunner")
	defer func() { done(err) }()
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, module), tenancy.ReasonReEvaluation)
	if err != nil {
		return 0, err
	}
	var req *write.RegradeRequest
	var affected []write.AffectedAttempt
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		if req, err = h.Store.ClaimQueued(ctx, h.now()); err != nil || req == nil {
			return err
		}
		affected, err = h.Store.AffectedAttempts(ctx, *req)
		return err
	}); err != nil || req == nil {
		return 0, err
	}
	tenants := map[uuid.UUID]int{}
	var failure error
	for _, a := range affected {
		changed, err := h.reEvaluate(ctx, *req, a)
		if err != nil {
			failure = err
			break
		}
		if changed {
			n++
			tenants[a.TenantID]++
		}
	}
	for t, count := range tenants {
		if err := h.auditTenant(ctx, *req, t, count); err != nil && failure == nil {
			failure = err
		}
	}
	status, last := RegradeCompleted, ""
	if failure != nil {
		status, last = RegradeFailed, failure.Error()
	}
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		return h.Store.FinishRequest(ctx, req.ID, status, len(affected), n, last, h.now())
	}); err != nil {
		return n, err
	}
	return n, failure
}

// auditTenant records content.re_evaluated in the audit of a tenant whose attempts were re-evaluated.
func (h Regrades) auditTenant(ctx context.Context, r write.RegradeRequest, tenant uuid.UUID, count int) error {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		return h.Audit.Record(ctx, audit.Event{Action: ActionContentReEvaluated, TargetType: r.Scope, TargetID: r.Target().String(),
			Result: audit.ResultSuccess, Reason: truncate(r.Reason, 500), Metadata: map[string]any{"regrade_request_id": r.ID.String(),
				"action": string(r.Action), "re_evaluated_attempts": count, "requested_by": r.RequestedBy.String()}})
	})
}

// reKeys namespaces the idempotency keys of attempt.re_evaluated learning events (UUIDv5 of attempt × request).
var reKeys = uuid.MustParse("0193a1f0-31f0-7031-8031-000000000032")

// reEvaluate re-evaluates one attempt in the scope of its tenant: new evaluation source re_evaluation (effective), old
// one superseded, attempt and answers unchanged, learning event attempt.re_evaluated (actor staff) + outbox.
func (h Regrades) reEvaluate(ctx context.Context, r write.RegradeRequest, ref write.AffectedAttempt) (changed bool, err error) {
	sc, err := authz.EventScope(ref.TenantID)
	if err != nil {
		return false, err
	}
	err = h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		a, err := h.Attempts.Store.Attempt(ctx, ref.ID)
		if err != nil {
			return err
		}
		prev, err := h.Store.Effective(ctx, a.ID)
		if err != nil || prev == nil {
			return err
		}
		if err := (evaluation.Evaluation{ID: prev.ID, Status: prev.Status, Effective: true}).CheckEditable(); err != nil {
			return problem.New(problem.EvaluationNotEditable, "")
		}
		items := prev.Items
		if len(items) == 0 {
			for _, it := range a.ItemResults() {
				items = append(items, evaluation.Item{Position: it.Position, MaxScore: it.MaxScore, Score: it.Score, IsCorrect: it.IsCorrect,
					Answered: it.Answered})
			}
		}
		affected := map[int]bool{}
		var questions []uuid.UUID
		for _, it := range a.Items {
			if r.ActivityVersionID != nil || (r.QuestionVersionID != nil && it.QuestionVersionID == *r.QuestionVersionID) {
				affected[it.Position] = true
				questions = append(questions, it.QuestionVersionID)
			}
		}
		if len(affected) == 0 {
			return nil
		}
		regraded := map[int]evaluation.Graded{}
		if r.Action == evaluation.UseNewKey {
			for _, it := range a.Items {
				if !affected[it.Position] {
					continue
				}
				resp, ok, err := h.Store.LastScoredResponse(ctx, a.ID, it.Position)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				var pres question.Presentation
				_ = json.Unmarshal(it.Presentation, &pres)
				g, err := h.Attempts.Delivery.Grade(ctx, *r.CorrectedVersionID, pres, resp)
				var pe *problem.Error
				if err != nil && (!errors.As(err, &pe) || pe.Code != problem.InvalidResponse) {
					return err
				}
				regraded[it.Position] = evaluation.Graded{Score: g.Score, IsCorrect: err == nil && g.IsCorrect}
			}
		}
		next, err := evaluation.Regrade(items, affected, r.Action, regraded)
		if err != nil {
			return err
		}
		plugin, v, _, err := h.Attempts.versionOf(ctx, a)
		if err != nil {
			return err
		}
		var results []activity.ItemResult
		for _, it := range next {
			if !it.Dropped {
				results = append(results, activity.ItemResult{Position: it.Position, MaxScore: it.MaxScore, Score: it.Score,
					IsCorrect: it.IsCorrect, Answered: it.Answered})
			}
		}
		res := plugin.Evaluate(v, results)
		if err := h.Store.Supersede(ctx, prev.ID); err != nil {
			return err
		}
		newID := ids.New()
		details, _ := json.Marshal(map[string]any{"items": next, "strategies": res.Strategies, "regrade_request_id": r.ID,
			"action": string(r.Action)})
		reason := truncate(r.Reason, 500)
		rid, sup, by := r.ID, prev.ID, r.RequestedBy
		if err := h.Attempts.Store.InsertEvaluation(ctx, write.EvaluationRow{ID: newID, AttemptID: a.ID, Source: evaluation.SourceReEvaluation,
			Status: evaluation.StatusFinal, Score: res.Score, MaxScore: res.MaxScore, Passed: res.Passed, Details: details, Effective: true,
			RegradeRequestID: &rid, SupersedesID: &sup, TriggeredBy: &by, Reason: &reason}); err != nil {
			return err
		}
		at := h.now()
		cv := a.SourceVersionID
		if err := h.Attempts.Learning.Write(ctx, write.LearningEvent{ID: ids.New(), OccurredAt: at, Type: EventAttemptReEvaluated,
			StudentID: a.StudentID, TenantID: a.TenantID, SchoolID: a.SchoolID, ActivityID: a.ActivityID, ContentID: a.ActivityID,
			ContentVersionID: &cv, ObjectiveIDs: a.ObjectiveIDs, Context: string(a.Mode), ActorType: "staff", ActorID: &by,
			IdempotencyKey: "rev:" + uuid.NewSHA1(reKeys, []byte(a.ID.String()+":"+r.ID.String())).String(),
			Payload: map[string]any{"attempt_id": a.ID, "previous_evaluation_id": prev.ID, "new_evaluation_id": newID,
				"previous_score": learningScore(prev.Score, prev.MaxScore), "new_score": learningScore(res.Score, res.MaxScore),
				"question_version_ids": uniqueUUIDs(questions), "reason": r.Reason,
				"trigger": map[string]any{"role_code": r.RequestedRole, "re_evaluation_id": r.ID}}}); err != nil {
			return err
		}
		t := a.TenantID
		var passed any
		if res.Passed != nil {
			passed = *res.Passed
		}
		if err := h.Attempts.Events.Publish(ctx, write.Event{Type: EventAttemptReEvaluated, SubjectType: "attempt", SubjectID: a.ID.String(),
			TenantID: &t, Payload: map[string]any{"attempt_id": a.ID, "student_id": a.StudentID, "previous_evaluation_id": prev.ID,
				"new_evaluation_id": newID, "previous_score": prev.Score, "new_score": res.Score, "max_score": res.MaxScore, "passed": passed,
				"action": string(r.Action), "regrade_request_id": r.ID, "question_version_ids": uniqueUUIDs(questions),
				"objective_ids": nonNilUUIDs(uniqueUUIDs(a.ObjectiveIDs)), "reason": r.Reason,
				"trigger": map[string]any{"role_code": r.RequestedRole, "re_evaluation_id": r.ID}}}); err != nil {
			return err
		}
		if h.Metrics != nil {
			h.Metrics.ReEvaluated(string(r.Action))
		}
		changed = true
		return nil
	})
	return changed, err
}

func uniqueUUIDs(in []uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, id := range in {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
