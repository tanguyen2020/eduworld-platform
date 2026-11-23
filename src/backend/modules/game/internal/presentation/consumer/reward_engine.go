package consumer

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/reward"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// Awarder is the Reward Engine use case (command.Progression).
type Awarder interface {
	AwardFromEvent(ctx context.Context, tenant, student uuid.UUID, t reward.Trigger) error
}

// sourceNS is the UUIDv5 namespace of the string sources of XP (interactables, areas): data-model xp_ledger.source_id.
var sourceNS = uuid.MustParse("0193a5f4-6c1e-7b2a-9d1f-00000000f074")

// RewardEngine is the consumer game.reward_engine (learning stream) and game.reward_engine_game (game stream) of F074
// (T020, F074-AC-01…AC-08, AC-11, AC-27): it maps a verified server event to a reward.Trigger — the client never sends
// XP (R5) — and calls the Reward Engine in the consumer transaction. A student the projection of the event tenant does
// not hold is retried, then dead-lettered (pkg/outbox.Consumer); an invalid payload is dead-lettered at once.
type RewardEngine struct {
	Awards Awarder
	Name   string // consumer name (metrics)
}

// RewardEngineOptions returns the consumer options of game.reward_engine (tenant events of the learning stream).
func RewardEngineOptions(h RewardEngine, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.RewardEngineName
	return outbox.ConsumerOptions{Module: "game", Name: messaging.RewardEngineName, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.RewardEngineLearningEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// RewardEngineGameOptions returns the consumer options of game.reward_engine_game (tenant events of the game stream).
func RewardEngineGameOptions(h RewardEngine, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.RewardEngineGameName
	return outbox.ConsumerOptions{Module: "game", Name: messaging.RewardEngineGameName, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.RewardEngineGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

func (c RewardEngine) name() string {
	if c.Name == "" {
		return messaging.RewardEngineName
	}
	return c.Name
}

// Handle implements outbox.Handler.
func (c RewardEngine) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(c.name(), invalid(e.EventType, "tenant event expected"))
	}
	student, t, err := Trigger(e)
	if err != nil {
		return reject(c.name(), err)
	}
	return c.Awards.AwardFromEvent(ctx, *e.TenantID, student, t)
}

// Trigger maps an event of the Reward Engine to the student and the trigger (exported for tests).
func Trigger(e outbox.Envelope) (uuid.UUID, reward.Trigger, error) {
	t := reward.Trigger{EventType: e.EventType, EventID: e.EventID, OccurredAt: e.OccurredAt.UTC()}
	switch e.EventType {
	case "activity.graded":
		var p struct {
			AttemptID        uuid.UUID  `json:"attempt_id"`
			StudentID        uuid.UUID  `json:"student_id"`
			ActivityID       *uuid.UUID `json:"activity_id"`
			Mode             string     `json:"mode"`
			Passed           *bool      `json:"passed"`
			CorrectCount     *int       `json:"correct_count"`
			ItemCount        *int       `json:"item_count"`
			FirstTryCorrect  *int       `json:"first_try_correct_count"`
			RapidGuess       bool       `json:"rapid_guess_flagged"`
			ContentVersionID *uuid.UUID `json:"content_version_id"`
			ContentGrades    []int      `json:"content_grade_levels"`
			RecommendationID *uuid.UUID `json:"recommendation_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.AttemptID == uuid.Nil || p.StudentID == uuid.Nil || len(p.Mode) > 64 || !grades(p.ContentGrades) {
			return uuid.Nil, t, invalid(e.EventType, "attempt_id, student_id and a valid mode are required")
		}
		t.SourceType, t.SourceID, t.ContentID, t.ContentVersionID, t.Mode = "activity", p.AttemptID, p.ActivityID, p.ContentVersionID, p.Mode
		t.CorrectCount, t.ItemCount, t.FirstTryCorrect, t.RapidGuess, t.ContentGrades = p.CorrectCount, p.ItemCount, p.FirstTryCorrect, p.RapidGuess, p.ContentGrades
		t.RecommendationID = p.RecommendationID
		if p.Passed == nil || *p.Passed { // activity.completed needs the pass rule met (PROGRESSION_SYSTEM §3.1)
			t.SourceCodes = []string{"activity.completed"}
		}
		if p.Mode == "minigame" {
			// F073: a free mini-game play is rewarded once through minigame.completed (reward_rule_id, plays of the day), never
			// a second time through its activity.graded.
			t.SourceCodes = nil
		}
		return p.StudentID, t, nil
	case "lesson.completed":
		var p struct {
			StudentID        uuid.UUID  `json:"student_id"`
			LessonID         uuid.UUID  `json:"lesson_id"`
			LessonVersionID  uuid.UUID  `json:"lesson_version_id"`
			SubjectID        *uuid.UUID `json:"subject_id"`
			GradeLevels      []int      `json:"grade_levels"`
			Mode             string     `json:"mode"`
			RecommendationID *uuid.UUID `json:"recommendation_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.StudentID == uuid.Nil || p.LessonVersionID == uuid.Nil || len(p.Mode) > 64 || !grades(p.GradeLevels) {
			return uuid.Nil, t, invalid(e.EventType, "student_id and lesson_version_id are required")
		}
		lesson, version := p.LessonID, p.LessonVersionID
		t.SourceType, t.SourceID, t.ContentID, t.ContentVersionID = "lesson", p.LessonVersionID, &lesson, &version
		t.SubjectID, t.ContentGrades, t.Mode, t.RecommendationID = p.SubjectID, p.GradeLevels, p.Mode, p.RecommendationID
		t.SourceCodes = []string{"lesson.completed"}
		return p.StudentID, t, nil
	case "learning_objective.mastered":
		var p struct {
			StudentID   uuid.UUID `json:"student_id"`
			ObjectiveID uuid.UUID `json:"objective_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.StudentID == uuid.Nil || p.ObjectiveID == uuid.Nil {
			return uuid.Nil, t, invalid(e.EventType, "student_id and objective_id are required")
		}
		obj := p.ObjectiveID
		// One posting per (student, objective): the source key, whatever the event id (repeat, decay, F074-AC-08).
		t.SourceType, t.SourceID, t.ContentID, t.SourceCodes = "objective", p.ObjectiveID, &obj, []string{"mastery.objective_mastered"}
		return p.StudentID, t, nil
	case "quest.completed":
		var p struct {
			QuestProgressID uuid.UUID  `json:"quest_progress_id"`
			QuestID         uuid.UUID  `json:"quest_id"`
			QuestVersionID  uuid.UUID  `json:"quest_version_id"`
			QuestType       string     `json:"quest_type"`
			StudentID       uuid.UUID  `json:"student_id"`
			SubjectID       *uuid.UUID `json:"subject_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.StudentID == uuid.Nil || p.QuestProgressID == uuid.Nil || p.QuestID == uuid.Nil || len(p.QuestType) > 64 {
			return uuid.Nil, t, invalid(e.EventType, "quest_progress_id, quest_id and student_id are required")
		}
		quest := p.QuestID
		t.SourceType, t.SourceID, t.ContentID, t.SubjectID = "quest", p.QuestProgressID, &quest, p.SubjectID
		switch p.QuestType {
		case "main":
			t.SourceCodes = []string{"quest.main_completed"}
		case "side", "learning", "exploration":
			t.SourceCodes = []string{"quest.side_completed"}
			if p.QuestVersionID != uuid.Nil {
				v := p.QuestVersionID // a repeatable side quest diminishes per quest version (periodic quests do not)
				t.ContentVersionID = &v
			}
		case "daily":
			t.SourceCodes = []string{"quest.daily_completed"}
		case "weekly":
			t.SourceCodes = []string{"quest.weekly_completed"}
		}
		return p.StudentID, t, nil
	case "minigame.completed":
		var p struct {
			PlayID          uuid.UUID  `json:"play_id"`
			StudentID       uuid.UUID  `json:"student_id"`
			ConfigID        uuid.UUID  `json:"minigame_config_id"`
			ConfigVersionID uuid.UUID  `json:"minigame_config_version_id"`
			CorrectCount    *int       `json:"correct_count"`
			ItemCount       *int       `json:"item_count"`
			CompletionMet   bool       `json:"completion_met"`
			RewardsEligible bool       `json:"rewards_eligible"`
			RewardRuleID    *uuid.UUID `json:"reward_rule_id"`
			GradeLevels     []int      `json:"grade_levels"`
			SubjectID       *uuid.UUID `json:"subject_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.PlayID == uuid.Nil || p.StudentID == uuid.Nil || p.ConfigID == uuid.Nil || p.ConfigVersionID == uuid.Nil || !grades(p.GradeLevels) {
			return uuid.Nil, t, invalid(e.EventType, "play_id, student_id and the minigame config are required")
		}
		config, version := p.ConfigID, p.ConfigVersionID
		t.SourceType, t.SourceID, t.ContentID, t.ContentVersionID, t.Mode = "minigame", p.PlayID, &config, &version, "minigame"
		t.CorrectCount, t.ItemCount, t.FirstTryCorrect, t.ContentGrades, t.SubjectID = p.CorrectCount, p.ItemCount, p.CorrectCount, p.GradeLevels, p.SubjectID
		t.RuleID = p.RewardRuleID
		if p.CompletionMet && p.RewardsEligible { // F073-AC-19: a play over max_plays_per_day earns nothing (practice only)
			t.SourceCodes = []string{"activity.completed"}
		}
		return p.StudentID, t, nil
	case "world.discovery_found":
		var p struct {
			StudentID      uuid.UUID `json:"student_id"`
			AreaID         string    `json:"area_id"`
			InteractableID string    `json:"interactable_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.StudentID == uuid.Nil || p.InteractableID == "" || len(p.InteractableID) > 64 || len(p.AreaID) > 64 {
			return uuid.Nil, t, invalid(e.EventType, "student_id and interactable_id are required")
		}
		t.SourceType, t.SourceID = "world", uuid.NewSHA1(sourceNS, []byte("discovery:"+p.InteractableID))
		t.SourceCodes = []string{"exploration.discovery"}
		return p.StudentID, t, nil
	case "world.area_completed":
		var p struct {
			StudentID uuid.UUID `json:"student_id"`
			AreaID    string    `json:"area_id"`
		}
		if err := decode(e, &p); err != nil {
			return uuid.Nil, t, err
		}
		if p.StudentID == uuid.Nil || p.AreaID == "" || len(p.AreaID) > 64 {
			return uuid.Nil, t, invalid(e.EventType, "student_id and area_id are required")
		}
		t.SourceType, t.SourceID = "world", uuid.NewSHA1(sourceNS, []byte("area:"+p.AreaID))
		t.SourceCodes = []string{"exploration.area_completed"}
		return p.StudentID, t, nil
	}
	return uuid.Nil, t, invalid(e.EventType, fmt.Sprintf("not a Reward Engine event (%s)", e.EventType))
}

func grades(gs []int) bool {
	if len(gs) > 9 {
		return false
	}
	return !slices.ContainsFunc(gs, func(g int) bool { return g < 1 || g > 9 })
}

// GradeTransitioner is ApplyGradeTransition (command.Progression).
type GradeTransitioner interface {
	ApplyGradeTransition(ctx context.Context, tenant, student uuid.UUID, grade int, changed bool, at time.Time) (bool, error)
}

// GradeTransition is the consumer game.grade_transition (F074 T035, F074-AC-25, AC-26): student.placement_changed
// (previous_grade_level ≠ grade_level) and student.synced (grade_level in changed_fields) of module identity apply the
// school-year transition of the student; any other change does nothing.
type GradeTransition struct{ Progression GradeTransitioner }

// GradeTransitionOptions returns the consumer options (tenant events of the identity stream).
func GradeTransitionOptions(h GradeTransition, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.GradeTransitionName, Stream: messaging.IdentityStream,
		Subjects: messaging.Subjects(messaging.GradeTransitionEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c GradeTransition) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(messaging.GradeTransitionName, invalid(e.EventType, "tenant event expected"))
	}
	var p struct {
		StudentID     uuid.UUID `json:"student_id"`
		GradeLevel    *int      `json:"grade_level"`
		PreviousGrade *int      `json:"previous_grade_level"`
		ChangedFields []string  `json:"changed_fields"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.GradeTransitionName, err)
	}
	if p.StudentID == uuid.Nil {
		return reject(messaging.GradeTransitionName, invalid(e.EventType, "student_id is required"))
	}
	if p.GradeLevel == nil {
		return nil
	}
	changed := false
	switch e.EventType {
	case "student.placement_changed":
		changed = p.PreviousGrade != nil && *p.PreviousGrade != *p.GradeLevel
	case "student.synced":
		changed = slices.Contains(p.ChangedFields, "grade_level")
	}
	_, err := c.Progression.ApplyGradeTransition(ctx, *e.TenantID, p.StudentID, *p.GradeLevel, changed, e.OccurredAt.UTC())
	return err
}
