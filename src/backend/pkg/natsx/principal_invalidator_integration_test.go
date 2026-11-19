//go:build integration

package natsx_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

type call struct {
	kind   string
	user   uuid.UUID
	tenant *uuid.UUID
}

type spyCache struct {
	mu    sync.Mutex
	calls []call
}

func (s *spyCache) add(c call) { s.mu.Lock(); s.calls = append(s.calls, c); s.mu.Unlock() }
func (s *spyCache) Invalidate(_ context.Context, u uuid.UUID, t *uuid.UUID) error {
	s.add(call{"user", u, t})
	return nil
}
func (s *spyCache) InvalidateTenant(_ context.Context, t uuid.UUID) error {
	s.add(call{"tenant", uuid.Nil, &t})
	return nil
}
func (s *spyCache) InvalidateAll(context.Context) error {
	s.add(call{"all", uuid.Nil, nil})
	return nil
}
func (s *spyCache) len() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) }

// F003-AC-23: each identity event invalidates the right part of the Principal cache, on every replica.
func TestPrincipalInvalidator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, url := testkit.StartNATS(ctx, t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	if err := natsx.EnsureStreams(ctx, js, "identity", natsx.StreamOptions{}); err != nil {
		t.Fatal(err)
	}
	replicas := []*spyCache{{}, {}}
	for _, c := range replicas {
		inv := &natsx.PrincipalInvalidator{JS: js, Cache: c}
		go func() { _ = inv.Run(ctx) }()
	}
	time.Sleep(500 * time.Millisecond)

	user, tenant := uuid.New(), uuid.New()
	events := []struct {
		typ     string
		payload map[string]any
		want    call
	}{
		{"user_role.changed", map[string]any{"user_id": user, "tenant_id": tenant}, call{"user", user, &tenant}},
		{"tenant_membership.changed", map[string]any{"user_id": user, "tenant_id": tenant}, call{"user", user, &tenant}},
		{"user.access_revoked", map[string]any{"user_id": user}, call{"user", user, nil}},
		{"user.status_changed", map[string]any{"user_id": user}, call{"user", user, nil}},
		{"enrollment.changed", map[string]any{"student_id": user}, call{"user", user, &tenant}},
		{"tenant.updated", map[string]any{"tenant_id": tenant}, call{"tenant", uuid.Nil, &tenant}},
		{"role.updated", map[string]any{"role_id": uuid.New()}, call{"all", uuid.Nil, nil}},
	}
	for _, ev := range events {
		data, _ := json.Marshal(map[string]any{"event_type": ev.typ, "tenant_id": tenant, "payload": ev.payload})
		if _, err := js.Publish(ctx, "ew.evt.identity."+ev.typ, data); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for replicas[0].len() < len(events) || replicas[1].len() < len(events) {
		if time.Now().After(deadline) {
			t.Fatalf("calls: %d / %d", replicas[0].len(), replicas[1].len())
		}
		time.Sleep(50 * time.Millisecond)
	}
	for r, c := range replicas {
		for i, ev := range events {
			got := c.calls[i]
			sameTenant := (got.tenant == nil) == (ev.want.tenant == nil) && (got.tenant == nil || *got.tenant == *ev.want.tenant)
			if got.kind != ev.want.kind || got.user != ev.want.user || !sameTenant {
				t.Fatalf("replica %d, %s: got %+v want %+v", r, ev.typ, got, ev.want)
			}
		}
	}
}
