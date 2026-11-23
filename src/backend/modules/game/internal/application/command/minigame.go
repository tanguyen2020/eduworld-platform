package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/minigame"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EventMiniGameCompleted is the end of a graded play (specs/events/payloads/game/minigame.completed.v1, owner F073).
const EventMiniGameCompleted = "minigame.completed"

// MiniGameMetrics observes the mini-game engine (eduworld_game_minigame_start_total{type,result},
// eduworld_game_minigame_completed_total{type}); nil = not observed.
type MiniGameMetrics interface {
	Start(minigameType, result string)
	Completed(minigameType string)
}

// MiniGames are the mini-game use cases of module game (F073 US2, US4): startMiniGame (a free play) and the consumer
// game.minigame_on_activity_graded. Module game pins the config version, asks module learning for the attempt
// (createAttempt, plugin minigame) and records the result of the play; it never grades, never scores and never grants
// XP/Xu/item (F073-AC-20: the Reward Engine applies reward_rule_id on minigame.completed).
type MiniGames struct {
	UoW      write.UnitOfWork
	Content  port.MiniGameContent
	World    port.WorldContent
	Plays    write.MiniGamePlayStore
	Events   write.EventPublisher
	Learning port.LearningAttempts // nil: startMiniGame answers 503
	Metrics  MiniGameMetrics
	// LearningRetry is the Retry-After of startMiniGame when module learning or its projection is not ready.
	LearningRetry time.Duration
	Now           func() time.Time
}

func (h MiniGames) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h MiniGames) metric(fn func(m MiniGameMetrics)) {
	if h.Metrics != nil {
		fn(h.Metrics)
	}
}

// ict is the day of the play limits (Indochina Time, data-model F073 play_date).
var playZone = time.FixedZone("ICT", 7*3600)

// PlayDate is the ICT day of t (YYYY-MM-DD).
func PlayDate(t time.Time) string { return t.In(playZone).Format("2006-01-02") }

// playNS namespaces the play id derived from the Idempotency-Key of startMiniGame (a retried start reuses the play and
// the attempt of module learning).
var playNS = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f073")

// MiniGameSession is MiniGameSession of startMiniGame (F073-AC-06): the Attempt (the client reads its items through
// GET /learning/activity-attempts/{id}), the pinned version and the display config — never an answer (AC-07).
type MiniGameSession struct {
	Attempt         MiniGameAttempt `json:"attempt"`
	PlayID          uuid.UUID       `json:"play_id"`
	MiniGameType    string          `json:"minigame_type"`
	ConfigVersionID uuid.UUID       `json:"minigame_config_version_id"`
	DeadlineAt      *time.Time      `json:"deadline_at,omitempty"`
	ServerTime      time.Time       `json:"server_time"`
	Config          map[string]any  `json:"config"`
}

// MiniGameAttempt is the Attempt part of MiniGameSession.
type MiniGameAttempt struct {
	AttemptID         uuid.UUID  `json:"attempt_id"`
	ActivityType      string     `json:"activity_type"`
	ActivityVersionID uuid.UUID  `json:"activity_version_id"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	ContextType       string     `json:"context_type"`
	ContextID         uuid.UUID  `json:"context_id"`
	StartedAt         time.Time  `json:"started_at"`
	DeadlineAt        *time.Time `json:"deadline_at,omitempty"`
	ServerTime        time.Time  `json:"server_time"`
}

// StartMiniGameInput is the body of startMiniGame.
type StartMiniGameInput struct {
	ConfigID    uuid.UUID
	ContextType string
	ContextID   *uuid.UUID
}

// Start implements startMiniGame (F073-AC-06, AC-12, AC-13): a student of the session tenant (403 otherwise) starts a
// config published, assigned to the tenant and of the grade of the student (404 otherwise); context assignment is 422
// (an assigned mini-game runs through startActivityAttempt). The current version is pinned, createAttempt of module
// learning runs outside the transaction with the Idempotency-Key forwarded (503 Retry-After when module learning or its
// projection is not ready, nothing recorded), then the play is recorded.
func (h MiniGames) Start(ctx context.Context, in StartMiniGameInput) (out MiniGameSession, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.StartMiniGame")
	defer func() { done(err) }()
	student, tenant, err := player(ctx)
	if err != nil {
		return out, err
	}
	switch in.ContextType {
	case "":
		in.ContextType = minigame.ContextFreePlay
	case minigame.ContextFreePlay:
	case minigame.ContextQuestStep:
		if in.ContextID == nil {
			return out, problem.Invalid("/context_id", problem.SubRequired, "is required with context_type quest_step")
		}
	case minigame.ContextAssignment:
		return out, problem.Invalid("/context_type", problem.SubInvalid, "an assigned mini-game starts through startActivityAttempt")
	default:
		return out, problem.Invalid("/context_type", problem.SubInvalid, "free_play or quest_step")
	}
	if in.ContextType == minigame.ContextFreePlay {
		in.ContextID = nil
	}
	at := h.now()
	playID := newID()
	if p, ok := idempotency.PendingFrom(ctx); ok {
		playID = uuid.NewSHA1(playNS, []byte(tenant.String()+":"+student.String()+":"+p.Key.Key.String()))
	}
	var row *port.MiniGameRow
	var st *port.WorldStudent
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if row, err = h.Content.CurrentMiniGame(ctx, in.ConfigID); err != nil {
			return err
		}
		if row == nil {
			return problem.New(problem.NotFound, "mini-game not found")
		}
		ok, err := h.Content.MiniGameAssigned(ctx, tenant, in.ConfigID)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.NotFound, "mini-game not found") // not assigned to the tenant (ADR-0029)
		}
		if st, err = h.World.Student(ctx, tenant, student); err != nil {
			return err
		}
		if st == nil || st.Deleted || st.Status != "active" || !row.Config.ForGrade(st.Grade) {
			return problem.New(problem.NotFound, "mini-game not found") // another grade: not offered (F073-AC-12)
		}
		return nil
	})
	if err != nil {
		h.metric(func(m MiniGameMetrics) { m.Start("unknown", "refused") })
		return out, err
	}
	typ := row.Config.Type
	if h.Learning == nil {
		h.metric(func(m MiniGameMetrics) { m.Start(typ, "unavailable") })
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning is not wired"), After: h.LearningRetry}
	}
	vid := row.VersionID
	created, err := h.Learning.CreateAttempt(ctx, port.LearningAttempt{StudentID: student, ContentID: in.ConfigID, ContentVersionID: &vid,
		ContentType: port.ContentMiniGameConfig, MiniGamePlayID: &playID})
	if errors.Is(err, port.ErrLearningUnavailable) {
		h.metric(func(m MiniGameMetrics) { m.Start(typ, "unavailable") })
		return out, RetryAfter{Err: problem.New(problem.Unavailable, "module learning is not ready for this mini-game").Wrap(err), After: h.LearningRetry}
	}
	if err != nil {
		h.metric(func(m MiniGameMetrics) { m.Start(typ, "refused") })
		return out, err
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Plays.LockStudent(ctx, student); err != nil {
			return err
		}
		school := st.SchoolID
		if _, err := h.Plays.InsertPlay(ctx, write.MiniGamePlay{ID: playID, TenantID: tenant, StudentID: student, SchoolID: &school,
			ConfigID: in.ConfigID, ConfigVersionID: vid, MiniGameType: typ, AttemptID: created.AttemptID, ContextType: in.ContextType,
			ContextID: in.ContextID, PlayDate: PlayDate(at), Status: minigame.StatusInProgress, StartedAt: at}); err != nil {
			return err
		}
		grade := st.Grade
		out = MiniGameSession{PlayID: playID, MiniGameType: typ, ConfigVersionID: vid, DeadlineAt: created.DeadlineAt, ServerTime: at,
			Config: row.Config.Display(&grade),
			Attempt: MiniGameAttempt{AttemptID: created.AttemptID, ActivityType: "minigame", ActivityVersionID: vid, Mode: "minigame",
				Status: created.Status, ContextType: "minigame_session", ContextID: playID, StartedAt: created.StartedAt, DeadlineAt: created.DeadlineAt,
				ServerTime: at}}
		return setResult(ctx, out)
	})
	if err == nil {
		h.metric(func(m MiniGameMetrics) { m.Start(typ, "ok") })
	}
	return out, err
}

// MiniGameGraded is the minigame block of activity.graded of module learning with the attempt fields module game reads.
type MiniGameGraded struct {
	TenantID        uuid.UUID
	AttemptID       uuid.UUID
	StudentID       uuid.UUID
	ConfigID        uuid.UUID
	ConfigVersionID uuid.UUID
	ContextType     string // context_type of the attempt (minigame_session, quest_step, assignment…)
	ContextID       uuid.UUID
	GameScore       int
	CompletionMet   bool
	Score           float64
	MaxScore        float64
	CorrectCount    *int
	ItemCount       *int
	OccurredAt      time.Time
}

// playContext maps the context of an attempt to the context of a play.
func playContext(attemptContext string) string {
	switch attemptContext {
	case "quest_step":
		return minigame.ContextQuestStep
	case "assignment":
		return minigame.ContextAssignment
	}
	return minigame.ContextFreePlay
}

// RecordResult is the consumer game.minigame_on_activity_graded (F073-AC-18, AC-19, AC-20): the graded attempt of a
// mini-game updates its play once (a replayed event finds the play completed and publishes nothing) and writes
// minigame.completed to the outbox of the same transaction, with rewards_eligible = completion_met and the plays of the
// day within limits.max_plays_per_day (the play still counts as practice). A play of a quest step or of an assignment has
// no row yet: it is created from the event. No XP, Xu or item here. Runs in the consumer transaction.
func (h MiniGames) RecordResult(ctx context.Context, g MiniGameGraded) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RecordMiniGameResult")
	defer func() { done(err) }()
	if err := h.Plays.LockStudent(ctx, g.StudentID); err != nil {
		return err
	}
	at := h.now()
	occurred := g.OccurredAt
	if occurred.IsZero() {
		occurred = at
	}
	p, err := h.Plays.PlayByAttempt(ctx, g.TenantID, g.AttemptID)
	if err != nil {
		return err
	}
	row, err := h.Content.MiniGameVersion(ctx, g.ConfigVersionID)
	if err != nil {
		return err
	}
	if row == nil {
		return problem.New(problem.NotFound, "the pinned mini-game version is unknown")
	}
	if p == nil {
		ctxType := playContext(g.ContextType)
		var ctxID *uuid.UUID
		if ctxType != minigame.ContextFreePlay && g.ContextID != uuid.Nil {
			id := g.ContextID
			ctxID = &id
		}
		np := write.MiniGamePlay{ID: newID(), TenantID: g.TenantID, StudentID: g.StudentID, ConfigID: g.ConfigID, ConfigVersionID: g.ConfigVersionID,
			MiniGameType: row.Config.Type, AttemptID: g.AttemptID, ContextType: ctxType, ContextID: ctxID, PlayDate: PlayDate(occurred),
			Status: minigame.StatusInProgress, StartedAt: at}
		if _, err := h.Plays.InsertPlay(ctx, np); err != nil {
			return err
		}
		if p, err = h.Plays.PlayByAttempt(ctx, g.TenantID, g.AttemptID); err != nil || p == nil {
			return err
		}
	}
	if p.Status == minigame.StatusCompleted || p.StudentID != g.StudentID {
		return nil // replayed event, or an attempt of another student
	}
	before, err := h.Plays.CountCompletedOn(ctx, g.TenantID, g.StudentID, p.ConfigID, p.PlayDate)
	if err != nil {
		return err
	}
	plays := before + 1
	eligible := row.Config.RewardsEligible(g.CompletionMet, plays)
	score, maxScore, gameScore, met := g.Score, g.MaxScore, max(g.GameScore, 0), g.CompletionMet
	p.Status, p.LearningScore, p.MaxScore, p.GameScore, p.CompletionMet, p.RewardsEligible = minigame.StatusCompleted, &score, &maxScore,
		&gameScore, &met, &eligible
	p.CorrectCount, p.ItemCount, p.CompletedAt = g.CorrectCount, g.ItemCount, &at
	if err := h.Plays.CompletePlay(ctx, *p); err != nil {
		return err
	}
	tenant := g.TenantID
	grades := row.Config.GradeLevels
	if grades == nil {
		grades = []int{}
	}
	subject := row.Config.SubjectID
	if err := h.Events.Publish(ctx, write.Event{Type: EventMiniGameCompleted, SubjectType: "minigame_play", SubjectID: p.ID.String(),
		PartitionKey: p.StudentID.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{"play_id": p.ID, "student_id": p.StudentID,
			"minigame_config_id": p.ConfigID, "minigame_config_version_id": p.ConfigVersionID, "minigame_type": p.MiniGameType,
			"attempt_id": p.AttemptID, "context_type": p.ContextType, "context_id": p.ContextID, "learning_score": score, "max_score": maxScore,
			"game_score": gameScore, "correct_count": g.CorrectCount, "item_count": g.ItemCount, "completion_met": met,
			"rewards_eligible": eligible, "plays_today": plays, "reward_rule_id": row.Config.RewardRuleID, "grade_levels": grades,
			"subject_id": &subject, "completed_at": at}}); err != nil {
		return err
	}
	typ := p.MiniGameType
	h.metric(func(m MiniGameMetrics) { m.Completed(typ) })
	return nil
}
