//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/realtime"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// T017 (F014-AC-04, AC-26): the Valkey adapter publishes session.session.replaced and session.session.ended (curfew
// with reopen_at) on ew:rt:user:<student_id>, the channel the realtime gateway (F090) subscribes for each connection.
func TestValkeyPublisher(t *testing.T) {
	ctx := context.Background()
	_, addr := testkit.StartValkey(ctx, t)
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	student, session := uuid.New(), uuid.New()
	got := make(chan rt.Envelope, 4)
	subCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	ready := make(chan struct{})
	go func() {
		_ = c.Receive(subCtx, c.B().Subscribe().Channel(realtime.Channel(student)).Build(), func(m valkey.PubSubMessage) {
			var msg rt.Envelope
			if json.Unmarshal([]byte(m.Message), &msg) == nil {
				got <- msg
			}
		})
	}()
	go func() {
		for subCtx.Err() == nil { // wait until the subscription is registered
			n, _ := c.Do(ctx, c.B().PubsubNumsub().Channel(realtime.Channel(student)).Build()).AsIntMap()
			if n[realtime.Channel(student)] > 0 {
				close(ready)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("subscription not ready")
	}
	p := realtime.ValkeyPublisher{C: c}
	at := time.Date(2026, 11, 19, 14, 30, 0, 0, time.UTC)
	if err := p.SessionReplaced(ctx, student, session, "android", at); err != nil {
		t.Fatal(err)
	}
	reopen := at.Add(9 * time.Hour)
	if err := p.SessionEnded(ctx, student, session, "curfew", at, &reopen); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"session.session.replaced", "session.session.ended"} {
		select {
		case e := <-got:
			var m struct {
				Type    string
				Payload map[string]any
			}
			if e.Message != nil {
				m.Type = e.Message.Type
				_ = json.Unmarshal(e.Message.Payload, &m.Payload)
			}
			// F090 research R1: RtPubSubEnvelope targeted at the session (the gateway closes only that connection).
			if e.V != 1 || e.Kind != rt.KindMessage || e.Target == nil || e.Target.GameSessionID == nil || *e.Target.GameSessionID != session {
				t.Fatalf("envelope %d: %+v", i, e)
			}
			if m.Type != want || m.Payload["game_session_id"] != session.String() {
				t.Fatalf("message %d: %+v", i, m)
			}
			if want == "session.session.ended" && (m.Payload["reason"] != "curfew" || m.Payload["reopen_at"] != float64(reopen.UnixMilli())) {
				t.Fatalf("ended payload: %+v", m.Payload)
			}
			if want == "session.session.replaced" && (m.Payload["new_device_platform"] != "android" || m.Payload["replaced_at"] != float64(at.UnixMilli())) {
				t.Fatalf("replaced payload: %+v", m.Payload)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("message %s not received", want)
		}
	}
}
