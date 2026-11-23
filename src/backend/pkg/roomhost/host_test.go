package roomhost_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// memRegistry is a TEST DOUBLE of the Valkey registry (same semantics: SET NX PX lease, compare-and-renew, orphan index
// scored by the last renew); the Valkey implementation is checked against a real Valkey in the game integration tests.
type memRegistry struct {
	mu     sync.Mutex
	clk    clock.Clock
	rooms  map[string]roomhost.Room
	owner  map[string]string
	expiry map[string]time.Time
	score  map[string]time.Time
}

func newMem(clk clock.Clock) *memRegistry {
	return &memRegistry{clk: clk, rooms: map[string]roomhost.Room{}, owner: map[string]string{}, expiry: map[string]time.Time{},
		score: map[string]time.Time{}}
}

func (m *memRegistry) Announce(_ context.Context, r roomhost.Room) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rooms[r.ID] = r
	m.score[r.ID] = time.Time{}
	return nil
}

func (m *memRegistry) live(id string) bool {
	return m.owner[id] != "" && m.clk.Now().Before(m.expiry[id])
}

func (m *memRegistry) Acquire(_ context.Context, id, owner string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live(id) {
		return false, nil
	}
	if r, ok := m.rooms[id]; ok {
		r.Owner = owner
		m.rooms[id] = r
	}
	m.owner[id], m.expiry[id] = owner, m.clk.Now().Add(ttl)
	return true, nil
}

func (m *memRegistry) Renew(_ context.Context, id, owner string, ttl time.Duration, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rooms[id]; !ok || !m.live(id) || m.owner[id] != owner {
		return false, nil
	}
	m.expiry[id], m.score[id] = now.Add(ttl), now
	return true, nil
}

func (m *memRegistry) Release(_ context.Context, id, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owner[id] == owner {
		delete(m.owner, id)
		m.score[id] = time.Time{}
	}
	return nil
}

func (m *memRegistry) Room(_ context.Context, id string) (*roomhost.Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[id]
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func (m *memRegistry) Forget(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, id)
	delete(m.owner, id)
	delete(m.score, id)
	return nil
}

func (m *memRegistry) Orphans(_ context.Context, before time.Time, limit int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for id, s := range m.score {
		if !s.After(before) && len(out) < limit {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *memRegistry) ownerOf(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.live(id) {
		return ""
	}
	return m.owner[id]
}

// store is the checkpoint store of the module (PostgreSQL in module game): the due time and whether the room ended.
type store struct {
	mu     sync.Mutex
	due    map[string]time.Time
	closed map[string]bool
}

func (s *store) Load(_ context.Context, r roomhost.Room) (roomhost.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed[r.ID] {
		return roomhost.Checkpoint{Room: r, Closed: true}, nil
	}
	if d, ok := s.due[r.ID]; ok {
		r.DueAt = &d
	}
	return roomhost.Checkpoint{Room: r}, nil
}

type handler struct {
	fired atomic.Int32
	by    sync.Map
	fail  atomic.Bool
}

func (h *handler) OnTimer(ctx context.Context, r roomhost.Room, _ time.Time) (bool, error) {
	if h.fail.Load() {
		return false, errors.New("database down")
	}
	h.fired.Add(1)
	h.by.Store(r.ID, roomhost.OwnerFrom(ctx))
	return true, nil
}

type metrics struct{ takeovers, fired atomic.Int32 }

func (m *metrics) Takeover(string)            { m.takeovers.Add(1) }
func (m *metrics) TimerLag(string, float64)   { m.fired.Add(1) }
func (m *metrics) LeaseLost(string)           {}
func (m *metrics) Rooms(string, int)          {}
func (m *metrics) TimerFailed(string, string) {}

func cfg(owner string) roomhost.Config {
	return roomhost.Config{Owner: owner, LeaseTTL: 10 * time.Second, RenewEvery: 3 * time.Second, WatchEvery: 2 * time.Second,
		TickEvery: 250 * time.Millisecond, Batch: 10, Grace: roomhost.Grace{"challenge_solo": 2 * time.Second, "boss": 120 * time.Second}}
}

// F076-AC-17, AC-18: two nodes see one room, only one holds the lease; the owner stops renewing (crash) → the other node
// takes the lease once it expired, restores the room from the checkpoint store and fires its timer once; the timer is
// the server clock plus the grace of the room type.
func TestHostLeaseFailoverTimerOnce(t *testing.T) {
	t0 := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	clk := clock.NewFake(t0)
	reg := newMem(clk)
	due := t0.Add(30 * time.Second)
	st := &store{due: map[string]time.Time{"r1": due}, closed: map[string]bool{}}
	h := &handler{}
	m := &metrics{}
	a := roomhost.NewHost(cfg("node-a"), reg, st, clk, m)
	b := roomhost.NewHost(cfg("node-b"), reg, st, clk, m)
	a.Handle("challenge_solo", h)
	b.Handle("challenge_solo", h)
	ctx := context.Background()
	if err := reg.Announce(ctx, roomhost.Room{ID: "r1", Type: "challenge_solo", TenantID: "t1", DueAt: &due}); err != nil {
		t.Fatal(err)
	}
	a.Step(ctx) // node a takes the orphan room
	b.Step(ctx) // node b finds it owned
	if reg.ownerOf("r1") != "node-a" || a.Hosting() != 1 || b.Hosting() != 0 {
		t.Fatalf("one owner: %q a=%d b=%d", reg.ownerOf("r1"), a.Hosting(), b.Hosting())
	}
	// stepB advances the server clock one second at a time, node b stepping every second (node a crashed: no steps).
	stepB := func(seconds int) {
		for range seconds {
			clk.Advance(time.Second)
			b.Step(ctx)
		}
	}
	// Within the lease nobody else takes it.
	stepB(5)
	if b.Hosting() != 0 {
		t.Fatal("taken over before the lease expired")
	}
	// After the lease TTL node b takes it over from the checkpoint (due time of PostgreSQL, not of the registry).
	st.mu.Lock()
	st.due["r1"] = t0.Add(40 * time.Second)
	st.mu.Unlock()
	stepB(10) // t0 + 15 s
	if reg.ownerOf("r1") != "node-b" || b.Hosting() != 1 || m.takeovers.Load() != 1 {
		t.Fatalf("failover: owner %q hosting %d takeovers %d", reg.ownerOf("r1"), b.Hosting(), m.takeovers.Load())
	}
	// Due at 40 s + grace 2 s of challenge_solo: not before (SC-004: the room continues on node b).
	stepB(26) // t0 + 41 s
	if h.fired.Load() != 0 {
		t.Fatal("fired before due + grace")
	}
	stepB(2)    // t0 + 43 s
	a.Step(ctx) // the crashed node coming back must not fire again (its lease is gone)
	if h.fired.Load() != 1 || b.Hosting() != 0 || a.Hosting() != 0 {
		t.Fatalf("timer once: fired %d, hosting a=%d b=%d", h.fired.Load(), a.Hosting(), b.Hosting())
	}
	if by, _ := h.by.Load("r1"); by != "node-b" {
		t.Fatalf("fired by %v", by)
	}
	if r, _ := reg.Room(ctx, "r1"); r != nil {
		t.Fatal("a done room is forgotten")
	}
}

// A failing handler keeps the room (retried at a later tick); a room closed in the checkpoint store is forgotten on
// takeover without firing; the grace is per room type.
func TestHostRetryClosedAndGrace(t *testing.T) {
	t0 := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	clk := clock.NewFake(t0)
	reg := newMem(clk)
	due := t0.Add(time.Second)
	st := &store{due: map[string]time.Time{}, closed: map[string]bool{"gone": true}}
	h := &handler{}
	host := roomhost.NewHost(cfg("node-a"), reg, st, clk, &metrics{})
	host.Handle("challenge_solo", h)
	host.Handle("boss", h)
	ctx := context.Background()
	for _, r := range []roomhost.Room{{ID: "gone", Type: "challenge_solo", DueAt: &due}, {ID: "solo", Type: "challenge_solo", DueAt: &due},
		{ID: "boss", Type: "boss", DueAt: &due}} {
		if err := reg.Announce(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	st.due["solo"], st.due["boss"] = due, due
	h.fail.Store(true)
	host.Step(ctx)
	if host.Hosting() != 2 {
		t.Fatalf("closed room hosted: %d", host.Hosting())
	}
	if r, _ := reg.Room(ctx, "gone"); r != nil {
		t.Fatal("closed room not forgotten")
	}
	clk.Advance(5 * time.Second)
	host.Step(ctx)
	if host.Hosting() != 2 || h.fired.Load() != 0 {
		t.Fatal("a failed timer keeps its room")
	}
	h.fail.Store(false)
	clk.Advance(2 * time.Second)
	host.Step(ctx)
	if h.fired.Load() != 1 || host.Hosting() != 1 {
		t.Fatalf("solo fired after the retry, boss waits its 120 s grace: fired %d hosting %d", h.fired.Load(), host.Hosting())
	}
	for range 120 {
		clk.Advance(time.Second)
		host.Step(ctx)
	}
	if h.fired.Load() != 2 || host.Hosting() != 0 {
		t.Fatalf("boss after its grace: fired %d hosting %d", h.fired.Load(), host.Hosting())
	}
	if roomhost.Grace(nil).For("unknown") != 0 {
		t.Fatal("no grace for an unknown type")
	}
}

// A room whose lease is lost (another node renewed after a pause) stops here; Close forgets it at once.
func TestHostLeaseLostAndClose(t *testing.T) {
	t0 := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	clk := clock.NewFake(t0)
	reg := newMem(clk)
	st := &store{due: map[string]time.Time{}, closed: map[string]bool{}}
	host := roomhost.NewHost(cfg("node-a"), reg, st, clk, &metrics{})
	host.Handle("challenge_solo", &handler{})
	ctx := context.Background()
	_ = reg.Announce(ctx, roomhost.Room{ID: "r", Type: "challenge_solo"})
	host.Step(ctx)
	if host.Hosting() != 1 {
		t.Fatal("hosted")
	}
	clk.Advance(11 * time.Second) // paused past the TTL: another node takes it
	if ok, _ := reg.Acquire(ctx, "r", "node-b", 10*time.Second); !ok {
		t.Fatal("node b acquires")
	}
	host.Step(ctx)
	if host.Hosting() != 0 {
		t.Fatal("a lost lease stops the room here")
	}
	_ = reg.Announce(ctx, roomhost.Room{ID: "s", Type: "challenge_solo"})
	host.Step(ctx)
	if err := host.Close(ctx, "s"); err != nil || host.Hosting() != 0 {
		t.Fatalf("close: %v %d", err, host.Hosting())
	}
	if r, _ := reg.Room(ctx, "s"); r != nil {
		t.Fatal("closed room forgotten")
	}
}
