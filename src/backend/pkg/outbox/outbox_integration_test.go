//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
)

type rig struct {
	e  *sample.Env
	js jetstream.JetStream
	nc *nats.Conn
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx := context.Background()
	e := sample.Setup(ctx, t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
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
	if err := natsx.EnsureStreams(ctx, js, "learning", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	return &rig{e: e, js: js, nc: nc}
}

func (r *rig) streamCount(t *testing.T, name string) uint64 {
	t.Helper()
	s, err := r.js.Stream(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.State.Msgs
}

func TestOutboxPipeline(t *testing.T) {
	r := newRig(t)
	t.Run("RelayCrashAndRerun", func(t *testing.T) { relayCrashAndRerun(t, r) })
	t.Run("ConsumerIdempotentAndScoped", func(t *testing.T) { consumerIdempotentAndScoped(t, r) })
	t.Run("DeadLetters", func(t *testing.T) { deadLetters(t, r) })
	t.Run("Grants", func(t *testing.T) { grants(t, r) })
}

// F003-AC-18 (TC-SVC-002): a relay crash after publishing part of a batch loses nothing and duplicates
// nothing (Nats-Msg-Id = event_id).
func relayCrashAndRerun(t *testing.T, r *rig) {
	ctx := authztest.Ctx(context.Background(), authztest.Staff(authztest.TenantA, "school_admin"))
	const total = 10
	if err := r.e.UoW.Do(ctx, func(ctx context.Context) error {
		for i := 0; i < total; i++ {
			if _, err := r.e.Writer.Publish(ctx, outbox.Message{Type: "widget.checked", Subject: outbox.Subject{Type: "widget", ID: fmt.Sprint(i)},
				Payload: map[string]any{"n": i}}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := r.streamCount(t, natsx.EventStream("learning"))
	var published atomic.Int32
	crashing, err := outbox.NewRelay(outbox.RelayOptions{Module: "learning", DB: r.e.Relay, JS: r.js, AfterPublish: func(uuid.UUID) error {
		if published.Add(1) == 4 {
			return errors.New("simulated crash")
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := crashing.RunOnce(context.Background()); err == nil {
		t.Fatal("simulated crash not reported")
	}
	var marked int
	_ = r.e.Migrator.QueryRow(`SELECT count(*) FROM infra.outbox_events WHERE published_at IS NOT NULL`).Scan(&marked)
	if marked != 0 {
		t.Fatalf("%d rows marked although the batch transaction rolled back", marked)
	}
	relay, _ := outbox.NewRelay(outbox.RelayOptions{Module: "learning", DB: r.e.Relay, JS: r.js})
	for {
		n, err := relay.RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	var pending int
	_ = r.e.Migrator.QueryRow(`SELECT count(*) FROM infra.outbox_events WHERE published_at IS NULL`).Scan(&pending)
	if pending != 0 {
		t.Fatalf("%d rows left unpublished", pending)
	}
	// Every outbox row (10 events + audit rows of earlier subtests) is in the stream exactly once.
	var rows uint64
	_ = r.e.Migrator.QueryRow(`SELECT count(*) FROM infra.outbox_events`).Scan(&rows)
	if got := r.streamCount(t, natsx.EventStream("learning")); got-before != rows-before {
		t.Fatalf("stream has %d new messages for %d rows (duplicates or losses)", got-before, rows-before)
	}
}

func envelopeFor(t *testing.T, r *rig, tenant *uuid.UUID, occurred time.Time) []byte {
	t.Helper()
	e := outbox.Envelope{EventID: ids.New(), EventType: "widget.checked", SchemaVersion: 1, OccurredAt: occurred, RecordedAt: occurred,
		Producer: "learning/sample", TenantID: tenant, Actor: outbox.Actor{Type: "system"}, Subject: outbox.Subject{Type: "widget", ID: "w"},
		IdempotencyKey: "k", Payload: json.RawMessage(`{}`)}
	raw, _ := json.Marshal(e)
	return raw
}

func publishRaw(t *testing.T, r *rig, subject string, data []byte) {
	t.Helper()
	if _, err := r.js.PublishMsg(context.Background(), &nats.Msg{Subject: subject, Data: data},
		jetstream.WithMsgID(ids.New().String())); err != nil {
		t.Fatal(err)
	}
}

type seen struct {
	mu     sync.Mutex
	events []outbox.Envelope
	scopes []string
}

func (s *seen) add(ctx context.Context, e outbox.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if sc, ok := tenancy.ScopeFrom(ctx); ok && len(sc.TenantIDs()) == 1 {
		s.scopes = append(s.scopes, sc.TenantIDs()[0].String())
	} else {
		s.scopes = append(s.scopes, "platform")
	}
}

func (s *seen) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func startConsumer(t *testing.T, r *rig, name, subject string, platform bool, h outbox.Handler, maxDeliver int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c, err := outbox.NewConsumer(ctx, outbox.ConsumerOptions{Module: "learning", Name: "learning." + name, Stream: natsx.EventStream("learning"),
		Subjects: []string{subject}, Platform: platform, UoW: r.e.UoW, DLQDB: r.e.App, JS: r.js, Handler: h,
		MaxDeliver: maxDeliver, BackOff: []time.Duration{20 * time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = c.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
}

// F003-AC-19, AC-38, AC-20: duplicates are skipped (also after midnight: the PK uses occurred_at of the
// envelope), the scope comes from the envelope, platform events only reach platform consumers.
func consumerIdempotentAndScoped(t *testing.T, r *rig) {
	got := &seen{}
	startConsumer(t, r, "idem_check", "ew.evt.learning.idem.checked", false, func(ctx context.Context, e outbox.Envelope) error { got.add(ctx, e); return nil }, 5)
	tenant := authztest.TenantA
	occurred := time.Date(2026, 11, 17, 23, 59, 30, 0, time.UTC)
	data := envelopeFor(t, r, &tenant, occurred)
	publishRaw(t, r, "ew.evt.learning.idem.checked", data)
	publishRaw(t, r, "ew.evt.learning.idem.checked", data) // redelivery of the same event (different Nats-Msg-Id, e.g. replay after midnight)
	waitFor(t, func() bool {
		var n int
		_ = r.e.Migrator.QueryRow(`SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.idem_check'`).Scan(&n)
		return n == 1 && got.len() >= 1
	})
	time.Sleep(300 * time.Millisecond)
	if got.len() != 1 || got.scopes[0] != tenant.String() {
		t.Fatalf("handled %d times, scopes %v", got.len(), got.scopes)
	}
	var at time.Time
	_ = r.e.Migrator.QueryRow(`SELECT event_occurred_at FROM infra.processed_events WHERE consumer = 'learning.idem_check'`).Scan(&at)
	if !at.Equal(occurred) {
		t.Fatalf("event_occurred_at = %v, want the envelope's %v", at, occurred)
	}
	// A platform event (tenant_id null) is dead-lettered by a tenant consumer.
	publishRaw(t, r, "ew.evt.learning.idem.checked", envelopeFor(t, r, nil, time.Now().UTC()))
	waitFor(t, func() bool {
		var n int
		_ = r.e.Platform.QueryRow(`SELECT count(*) FROM infra.dead_letters WHERE consumer = 'learning.idem_check'`).Scan(&n)
		return n == 1
	})
}

// F003-AC-21: after MaxDeliver failures (or a permanent error) the message goes to DLQ_<MODULE> and
// infra.dead_letters, is terminated, and later messages still flow.
func deadLetters(t *testing.T, r *rig) {
	var calls atomic.Int32
	okAfter := &seen{}
	startConsumer(t, r, "always_fails", "ew.evt.learning.dlq.checked", false, func(ctx context.Context, e outbox.Envelope) error {
		calls.Add(1)
		if e.Subject.ID == "poison" {
			return errors.New("transient forever")
		}
		if e.Subject.ID == "foreign" {
			return outbox.Permanent(errors.New("references a resource of another tenant"))
		}
		okAfter.add(ctx, e)
		return nil
	}, 2)
	tenant := authztest.TenantB
	dlqBefore := r.streamCount(t, natsx.DLQStream("learning"))
	for _, id := range []string{"poison", "foreign", "fine"} {
		e := outbox.Envelope{EventID: ids.New(), EventType: "widget.checked", SchemaVersion: 1, OccurredAt: time.Now().UTC(), RecordedAt: time.Now().UTC(),
			Producer: "learning/sample", TenantID: &tenant, Actor: outbox.Actor{Type: "system"}, Subject: outbox.Subject{Type: "widget", ID: id},
			IdempotencyKey: id, Payload: json.RawMessage(`{}`)}
		raw, _ := json.Marshal(e)
		publishRaw(t, r, "ew.evt.learning.dlq.checked", raw)
	}
	publishRaw(t, r, "ew.evt.learning.dlq.checked", []byte(`{"not":"an envelope"}`))
	waitFor(t, func() bool {
		var n int
		_ = r.e.Platform.QueryRow(`SELECT count(*) FROM infra.dead_letters WHERE consumer = 'learning.always_fails'`).Scan(&n)
		return n == 3 && okAfter.len() == 1
	})
	if got := r.streamCount(t, natsx.DLQStream("learning")) - dlqBefore; got != 3 {
		t.Fatalf("DLQ stream got %d messages, want 3", got)
	}
	var count int
	_ = r.e.Platform.QueryRow(`SELECT delivery_count FROM infra.dead_letters WHERE consumer = 'learning.always_fails' AND error LIKE '%transient%'`).Scan(&count)
	if count != 2 {
		t.Fatalf("poison message dead-lettered after %d deliveries, want 2", count)
	}
}

// F003-AC-35: infra tables have no RLS and exactly the grants of ADR-0089 §4; no DELETE/TRUNCATE.
func grants(t *testing.T, r *rig) {
	var rls int
	_ = r.e.Migrator.QueryRow(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'infra' AND c.relrowsecurity`).Scan(&rls)
	if rls != 0 {
		t.Fatalf("%d infra tables have RLS", rls)
	}
	type g struct {
		role, table, priv string
		want              bool
	}
	cases := []g{
		{"learning_app", "infra.outbox_events", "INSERT", true},
		{"learning_app", "infra.outbox_events", "SELECT", false},
		{"learning_app", "infra.outbox_events", "UPDATE", false},
		{"learning_app", "infra.processed_events", "SELECT", true},
		{"learning_app", "infra.idempotency_records", "INSERT", true},
		{"learning_app", "infra.idempotency_records", "UPDATE", false},
		{"learning_app", "infra.dead_letters", "INSERT", true},
		{"learning_app", "infra.dead_letters", "SELECT", false},
		{"learning_relay", "infra.outbox_events", "SELECT", true},
		{"learning_relay", "infra.outbox_events", "INSERT", false},
		{"learning_platform", "infra.outbox_events", "SELECT", false},
		{"learning_platform", "infra.dead_letters", "SELECT", true},
	}
	for _, role := range []string{"learning_app", "learning_platform", "learning_relay", "learning_partition_admin"} {
		for _, tbl := range []string{"infra.outbox_events", "infra.processed_events", "infra.dead_letters", "infra.idempotency_records",
			"infra.outbox_events_default"} {
			cases = append(cases, g{role, tbl, "DELETE", false}, g{role, tbl, "TRUNCATE", false})
		}
	}
	for _, c := range cases {
		var has bool
		if err := r.e.Migrator.QueryRow(`SELECT has_table_privilege($1, $2, $3)`, c.role, c.table, c.priv).Scan(&has); err != nil {
			t.Fatal(err)
		}
		if has != c.want {
			t.Errorf("%s %s on %s = %v, want %v", c.role, c.priv, c.table, has, c.want)
		}
	}
	var relayUpdate, relayOther bool
	_ = r.e.Migrator.QueryRow(`SELECT has_column_privilege('learning_relay', 'infra.outbox_events', 'published_at', 'UPDATE'),
		has_column_privilege('learning_relay', 'infra.outbox_events', 'envelope', 'UPDATE')`).Scan(&relayUpdate, &relayOther)
	if !relayUpdate || relayOther {
		t.Fatalf("relay column UPDATE: published_at=%v envelope=%v", relayUpdate, relayOther)
	}
	// A daily partition has no direct grant (access only through the parent).
	var part string
	_ = r.e.Migrator.QueryRow(`SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname = 'outbox_events' AND c.relname <> 'outbox_events_default' LIMIT 1`).Scan(&part)
	var direct bool
	_ = r.e.Migrator.QueryRow(`SELECT has_table_privilege('learning_app', 'infra.' || $1, 'INSERT')`, part).Scan(&direct)
	if part == "" || direct {
		t.Fatalf("partition %q grants direct INSERT to learning_app: %v", part, direct)
	}
}
