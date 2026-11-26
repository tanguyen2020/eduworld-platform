package consumer

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// AnalyticsIngester is the ingestion use case (command.AnalyticsIngest).
type AnalyticsIngester interface {
	IngestDomainEvent(ctx context.Context, e command.DomainEvent) error
}

// Analytics is the durable consumer analytics (F080-AC-05, GAME_ANALYTICS §5): every event of the catalog of its stream
// becomes pseudonymous rows of analytics.events_raw, idempotent per event_id (processed_events of pkg/outbox, and
// ON CONFLICT DO NOTHING on the row). One consumer per producer stream.
type Analytics struct {
	Ingest AnalyticsIngester
	Name   string
}

func analyticsOptions(h Analytics, name string, stream string, subjects []string, uow *entx.UnitOfWork, dlq *sql.DB,
	js jetstream.JetStream) outbox.ConsumerOptions {
	h.Name = name
	return outbox.ConsumerOptions{Module: "game", Name: name, Stream: stream, Subjects: subjects, UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// AnalyticsGameOptions returns the options of game.analytics (game stream).
func AnalyticsGameOptions(h Analytics, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return analyticsOptions(h, messaging.AnalyticsGameName, messaging.GameStream, messaging.GameSubjects(analytics.Events["game"]), uow, dlq, js)
}

// AnalyticsCompetitionOptions returns the options of game.analytics_competition.
func AnalyticsCompetitionOptions(h Analytics, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return analyticsOptions(h, messaging.AnalyticsCompetitionName, messaging.CompetitionStream,
		messaging.CompetitionSubjects(analytics.Events["competition"]), uow, dlq, js)
}

// AnalyticsLearningOptions returns the options of game.analytics_learning.
func AnalyticsLearningOptions(h Analytics, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return analyticsOptions(h, messaging.AnalyticsLearningName, messaging.LearningStream, messaging.LearningSubjects(analytics.Events["learning"]),
		uow, dlq, js)
}

// AnalyticsChatOptions returns the options of game.analytics_chat (chat.daily_stats: counts only, no content).
func AnalyticsChatOptions(h Analytics, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return analyticsOptions(h, messaging.AnalyticsChatName, messaging.ChatStream, messaging.ChatSubjects(analytics.Events["chat"]), uow, dlq, js)
}

// Handle implements outbox.Handler.
func (c Analytics) Handle(ctx context.Context, e outbox.Envelope) error {
	name := c.Name
	if name == "" {
		name = messaging.AnalyticsGameName
	}
	if e.TenantID == nil {
		return reject(name, invalid(e.EventType, "tenant event expected"))
	}
	var payload map[string]any
	if err := json.Unmarshal(e.Payload, &payload); err != nil || payload == nil {
		return reject(name, invalid(e.EventType, "payload must be a JSON object"))
	}
	return c.Ingest.IngestDomainEvent(ctx, command.DomainEvent{EventID: e.EventID, EventType: e.EventType, OccurredAt: e.OccurredAt,
		TenantID: *e.TenantID, Payload: payload})
}
