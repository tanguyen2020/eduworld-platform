package command

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mastery"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// AdHocSource is AdHocAttemptStart.source.
type AdHocSource struct {
	Kind          string      `json:"kind"`
	Interleave    bool        `json:"interleave"`
	QuestionSetID *uuid.UUID  `json:"question_set_id"`
	SubjectID     *uuid.UUID  `json:"subject_id"`
	GradeLevel    *int        `json:"grade_level"`
	TopicID       *uuid.UUID  `json:"topic_id"`
	TopicIDs      []uuid.UUID `json:"topic_ids"`
	ObjectiveID   *uuid.UUID  `json:"objective_id"`
	Difficulty    *int        `json:"difficulty"`
	QuestionCount *int        `json:"question_count"`
}

// StartAdHocInput is AdHocAttemptStart.
type StartAdHocInput struct {
	Mode             string      `json:"mode"`
	RecommendationID *uuid.UUID  `json:"recommendation_id"`
	Source           AdHocSource `json:"source"`
	ContextType      string      `json:"context_type"`
	ContextID        *uuid.UUID  `json:"context_id"`
	AcceptLanguage   string      `json:"-"`
}

// AdHocQuestionCount is the default number of questions of a selection practice (AdHocAttemptStart.question_count).
const AdHocQuestionCount = 10

// Source kinds of an ad-hoc attempt (AdHocAttemptStart.source.kind).
const (
	kindRetryWrong = "retry_wrong"
	kindDiagnostic = "diagnostic"
)

// adHocKinds are the source kinds each ad-hoc mode accepts (F030 practice; F032 retry_wrong and diagnostic; assignment
// is F041).
var adHocKinds = map[attempt.Mode][]string{
	attempt.ModePractice:   {activity.SourceQuestionSet, activity.SourceSelection},
	attempt.ModeRetryWrong: {kindRetryWrong},
	attempt.ModeDiagnostic: {kindDiagnostic},
	// F041 (OQ F062-2): a school assignment on a question set.
	attempt.ModeAssignment: {activity.SourceQuestionSet},
}

// quizTypes are the question types of a quiz: an ad-hoc attempt made of them only is a quiz, otherwise an
// interactive_exercise [Proposed].
var quizTypes = []string{"single_choice", "multiple_choice", "true_false", "image_based"}

func adHocType(items []attempt.Item) string {
	for _, it := range items {
		if !slices.Contains(quizTypes, it.QuestionType) {
			return string(activity.InteractiveExercise)
		}
	}
	return string(activity.Quiz)
}

// validateAdHoc checks the combinations the OpenAPI schema cannot express (F032-AC-07, AC-24, AC-25).
func (h Attempts) validateAdHoc(in StartAdHocInput) error {
	mode := attempt.Mode(in.Mode)
	kinds, ok := adHocKinds[mode]
	if !ok || ((mode == attempt.ModeRetryWrong || mode == attempt.ModeDiagnostic) && h.Self == nil) || (mode == attempt.ModeAssignment && h.Gate == nil) {
		return problem.Invalid("/mode", problem.SubInvalid, "this mode is not available for an ad-hoc attempt in this release")
	}
	switch {
	case mode == attempt.ModeAssignment && (in.ContextType != "assignment" || in.ContextID == nil):
		return problem.Invalid("/context_id", problem.SubRequired, "an assignment attempt needs context_type assignment and its context_id")
	case mode != attempt.ModeAssignment && in.ContextType != "" && in.ContextType != "free_play":
		return problem.Invalid("/context_type", problem.SubInvalid, "this context is not available in this release")
	}
	src := in.Source
	if !slices.Contains(kinds, src.Kind) {
		return problem.Invalid("/source/kind", problem.SubInvalid, "does not match the mode")
	}
	if src.Interleave && src.Kind != activity.SourceSelection {
		return problem.Invalid("/source/interleave", problem.SubInvalid, "only a selection practice is interleaved (not exam or diagnostic)")
	}
	if len(src.TopicIDs) > 0 && !src.Interleave {
		return problem.Invalid("/source/topic_ids", problem.SubInvalid, "topic_ids go with interleave")
	}
	switch src.Kind {
	case activity.SourceQuestionSet:
		if src.QuestionSetID == nil {
			return problem.Invalid("/source/question_set_id", problem.SubRequired, "is required with kind question_set")
		}
	case activity.SourceSelection:
		if src.Interleave {
			if h.Self == nil {
				return problem.Invalid("/source/interleave", problem.SubInvalid, "interleaving is not available in this release")
			}
			if len(src.TopicIDs) < 2 && src.SubjectID == nil {
				return problem.Invalid("/source/topic_ids", problem.SubRequired, "an interleaved practice needs two topics or a subject")
			}
		} else if src.SubjectID == nil && src.TopicID == nil && src.ObjectiveID == nil {
			return problem.Invalid("/source/subject_id", problem.SubRequired, "a selection needs a subject, topic or objective")
		}
	case kindDiagnostic:
		if src.TopicID == nil && (src.SubjectID == nil || src.GradeLevel == nil) {
			return problem.Invalid("/source/subject_id", problem.SubRequired, "a diagnostic needs a topic, or a subject and a grade level")
		}
	}
	return nil
}

// adHocContext is the context of an ad-hoc attempt; an open attempt of the same mode and context is resumed (F032-AC-08).
func adHocContext(mode attempt.Mode, src AdHocSource, studentID, attemptID uuid.UUID) (string, uuid.UUID) {
	first := func(ids ...*uuid.UUID) uuid.UUID {
		for _, id := range ids {
			if id != nil {
				return *id
			}
		}
		return attemptID
	}
	switch mode {
	case attempt.ModeRetryWrong:
		return "retry_wrong", first(src.SubjectID, &studentID)
	case attempt.ModeDiagnostic:
		return "diagnostic", first(src.TopicID, src.SubjectID)
	}
	if src.Kind == activity.SourceQuestionSet {
		return "question_set", *src.QuestionSetID
	}
	if src.Interleave {
		var topic *uuid.UUID
		if len(src.TopicIDs) > 0 {
			topic = &src.TopicIDs[0]
		}
		return "selection", first(src.SubjectID, topic)
	}
	return "selection", first(src.ObjectiveID, src.TopicID, src.SubjectID)
}

// adHocPlan is what a start pins: the question versions in order and, for retry_wrong, the review entry of each one.
type adHocPlan struct {
	qids    []uuid.UUID
	shuffle bool
	entries map[uuid.UUID]write.WrongEntryRow
}

// StartAdHoc starts a practice without a fixed activity (F030-AC-27, F032-AC-01…AC-08, AC-22, AC-24, AC-25, AC-32):
// the current release of a question set, a selection of published questions of the tenant (own or assigned) with an
// adaptive difficulty and interleaving, a review of the wrong questions with siblings, or a warm-up diagnostic; the
// seed is stored, no deadline, 422 QUESTION_POOL_INSUFFICIENT when the pool is too small. Assignment is F041.
func (h Attempts) StartAdHoc(ctx context.Context, in StartAdHocInput) (out dto.AttemptView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.StartAdHocAttempt")
	defer func() { done(err) }()
	p, err := student(ctx)
	if err != nil {
		return out, err
	}
	if err := h.validateAdHoc(in); err != nil {
		return out, err
	}
	mode, src := attempt.Mode(in.Mode), in.Source
	if mode == attempt.ModeAssignment {
		return h.startAssignmentSet(ctx, p, in)
	}
	key := idemKey(ctx)
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if prev, err := h.Store.AttemptByKey(ctx, p.UserID, key); err != nil || prev != nil {
			if err == nil {
				out, err = h.view(ctx, *prev, nil, nil, nil, nil)
			}
			return err
		}
		school, grade, err := h.studentSchool(ctx, p)
		if err != nil {
			return err
		}
		rec, err := h.followedRecommendation(ctx, p, in.RecommendationID)
		if err != nil {
			return err
		}
		seed, id, now := randomSeed(), ids.New(), h.now()
		ctxType, ctxID := adHocContext(mode, src, p.UserID, id)
		sourceKind, sourceVersion := "selection", id
		var setItems []uuid.UUID
		var open *attempt.Attempt
		if src.Kind == activity.SourceQuestionSet {
			if sourceVersion, setItems, err = h.questionSetItems(ctx, *src.QuestionSetID, nil, seed); err != nil {
				return err
			}
			sourceKind = "question_set"
			open, err = h.Store.OpenPractice(ctx, p.UserID, sourceVersion)
		} else {
			open, err = h.Store.OpenAdHoc(ctx, p.UserID, mode, ctxType, ctxID)
		}
		if err != nil {
			return err
		}
		if open != nil && open.CheckActive(now, h.policy()) == nil && !open.Abandon(now, h.policy()) {
			if out, err = h.view(ctx, *open, nil, nil, nil, nil); err != nil {
				return err
			}
			return idempotency.SetResult(ctx, out)
		}
		if err := h.checkStartRate(ctx, p.UserID); err != nil {
			return err
		}
		plan := adHocPlan{qids: setItems, shuffle: true}
		if src.Kind != activity.SourceQuestionSet {
			if plan, err = h.plan(ctx, p, mode, src, grade, seed, now); err != nil {
				return err
			}
		}
		items, rendered, err := h.pinItems(ctx, plan.qids, seed, plan.shuffle, nil)
		if err != nil {
			return err
		}
		a := attempt.New(attempt.Start{ID: id, TenantID: *p.TenantID, SchoolID: school, StudentID: p.UserID, Mode: mode,
			ContextType: ctxType, ContextID: ctxID, ActivityType: adHocType(items), SourceKind: sourceKind, SourceVersionID: sourceVersion, Seed: seed,
			Now: now, DisplayLanguage: i18ntext.Locale(in.AcceptLanguage), RecommendationID: recommendationID(rec), IdempotencyKey: key, Items: items})
		if err := h.Store.InsertAttempt(ctx, a); err != nil {
			return err
		}
		if err := h.bindReviews(ctx, a, plan.entries); err != nil {
			return err
		}
		if err := h.startRecommendation(ctx, rec, a.ID); err != nil {
			return err
		}
		if err := h.started(ctx, a); err != nil {
			return err
		}
		if out, err = h.view(ctx, a, rendered, nil, nil, nil); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, out)
	})
	return out, err
}

// plan chooses the questions of a selection, retry_wrong or diagnostic start.
func (h Attempts) plan(ctx context.Context, p authz.Principal, mode attempt.Mode, src AdHocSource, grade *int, seed int64, now time.Time) (adHocPlan, error) {
	count := AdHocQuestionCount
	if src.QuestionCount != nil {
		count = *src.QuestionCount
	}
	var recent []uuid.UUID
	if h.Self != nil {
		var err error
		if recent, err = h.Self.Reader.RecentQuestions(ctx, p.UserID, now.Add(-h.Self.Config.AvoidRecent)); err != nil {
			return adHocPlan{}, err
		}
	}
	switch {
	case mode == attempt.ModeRetryWrong:
		return h.retryPlan(ctx, p, src, count, recent, seed, now)
	case mode == attempt.ModeDiagnostic:
		return h.diagnosticPlan(ctx, src, grade, recent, seed)
	case src.Interleave:
		return h.interleavePlan(ctx, p, src, grade, count, recent, seed)
	}
	f := selection.Filters{SubjectID: src.SubjectID}
	if src.TopicID != nil {
		f.TopicIDs = []uuid.UUID{*src.TopicID}
	}
	if src.ObjectiveID != nil {
		f.ObjectiveIDs = []uuid.UUID{*src.ObjectiveID}
	}
	if grade != nil {
		f.GradeLevels = []int{*grade}
	}
	if src.Difficulty != nil {
		f.Difficulty = &selection.Range{Min: *src.Difficulty, Max: *src.Difficulty}
	} else if h.Self != nil {
		level, err := h.currentLevel(ctx, p, src)
		if err != nil {
			return adHocPlan{}, err
		}
		r := selection.DifficultyForLevel(level)
		f.Difficulty = &r
	}
	cs, err := h.Delivery.Select(ctx, selection.Request{Rule: selection.Rule{Filters: f, Count: count}, Seed: uint64(seed), Recent: recent})
	if err != nil {
		return adHocPlan{}, err
	}
	return adHocPlan{qids: versionsOf(cs), shuffle: true}, nil
}

func versionsOf(cs []selection.Candidate) []uuid.UUID {
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.VersionID
	}
	return out
}

func poolInsufficient(err error, msg string) error {
	if errors.Is(err, selection.ErrPoolInsufficient) {
		return problem.New(problem.QuestionPoolInsufficient, msg)
	}
	return err
}

// currentLevel is the mastery level a selection adapts to (F032-AC-05): the level of the objective after decay, or for a
// topic or subject the level of the mean value after decay of its objectives with evidence; beginning without evidence.
func (h Attempts) currentLevel(ctx context.Context, p authz.Principal, src AdHocSource) (string, error) {
	cfg, err := h.Self.Masteries.ActiveModel(ctx)
	if err != nil {
		return "", err
	}
	m, err := ModelOf(cfg)
	if err != nil {
		return "", err
	}
	rows, err := h.Self.Reader.StudentMasteries(ctx, p.UserID)
	if err != nil {
		return "", err
	}
	in := map[uuid.UUID]bool{}
	if src.ObjectiveID != nil {
		in[*src.ObjectiveID] = true
	} else {
		f := write.ObjectiveFilter{SubjectID: src.SubjectID}
		if src.TopicID != nil {
			f.TopicIDs = []uuid.UUID{*src.TopicID}
		}
		los, err := h.Self.Reader.Objectives(ctx, f)
		if err != nil {
			return "", err
		}
		for _, o := range los {
			in[o.ID] = true
		}
	}
	now := h.now()
	sum, n := 0.0, 0
	for _, r := range rows {
		if !in[r.ObjectiveID] || r.State.EvidenceCount == 0 {
			continue
		}
		v, level := m.Effective(r.State, now)
		if src.ObjectiveID != nil {
			return string(level), nil
		}
		sum += v
		n++
	}
	if n == 0 {
		return string(mastery.Beginning), nil
	}
	t, v := m.P.Thresholds, sum/float64(n)
	switch {
	case v < t.Developing:
		return string(mastery.Beginning), nil
	case v < t.Proficient:
		return string(mastery.Developing), nil
	case v < t.Mastered:
		return string(mastery.Proficient), nil
	}
	return string(mastery.Mastered), nil
}

// interleavePlan splits the questions between 2–4 learned topics of a subject by interleave_mix and orders them with
// at most interleave_mix.max_consecutive in a row from one topic (F032-AC-24).
func (h Attempts) interleavePlan(ctx context.Context, p authz.Principal, src AdHocSource, grade *int, count int, recent []uuid.UUID,
	seed int64) (adHocPlan, error) {
	mix := h.Self.Config.Mix
	topics := slices.Clone(src.TopicIDs)
	if len(topics) == 0 {
		var err error
		if topics, err = h.Self.Reader.LearnedTopics(ctx, p.UserID, *src.SubjectID); err != nil {
			return adHocPlan{}, err
		}
	}
	maxTopics := mix.MaxTopics
	if grade != nil && *grade <= 3 && count <= 5 {
		maxTopics = mix.MinTopics // g1_3: a short round mixes two topics only (LEARNING_EXPERIENCE §5.2)
	}
	if len(topics) > maxTopics {
		topics = topics[:maxTopics]
	}
	if len(topics) < mix.MinTopics {
		return adHocPlan{}, problem.Invalid("/source/interleave", problem.SubInvalid, "an interleaved practice needs at least two topics already learned")
	}
	var picked []selection.Candidate
	for i, n := range mix.Split(count, topics) {
		if n == 0 {
			continue
		}
		f := selection.Filters{SubjectID: src.SubjectID, TopicIDs: []uuid.UUID{topics[i]}}
		if grade != nil {
			f.GradeLevels = []int{*grade}
		}
		cs, err := h.Delivery.Select(ctx, selection.Request{Rule: selection.Rule{Filters: f, Count: n}, Seed: uint64(seed) + uint64(i), Recent: recent})
		if err != nil {
			return adHocPlan{}, err
		}
		picked = append(picked, cs...)
	}
	return adHocPlan{qids: versionsOf(selection.Interleave(picked, mix.MaxConsecutive, uint64(seed))), shuffle: false}, nil
}

// retryPlan reviews the wrong questions of the student, the due ones first, each through a sibling (F032-AC-22).
func (h Attempts) retryPlan(ctx context.Context, p authz.Principal, src AdHocSource, count int, recent []uuid.UUID, seed int64,
	now time.Time) (adHocPlan, error) {
	entries, err := h.Self.Wrong.DueEntries(ctx, p.UserID, src.SubjectID, now.AddDate(100, 0, 0), count)
	if err != nil {
		return adHocPlan{}, err
	}
	if len(entries) == 0 {
		return adHocPlan{}, problem.New(problem.QuestionPoolInsufficient, "there is no question to review")
	}
	wrong := make([]selection.WrongItem, len(entries))
	for i, e := range entries {
		wrong[i] = selection.WrongItem{QuestionID: e.QuestionID, ObjectiveIDs: e.ObjectiveIDs, Type: e.QuestionType, Difficulty: e.Difficulty}
	}
	pool, err := h.Delivery.Pool(ctx, src.SubjectID)
	if err != nil {
		return adHocPlan{}, err
	}
	picks, err := selection.Siblings(pool, wrong, recent, uint64(seed))
	if err != nil {
		return adHocPlan{}, poolInsufficient(err, "no sibling nor original question is published")
	}
	plan := adHocPlan{shuffle: true, entries: map[uuid.UUID]write.WrongEntryRow{}}
	for _, pk := range picks {
		plan.qids = append(plan.qids, pk.Candidate.VersionID)
		plan.entries[pk.Candidate.VersionID] = entries[pk.Wrong]
	}
	return plan, nil
}

// bindReviews records which item of a retry_wrong attempt reviews each entry (its answer moves the entry on).
func (h Attempts) bindReviews(ctx context.Context, a attempt.Attempt, entries map[uuid.UUID]write.WrongEntryRow) error {
	for _, it := range a.Items {
		e, ok := entries[it.QuestionVersionID]
		if !ok {
			continue
		}
		attemptID, pos := a.ID, it.Position
		e.RetryAttemptID, e.RetryPosition = &attemptID, &pos
		ok, err := h.Self.Wrong.UpdateEntry(ctx, e)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.VersionMismatch, "the review list changed meanwhile").Retry()
		}
	}
	return nil
}

// diagnosticPlan is the warm-up of LEARNING_ANALYTICS §3.5 (F032-AC-25): the objectives of the topic, or of the subject
// and grade, and their direct prerequisites (prerequisites first), 2–3 questions each, at most 15 (grades 1–3) or 20.
func (h Attempts) diagnosticPlan(ctx context.Context, src AdHocSource, grade *int, recent []uuid.UUID, seed int64) (adHocPlan, error) {
	f := write.ObjectiveFilter{WithPrerequisites: true}
	if src.TopicID != nil {
		f.TopicIDs = []uuid.UUID{*src.TopicID}
	} else {
		f.SubjectID, f.Grade = src.SubjectID, src.GradeLevel
	}
	los, err := h.Self.Reader.Objectives(ctx, f)
	if err != nil {
		return adHocPlan{}, err
	}
	focal := func(o write.ObjectiveInfo) bool {
		if src.TopicID != nil {
			return o.TopicID == *src.TopicID
		}
		return o.SubjectID == *src.SubjectID && o.Grade == *src.GradeLevel
	}
	var pre, main []uuid.UUID
	subject := src.SubjectID
	for _, o := range los {
		if focal(o) {
			main = append(main, o.ID)
			s := o.SubjectID
			subject = &s
		} else {
			pre = append(pre, o.ID)
		}
	}
	if len(main) == 0 {
		return adHocPlan{}, problem.New(problem.QuestionPoolInsufficient, "the diagnostic has no learning objective")
	}
	pool, err := h.Delivery.Pool(ctx, subject)
	if err != nil {
		return adHocPlan{}, err
	}
	d := h.Self.Config.Diagnostic
	cs, err := selection.Diagnostic(pool, append(pre, main...), d.PerObjectiveMax, d.MaxQuestionsFor(grade), recent, uint64(seed))
	if err != nil {
		return adHocPlan{}, poolInsufficient(err, "no published question for the objectives of the diagnostic")
	}
	return adHocPlan{qids: versionsOf(cs), shuffle: true}, nil
}
