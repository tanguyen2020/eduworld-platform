package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// StartActivityInput is ActivityAttemptStart of startActivityAttempt.
type StartActivityInput struct {
	ActivityID       uuid.UUID
	Mode             string
	ContextType      string
	ContextID        *uuid.UUID
	RecommendationID *uuid.UUID
	AcceptLanguage   string
}

// publicStartModes are the modes a student starts from an activity in F030 (assignment F041, retry_wrong and
// diagnostic F032; quest and challenge through the facade of module game).
var publicStartModes = []attempt.Mode{attempt.ModePractice, attempt.ModeMinigame}

var practiceContexts = []string{"free_play", "lesson", "learning_path", "recommendation"}

func (h Attempts) studentSchool(ctx context.Context, p authz.Principal) (uuid.UUID, *int, error) {
	school, grade, err := h.Students.StudentSchool(ctx, p.UserID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if school == nil {
		// Not in the projection of the tenant (ref.students, F020): no learning data can be attributed [Proposed].
		return uuid.Nil, nil, problem.New(problem.PermissionDenied, "the student is not enrolled in this tenant")
	}
	return *school, grade, nil
}

func (h Attempts) checkStartRate(ctx context.Context, studentID uuid.UUID) error {
	rate := h.StartRate
	if rate <= 0 {
		rate = AttemptStartRate
	}
	n, err := h.Store.StartedSince(ctx, studentID, h.now().Add(-time.Hour))
	if err != nil {
		return err
	}
	if n >= rate {
		return problem.New(problem.RateLimited, "attempt_start_rate reached").Retry()
	}
	return nil
}

// replayed returns the attempt a student already started with this Idempotency-Key, or the open practice attempt
// on the same version (F030-AC-04, AC-05), as the response of the start.
func (h Attempts) resumeView(ctx context.Context, a attempt.Attempt) (dto.AttemptView, error) {
	_, _, src, err := h.versionOf(ctx, a)
	if err != nil {
		return dto.AttemptView{}, err
	}
	return h.view(ctx, a, nil, src, nil, nil)
}

// StartActivity starts (or resumes) an attempt on the current release of an activity (F030-AC-01…AC-06): the version
// and every item are pinned at start, the client gets render payloads without answers.
func (h Attempts) StartActivity(ctx context.Context, in StartActivityInput) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StartActivityAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	mode := attempt.Mode(in.Mode)
	if mode == "" {
		mode = attempt.ModePractice
	}
	if !contains(publicStartModes, mode) {
		return out, problem.Invalid("/mode", problem.SubInvalid, "this mode is not available from an activity in this release")
	}
	ctxType, ctxID := in.ContextType, in.ActivityID
	if ctxType == "" {
		ctxType = "free_play"
	}
	if !contains(practiceContexts, ctxType) {
		return out, problem.Invalid("/context_type", problem.SubInvalid, "this context is not available in this release")
	}
	if ctxType != "free_play" {
		if in.ContextID == nil {
			return out, problem.Invalid("/context_id", problem.SubRequired, "is required with context_type "+ctxType)
		}
		ctxID = *in.ContextID
	}
	if mode == attempt.ModeMinigame {
		ctxType = "minigame_session"
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out, err = h.resumeView(ctx, *prev)
			}
			return err
		}
		school, _, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		src, err := LoadActivityVersion(ctx, h.Docs, h.Registry, in.ActivityID, nil)
		if err != nil {
			return err
		}
		rec, err := h.followedRecommendation(ctx, p, in.RecommendationID)
		if err != nil {
			return err
		}
		if ctxType == "lesson" && h.Lessons != nil {
			if err := h.Lessons.Pin(ctx, PinInput{StudentID: p.UserID, TenantID: *p.TenantID, SchoolID: school, LessonID: ctxID,
				ActivityID: in.ActivityID, Mode: string(mode), RecommendationID: recommendationID(rec)}); err != nil {
				return err
			}
		}
		now := h.now()
		if mode == attempt.ModePractice {
			open, err := h.Store.OpenPractice(ctx, p.UserID, src.Version.ID)
			if err != nil {
				return err
			}
			if open != nil && open.CheckActive(now, h.policy()) == nil && !open.Abandon(now, h.policy()) {
				out, err = h.view(ctx, *open, nil, &src, nil, nil)
				if err != nil {
					return err
				}
				return idempotency.SetResult(ctx, out)
			}
			if open != nil {
				return problem.New(problem.Conflict, "the previous attempt is being closed; retry shortly").Retry()
			}
		}
		if err := h.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		a, rendered, err := h.newActivityAttempt(ctx, p, school, src, mode, ctxType, ctxID, recommendationID(rec), in.AcceptLanguage, key, now)
		if err != nil {
			return err
		}
		if err := h.startRecommendation(ctx, rec, a.ID); err != nil {
			return err
		}
		if out, err = h.view(ctx, a, rendered, &src, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

func contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// displayLanguage is the language of the attempt (ADR-0056): the language asked when the activity has it, otherwise
// its content language; fixed for the whole attempt.
func displayLanguage(acceptLanguage string, title json.RawMessage, contentLanguage string) string {
	want := i18ntext.Locale(acceptLanguage)
	_, served := i18ntext.Resolve(title, want, contentLanguage)
	return served
}

// newActivityAttempt pins the items of an activity version and inserts the attempt with its activity.started events.
func (h Attempts) newActivityAttempt(ctx context.Context, p authz.Principal, school uuid.UUID, src ActivitySource, mode attempt.Mode, ctxType string,
	ctxID uuid.UUID, recommendation *uuid.UUID, acceptLanguage, key string, now time.Time) (attempt.Attempt, []question.Rendered, error) {
	seed := randomSeed()
	allowed := src.Plugin.AllowedQuestionTypes(src.Config)
	qids, err := h.sourceQuestions(ctx, src.Config.Content.QuestionSource, src.Version.SubjectID, src.Version.GradeLevels, allowed, seed)
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	items, rendered, err := h.pinItems(ctx, qids, seed, activity.Bool(src.Config.Interaction.ShuffleItems, true), allowed)
	if err != nil {
		return attempt.Attempt{}, nil, err
	}
	activityID := src.Doc.ID
	a := attempt.New(attempt.Start{ID: ids.New(), TenantID: *p.TenantID, SchoolID: school, StudentID: p.UserID, Mode: mode, ContextType: ctxType,
		ContextID: ctxID, ActivityID: &activityID, ActivityType: src.Body.ActivityType, SourceKind: "activity", SourceVersionID: src.Version.ID,
		Seed: seed, Now: now, TimeLimitSeconds: src.TimeLimit(), ActivityFeedback: src.Config.Feedback.Timing,
		DisplayLanguage: displayLanguage(acceptLanguage, src.Version.Title, src.Version.ContentLanguage), RecommendationID: recommendation,
		IdempotencyKey: key, Items: items})
	if err := h.Store.InsertAttempt(ctx, a); err != nil {
		return a, nil, err
	}
	return a, rendered, h.started(ctx, a)
}

// RespondInput is ActivityResponseSubmission (kind question).
type RespondInput struct {
	AttemptID       uuid.UUID
	Position        int
	Response        json.RawMessage
	ClientElapsedMS *int64
}

// Respond grades one response on the server (F030-AC-08…AC-11): answer key of the pinned version, server times,
// append-only answer and learning event question.answered in one transaction; the result follows FeedbackTiming.
// Score, correctness and durations sent by the client are never read (ADR-0016).
func (h Attempts) Respond(ctx context.Context, in RespondInput) (out dto.ResponseResultView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SubmitActivityResponse")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if in.Position < 1 {
		return out, problem.Invalid("/position", problem.SubRequired, "is required for a question response")
	}
	if len(in.Response) == 0 || string(in.Response) == "null" {
		return out, problem.New(problem.InvalidResponse, "the response is missing")
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, in.AttemptID)
		if err != nil {
			return err
		}
		_, grade, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		_, v, _, err := h.versionOf(ctx, a)
		if err != nil {
			return err
		}
		pol := h.policy()
		now := h.now()
		ans, err := a.Respond(in.Position, now, pol, pol.AnswerLimit(grade), activity.Bool(v.Config.Interaction.AllowUnscoredRetry, true), in.ClientElapsedMS)
		if err != nil {
			return attemptProblem(err)
		}
		it, _ := a.Item(in.Position)
		var pres question.Presentation
		_ = json.Unmarshal(it.Presentation, &pres)
		g, err := h.Delivery.Grade(ctx, it.QuestionVersionID, pres, in.Response)
		if err != nil {
			return err
		}
		score := min(max(g.Score, 0), it.MaxScore)
		// F032: min_response_ms and the review metadata come from the pinned version (rapid_guess, review list).
		var qv dto.QuestionVersion
		if h.Self != nil {
			if qv, err = h.Questions.QuestionVersionByID(ctx, it.QuestionVersionID); err != nil {
				return err
			}
		}
		rapid := h.Self.rapidGuess(a, *it, qv, ans, grade)
		a.Record(ans, g.IsCorrect, score)
		it, _ = a.Item(in.Position)
		if err := h.Store.UpdateItem(ctx, *it); err != nil {
			return err
		}
		if err := h.Store.UpdateAttempt(ctx, a, a.RowVersion); err != nil {
			return err
		}
		if err := h.Store.InsertAnswer(ctx, write.AnswerRow{ID: ids.New(), TenantID: a.TenantID, SchoolID: a.SchoolID, AttemptID: a.ID,
			Position: in.Position, QuestionVersionID: it.QuestionVersionID, Response: in.Response, IsCorrect: g.IsCorrect, Score: score,
			AnsweredAt: now, ClientElapsedMS: in.ClientElapsedMS, ResponseMS: ans.ResponseMS, AttemptNo: ans.AttemptNo, IsScored: ans.IsScored,
			IdempotencyKey: key, HintsUsed: ans.HintsUsed, RapidGuess: rapid}); err != nil {
			return err
		}
		if err := h.Self.recordAnswer(ctx, a, *it, qv, ans, g.IsCorrect, rapid, now); err != nil {
			return err
		}
		if err := h.learningEvent(ctx, a, EventQuestionAnswered, "qa:"+a.ID.String()+":"+itoa(in.Position)+":"+itoa(ans.AttemptNo), now,
			it.ObjectiveIDs, "student", map[string]any{"attempt_id": a.ID, "position": in.Position, "question_version_id": it.QuestionVersionID,
				"question_type": it.QuestionType, "is_correct": g.IsCorrect, "is_scored": ans.IsScored, "score": learningScore(score, it.MaxScore), "response_ms": ans.ResponseMS,
				"attempt_no": ans.AttemptNo, "hints_used": ans.HintsUsed, "rapid_guess": rapid}); err != nil {
			return err
		}
		// rapid_guess is never returned to the student (F032-AC-20, R19).
		out = dto.ResponseResultView{Accepted: true, Position: in.Position, HintsUsed: ans.HintsUsed}
		if a.ShowsItemFeedback() {
			c, s := g.IsCorrect, score
			out.IsCorrect, out.Score = &c, &s
			if activity.Bool(v.Config.Feedback.ShowExplanation, true) {
				exp, err := h.Delivery.Explanation(ctx, it.QuestionVersionID)
				if err != nil {
					return err
				}
				out.Explanation = attemptview.Truncate(attemptview.RichText(exp, a.DisplayLanguage), 4000)
			}
			fb, err := h.optionFeedback(ctx, it.QuestionVersionID, g.SelectedOptionIDs, a.DisplayLanguage)
			if err != nil {
				return err
			}
			out.Feedback = fb
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// optionFeedback is the feedback of the (first) option chosen, when the question has one (ACTIVITY_ENGINE §8.2).
func (h Attempts) optionFeedback(ctx context.Context, versionID uuid.UUID, selected []string, lang string) (*dto.AnswerFeedbackView, error) {
	if len(selected) == 0 {
		return nil, nil
	}
	fbs, err := h.Delivery.OptionFeedback(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return attemptview.OptionFeedback(fbs, selected[0], lang), nil
}

// Pause sets paused_at of an in-progress attempt (F030-AC-17).
func (h Attempts) Pause(ctx context.Context, id uuid.UUID) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.PauseActivityAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, id)
		if err != nil {
			return err
		}
		row := a.RowVersion
		if err := a.Pause(h.now(), h.policy()); err != nil {
			return attemptProblem(err)
		}
		if err := h.Store.UpdateAttempt(ctx, a, row); err != nil {
			return err
		}
		_, _, src, err := h.versionOf(ctx, a)
		if err != nil {
			return err
		}
		if out, err = h.view(ctx, a, nil, src, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// Submit submits an attempt once (F030-AC-12, AC-13): conditional transition on the row version, score of the plugin
// from the scored answers, duration on the server clock, effective evaluation, learning events and outbox
// activity.submitted + activity.graded in one transaction. Another key after the submit is 409
// ATTEMPT_ALREADY_SUBMITTED; the same key replays the stored answer (pkg/httpx).
func (h Attempts) Submit(ctx context.Context, id uuid.UUID) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.SubmitActivityAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		a, err := h.loadOwn(ctx, p, id)
		if err != nil {
			return err
		}
		if a.Status == attempt.StatusSubmitted || a.Status == attempt.StatusGraded {
			return problem.New(problem.AttemptAlreadySubmitted, "")
		}
		ev, src, err := h.gradeAttempt(ctx, &a, attempt.ReasonManual)
		if err != nil {
			return err
		}
		if out, err = h.view(ctx, a, nil, src, nil, ev); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// gradeAttempt submits and grades an attempt in the transaction of ctx (manual submit or timeout of the worker).
func (h Attempts) gradeAttempt(ctx context.Context, a *attempt.Attempt, reason string) (*dto.EvaluationView, *ActivitySource, error) {
	plugin, v, src, err := h.versionOf(ctx, *a)
	if err != nil {
		return nil, nil, err
	}
	row := a.RowVersion
	res := plugin.Evaluate(v, a.ItemResults())
	if err := a.Submit(h.now(), h.policy(), reason, res); err != nil {
		return nil, nil, attemptProblem(err)
	}
	if err := h.Store.UpdateAttempt(ctx, *a, row); err != nil {
		if pe, ok := problemOf(err); ok && pe.Code == problem.VersionMismatch {
			return nil, nil, problem.New(problem.AttemptAlreadySubmitted, "")
		}
		return nil, nil, err
	}
	evID := ids.New()
	details, _ := json.Marshal(map[string]any{"items": a.ItemResults(), "strategies": res.Strategies})
	if err := h.Store.InsertEvaluation(ctx, write.EvaluationRow{ID: evID, AttemptID: a.ID, Source: "system", Status: "final", Score: res.Score,
		MaxScore: res.MaxScore, Passed: res.Passed, Details: details, Effective: true}); err != nil {
		return nil, nil, err
	}
	if err := h.graded(ctx, *a, res, evID); err != nil {
		return nil, nil, err
	}
	return &dto.EvaluationView{EvaluationID: evID, AttemptID: a.ID, Source: "system", Status: "final", Score: res.Score, MaxScore: res.MaxScore,
		Passed: res.Passed, Effective: true, CreatedAt: *a.GradedAt}, src, nil
}
