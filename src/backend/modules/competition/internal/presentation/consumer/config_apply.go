package consumer

import (
	"context"
	"database/sql"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// ConfigApplier is what the consumer needs of command.ConfigApply.
type ConfigApplier interface {
	Apply(ctx context.Context, configType string) (int, error)
}

// ConfigApply is the consumer competition.config_apply: competition_config.published applies the seasons or the
// leaderboard definitions in effect (F108-AC-08; idempotent, a release effective later is applied by the job
// competition-config-apply). The other config types need no copy.
type ConfigApply struct{ H ConfigApplier }

// ConfigApplyOptions returns its options.
func ConfigApplyOptions(h ConfigApply, uow *entx.UnitOfWork, dlq *sql.DB, js jetstream.JetStream) outbox.ConsumerOptions {
	return outbox.ConsumerOptions{Module: "competition", Name: messaging.ConfigApplyName, Stream: messaging.CompetitionStream,
		Subjects: messaging.Subjects("competition", messaging.ConfigApplyEvents), UoW: uow, DLQDB: dlq, JS: js, Handler: h.Handle}
}

// Handle implements outbox.Handler.
func (c ConfigApply) Handle(ctx context.Context, e outbox.Envelope) error {
	var p struct {
		ConfigType string `json:"config_type"`
	}
	if err := decode(e, &p); err != nil || p.ConfigType == "" {
		return outbox.Permanent(invalid(e.EventType, "config_type"))
	}
	_, err := c.H.Apply(ctx, p.ConfigType)
	return err
}
