//go:build integration

package lifecycle_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/lifecycle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample"
)

// F003-AC-30 (TC-SVC-002): SIGTERM in the middle of a batch: the running handler finishes and acks,
// unacknowledged messages are redelivered to the next worker, nothing is handled twice.
func TestWorkerShutdownMidBatch(t *testing.T) {
	ctx := context.Background()
	e := sample.Setup(ctx, t)
	tenancy.Configure(tenancy.Hooks{Recorder: e.Audit})
	t.Cleanup(func() { tenancy.Configure(tenancy.Hooks{}) })
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	if err := natsx.EnsureStreams(ctx, js, "learning", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	const total = 6
	tenant := authztest.TenantA
	for i := 0; i < total; i++ {
		env := outbox.Envelope{EventID: ids.New(), EventType: "widget.checked", SchemaVersion: 1, OccurredAt: time.Now().UTC(),
			RecordedAt: time.Now().UTC(), Producer: "learning/sample", TenantID: &tenant, Actor: outbox.Actor{Type: "system"},
			Subject: outbox.Subject{Type: "widget", ID: "w"}, IdempotencyKey: "k", Payload: json.RawMessage(`{}`)}
		raw, _ := json.Marshal(env)
		if _, err := js.Publish(ctx, "ew.evt.learning.widget.checked", raw); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	handled := map[string]int{}
	first := make(chan struct{}, 1)
	handler := func(ctx context.Context, env outbox.Envelope) error {
		select {
		case first <- struct{}{}:
		default:
		}
		time.Sleep(150 * time.Millisecond)
		mu.Lock()
		handled[env.EventID.String()]++
		mu.Unlock()
		return nil
	}
	start := func() *lifecycle.Group {
		g := lifecycle.NewGroup(ctx)
		c, err := outbox.NewConsumer(ctx, outbox.ConsumerOptions{Module: "learning", Name: "learning.shutdown_check", Stream: natsx.EventStream("learning"),
			Subjects: []string{"ew.evt.learning.widget.checked"}, UoW: e.UoW, DLQDB: e.App, JS: js, Handler: handler, AckWait: 2 * time.Second, MaxInFlight: 1})
		if err != nil {
			t.Fatal(err)
		}
		g.Go("consumer", c.Run)
		return g
	}
	g := start()
	<-first
	if err := g.Stop(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	afterFirst := len(handled)
	mu.Unlock()
	if afterFirst == 0 || afterFirst == total {
		t.Fatalf("first worker handled %d of %d: the stop must interrupt the batch", afterFirst, total)
	}
	t.Logf("first worker handled %d of %d before stopping", afterFirst, total)
	// The next worker picks up whatever was not acknowledged.
	g2 := start()
	deadline := time.Now().Add(20 * time.Second)
	for {
		mu.Lock()
		n := len(handled)
		mu.Unlock()
		if n == total {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d events handled after restart", n, total)
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = g2.Stop(10 * time.Second)
	for id, n := range handled {
		if n != 1 {
			t.Fatalf("event %s handled %d times", id, n)
		}
	}
	var processed int
	_ = e.Migrator.QueryRow(`SELECT count(*) FROM infra.processed_events WHERE consumer = 'learning.shutdown_check'`).Scan(&processed)
	if processed != total {
		t.Fatalf("processed_events = %d", processed)
	}
}
