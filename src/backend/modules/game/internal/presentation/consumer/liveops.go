package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/liveops"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// LiveOpsRecorder is the live-ops progress use case (command.LiveOps).
type LiveOpsRecorder interface {
	RecordProgress(ctx context.Context, tenant uuid.UUID, f liveops.Fact) error
}

// LiveOpsProgress is the consumer game.liveops_progress* (F077 T024, T027, T029): a verified server event (quest.completed,
// world.discovery_found, activity.graded, party.activity_completed, match.completed) becomes a fact of each student it
// concerns — event points, community goal contributions and season points, idempotent per source event (R5: never a
// client claim). One durable consumer per stream, same handler.
type LiveOpsProgress struct {
	LiveOps LiveOpsRecorder
	Name    string
}

// LiveOpsProgressOptions returns the options of game.liveops_progress (game stream).
func LiveOpsProgressOptions(h LiveOpsProgress, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.LiveOpsProgressName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.GameStream,
		Subjects: messaging.GameSubjects(messaging.LiveOpsProgressGameEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// LiveOpsProgressLearningOptions returns the options of game.liveops_progress_learning.
func LiveOpsProgressLearningOptions(h LiveOpsProgress, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.LiveOpsProgressLearningName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.LearningStream,
		Subjects: messaging.LearningSubjects(messaging.LiveOpsProgressLearningEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// LiveOpsProgressCompetitionOptions returns the options of game.liveops_progress_competition.
func LiveOpsProgressCompetitionOptions(h LiveOpsProgress, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = messaging.LiveOpsProgressCompetitionName
	return outbox.ConsumerOptions{Module: "game", Name: h.Name, Stream: messaging.CompetitionStream,
		Subjects: messaging.CompetitionSubjects(messaging.LiveOpsProgressCompetitionEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

func (c LiveOpsProgress) name() string {
	if c.Name == "" {
		return messaging.LiveOpsProgressName
	}
	return c.Name
}

// Handle implements outbox.Handler.
func (c LiveOpsProgress) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID == nil {
		return reject(c.name(), invalid(e.EventType, "tenant event expected"))
	}
	var payload map[string]any
	if err := json.Unmarshal(e.Payload, &payload); err != nil || payload == nil {
		return reject(c.name(), invalid(e.EventType, "payload must be a JSON object"))
	}
	students := StudentsOf(payload)
	if len(students) == 0 {
		return reject(c.name(), invalid(e.EventType, "no student in the payload"))
	}
	f := FactOf(e, payload)
	for _, s := range students {
		f.StudentID = s
		if err := c.LiveOps.RecordProgress(ctx, *e.TenantID, f); err != nil {
			return err
		}
	}
	return nil
}

// FactOf maps a verified server event to a live-ops fact (exported for tests): the quest of quest.completed, the correct
// answers and the rapid-guess flag of activity.graded (suspicious_speed: never contributes), its subject and objectives.
func FactOf(e outbox.Envelope, p map[string]any) liveops.Fact {
	f := liveops.Fact{EventType: e.EventType, EventID: e.EventID, At: e.OccurredAt.UTC()}
	if f.At.IsZero() {
		f.At = time.Now().UTC()
	}
	uuidOf := func(k string) *uuid.UUID {
		s, _ := p[k].(string)
		if id, err := uuid.Parse(s); err == nil && id != uuid.Nil {
			return &id
		}
		return nil
	}
	f.QuestID = uuidOf("quest_id")
	f.QuestType, _ = p["quest_type"].(string)
	f.Recurrence, _ = p["recurrence"].(string)
	f.SubjectID = uuidOf("subject_id")
	if v, ok := p["rapid_guess_flagged"].(bool); ok {
		f.RapidGuess = v
	}
	if n, ok := p["first_try_correct_count"].(float64); ok {
		f.Correct = int(n)
	} else if n, ok := p["correct_count"].(float64); ok {
		f.Correct = int(n)
	}
	if os, ok := p["objective_ids"].([]any); ok {
		for _, o := range os {
			s, _ := o.(string)
			if id, err := uuid.Parse(s); err == nil {
				f.ObjectiveIDs = append(f.ObjectiveIDs, id)
			}
		}
	}
	return f
}

// SeasonProjector is the season projection use case (command.LiveOps).
type SeasonProjector interface {
	ProjectSeason(ctx context.Context, e command.SeasonEvent) error
}

// SeasonProjection is the consumer game.season_projection (F077 T029, F077-AC-17, AC-20): season.started and
// season.ended of module competition (platform events, owner F106) keep ref.seasons by source_version, open the
// published track of the season and close it (unclaimed reached tiers granted).
type SeasonProjection struct {
	Seasons SeasonProjector
}

// SeasonProjectionOptions returns the outbox consumer options (platform events of the competition stream).
func SeasonProjectionOptions(h SeasonProjection, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "game", Name: messaging.SeasonProjectionName, Stream: messaging.CompetitionStream,
		Subjects: messaging.CompetitionSubjects(messaging.SeasonProjectionEvents), Platform: true, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c SeasonProjection) Handle(ctx context.Context, e outbox.Envelope) error {
	if e.TenantID != nil {
		return reject(messaging.SeasonProjectionName, invalid(e.EventType, "platform event expected"))
	}
	var p struct {
		SeasonID      uuid.UUID  `json:"season_id"`
		Code          string     `json:"code"`
		Name          string     `json:"name"`
		ThemeCode     *string    `json:"theme_code"`
		StartsAt      *time.Time `json:"starts_at"`
		EndsAt        *time.Time `json:"ends_at"`
		EndedAt       *time.Time `json:"ended_at"`
		SourceVersion int64      `json:"source_version"`
	}
	if err := decode(e, &p); err != nil {
		return reject(messaging.SeasonProjectionName, err)
	}
	if p.SeasonID == uuid.Nil || p.Code == "" || p.SourceVersion < 1 || (e.EventType == "season.started" && (p.StartsAt == nil || p.EndsAt == nil)) {
		return reject(messaging.SeasonProjectionName, invalid(e.EventType, "season_id, code, source_version and the dates are required"))
	}
	return c.Seasons.ProjectSeason(ctx, command.SeasonEvent{Type: e.EventType, SeasonID: p.SeasonID, Code: p.Code, Name: p.Name, ThemeCode: p.ThemeCode,
		StartsAt: p.StartsAt, EndsAt: p.EndsAt, EndedAt: p.EndedAt, SourceVersion: p.SourceVersion})
}
