package query

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/exam"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Exams holds the read use cases of the exam engine (F042): sessions for staff and students, the library, the papers
// preview, the result and the analysis of an attempt under the release policy of its session.
type Exams struct {
	Session    read.Session
	Model      read.ExamReadModel
	Attempts   Attempts // attempt reads and the delivery (render, explanation, option feedback, correct options)
	Cursors    *page.Codec
	Clock      func() time.Time
	ScoreScale float64
}

func (q Exams) now() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

func (q Exams) scale() float64 {
	if q.ScoreScale > 0 {
		return q.ScoreScale
	}
	return 10
}

// sessionView is the staff view (policies, accommodations).
func sessionView(s read.ExamSessionRow, now time.Time) dto.ExamSessionView {
	var acc []dto.ExamAccommodationView
	_ = json.Unmarshal(s.Accommodations, &acc)
	return dto.ExamSessionView{ExamSessionID: s.ID, Title: s.Title, Type: s.Type, IsOfficial: s.IsOfficial, SubjectID: s.SubjectID,
		GradeLevel: s.GradeLevel, WindowStart: s.WindowStart, WindowEnd: s.WindowEnd, DurationSeconds: s.DurationSec,
		QuestionCount: s.QuestionCount, MaxAttempts: s.MaxAttempts, Status: s.Status, BlueprintVersionID: s.BlueprintVersionID,
		FeedbackTiming: s.FeedbackTiming, ResultRelease: s.ResultRelease, ResultsReleasedAt: s.ReleasedAt, SchoolID: s.SchoolID,
		CourseID: s.CourseID, AssignmentID: s.AssignmentID, Audience: s.Audience, VariantStrategy: s.Strategy, VariantCount: s.VariantCount,
		LateStartPolicy: s.LateStartPolicy, ScorePolicy: s.ScorePolicy, ShowCorrectAnswers: s.ShowCorrect, ShowExplanations: s.ShowExplanations,
		Accommodations: acc, ServerTime: now, CreatedAt: s.CreatedAt, Version: s.Version, IsEfficacy: isEfficacy(s)}
}

// isEfficacy marks a session of an efficacy wave for the staff (F035-AC-17: read-only; never the wave, study nor arm).
func isEfficacy(s read.ExamSessionRow) *bool {
	if s.EfficacyWaveID == nil {
		return nil
	}
	t := true
	return &t
}

// studentView hides the accommodations of others and adds the student's attempts and effective duration.
func studentView(s read.ExamSessionRow, part *read.ExamParticipationRow, now time.Time) dto.ExamSessionView {
	v := sessionView(s, now)
	v.Accommodations = nil
	v.IsEfficacy = nil // F035-AC-17: a student never learns the session is an efficacy test
	if part != nil {
		used, d := part.AttemptsUsed, part.EffectiveDurationSec
		v.AttemptsUsed, v.EffectiveDurationSecond = &used, &d
	}
	return v
}

// ExamListInput are the filters of listExamSessions and listExamLibrary.
type ExamListInput struct {
	SubjectID  *uuid.UUID
	GradeLevel *int
	Type       string
}

// Sessions is listExamSessions: a student sees the sessions assigned to it; staff with lms.assignment.read the sessions
// of the schools and subjects of their grants (F042-AC-06, AC-17).
func (q Exams) Sessions(ctx context.Context, in ExamListInput, pg PageInput) (out Page[dto.ExamSessionView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListExamSessions")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	f := read.ExamSessionFilter{SubjectID: in.SubjectID, Type: in.Type}
	if p.UserType == authz.UserTypeStudent {
		if p.TenantID == nil {
			return out, problem.New(problem.TenantHeaderRequired, "")
		}
		f.StudentID = &p.UserID
	} else {
		if p, err = staffPrincipal(ctx); err != nil {
			return out, err
		}
		if !holds(p, authz.PermLmsAssignmentRead) {
			return out, authz.Require(ctx, p, authz.PermLmsAssignmentRead, authz.Resource{TenantID: p.TenantID})
		}
		f.Scopes = scopes(p, authz.PermLmsAssignmentRead)
	}
	return q.list(ctx, p, f, pg)
}

// Library is listExamLibrary (F042-AC-26): a student only; the open unofficial practice_test/mock sessions of the
// library whose audience holds the student, in its tenant.
func (q Exams) Library(ctx context.Context, in ExamListInput, pg PageInput) (out Page[dto.ExamSessionView], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListExamLibrary")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return out, problem.New(problem.PermissionDenied, "the exam library is for students")
	}
	if in.Type != "" && in.Type != string(exam.TypePracticeTest) && in.Type != string(exam.TypeMock) {
		return out, problem.Invalid("/type", problem.SubInvalid, "the library holds practice_test and mock")
	}
	now := q.now()
	return q.list(ctx, p, read.ExamSessionFilter{StudentID: &p.UserID, Library: true, OpenAt: &now, SubjectID: in.SubjectID,
		GradeLevel: in.GradeLevel, Type: in.Type}, pg)
}

func (q Exams) list(ctx context.Context, p authz.Principal, f read.ExamSessionFilter, pg PageInput) (out Page[dto.ExamSessionView], err error) {
	after, limit, err := keyset(pg, q.Cursors)
	if err != nil {
		return out, err
	}
	f.After, f.Limit = after, limit+1
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Model.Sessions(ctx, f)
		if err != nil {
			return err
		}
		now := q.now()
		views := make([]dto.ExamSessionView, 0, len(rows))
		for _, s := range rows {
			if f.StudentID == nil {
				views = append(views, sessionView(s, now))
				continue
			}
			part, err := q.Model.Participation(ctx, s.ID, *f.StudentID)
			if err != nil {
				return err
			}
			views = append(views, studentView(s, part, now))
		}
		out = pageOf(views, limit, func(v dto.ExamSessionView) uuid.UUID { return v.ExamSessionID }, q.Cursors)
		return nil
	})
	return out, err
}

// readable reads a session for staff with perm on its school and subject (404 outside the schools of the reader).
func (q Exams) readable(ctx context.Context, p authz.Principal, perm authz.Permission, id uuid.UUID) (*read.ExamSessionRow, error) {
	s, err := q.Model.Session(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	if err := authorizeSchool(ctx, p, perm, s.SchoolID, &s.SubjectID); err != nil {
		return nil, err
	}
	return s, nil
}

// Get is getExamSession: a student of the audience (404 otherwise), staff with lms.assignment.read in scope.
func (q Exams) Get(ctx context.Context, id uuid.UUID) (out dto.ExamSessionView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetExamSession")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if p.UserType == authz.UserTypeStudent {
			s, err := q.Model.Session(ctx, id)
			if err != nil {
				return err
			}
			if s == nil {
				return problem.New(problem.NotFound, "")
			}
			part, err := q.Model.Participation(ctx, id, p.UserID)
			if err != nil {
				return err
			}
			if part == nil {
				return problem.New(problem.NotFound, "")
			}
			out = studentView(*s, part, q.now())
			return nil
		}
		sp, err := staffPrincipal(ctx)
		if err != nil {
			return err
		}
		s, err := q.readable(ctx, sp, authz.PermLmsAssignmentRead, id)
		if err != nil {
			return err
		}
		out = sessionView(*s, q.now())
		return nil
	})
	return out, err
}

// Papers is listExamSessionPapers (F042-AC-04): the locked variants for the staff who create exams in scope
// (lms.assignment.write); a student is 403 PERMISSION_DENIED.
func (q Exams) Papers(ctx context.Context, id uuid.UUID) (out []dto.ExamPaperView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListExamSessionPapers")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if p.UserType == authz.UserTypeStudent {
		return out, problem.New(problem.PermissionDenied, "the papers are previewed by the staff who create exams")
	}
	if p, err = staffPrincipal(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		s, err := q.readable(ctx, p, authz.PermLmsAssignmentWrite, id)
		if err != nil {
			return err
		}
		if s.EfficacyWaveID != nil {
			// F035-AC-16: the papers of an efficacy session are never shown to tenant staff.
			return problem.New(problem.PermissionDenied, "an efficacy exam session is managed by the efficacy study")
		}
		titles := map[string]string{}
		for _, sec := range sessionSections(*s) {
			titles[sec.SectionID] = sec.Title
		}
		ps, err := q.Model.Papers(ctx, id)
		if err != nil {
			return err
		}
		out = make([]dto.ExamPaperView, 0, len(ps))
		for _, pp := range ps {
			v := dto.ExamPaperView{ExamPaperID: pp.ID, VariantNo: pp.VariantNo, Seed: strconv.FormatInt(pp.Seed, 10), ExpectedP: pp.ExpectedP, BalanceWarning: pp.BalanceWarning,
				TotalPoints: pp.TotalPoints, Sections: []dto.ExamPaperSectionView{}}
			for _, it := range pp.Items {
				n := len(v.Sections)
				if n == 0 || v.Sections[n-1].SectionID != it.SectionID {
					v.Sections = append(v.Sections, dto.ExamPaperSectionView{SectionID: it.SectionID, Title: titles[it.SectionID]})
					n++
				}
				v.Sections[n-1].Items = append(v.Sections[n-1].Items, dto.ExamPaperItemView{Position: it.Position,
					QuestionVersionID: it.QuestionVersionID, Points: it.Points})
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

type examSection struct {
	SectionID string `json:"section_id"`
	Title     string `json:"title"`
}

func sessionSections(s read.ExamSessionRow) []examSection {
	var out []examSection
	_ = json.Unmarshal(s.Sections, &out)
	return out
}

// examAttempt is an exam attempt with its session, read for its student (results released) or staff in scope.
type examAttempt struct {
	a       attempt.Attempt
	s       read.ExamSessionRow
	answers map[int]json.RawMessage
	paper   *read.ExamPaperRow
}

// loadResult reads a graded exam attempt (F042-AC-17, AC-20, AC-22): the student owner once the results are visible
// (403 EXAM_RESULTS_NOT_RELEASED before), staff with lms.assignment.read in the school and subject of the session;
// another student or tenant → 404; not graded yet → 409.
func (q Exams) loadResult(ctx context.Context, id uuid.UUID) (out examAttempt, err error) {
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		a, err := q.Attempts.Model.Attempt(ctx, id)
		if err != nil {
			var pe *problem.Error
			if errors.As(err, &pe) && pe.Code == problem.AttemptNotFound {
				return problem.New(problem.NotFound, "")
			}
			return err
		}
		if a.Mode != attempt.ModeExam {
			return problem.New(problem.NotFound, "")
		}
		s, err := q.Model.Session(ctx, a.ContextID)
		if err != nil {
			return err
		}
		if s == nil {
			return problem.New(problem.NotFound, "")
		}
		if p.UserType == authz.UserTypeStudent {
			if a.StudentID != p.UserID {
				return problem.New(problem.NotFound, "")
			}
			if a.Status != attempt.StatusGraded || !exam.ResultsVisible(s.ResultRelease, exam.Status(s.Status)) {
				return problem.New(problem.ExamResultsNotReleased, "")
			}
		} else {
			sp, err := staffPrincipal(ctx)
			if err != nil {
				return err
			}
			if _, err := q.readable(ctx, sp, authz.PermLmsAssignmentRead, s.ID); err != nil {
				return err
			}
			if a.Status != attempt.StatusGraded {
				return problem.New(problem.Conflict, "the attempt is not graded yet")
			}
		}
		out.a, out.s = a, *s
		// F042-AC-28: after a regrade the effective evaluation holds the score (the attempt keeps its original score).
		ev, err := q.Attempts.Model.Evaluation(ctx, id)
		if err != nil {
			return err
		}
		if ev != nil {
			sc, mx := ev.Score, ev.MaxScore
			out.a.Score, out.a.MaxScore = &sc, &mx
		}
		if out.answers, err = q.Model.LatestAnswers(ctx, id); err != nil {
			return err
		}
		paperID, err := q.Model.AttemptPaper(ctx, id)
		if err != nil || paperID == nil {
			return err
		}
		ps, err := q.Model.Papers(ctx, s.ID)
		if err != nil {
			return err
		}
		for i := range ps {
			if ps[i].ID == *paperID {
				out.paper = &ps[i]
			}
		}
		return nil
	})
	return out, err
}

// sectionScores sums the item scores per section of the paper (EXAM_ENGINE §10); a section without points is left out.
func (q Exams) sectionScores(l examAttempt) []dto.ExamSectionScoreView {
	section := map[uuid.UUID]string{}
	if l.paper != nil {
		for _, it := range l.paper.Items {
			section[it.QuestionVersionID] = it.SectionID
		}
	}
	titles := map[string]string{}
	var order []string
	for _, sec := range sessionSections(l.s) {
		titles[sec.SectionID] = sec.Title
		order = append(order, sec.SectionID)
	}
	sums := map[string]*dto.ExamSectionScoreView{}
	for _, it := range l.a.Items {
		id := section[it.QuestionVersionID]
		if id == "" {
			continue
		}
		v := sums[id]
		if v == nil {
			v = &dto.ExamSectionScoreView{SectionID: id, Title: titles[id]}
			sums[id] = v
		}
		v.MaxScore += it.MaxScore
		if it.FinalScore != nil {
			v.Score += *it.FinalScore
		}
	}
	out := []dto.ExamSectionScoreView{}
	for _, id := range order {
		if v := sums[id]; v != nil && v.MaxScore > 0 {
			out = append(out, *v)
		}
	}
	return out
}

// Result is getExamResult (F042-AC-20, AC-22): the score on score_scale, the section scores and the items with the
// response, correctness and, as the session shows them, explanation and option feedback. Never a ranking.
func (q Exams) Result(ctx context.Context, id uuid.UUID) (out dto.ReviewView, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetExamResult")
	defer func() { done(err) }()
	l, err := q.loadResult(ctx, id)
	if err != nil {
		return out, "", err
	}
	out = dto.ReviewView{AttemptID: l.a.ID, Score: l.a.Score, MaxScore: l.a.MaxScore, WeakObjectives: []uuid.UUID{}, Items: []dto.ReviewItemView{}}
	if l.a.Score != nil && l.a.MaxScore != nil {
		v := exam.ScaledScore(*l.a.Score, *l.a.MaxScore, q.scale())
		out.ScoreScaled = &v
	}
	out.SectionScores = q.sectionScores(l)
	rendered, err := q.Attempts.rendered(ctx, l.a)
	if err != nil {
		return out, "", err
	}
	byID := map[uuid.UUID]question.Rendered{}
	for _, r := range rendered {
		byID[r.QuestionVersionID] = r
	}
	weak := map[uuid.UUID]bool{}
	for _, it := range l.a.Items {
		item := dto.ReviewItemView{Position: it.Position, Question: attemptview.Item(it, byID[it.QuestionVersionID]), IsCorrect: it.FinalIsCorrect,
			Score: it.FinalScore, ObjectiveIDs: nonNil(it.ObjectiveIDs)}
		if r, ok := l.answers[it.Position]; ok {
			item.Response = r
			if fb, err := q.optionFeedback(ctx, l, it.QuestionVersionID, r); err != nil {
				return out, "", err
			} else {
				item.Feedback = fb
			}
		}
		if exp, err := q.explanation(ctx, l, it.QuestionVersionID); err != nil {
			return out, "", err
		} else {
			item.Explanation = exp
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

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// explanation is the explanation of an item when the session shows explanations and its feedback timing allows it.
func (q Exams) explanation(ctx context.Context, l examAttempt, versionID uuid.UUID) (json.RawMessage, error) {
	if !l.s.ShowExplanations || l.s.FeedbackTiming == exam.FeedbackNever {
		return nil, nil
	}
	exp, err := q.Attempts.Delivery.Explanation(ctx, versionID)
	if err != nil || len(exp) == 0 {
		return nil, err
	}
	node, _ := i18ntext.Resolve(exp, l.a.DisplayLanguage, l.a.DisplayLanguage)
	if len(node) > 16000 {
		return nil, nil
	}
	return node, nil
}

// optionFeedback is the feedback of the chosen option when the session shows the correct answers.
func (q Exams) optionFeedback(ctx context.Context, l examAttempt, versionID uuid.UUID, resp json.RawMessage) (*dto.AnswerFeedbackView, error) {
	if !l.s.ShowCorrect || l.s.FeedbackTiming == exam.FeedbackNever {
		return nil, nil
	}
	opt := attemptview.SelectedOption(resp)
	if opt == "" {
		return nil, nil
	}
	fbs, err := q.Attempts.Delivery.OptionFeedback(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return attemptview.OptionFeedback(fbs, opt, l.a.DisplayLanguage), nil
}

// Analysis is getExamAttemptAnalysis (F042-AC-22, EXAM_ENGINE §10): correct/total per objective, the wrong items with
// the response, the correct options and explanation as the session shows them, and the 1–3 weakest objectives
// (correct ratio < 60 % first) to review. Never a ranking.
func (q Exams) Analysis(ctx context.Context, id uuid.UUID) (out dto.ExamAnalysisView, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetExamAttemptAnalysis")
	defer func() { done(err) }()
	l, err := q.loadResult(ctx, id)
	if err != nil {
		return out, "", err
	}
	out = dto.ExamAnalysisView{AttemptID: l.a.ID, ExamSessionID: l.s.ID, DurationMS: l.a.DurationMS, SectionScores: q.sectionScores(l),
		ObjectiveBreakdown: []dto.ExamObjectiveView{}, WrongItems: []dto.ExamWrongItemView{}, ReviewSuggestions: []dto.ExamSuggestionView{}}
	if l.a.Score != nil && l.a.MaxScore != nil {
		out.Score, out.MaxScore = *l.a.Score, *l.a.MaxScore
		out.ScoreScaled = exam.ScaledScore(out.Score, out.MaxScore, q.scale())
	}
	rendered, err := q.Attempts.rendered(ctx, l.a)
	if err != nil {
		return out, "", err
	}
	byID := map[uuid.UUID]question.Rendered{}
	for _, r := range rendered {
		byID[r.QuestionVersionID] = r
	}
	idx := map[uuid.UUID]int{}
	for _, it := range l.a.Items {
		correct := it.FinalIsCorrect != nil && *it.FinalIsCorrect
		for _, o := range it.ObjectiveIDs {
			i, ok := idx[o]
			if !ok {
				i = len(out.ObjectiveBreakdown)
				idx[o] = i
				out.ObjectiveBreakdown = append(out.ObjectiveBreakdown, dto.ExamObjectiveView{ObjectiveID: o})
			}
			out.ObjectiveBreakdown[i].Total++
			if correct {
				out.ObjectiveBreakdown[i].Correct++
			}
		}
		if correct {
			continue
		}
		q2 := attemptview.Item(it, byID[it.QuestionVersionID])
		w := dto.ExamWrongItemView{Position: it.Position, QuestionVersionID: it.QuestionVersionID, Question: &q2, ObjectiveIDs: nonNil(it.ObjectiveIDs)}
		if r, ok := l.answers[it.Position]; ok {
			w.Response = r
			if w.Feedback, err = q.optionFeedback(ctx, l, it.QuestionVersionID, r); err != nil {
				return out, "", err
			}
		}
		if l.s.ShowCorrect && l.s.FeedbackTiming != exam.FeedbackNever {
			if w.CorrectOptionIDs, err = q.Attempts.Delivery.CorrectOptions(ctx, it.QuestionVersionID); err != nil {
				return out, "", err
			}
		}
		if w.Explanation, err = q.explanation(ctx, l, it.QuestionVersionID); err != nil {
			return out, "", err
		}
		out.WrongItems = append(out.WrongItems, w)
	}
	out.ReviewSuggestions = suggestions(out.ObjectiveBreakdown)
	return out, l.a.DisplayLanguage, nil
}

// suggestions are the 1–3 weakest objectives (ratio < 60 % first, then any objective with a wrong item), practice links.
func suggestions(rows []dto.ExamObjectiveView) []dto.ExamSuggestionView {
	type cand struct {
		id    uuid.UUID
		ratio float64
	}
	var weak, some []cand
	for _, r := range rows {
		ratio := float64(r.Correct) / float64(r.Total)
		switch {
		case ratio < 0.6:
			weak = append(weak, cand{r.ObjectiveID, ratio})
		case ratio < 1:
			some = append(some, cand{r.ObjectiveID, ratio})
		}
	}
	pick := weak
	if len(pick) == 0 {
		pick = some
	}
	slices.SortStableFunc(pick, func(a, b cand) int {
		switch {
		case a.ratio < b.ratio:
			return -1
		case a.ratio > b.ratio:
			return 1
		}
		return 0
	})
	out := []dto.ExamSuggestionView{}
	for _, c := range pick {
		if len(out) == 3 {
			break
		}
		out = append(out, dto.ExamSuggestionView{ObjectiveID: c.id, CorrectRatio: float64(int(c.ratio*100+0.5)) / 100, Kind: "practice"})
	}
	return out
}
