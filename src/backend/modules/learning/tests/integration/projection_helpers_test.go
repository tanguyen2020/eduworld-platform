//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
)

// projection runs the consumer learning.projection_identity against js until the end of the test.
type projection struct {
	e  *env
	js jetstream.JetStream
	w  *outbox.Writer // producer identity (test publisher)
}

func startProjection(ctx context.Context, t *testing.T, e *env) *projection {
	t.Helper()
	js := jetStream(ctx, t)
	o := consumer.IdentityProjectionOptions(consumer.IdentityProjection{Upsert: command.UpsertProjection{Refs: write.RefRepo{}}},
		e.uow, e.app, js)
	o.BackOff = []time.Duration{300 * time.Millisecond} // retries quickly in tests (production: 1 m, 5 m, 30 m)
	o.MaxDeliver = 20
	c, err := outbox.NewConsumer(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	w, err := outbox.NewWriter("identity", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return &projection{e: e, js: js, w: w}
}

// publish publishes an identity event (tenant nil = platform event) and returns its envelope.
func (p *projection) publish(ctx context.Context, t *testing.T, typ string, tenant *uuid.UUID, payload map[string]any) outbox.Envelope {
	t.Helper()
	entity := typ[:strings.IndexByte(typ, '.')]
	m := outbox.Message{Type: typ, Subject: outbox.Subject{Type: entity, ID: uuid.NewString()}, TenantID: tenant,
		Platform: tenant == nil, Payload: payload}
	env, err := p.w.Envelope(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	p.publishEnvelope(ctx, t, env)
	return env
}

func (p *projection) publishEnvelope(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.js.Publish(ctx, natsx.EventSubject("identity", env.EventType), raw); err != nil {
		t.Fatal(err)
	}
}

// processed waits until the consumer recorded the event in infra.processed_events.
func (p *projection) processed(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	waitFor(t, 15*time.Second, "processed "+env.EventType, func() bool {
		return count(ctx, t, p.e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.projection_identity'
			AND event_id = $1`, env.EventID) == 1
	})
}

// deadLettered waits until the event is in infra.dead_letters of the consumer.
func (p *projection) deadLettered(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	waitFor(t, 15*time.Second, "dead letter "+env.EventType, func() bool {
		return count(ctx, t, p.e.migrator, `SELECT count(*) FROM infra.dead_letters WHERE consumer = 'learning.projection_identity'
			AND event_id = $1`, env.EventID) == 1
	})
}

func consumed(eventType, result string) float64 {
	return testutil.ToFloat64(outbox.ConsumedCounter().WithLabelValues("learning", "learning.projection_identity", eventType, result))
}

func tenantPayload(id uuid.UUID, code, status string, version int) map[string]any {
	return map[string]any{"tenant_id": id.String(), "code": code, "name": "Tenant " + code, "tenant_type": "school",
		"status": status, "changed_fields": []string{}, "source_version": version}
}
