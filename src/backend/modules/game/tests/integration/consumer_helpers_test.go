//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// bus is NATS JetStream with the identity and game streams, and an identity test publisher (TEST DOUBLE of the
// identity relay — R17: envelopes are built with the real outbox.Writer of producer identity).
type bus struct {
	js jetstream.JetStream
	w  *outbox.Writer
}

func newBus(ctx context.Context, t *testing.T) *bus {
	t.Helper()
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"identity", "game"} {
		if err := natsx.EnsureStreams(ctx, js, m, natsx.StreamOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	w, err := outbox.NewWriter("identity", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return &bus{js: js, w: w}
}

// run starts the consumer o until the end of the test (fast retries).
func (b *bus) run(ctx context.Context, t *testing.T, o outbox.ConsumerOptions) {
	t.Helper()
	o.BackOff = []time.Duration{300 * time.Millisecond}
	o.MaxDeliver = 20
	c, err := outbox.NewConsumer(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
}

// publish publishes an identity event (tenant nil = platform event) and returns its envelope.
func (b *bus) publish(ctx context.Context, t *testing.T, typ string, tenant *uuid.UUID, payload map[string]any) outbox.Envelope {
	t.Helper()
	entity := typ[:strings.IndexByte(typ, '.')]
	env, err := b.w.Envelope(ctx, outbox.Message{Type: typ, Subject: outbox.Subject{Type: entity, ID: uuid.NewString()}, TenantID: tenant,
		Platform: tenant == nil, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	b.publishEnvelope(ctx, t, env)
	return env
}

func (b *bus) publishEnvelope(ctx context.Context, t *testing.T, env outbox.Envelope) {
	t.Helper()
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.js.Publish(ctx, natsx.EventSubject("identity", env.EventType), raw); err != nil {
		t.Fatal(err)
	}
}

// waitFor polls cond until it holds or the timeout ends.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// processed waits until consumer recorded the event in infra.processed_events.
func (e *env) processed(ctx context.Context, t *testing.T, consumer string, env outbox.Envelope) {
	t.Helper()
	waitFor(t, 20*time.Second, "processed "+env.EventType, func() bool {
		return count(ctx, t, e.migrator, `SELECT count(*) FROM infra.processed_events WHERE consumer = $1 AND event_id = $2`,
			consumer, env.EventID) == 1
	})
}

// deadLettered waits until the event is in infra.dead_letters of consumer.
func (e *env) deadLettered(ctx context.Context, t *testing.T, consumer string, env outbox.Envelope) {
	t.Helper()
	waitFor(t, 20*time.Second, "dead letter "+env.EventType, func() bool {
		return count(ctx, t, e.migrator, `SELECT count(*) FROM infra.dead_letters WHERE consumer = $1 AND event_id = $2`,
			consumer, env.EventID) == 1
	})
}

// fixture loads an identity event fixture of F013 T045 (modules/identity/tests/fixtures/events, contract-first payload
// schemas) and points it at student and tenant with a fresh event id.
func fixture(t *testing.T, name string, student, tenant uuid.UUID) outbox.Envelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "src", "backend", "modules", "identity", "tests", "fixtures", "events", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(raw), "0193a5f4-6c1e-7b2a-9d1f-000000000005", student.String())
	s = strings.ReplaceAll(s, "0193a5f4-6c1e-7b2a-9d1f-000000000001", tenant.String())
	var env outbox.Envelope
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		t.Fatal(err)
	}
	env.EventID = uuid.Must(uuid.NewV7())
	env.OccurredAt = time.Now().UTC()
	return env
}
