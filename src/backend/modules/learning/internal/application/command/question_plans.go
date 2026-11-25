package command

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question/selection"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Question plans of competition matches (F105 US1/US2, COMPETITION_SYSTEM §4): module competition asks module learning
// for the questions of a match (createQuestionPlan) and sends every answer to be graded here (scorePlanAnswer) — the
// answer key never leaves module learning (ADR-0016). Each participant gets one attempt mode competition (context
// match) whose items are the plan, so every graded answer is learning evidence (attempt_answers, question.answered with
// context competition: low mastery weight, ADR-0017) and the questions count as seen for avoid_recent.

// planContext is the attempts.context_type of a match attempt (attempts_context_type_ck).
const planContext = "match"

// RecentQuestionReader lists the questions a student met since a time (assessment attempts, QUESTION_BANK §6 avoid_recent).
type RecentQuestionReader interface {
	RecentQuestions(ctx context.Context, studentID uuid.UUID, since time.Time) ([]uuid.UUID, error)
}

// QuestionPlanConfig are the parameters of the plans (CONFIG_DEFAULTS through module.go).
type QuestionPlanConfig struct {
	// AvoidRecent is avoid_recent.window_days of QUESTION_BANK §6 (7 days, COMPETITION_SYSTEM §3.1).
	AvoidRecent time.Duration
	// Slack is added to the playing time of the plan for its expires_at (lobby, countdown, reveal pauses, failover).
	Slack time.Duration
	// RevealFor is the pause after each question (reveal, next question) counted in the playing time.
	RevealFor time.Duration
}

// DefaultQuestionPlanConfig: avoid_recent 7 days, 15 min slack, 10 s per question for reveal and transitions [Proposed F105].
func DefaultQuestionPlanConfig() QuestionPlanConfig {
	return QuestionPlanConfig{AvoidRecent: 7 * 24 * time.Hour, Slack: 15 * time.Minute, RevealFor: 10 * time.Second}
}

// QuestionPlans holds the write use cases of the question plans; Attempts provides the attempt engine (store, delivery,
// learning events) and Plans stores the plans.
type QuestionPlans struct {
	Attempts Attempts
	Plans    write.QuestionPlanStore
	Recent   RecentQuestionReader
	Config   QuestionPlanConfig
}

// CreatePlanInput is QuestionPlanRequest of createQuestionPlan.
type CreatePlanInput struct {
	MatchID          uuid.UUID
	Mode             string
	GradeLevel       int
	SubjectID        uuid.UUID
	TopicIDs         []uuid.UUID
	DifficultyMin    int
	DifficultyMax    int
	QuestionCount    int
	TimePerQuestionS int
	Participants     []uuid.UUID
	// F103 (boss rounds of module game, research R1): the context of the attempts (boss_encounter + encounter id), the
	// attempt mode (challenge) and the published activity whose question source the plan draws from (subject not needed).
	ContextType     string
	ContextID       uuid.UUID
	AttemptMode     string
	ActivityID      *uuid.UUID
	ActivityVersion *int
}

// Boss plans (F103): the context and attempt mode module game asks for.
const (
	PlanContextBoss  = "boss_encounter"
	PlanModeBoss     = "boss"
	PlanAttemptBoss  = "challenge"
	planAttemptMatch = "competition"
)

// boss reports a plan of a boss round.
func (in CreatePlanInput) boss() bool { return in.ContextType == PlanContextBoss }

// PlanItemView is one question of a plan as module competition receives it: render data only (no answer data).
type PlanItemView struct {
	Index             int
	QuestionVersionID uuid.UUID
	QuestionType      string
	TimeLimitSeconds  int
	RenderPayload     map[string]any
}

// PlanView is QuestionPlan of createQuestionPlan.
type PlanView struct {
	PlanID      uuid.UUID
	MatchID     uuid.UUID
	PlanVersion string
	ExpiresAt   time.Time
	Items       []PlanItemView
}

// PlanScore is PlanAnswerScore of scorePlanAnswer.
type PlanScore struct {
	PlanID          uuid.UUID `json:"plan_id"`
	Index           int       `json:"index"`
	StudentID       uuid.UUID `json:"student_id"`
	IsCorrect       bool      `json:"is_correct"`
	ScoreFraction   float64   `json:"score_fraction"`
	AttemptAnswerID uuid.UUID `json:"attempt_answer_id"`
	Late            bool      `json:"late"`
	// RapidGuess: answered faster than rapid_guess_min_response_ms (never for mode competition, F103 anti-cheat).
	RapidGuess bool `json:"rapid_guess"`
}

// ScorePlanInput is PlanAnswerScoreRequest of scorePlanAnswer.
type ScorePlanInput struct {
	PlanID          uuid.UUID
	Index           int
	StudentID       uuid.UUID
	Response        json.RawMessage
	ReceivedAt      time.Time
	ClientElapsedMS *int64
}

var planModes = []string{"battle_1v1", "team_battle", "coop_challenge", "timed_quiz", "tournament", PlanModeBoss}

func planTenant(ctx context.Context) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, problem.New(problem.Internal, "question plans need the tenant scope of the match")
	}
	id, single := s.SessionTenantID()
	if !single {
		return uuid.Nil, problem.New(problem.Internal, "question plans need one session tenant")
	}
	return id, nil
}

// planSeed is the seed of the plan of a match: the same match always draws the same questions (retries, failover).
func planSeed(match uuid.UUID) int64 {
	return int64(binary.BigEndian.Uint64(match[8:]) & 0x7fffffffffffffff)
}

// planVersion pins the question versions of a plan (competition.matches.question_plan_version, ADR-0043).
func planVersion(items []write.QuestionPlanItem) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = it.QuestionVersionID.String()
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return "qp1-" + hex.EncodeToString(sum[:12])
}

func (in CreatePlanInput) validate() error {
	switch {
	case in.MatchID == uuid.Nil:
		return problem.Invalid("/match_id", problem.SubRequired, "is required")
	case !slices.Contains(planModes, in.Mode):
		return problem.Invalid("/mode", problem.SubInvalid, "is not a competition mode")
	case in.GradeLevel < 1 || in.GradeLevel > 9:
		return problem.Invalid("/grade_level", problem.SubInvalid, "is between 1 and 9")
	case in.ContextType != "" && in.ContextType != planContext && in.ContextType != PlanContextBoss:
		return problem.Invalid("/context_type", problem.SubInvalid, "match or boss_encounter")
	case in.boss() && (in.ContextID == uuid.Nil || in.ActivityID == nil || in.Mode != PlanModeBoss || in.AttemptMode != PlanAttemptBoss ||
		len(in.Participants) != 1):
		return problem.Invalid("/context_id", problem.SubInvalid, "a boss round needs context_id, activity_ref, mode boss, attempt_mode challenge and one participant")
	case !in.boss() && (in.Mode == PlanModeBoss || (in.AttemptMode != "" && in.AttemptMode != planAttemptMatch)):
		return problem.Invalid("/mode", problem.SubInvalid, "mode boss and attempt_mode challenge are for boss_encounter plans")
	case in.SubjectID == uuid.Nil && in.ActivityID == nil:
		return problem.Invalid("/subject_id", problem.SubRequired, "is required")
	case in.QuestionCount < 1 || in.QuestionCount > 30:
		return problem.Invalid("/question_count", problem.SubInvalid, "is between 1 and 30")
	case in.TimePerQuestionS < 5 || in.TimePerQuestionS > 120:
		return problem.Invalid("/time_per_question_seconds", problem.SubInvalid, "is between 5 and 120")
	case len(in.Participants) < 1 || len(in.Participants) > 40:
		return problem.Invalid("/participants", problem.SubInvalid, "has 1 to 40 students")
	}
	return nil
}

// CreatePlan implements createQuestionPlan (F105-AC-01, AC-02): published questions visible to the tenant of the match
// (own or assigned, F022 Delivery), of the grade, subject and topics, avoiding what a participant met in the last 7 days,
// pinned at their version; render payloads only. Idempotent by match: a second call returns the same plan.
func (h QuestionPlans) CreatePlan(ctx context.Context, in CreatePlanInput) (out PlanView, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.CreateQuestionPlan")
	defer func() { done(err) }()
	if err := in.validate(); err != nil {
		return out, err
	}
	tenant, err := planTenant(ctx)
	if err != nil {
		return out, err
	}
	a := h.Attempts
	err = a.UoW.Do(ctx, func(ctx context.Context) error {
		prev, err := h.Plans.PlanByMatch(ctx, in.MatchID)
		if err != nil {
			return err
		}
		if prev != nil {
			out, err = h.view(ctx, *prev)
			return err
		}
		now := a.now()
		seed := planSeed(in.MatchID)
		schools := make(map[uuid.UUID]uuid.UUID, len(in.Participants))
		var recent []uuid.UUID
		for _, s := range in.Participants {
			school, _, err := a.Students.StudentSchool(ctx, s)
			if err != nil {
				return err
			}
			if school == nil {
				return problem.Invalid("/participants", problem.SubInvalid, "a participant is not a student of the tenant")
			}
			schools[s] = *school
			if h.Recent != nil {
				seen, err := h.Recent.RecentQuestions(ctx, s, now.Add(-h.Config.AvoidRecent))
				if err != nil {
					return err
				}
				recent = append(recent, seen...)
			}
		}
		var qids []uuid.UUID
		var allowed []string
		subject := in.SubjectID
		if in.ActivityID != nil {
			// F103: the questions of the published activity of the boss requirement (its question source), at most
			// question_count, for the grade module game chose (per_grade).
			src, err := h.activitySource(ctx, *in.ActivityID, in.ActivityVersion)
			if err != nil {
				return err
			}
			allowed = src.Plugin.AllowedQuestionTypes(src.Config)
			if qids, err = a.sourceQuestions(ctx, src.Config.Content.QuestionSource, src.Version.SubjectID, src.Version.GradeLevels, allowed,
				seed); err != nil {
				return err
			}
			if len(qids) > in.QuestionCount {
				qids = qids[:in.QuestionCount]
			}
			if len(qids) == 0 {
				return problem.New(problem.QuestionPoolInsufficient, "the activity has no question")
			}
			if src.Version.SubjectID != nil {
				subject = *src.Version.SubjectID
			}
		} else {
			f := selection.Filters{SubjectID: &in.SubjectID, GradeLevels: []int{in.GradeLevel}, TopicIDs: in.TopicIDs}
			if in.DifficultyMin > 0 || in.DifficultyMax > 0 {
				f.Difficulty = &selection.Range{Min: in.DifficultyMin, Max: in.DifficultyMax}
			}
			cs, err := a.Delivery.Select(ctx, selection.Request{Rule: selection.Rule{Filters: f, Count: in.QuestionCount}, Seed: uint64(seed), Recent: recent})
			if err != nil {
				return err
			}
			qids = versionsOf(cs)
		}
		items, rendered, err := a.pinItems(ctx, qids, seed, false, allowed)
		if err != nil {
			return err
		}
		planID := ids.New()
		row := write.QuestionPlanRow{PlanID: planID, TenantID: tenant, MatchID: in.MatchID, Mode: in.Mode, GradeLevel: in.GradeLevel,
			SubjectID: subject, Seed: seed, Participants: in.Participants, CreatedAt: now}
		for i, it := range items {
			row.Items = append(row.Items, write.QuestionPlanItem{Index: i, QuestionVersionID: it.QuestionVersionID, QuestionType: it.QuestionType,
				TimeLimitSeconds: in.TimePerQuestionS, MaxScore: it.MaxScore, Presentation: it.Presentation, ObjectiveIDs: it.ObjectiveIDs})
		}
		row.PlanVersion = planVersion(row.Items)
		play := time.Duration(len(items)) * (time.Duration(in.TimePerQuestionS)*time.Second + h.Config.RevealFor)
		row.ExpiresAt = now.Add(play + h.Config.Slack)
		if err := h.Plans.InsertPlan(ctx, row); err != nil {
			return err
		}
		// One attempt mode competition per participant (context match): its items are the plan. A boss round (F103) opens
		// one attempt mode challenge, context boss_encounter of the encounter: the learning record of that student only.
		mode, ctxType, ctxID := attempt.ModeCompetition, planContext, in.MatchID
		if in.boss() {
			mode, ctxType, ctxID = attempt.ModeChallenge, PlanContextBoss, in.ContextID
		}
		for _, s := range in.Participants {
			own := make([]attempt.Item, len(items))
			for i, it := range items {
				it.ID = ids.New()
				own[i] = it
			}
			att := attempt.New(attempt.Start{ID: ids.New(), TenantID: tenant, SchoolID: schools[s], StudentID: s, Mode: mode,
				ContextType: ctxType, ContextID: ctxID, ActivityType: "quiz", SourceKind: "selection", SourceVersionID: planID, Seed: seed,
				Now: now, ActivityFeedback: string(attempt.FeedbackAfterClose), IdempotencyKey: planAttemptKey(planID), Items: own})
			if err := a.Store.InsertAttempt(ctx, att); err != nil {
				return err
			}
		}
		out = planView(row, rendered)
		return nil
	})
	return out, err
}

// planAttemptKey is the idempotency key of the match attempt of a plan (attempts_idem_uq per student).
func planAttemptKey(plan uuid.UUID) string { return "plan:" + plan.String() }

// view renders an existing plan again (same seed: same presentation, F022-AC-25).
func (h QuestionPlans) view(ctx context.Context, p write.QuestionPlanRow) (PlanView, error) {
	vs := make([]uuid.UUID, len(p.Items))
	for i, it := range p.Items {
		vs[i] = it.QuestionVersionID
	}
	rendered, _, err := h.Attempts.Delivery.Render(ctx, vs, uint64(p.Seed))
	if err != nil {
		return PlanView{}, err
	}
	return planView(p, rendered), nil
}

func planView(p write.QuestionPlanRow, rendered []question.Rendered) PlanView {
	out := PlanView{PlanID: p.PlanID, MatchID: p.MatchID, PlanVersion: p.PlanVersion, ExpiresAt: p.ExpiresAt}
	for i, it := range p.Items {
		payload := map[string]any{}
		if i < len(rendered) {
			for k, v := range rendered[i].Payload {
				payload[k] = v
			}
			payload["stem"] = rendered[i].Stem
		}
		out.Items = append(out.Items, PlanItemView{Index: it.Index, QuestionVersionID: it.QuestionVersionID, QuestionType: it.QuestionType,
			TimeLimitSeconds: it.TimeLimitSeconds, RenderPayload: payload})
	}
	return out
}

// ScoreAnswer implements scorePlanAnswer (F105-AC-04, AC-09): grades the response with the answer key of the pinned
// version, records it as attempt_answer of the match attempt of the student (mode competition) with its learning event,
// once per (plan, index, student): a second call returns the first result. An answer received after expires_at is late
// and not scored.
func (h QuestionPlans) ScoreAnswer(ctx context.Context, in ScorePlanInput) (out PlanScore, err error) {
	ctx, done := observability.UseCase(ctx, "learning.facade.ScorePlanAnswer")
	defer func() { done(err) }()
	if len(in.Response) == 0 || string(in.Response) == "null" {
		return out, problem.New(problem.InvalidResponse, "the response is missing")
	}
	// Concurrent calls for one student race on the row versions of the attempt and the item: retry, the second run
	// finds the answer recorded.
	for try := 0; ; try++ {
		out, err = h.scoreOnce(ctx, in)
		var pe *problem.Error
		if try < 3 && errors.As(err, &pe) && pe.Code == problem.VersionMismatch {
			continue
		}
		return out, err
	}
}

func (h QuestionPlans) scoreOnce(ctx context.Context, in ScorePlanInput) (out PlanScore, err error) {
	a := h.Attempts
	err = a.UoW.Do(ctx, func(ctx context.Context) error {
		p, err := h.Plans.Plan(ctx, in.PlanID)
		if err != nil {
			return err
		}
		if p == nil || !slices.Contains(p.Participants, in.StudentID) {
			return problem.New(problem.NotFound, "no such plan for this student")
		}
		if in.Index < 0 || in.Index >= len(p.Items) {
			return problem.Invalid("/index", problem.SubInvalid, "is not a question of the plan")
		}
		att, err := a.Store.AttemptByKey(ctx, in.StudentID, planAttemptKey(p.PlanID))
		if err != nil {
			return err
		}
		if att == nil {
			return problem.New(problem.NotFound, "no match attempt for this student")
		}
		pos := in.Index + 1
		it, err := att.Item(pos)
		if err != nil {
			return problem.Invalid("/index", problem.SubInvalid, "is not a question of the plan")
		}
		out = PlanScore{PlanID: p.PlanID, Index: in.Index, StudentID: in.StudentID}
		if it.AnswerCount > 0 { // idempotent: the first answer stays (COMPETITION_SYSTEM §4)
			if it.FinalIsCorrect == nil {
				out.Late = true
				return nil
			}
			out.IsCorrect = *it.FinalIsCorrect
			if it.FinalScore != nil && it.MaxScore > 0 {
				out.ScoreFraction = min(max(*it.FinalScore/it.MaxScore, 0), 1)
			}
			return nil
		}
		late := in.ReceivedAt.After(p.ExpiresAt)
		var pres question.Presentation
		_ = json.Unmarshal(it.Presentation, &pres)
		g, err := a.Delivery.Grade(ctx, it.QuestionVersionID, pres, in.Response)
		if err != nil {
			return err
		}
		now := a.now()
		ans, err := att.Respond(pos, now, a.policy(), 1, false, in.ClientElapsedMS)
		if err != nil {
			return attemptProblem(err)
		}
		ans.IsScored = !late
		grade := p.GradeLevel
		rapid := attempt.DefaultRapidGuess().Flag(attempt.GuessInput{Mode: att.Mode, QuestionType: it.QuestionType, Grade: &grade,
			ResponseMS: ans.ResponseMS, Scored: ans.IsScored})
		score := min(max(g.Score, 0), it.MaxScore)
		if late {
			score = 0
		}
		att.Record(ans, g.IsCorrect && !late, score)
		it, _ = att.Item(pos)
		if err := a.Store.UpdateItem(ctx, *it); err != nil {
			return err
		}
		if err := a.Store.UpdateAttempt(ctx, *att, att.RowVersion); err != nil {
			return err
		}
		answerID := ids.New()
		key := "plan:" + p.PlanID.String()[:8] + ":" + itoa(in.Index)
		if err := a.Store.InsertAnswer(ctx, write.AnswerRow{ID: answerID, TenantID: att.TenantID, SchoolID: att.SchoolID, AttemptID: att.ID,
			Position: pos, QuestionVersionID: it.QuestionVersionID, Response: in.Response, IsCorrect: g.IsCorrect && !late, Score: score,
			AnsweredAt: in.ReceivedAt.UTC(), ClientElapsedMS: in.ClientElapsedMS, ResponseMS: ans.ResponseMS, AttemptNo: ans.AttemptNo,
			IsScored: ans.IsScored, IdempotencyKey: key}); err != nil {
			return err
		}
		if err := a.learningEvent(ctx, *att, EventQuestionAnswered, "qa:"+att.ID.String()+":"+itoa(pos)+":1", in.ReceivedAt.UTC(),
			it.ObjectiveIDs, "student", map[string]any{"attempt_id": att.ID, "position": pos, "question_version_id": it.QuestionVersionID,
				"question_type": it.QuestionType, "is_correct": g.IsCorrect && !late, "is_scored": ans.IsScored,
				"score": learningScore(score, it.MaxScore), "response_ms": ans.ResponseMS, "attempt_no": ans.AttemptNo, "hints_used": 0,
				"rapid_guess": rapid}); err != nil {
			return err
		}
		out.IsCorrect, out.Late, out.AttemptAnswerID, out.RapidGuess = g.IsCorrect && !late, late, answerID, rapid
		if it.MaxScore > 0 {
			out.ScoreFraction = min(max(score/it.MaxScore, 0), 1)
		}
		return nil
	})
	return out, err
}

// activitySource loads the published activity of a boss requirement (F103): the version number given (still published)
// or the current release.
func (h QuestionPlans) activitySource(ctx context.Context, id uuid.UUID, versionNo *int) (ActivitySource, error) {
	a := h.Attempts
	if versionNo == nil {
		return LoadActivityVersion(ctx, a.Docs, a.Registry, id, nil)
	}
	v, err := a.Docs.DocVersion(ctx, contentmodel.TypeActivity, id, *versionNo)
	if err != nil {
		return ActivitySource{}, err
	}
	if v.Status != contentmodel.StatusPublished {
		return ActivitySource{}, problem.New(problem.ContentNotPublished, "the activity version is not published")
	}
	return LoadActivityVersion(ctx, a.Docs, a.Registry, id, &v.ID)
}
