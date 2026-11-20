package command

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	mrand "math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/attemptview"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/questionset"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Domain events of the attempt engine (EVENT_DESIGN §4.3; F030 owns every activity.* payload schema).
const (
	EventActivityStarted   = "activity.started"
	EventActivitySubmitted = "activity.submitted"
	EventActivityGraded    = "activity.graded"
	EventActivityAbandoned = "activity.abandoned"
	EventQuestionAnswered  = "question.answered"
)

// AttemptStartRate is attempt_start_rate (CONFIG_DEFAULTS: 20 per hour, owner ANTI_CHEAT [Proposed]) until module learning
// reads platform settings.
const AttemptStartRate = 20

// AttemptMetrics counts the engine (eduworld_attempt_started_total{activity_type,mode}, eduworld_attempt_submitted_total{result},
// eduworld_attempt_expired_total); nil = not counted.
type AttemptMetrics interface {
	Started(activityType, mode string)
	Submitted(result string)
	Expired(kind string)
}

// Attempts holds the write use cases of the Activity & Attempt Engine (F030 US1–US3, US5, US6; ACTIVITY_ENGINE §6–§11).
type Attempts struct {
	UoW       write.UnitOfWork
	Store     write.AttemptStore
	Learning  write.LearningEventWriter
	Events    write.EventPublisher
	Docs      write.DocStore
	Questions write.QuestionStore
	Delivery  port.QuestionDelivery
	Students  port.StudentDirectory
	Registry  *activity.Registry
	Policy    attempt.Policy
	StartRate int // attempt_start_rate per hour (0 = AttemptStartRate)
	Clock     func() time.Time
	Metrics   AttemptMetrics
	// Lessons pins the lesson session of an attempt started with context_type lesson (F031 US6); nil = no session.
	Lessons LessonPinner
	// Self are the self-study stores of F032 (review list, recommendations, rapid_guess); nil = F030 behaviour.
	Self *SelfStudy
	// Gate admits the attempts of a school assignment (F041 mode assignment); nil = the mode is refused.
	Gate AssignmentGate
	// Rate counts answer_rate (T053); nil = not limited. AnswerRate zero = DefaultAnswerRate.
	Rate       RateLimiter
	AnswerRate AnswerRate
	// Progress, Media and Signer serve the media activities (F065 video_lesson): heartbeats, branch choices, the
	// ready check of the video and its signed URLs. MediaMetrics may be nil.
	Progress     write.MediaProgressStore
	Media        write.MediaStore
	Signer       port.URLSigner
	MediaMetrics VideoMetrics
	// Runs stores the runs of the simulation attempts (F066); SimMetrics may be nil.
	Runs       write.SimulationRunStore
	SimMetrics SimulationMetrics
	// ExamRelease is the result release of an exam session and whether its results are visible (F042); nil = immediate.
	ExamRelease func(ctx context.Context, sessionID uuid.UUID) (string, bool, error)
	// EfficacyWave is the efficacy wave of an exam session (F035); nil = no efficacy session.
	EfficacyWave func(ctx context.Context, sessionID uuid.UUID) (*uuid.UUID, error)
	// Arms lists the experiment arms ("<code>:<arm>") of a student for the learning events (F035-AC-25); nil = none.
	Arms func(ctx context.Context, tenant, student uuid.UUID) ([]string, error)
	// Experience resolves the live experiment arms of a student for the adaptive practice difficulty (F035-AC-24); nil =
	// no experiment.
	Experience ExperienceResolver
}

// LessonPinner opens or reuses the lesson session of a start in the context of a lesson (command.Lessons.Pin).
type LessonPinner interface {
	Pin(ctx context.Context, in PinInput) error
}

func (h Attempts) now() time.Time { return now(h.Clock).UTC() }

func (h Attempts) policy() attempt.Policy {
	if h.Policy.ResumeWindow == 0 {
		return attempt.DefaultPolicy()
	}
	return h.Policy
}

// student is the principal of the game client: a student of the session tenant (R23: the game_player guard ran
// before; the use case checks it again).
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

// attemptProblem maps the lifecycle errors of the aggregate to their problem codes (ACTIVITY_ENGINE §9).
func attemptProblem(err error) error {
	switch {
	case errors.Is(err, attempt.ErrNotInProgress):
		return problem.New(problem.AttemptNotInProgress, "")
	case errors.Is(err, attempt.ErrExpired):
		return problem.New(problem.AttemptExpired, "")
	case errors.Is(err, attempt.ErrDeadlinePassed):
		return problem.New(problem.AttemptDeadlinePassed, "")
	case errors.Is(err, attempt.ErrAlreadySubmitted):
		return problem.New(problem.AttemptAlreadySubmitted, "")
	case errors.Is(err, attempt.ErrPositionInvalid):
		return problem.Invalid("/position", problem.SubInvalid, "no item at this position")
	case errors.Is(err, attempt.ErrAnswerLimit):
		return problem.New(problem.AnswerLimitReached, "")
	case errors.Is(err, attempt.ErrHintsNotAllowed):
		return problem.New(problem.HintsNotAllowed, "")
	case errors.Is(err, attempt.ErrHintsExhausted):
		return problem.New(problem.HintsExhausted, "")
	}
	return err
}

func randomSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano() & (1<<62 - 1)
	}
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<62 - 1))
}

// loadOwn loads an attempt of the student of ctx (another student's attempt is 404 ATTEMPT_NOT_FOUND, F030-AC-06); an exam
// attempt is 409 CONFLICT (F042: only the exam routes act on it).
func (h Attempts) loadOwn(ctx context.Context, p authz.Principal, id uuid.UUID) (attempt.Attempt, error) {
	a, err := h.Store.Attempt(ctx, id)
	if err != nil {
		return a, err
	}
	if a.StudentID != p.UserID {
		return attempt.Attempt{}, problem.New(problem.AttemptNotFound, "")
	}
	if a.Mode == attempt.ModeExam {
		// F042: an exam attempt is answered, submitted and reviewed through /exams (deadline, device, release policy).
		return attempt.Attempt{}, problem.New(problem.Conflict, "an exam attempt is answered through the exam routes")
	}
	return a, nil
}

// ---------- activity versions

// ActivitySource is a published activity version with its plugin.
type ActivitySource = attemptview.Source

// LoadActivityVersion reads one version of an activity visible in the scope of ctx with its plugin.
func LoadActivityVersion(ctx context.Context, docs write.DocStore, reg *activity.Registry, activityID uuid.UUID, versionID *uuid.UUID) (ActivitySource, error) {
	d, err := docs.Doc(ctx, contentmodel.TypeActivity, activityID)
	if err != nil {
		return ActivitySource{}, err
	}
	if versionID == nil {
		if d.CurrentVersionID == nil {
			return ActivitySource{}, problem.New(problem.ContentNotPublished, "the activity is not published")
		}
		versionID = d.CurrentVersionID
	}
	vs, err := docs.DocVersions(ctx, contentmodel.TypeActivity, activityID)
	if err != nil {
		return ActivitySource{}, err
	}
	src := ActivitySource{Doc: d}
	found := false
	for _, v := range vs {
		if v.ID == *versionID {
			src.Version, found = v, true
		}
	}
	if !found {
		return src, problem.New(problem.ContentNotPublished, "the activity version is not published")
	}
	src.Body = attemptview.ParseBody(src.Version.Body)
	if src.Plugin, err = ActivityPlugin(reg, src.Body.ActivityType); err != nil {
		return src, err
	}
	if src.Config, err = activity.ParseConfig(src.Body.Config); err != nil {
		return src, problem.New(problem.ActivityConfigInvalid, "the published config cannot be read")
	}
	return src, nil
}

// adHocVersion is the plugin view of an attempt without activity (question set or selection): question-based, every
// answer graded automatically, no pass threshold.
func (h Attempts) adHocVersion(a attempt.Attempt) (activity.Plugin, activity.Version, error) {
	p, err := ActivityPlugin(h.Registry, a.ActivityType)
	if err != nil {
		return nil, activity.Version{}, err
	}
	return p, activity.Version{VersionID: a.SourceVersionID, Type: activity.Type(a.ActivityType),
		Config: activity.Config{Evaluation: activity.Evaluation{Strategies: []string{"auto"}}}}, nil
}

// versionOf returns the plugin and version of an attempt (the pinned activity version, R31).
func (h Attempts) versionOf(ctx context.Context, a attempt.Attempt) (activity.Plugin, activity.Version, *ActivitySource, error) {
	if a.ActivityID == nil {
		p, v, err := h.adHocVersion(a)
		return p, v, nil, err
	}
	src, err := LoadActivityVersion(ctx, h.Docs, h.Registry, *a.ActivityID, &a.SourceVersionID)
	if err != nil {
		return nil, activity.Version{}, nil, err
	}
	return src.Plugin, src.DomainVersion(), &src, nil
}

// ---------- items

// pinItems pins the question versions of a start (F030-AC-01): fixed questions, the current release of a question set
// or a selection from the published pool (seed reproducible); each item keeps its render presentation, payload hash,
// type, objectives and max score. Items are shuffled by seed unless the activity says otherwise.
func (h Attempts) pinItems(ctx context.Context, versionIDs []uuid.UUID, seed int64, shuffle bool, allowed []string) ([]attempt.Item, []question.Rendered, error) {
	if len(versionIDs) == 0 {
		return nil, nil, problem.New(problem.QuestionPoolInsufficient, "the activity has no question")
	}
	order := slices.Clone(versionIDs)
	if shuffle {
		r := mrand.New(mrand.NewPCG(uint64(seed), uint64(seed)>>1|1))
		r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	rendered, pres, err := h.Delivery.Render(ctx, order, uint64(seed))
	if err != nil {
		return nil, nil, err
	}
	byID := map[uuid.UUID]int{}
	for i, r := range rendered {
		byID[r.QuestionVersionID] = i
	}
	items := make([]attempt.Item, 0, len(order))
	outRendered := make([]question.Rendered, 0, len(order))
	for _, id := range order {
		i, ok := byID[id]
		if !ok {
			return nil, nil, problem.New(problem.ContentNotPublished, "a question of the activity is not published")
		}
		qv, err := h.Questions.QuestionVersionByID(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if len(allowed) > 0 && !slices.Contains(allowed, qv.Type) {
			return nil, nil, problem.New(problem.ActivityConfigInvalid, "a question type is not allowed by the activity")
		}
		pj, _ := json.Marshal(pres[i])
		rj, _ := json.Marshal(rendered[i])
		sum := sha256.Sum256(rj)
		maxScore := rendered[i].MaxScore
		if maxScore <= 0 {
			maxScore = 1
		}
		items = append(items, attempt.Item{ID: ids.New(), Position: len(items) + 1, QuestionVersionID: id, QuestionType: string(rendered[i].Type),
			ObjectiveIDs: qv.ObjectiveIDs, Presentation: pj, PayloadHash: hex.EncodeToString(sum[:]), MaxScore: maxScore})
		outRendered = append(outRendered, rendered[i])
	}
	return items, outRendered, nil
}

// sourceQuestions resolves the question versions of a question source (activity-base.schema.json#/$defs/questionSource).
func (h Attempts) sourceQuestions(ctx context.Context, src activity.QuestionSource, subject *uuid.UUID, grades []int, allowed []string, seed int64) ([]uuid.UUID, error) {
	switch src.Kind {
	case activity.SourceQuestions:
		out := make([]uuid.UUID, len(src.Questions))
		for i, q := range src.Questions {
			out[i] = q.QuestionVersionID
		}
		return out, nil
	case activity.SourceQuestionSet:
		if src.QuestionSetID == nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "question_set_id is missing")
		}
		_, ids, err := h.questionSetItems(ctx, *src.QuestionSetID, nil, seed)
		return ids, err
	case activity.SourceSelection:
		if src.Selection == nil {
			return nil, problem.New(problem.ActivityConfigInvalid, "selection is missing")
		}
		s := src.Selection
		f := selection.Filters{SubjectID: subject, GradeLevels: grades, TopicIDs: s.TopicIDs, ObjectiveIDs: s.ObjectiveIDs, Types: s.QuestionTypes}
		if len(f.Types) == 0 {
			f.Types = allowed
		}
		if s.DifficultyMin != nil || s.DifficultyMax != nil {
			r := selection.Range{Min: 1, Max: 5}
			if s.DifficultyMin != nil {
				r.Min = *s.DifficultyMin
			}
			if s.DifficultyMax != nil {
				r.Max = *s.DifficultyMax
			}
			f.Difficulty = &r
		}
		return h.selectQuestions(ctx, f, s.Count, seed)
	}
	return nil, problem.New(problem.ActivityConfigInvalid, "unknown question source")
}

// selectQuestions picks count published questions of the pool of the tenant (QUESTION_BANK §6, seed reproducible).
func (h Attempts) selectQuestions(ctx context.Context, f selection.Filters, count int, seed int64) ([]uuid.UUID, error) {
	cs, err := h.Delivery.Select(ctx, selection.Request{Rule: selection.Rule{Filters: f, Count: count}, Seed: uint64(seed)})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, len(cs))
	for i, c := range cs {
		out[i] = c.VersionID
	}
	return out, nil
}

// questionSetItems returns the current release of a question set and the question versions of a start: the fixed items
// of the release, then the questions drawn by each selection rule of the release with selection.Select and the seed of
// the attempt (rule_based and mixed sets, OQ F030 owner 2026-11-20); every version is pinned by the attempt (R31) and a
// rule the pool cannot satisfy is 422 QUESTION_POOL_INSUFFICIENT (the pool of the F024 pool stats).
func (h Attempts) questionSetItems(ctx context.Context, setID uuid.UUID, versionID *uuid.UUID, seed int64) (uuid.UUID, []uuid.UUID, error) {
	d, err := h.Docs.Doc(ctx, contentmodel.TypeQuestionSet, setID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if versionID != nil {
		d.CurrentVersionID = versionID // the version pinned by a school assignment (R31)
	}
	if d.CurrentVersionID == nil {
		return uuid.Nil, nil, problem.New(problem.ContentNotPublished, "the question set is not published")
	}
	vs, err := h.Docs.DocVersions(ctx, contentmodel.TypeQuestionSet, setID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	for _, v := range vs {
		if v.ID != *d.CurrentVersionID {
			continue
		}
		var body struct {
			Definition questionset.Definition `json:"definition"`
		}
		_ = json.Unmarshal(v.Body, &body)
		var out []uuid.UUID
		for _, it := range body.Definition.Items {
			out = append(out, it.QuestionVersionID)
		}
		if len(out) == 0 && len(body.Definition.SelectionRules) == 0 {
			for _, it := range v.Items {
				out = append(out, it.QuestionVersionID)
			}
		}
		var drawn []uuid.UUID
		for i, r := range body.Definition.SelectionRules {
			rule := r.Selection()
			rule.Filters.ExcludeQuestionIDs = append(rule.Filters.ExcludeQuestionIDs, drawn...)
			cs, err := h.Delivery.Select(ctx, selection.Request{Rule: rule, Seed: uint64(seed) + uint64(i)})
			if err != nil {
				return v.ID, nil, err
			}
			for _, c := range cs {
				out = append(out, c.VersionID)
				drawn = append(drawn, c.QuestionID)
			}
		}
		if len(out) == 0 {
			return v.ID, nil, problem.New(problem.QuestionPoolInsufficient, "the question set has no question")
		}
		return v.ID, out, nil
	}
	return uuid.Nil, nil, problem.New(problem.ContentNotPublished, "the question set is not published")
}

// ---------- learning events and domain events

func modeContext(m attempt.Mode) string { return string(m) }

func (h Attempts) learningEvent(ctx context.Context, a attempt.Attempt, typ, key string, at time.Time, objectives []uuid.UUID, actor string, payload any) error {
	if objectives == nil {
		objectives = []uuid.UUID{}
	}
	cv := a.SourceVersionID
	var content *uuid.UUID
	if a.ActivityID != nil {
		content = a.ActivityID
	}
	wave, err := h.efficacyWave(ctx, a)
	if err != nil {
		return err
	}
	var arms []string
	if h.Arms != nil && wave == nil && a.Mode != attempt.ModeExam {
		if arms, err = h.Arms(ctx, a.TenantID, a.StudentID); err != nil {
			return err
		}
	}
	return h.Learning.Write(ctx, write.LearningEvent{ID: ids.New(), OccurredAt: at, Type: typ, StudentID: a.StudentID, TenantID: a.TenantID,
		SchoolID: a.SchoolID, ActivityID: a.ActivityID, ContentID: content, ContentVersionID: &cv, ObjectiveIDs: objectives,
		Context: modeContext(a.Mode), ActorType: actor, Payload: payload, IdempotencyKey: key, EfficacyWaveID: wave, ExperimentArms: arms})
}

// efficacyWave is the efficacy wave of an exam attempt (F035-AC-15), nil for any other attempt.
func (h Attempts) efficacyWave(ctx context.Context, a attempt.Attempt) (*uuid.UUID, error) {
	if a.ContextType != examContextType || h.EfficacyWave == nil {
		return nil, nil
	}
	return h.EfficacyWave(ctx, a.ContextID)
}

func (h Attempts) publish(ctx context.Context, a attempt.Attempt, typ string, payload any) error {
	if h.Events == nil {
		return nil
	}
	// F035-AC-15: an attempt of an efficacy exam session publishes nothing (no mastery, progress, XP, dashboard).
	if wave, err := h.efficacyWave(ctx, a); err != nil || wave != nil {
		return err
	}
	t := a.TenantID
	return h.Events.Publish(ctx, write.Event{Type: typ, SubjectType: "attempt", SubjectID: a.ID.String(), TenantID: &t, Payload: payload})
}

func nullableID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// started records activity.started (learning event + outbox, EVENT_DESIGN §4.3).
func (h Attempts) started(ctx context.Context, a attempt.Attempt) error {
	if err := h.learningEvent(ctx, a, EventActivityStarted, "start:"+a.ID.String(), a.StartedAt, a.ObjectiveIDs, "student",
		map[string]any{"source_kind": sourceKindOf(a), "activity_type": a.ActivityType, "attempt_id": a.ID}); err != nil {
		return err
	}
	if h.Metrics != nil {
		h.Metrics.Started(a.ActivityType, string(a.Mode))
	}
	return h.publish(ctx, a, EventActivityStarted, map[string]any{"attempt_id": a.ID, "student_id": a.StudentID, "school_id": a.SchoolID,
		"activity_id": nullableID(a.ActivityID), "activity_type": a.ActivityType, "mode": a.Mode, "context_type": a.ContextType,
		"context_id": a.ContextID, "content_version_id": a.SourceVersionID, "item_count": len(a.Items), "deadline_at": a.DeadlineAt,
		"recommendation_id": nullableID(a.RecommendationID)})
}

// sourceKindOf is payload_activity_started.source_kind.
func sourceKindOf(a attempt.Attempt) string {
	switch {
	case a.SourceKind == "question_set":
		return "question_set"
	case a.Mode == attempt.ModeQuest:
		return "quest_step"
	case a.Mode == attempt.ModeChallenge:
		return "challenge"
	}
	return "learning_activity"
}

// graded records activity.submitted and activity.graded with the effective evaluation (F030-AC-12).
func (h Attempts) graded(ctx context.Context, a attempt.Attempt, r activity.Result, evaluationID uuid.UUID) error {
	at := *a.GradedAt
	correct := r.CorrectCount
	submitted := map[string]any{"attempt_id": a.ID, "activity_id": nullableID(a.ActivityID), "activity_version_id": a.SourceVersionID,
		"activity_type": a.ActivityType, "status": string(attempt.StatusGraded), "score": learningScore(*a.Score, *a.MaxScore), "correct_count": correct,
		"item_count": len(a.Items), "hints_used": 0, "duration_ms": *a.DurationMS, "submit_reason": *a.SubmitReason}
	if err := h.learningEvent(ctx, a, EventActivitySubmitted, "sub:"+a.ID.String(), *a.SubmittedAt, a.ObjectiveIDs, "student",
		submittedLearningPayload(submitted)); err != nil {
		return err
	}
	release, visibleAt, err := h.resultRelease(ctx, a, at)
	if err != nil {
		return err
	}
	gradedPayload := map[string]any{"attempt_id": a.ID, "evaluation_id": evaluationID, "activity_type": a.ActivityType, "score": learningScore(*a.Score, *a.MaxScore),
		"correct_count": correct, "item_count": len(a.Items), "result_release": release}
	if r.Passed != nil {
		gradedPayload["passed"] = *r.Passed
	}
	if err := h.learningEvent(ctx, a, EventActivityGraded, "grd:"+evaluationID.String(), at, a.ObjectiveIDs, "system", gradedPayload); err != nil {
		return err
	}
	if err := h.publish(ctx, a, EventActivitySubmitted, map[string]any{"attempt_id": a.ID, "student_id": a.StudentID, "school_id": a.SchoolID,
		"activity_id": nullableID(a.ActivityID), "activity_type": a.ActivityType, "content_version_id": a.SourceVersionID, "mode": a.Mode,
		"context_type": a.ContextType, "context_id": a.ContextID, "status": string(attempt.StatusGraded), "score": *a.Score,
		"max_score": *a.MaxScore, "correct_count": correct, "item_count": len(a.Items), "hints_used": 0, "rapid_guess_count": 0,
		"duration_ms": *a.DurationMS, "submit_reason": *a.SubmitReason, "objective_ids": nonNilUUIDs(a.ObjectiveIDs),
		"assignment_id": assignmentOf(a)}); err != nil {
		return err
	}
	ev := map[string]any{"attempt_id": a.ID, "student_id": a.StudentID, "evaluation_id": evaluationID, "activity_id": nullableID(a.ActivityID),
		"activity_type": a.ActivityType, "mode": a.Mode, "source": "system", "strategies": r.Strategies, "score": *a.Score,
		"max_score": *a.MaxScore, "correct_count": correct, "item_count": len(a.Items), "result_release": release,
		"results_visible_at": visibleAt, "objective_ids": nonNilUUIDs(a.ObjectiveIDs), "assignment_id": assignmentOf(a)}
	if r.Passed != nil {
		ev["passed"] = *r.Passed
	}
	if h.Metrics != nil {
		h.Metrics.Submitted(*a.SubmitReason)
	}
	return h.publish(ctx, a, EventActivityGraded, ev)
}

// assignmentOf is the assignment of an attempt started for a school assignment (nil otherwise).
func assignmentOf(a attempt.Attempt) *uuid.UUID {
	if a.ContextType != "assignment" {
		return nil
	}
	id := a.ContextID
	return &id
}

// resultRelease is the ResultRelease of an attempt (immediate outside an assignment) and when its results are visible
// (nil while an assignment has not released them, F041-AC-19).
func (h Attempts) resultRelease(ctx context.Context, a attempt.Attempt, at time.Time) (string, *time.Time, error) {
	if a.ContextType == examContextType && h.ExamRelease != nil {
		release, visible, err := h.ExamRelease(ctx, a.ContextID)
		if err != nil || !visible {
			return release, nil, err
		}
		return release, &at, nil
	}
	id := assignmentOf(a)
	if id == nil || h.Gate == nil {
		return "immediate", &at, nil
	}
	release, visible, err := h.Gate.Release(ctx, *id)
	if err != nil {
		return "", nil, err
	}
	if release == "" {
		release = "immediate"
	}
	if !visible {
		return release, nil, nil
	}
	return release, &at, nil
}

// submittedLearningPayload is payload_activity_submitted of learning-event.schema.json (no mode/context: envelope).
func submittedLearningPayload(m map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"attempt_id", "activity_id", "activity_version_id", "activity_type", "status", "score", "correct_count",
		"item_count", "hints_used", "duration_ms", "submit_reason"} {
		if v, ok := m[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}

func nonNilUUIDs(v []uuid.UUID) []uuid.UUID {
	if v == nil {
		return []uuid.UUID{}
	}
	return v
}

// ---------- views

// view renders an attempt for the command responses (start, pause, submit).
func (h Attempts) view(ctx context.Context, a attempt.Attempt, rendered []question.Rendered, src *ActivitySource, saved []dto.SavedResponseView,
	ev *dto.EvaluationView) (dto.AttemptView, error) {
	if rendered == nil && len(a.Items) > 0 {
		ids := make([]uuid.UUID, len(a.Items))
		for i, it := range a.Items {
			ids[i] = it.QuestionVersionID
		}
		var err error
		if rendered, _, err = h.Delivery.Render(ctx, ids, uint64(a.Seed)); err != nil {
			return dto.AttemptView{}, err
		}
	}
	var render *dto.ActivityRenderView
	if src != nil {
		r := attemptview.Render(*src, h.Registry, a.DisplayLanguage)
		render = &r
	}
	v, err := h.withTimeline(ctx, a, src, attemptview.Attempt(a, attemptview.Items(a.Items, rendered), saved, render, ev, h.now()))
	if err != nil {
		return v, err
	}
	return h.simulationView(ctx, a, v)
}

// learningScore is $defs/score of learning-event.schema.json ({raw, max, scaled}); the mastery outcome is raw / max.
func learningScore(raw, maxScore float64) map[string]any {
	s := map[string]any{"raw": raw, "max": maxScore}
	if maxScore > 0 {
		s["scaled"] = min(1, max(0, raw/maxScore))
	}
	return s
}
