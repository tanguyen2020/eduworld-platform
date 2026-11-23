//go:build integration

package integration

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/room"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

type roomMetrics struct{ takeovers, fired atomic.Int32 }

func (m *roomMetrics) Takeover(string)            { m.takeovers.Add(1) }
func (m *roomMetrics) TimerLag(string, float64)   { m.fired.Add(1) }
func (m *roomMetrics) LeaseLost(string)           {}
func (m *roomMetrics) Rooms(string, int)          {}
func (m *roomMetrics) TimerFailed(string, string) {}

// F076-AC-17, AC-18, SC-004 (real Valkey): the room of a challenge attempt is announced at start; two room host nodes
// see it, one holds the lease; the owner stops (crash) → after the lease TTL the other node takes it over, restores
// the due time from the PostgreSQL checkpoint and its timer ends the attempt once on the server clock (+ grace of the
// room type); the room is forgotten in Valkey.
func TestChallengeRoomFailover(t *testing.T) {
	ctx := context.Background()
	e := setupChallenges(ctx, t)
	_, addr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	reg := roomhost.ValkeyRegistry{C: vk, Index: "ew:rt:rooms:game"}
	adapter := room.Challenges{Registry: reg, Type: "challenge_solo"}
	e.challengeRoom = adapter
	adapter.Expirer = e.challengesUC()
	cfg := func(owner string) roomhost.Config {
		return roomhost.Config{Owner: owner, LeaseTTL: time.Second, RenewEvery: 300 * time.Millisecond, WatchEvery: 200 * time.Millisecond,
			TickEvery: 50 * time.Millisecond, Batch: 10, Grace: roomhost.Grace{"challenge_solo": 2 * time.Second}}
	}
	m := &roomMetrics{}
	a := roomhost.NewHost(cfg("node-a"), reg, adapter, e.clock, m)
	b := roomhost.NewHost(cfg("node-b"), reg, adapter, e.clock, m)
	a.Handle("challenge_solo", adapter)
	b.Handle("challenge_solo", adapter)

	an := e.studentOfGrade(ctx, t, e.tenantA, e.schoolA, 3)
	st := e.startChallenge(t, an, chlTime3, "hard", "")
	if st.status != http.StatusCreated {
		t.Fatalf("start: %s", st.raw)
	}
	attempt := st.str("challenge_attempt_id")
	rec, err := reg.Room(ctx, attempt)
	if err != nil || rec == nil || rec.Type != "challenge_solo" || rec.TenantID != an.tenant.String() || rec.DueAt == nil {
		t.Fatalf("room announced: %+v %v", rec, err)
	}
	a.Step(ctx)
	b.Step(ctx)
	if a.Hosting() != 1 || b.Hosting() != 0 {
		t.Fatalf("one owner: a=%d b=%d", a.Hosting(), b.Hosting())
	}
	// node a crashes: no more steps; its lease expires in Valkey after the TTL (wall clock).
	time.Sleep(1200 * time.Millisecond)
	e.clock.Add(2 * time.Second)
	b.Step(ctx)
	if b.Hosting() != 1 || m.takeovers.Load() != 1 {
		t.Fatalf("takeover: b=%d takeovers=%d", b.Hosting(), m.takeovers.Load())
	}
	// Before due + grace nothing ends; then the timer of node b ends the attempt once.
	e.clock.Add(440 * time.Second)
	b.Step(ctx)
	if s, _ := e.attemptStatus(ctx, t, attempt); s != "in_progress" {
		t.Fatalf("ended early: %s", s)
	}
	e.clock.Add(15 * time.Second)
	b.Step(ctx)
	b.Step(ctx)
	a.Step(ctx) // the old owner comes back: its lease is gone, it fires nothing
	if s, why := e.attemptStatus(ctx, t, attempt); s != "failed" || why != "deadline_passed" {
		t.Fatalf("timer: %s %s", s, why)
	}
	if ev := e.completedEvents(ctx, t, attempt); len(ev) != 1 {
		t.Fatalf("timer fired twice: %d", len(ev))
	}
	if rec, err := reg.Room(ctx, attempt); err != nil || rec != nil || a.Hosting() != 0 || b.Hosting() != 0 {
		t.Fatalf("room forgotten: %+v %v a=%d b=%d", rec, err, a.Hosting(), b.Hosting())
	}
	// An attempt that ends otherwise (abandon) closes its room at once (after the cooldown of the hard failure).
	e.clock.Add(61 * time.Second)
	st2 := e.startChallenge(t, an, chlTime3, "easy", "")
	a2 := st2.str("challenge_attempt_id")
	if rec, _ := reg.Room(ctx, a2); rec == nil {
		t.Fatal("second room announced")
	}
	if r := e.abandonChallenge(t, an, a2, ""); r.status != http.StatusOK {
		t.Fatalf("abandon: %s", r.raw)
	}
	if rec, _ := reg.Room(ctx, a2); rec != nil {
		t.Fatal("room of an abandoned attempt closed")
	}
}
