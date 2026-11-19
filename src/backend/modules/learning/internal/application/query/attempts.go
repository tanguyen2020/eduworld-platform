package query

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Attempts are the read use cases of the attempt engine for the student who owns the attempts (F030 US1–US3): resume,
// history, review, and the published activity. They read the pinned versions only and never grade (R31, ADR-0087).
type Attempts struct {
	Session  read.Session
	Model    read.AttemptReadModel
	Docs     read.DocReadModel
	Delivery Delivery
	Registry *activity.Registry
	Cursors  *page.Codec
	Clock    func() time.Time
}

func (q Attempts) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func student(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return p, problem.New(problem.GameAccessStudentOnly, "")
	}
	return p, nil
}

func (q Attempts) source(ctx context.Context, activityID, versionID uuid.UUID) (attemptview.Source, error) {
	d, v, err := q.Model.ActivityVersion(ctx, activityID, versionID)
	if err != nil {
		return attemptview.Source{}, err
	}
	src := attemptview.Source{Doc: d, Version: v, Body: attemptview.ParseBody(v.Body)}
	p, err := q.Registry.Resolve(src.Body.ActivityType)
	if err != nil {
		return src, problem.New(problem.ActivityTypeNotSupported, "")
	}
	src.Plugin = p
	src.Config, _ = activity.ParseConfig(src.Body.Config)
	return src, nil
}

// loaded is an attempt with what its views need, read in one session.
type loaded struct {
	a       attempt.Attempt
	answers []read.SavedAnswer
	ev      *dto.EvaluationView
	src     *attemptview.Source
}

func (q Attempts) load(ctx context.Context, id uuid.UUID) (out loaded, err error) {
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		a, err := q.Model.Attempt(ctx, id)
		if err != nil {
			return err
		}
		if a.StudentID != p.UserID {
			return problem.New(problem.AttemptNotFound, "")
		}
		out.a = a
		if out.answers, err = q.Model.LastAnswers(ctx, id); err != nil {
			return err
		}
		if out.ev, err = q.Model.Evaluation(ctx, id); err != nil {
			return err
		}
		if a.ActivityID != nil {
			src, err := q.source(ctx, *a.ActivityID, a.SourceVersionID)
			if err != nil {
				return err
			}
			out.src = &src
		}
		return nil
	})
	return out, err
}

func (q Attempts) rendered(ctx context.Context, a attempt.Attempt) ([]question.Rendered, error) {
	ids := make([]uuid.UUID, len(a.Items))
	for i, it := range a.Items {
		ids[i] = it.QuestionVersionID
	}
	if len(ids) == 0 {
		return nil, nil
	}
	r, _, err := q.Delivery.Render(ctx, ids, uint64(a.Seed))
	return r, err
}

// Get returns an attempt of the student for resume (F030-AC-16): items rendered on the pinned versions with the same
// presentation, the saved responses, the deadline and server time, the effective evaluation once graded. An abandoned
// attempt is 410 ATTEMPT_EXPIRED (F030-AC-18).
func (q Attempts) Get(ctx context.Context, id uuid.UUID) (out dto.AttemptView, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetActivityAttempt")
	defer func() { done(err) }()
	l, err := q.load(ctx, id)
	if err != nil {
		return out, "", err
	}
	if l.a.Status == attempt.StatusAbandoned {
		return out, "", problem.New(problem.AttemptExpired, "")
	}
	rendered, err := q.rendered(ctx, l.a)
	if err != nil {
		return out, "", err
	}
	saved := make([]dto.SavedResponseView, 0, len(l.answers))
	hints := map[int]int{}
	for _, it := range l.a.Items {
		hints[it.Position] = it.HintsRevealed
	}
	for _, s := range l.answers {
		saved = append(saved, dto.SavedResponseView{Position: s.Position, Response: s.Response, HintsRevealed: hints[s.Position]})
	}
	var render *dto.ActivityRenderView
	if l.src != nil {
		r := attemptview.Render(*l.src, q.Registry, l.a.DisplayLanguage)
		render = &r
	}
	return attemptview.Attempt(l.a, attemptview.Items(l.a.Items, rendered), saved, render, l.ev, q.now()), l.a.DisplayLanguage, nil
}

// ListMineInput filters listMyActivityAttempts.
type ListMineInput struct {
	Mode, Status, ActivityType string
	Page                       PageInput
}

// ListMine lists the attempts of the student only, newest first (F030-AC-07).
func (q Attempts) ListMine(ctx context.Context, in ListMineInput) (out Page[dto.AttemptSummaryView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListMyActivityAttempts")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Page.Limit)
	cur, err := q.Cursors.Decode(in.Page.Cursor)
	if err != nil {
		return out, problem.New(problem.BadRequest, "invalid cursor")
	}
	f := read.AttemptFilter{Mode: in.Mode, Status: in.Status, ActivityType: in.ActivityType, Limit: limit + 1}
	if cur != nil {
		id := cur.ID
		f.After = &id
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		as, err := q.Model.List(ctx, p.UserID, f)
		if err != nil {
			return err
		}
		out.Limit = limit
		if len(as) > limit {
			as, out.HasMore = as[:limit], true
		}
		for _, a := range as {
			s := dto.AttemptSummaryView{AttemptID: a.ID, ActivityID: a.ActivityID, ActivityType: a.ActivityType, Mode: string(a.Mode),
				Status: string(a.Status), StartedAt: a.StartedAt, SubmittedAt: a.SubmittedAt, Score: a.Score, MaxScore: a.MaxScore}
			if a.ActivityID != nil {
				if _, v, err := q.Model.ActivityVersion(ctx, *a.ActivityID, a.SourceVersionID); err == nil {
					t, _ := i18ntext.Resolve(v.Title, a.DisplayLanguage, v.ContentLanguage)
					s.Title = attemptview.Truncate(plainTitle(t), 255)
				}
			}
			out.Data = append(out.Data, s)
		}
		if out.HasMore && len(as) > 0 {
			next := q.Cursors.Encode(page.Cursor{ID: as[len(as)-1].ID})
			out.NextCursor = &next
		}
		return nil
	})
	if out.Data == nil {
		out.Data = []dto.AttemptSummaryView{}
	}
	return out, err
}

func plainTitle(raw []byte) string {
	s := string(raw)
	if len(s) >= 2 && s[0] == '"' {
		return attemptview.RichText(raw, "vi")
	}
	return ""
}

// Review returns the graded attempt item by item (F030-AC-15): correctness, score, explanation and the feedback of
// the chosen option, when FeedbackTiming allows it; before grading 409 CONFLICT.
func (q Attempts) Review(ctx context.Context, id uuid.UUID) (out dto.ReviewView, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetActivityAttemptReview")
	defer func() { done(err) }()
	l, err := q.load(ctx, id)
	if err != nil {
		return out, "", err
	}
	switch {
	case l.a.Status == attempt.StatusAbandoned:
		return out, "", problem.New(problem.AttemptExpired, "")
	case l.a.Status != attempt.StatusGraded:
		return out, "", problem.New(problem.Conflict, "the attempt is not graded yet")
	case l.a.Feedback == attempt.FeedbackNever || l.a.Feedback == attempt.FeedbackAfterClose:
		return out, "", problem.New(problem.PermissionDenied, "the review of this attempt is not released")
	}
	rendered, err := q.rendered(ctx, l.a)
	if err != nil {
		return out, "", err
	}
	byID := map[uuid.UUID]question.Rendered{}
	for _, r := range rendered {
		byID[r.QuestionVersionID] = r
	}
	answers := map[int]read.SavedAnswer{}
	for _, s := range l.answers {
		answers[s.Position] = s
	}
	showExplanation := l.src == nil || activity.Bool(l.src.Config.Feedback.ShowExplanation, true)
	out = dto.ReviewView{AttemptID: l.a.ID, Score: l.a.Score, MaxScore: l.a.MaxScore, WeakObjectives: []uuid.UUID{}}
	weak := map[uuid.UUID]bool{}
	for _, it := range l.a.Items {
		item := dto.ReviewItemView{Position: it.Position, Question: attemptview.Item(it, byID[it.QuestionVersionID]), IsCorrect: it.FinalIsCorrect,
			Score: it.FinalScore, ObjectiveIDs: it.ObjectiveIDs}
		if item.ObjectiveIDs == nil {
			item.ObjectiveIDs = []uuid.UUID{}
		}
		if s, ok := answers[it.Position]; ok {
			item.Response = s.Response
			if opt := attemptview.SelectedOption(s.Response); opt != "" {
				fbs, err := q.Delivery.OptionFeedback(ctx, it.QuestionVersionID)
				if err != nil {
					return out, "", err
				}
				item.Feedback = attemptview.OptionFeedback(fbs, opt, l.a.DisplayLanguage)
			}
		}
		if showExplanation {
			exp, err := q.Delivery.Explanation(ctx, it.QuestionVersionID)
			if err != nil {
				return out, "", err
			}
			if len(exp) > 0 {
				item.Explanation, _ = i18ntext.Resolve(exp, l.a.DisplayLanguage, l.a.DisplayLanguage)
			}
		}
		if it.FinalIsCorrect == nil || !*it.FinalIsCorrect {
			for _, o := range it.ObjectiveIDs {
				if !weak[o] {
					weak[o] = true
					out.WeakObjectives = append(out.WeakObjectives, o)
				}
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, l.a.DisplayLanguage, nil
}

// Activity returns the published activity for a student (getLearningActivity): render payload of the current
// release in the display language, no answer (F030-AC-02); unpublished 409 CONTENT_NOT_PUBLISHED, invisible 404.
func (q Attempts) Activity(ctx context.Context, id uuid.UUID, acceptLanguage string) (out dto.ActivityRenderView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLearningActivity")
	defer func() { done(err) }()
	if _, err := student(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		d, err := q.Docs.Doc(ctx, "activity", id)
		if err != nil {
			return err
		}
		if d.CurrentVersionID == nil {
			return problem.New(problem.ContentNotPublished, "the activity is not published")
		}
		src, err := q.source(ctx, id, *d.CurrentVersionID)
		if err != nil {
			return err
		}
		out = attemptview.Render(src, q.Registry, i18ntext.Locale(acceptLanguage))
		return nil
	})
	return out, err
}

// AttemptResult is AttemptResult of the facade operation getAttemptResult: status and graded result, never an answer.
type AttemptResult struct {
	AttemptID        uuid.UUID
	StudentID        uuid.UUID
	Status           string
	Score, MaxScore  *float64
	CorrectCount     *int
	QuestionCount    int
	SubmittedAt      *time.Time
	GradedAt         *time.Time
	ContentVersionID uuid.UUID
	ObjectiveIDs     []uuid.UUID
}

// Result returns the result of an attempt in the TenantScope of ctx (F030-AC-26: another tenant is 404) for module game.
func (q Attempts) Result(ctx context.Context, id uuid.UUID) (out AttemptResult, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.GetAttemptResult")
	defer func() { done(err) }()
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		a, err := q.Model.Attempt(ctx, id)
		if err != nil {
			return err
		}
		out = AttemptResult{AttemptID: a.ID, StudentID: a.StudentID, Status: string(a.Status), Score: a.Score, MaxScore: a.MaxScore,
			QuestionCount: len(a.Items), SubmittedAt: a.SubmittedAt, GradedAt: a.GradedAt, ContentVersionID: a.SourceVersionID,
			ObjectiveIDs: a.ObjectiveIDs}
		if a.Status == attempt.StatusGraded {
			n := 0
			for _, it := range a.Items {
				if it.FinalIsCorrect != nil && *it.FinalIsCorrect {
					n++
				}
			}
			out.CorrectCount = &n
		}
		return nil
	})
	return out, err
}
