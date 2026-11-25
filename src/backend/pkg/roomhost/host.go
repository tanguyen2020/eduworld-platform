package roomhost

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock"
)

// Room is the registry record of a hosted room (Valkey ew:rt:room:<room_id>, VALKEY_DESIGN §3.1): its type (grace,
// handler), its tenant and the server time its timer is due. The owner is whoever holds the lease
// (ew:rt:room:<room_id>:lease). The source of truth of the room state is the database of the module (checkpoint).
type Room struct {
	ID       string
	Type     string // challenge_solo (F076); boss, battle… later
	TenantID string
	DueAt    *time.Time // nil: no timer
	Owner    string     // last owner written in the record ("" = never owned)
}

// Checkpoint is the room as the module reads it back from its database on takeover (F076-AC-17): the due time of the
// database wins over the registry; Closed = the room ended there (it is forgotten, its timer never fires).
type Checkpoint struct {
	Room   Room
	Closed bool
}

// CheckpointStore reads the checkpoint of a room; implemented by the module that owns the room type (PostgreSQL).
type CheckpointStore interface {
	Load(ctx context.Context, room Room) (Checkpoint, error)
}

// Handler is the business side of a room type, implemented by the module (never in this package, R13).
type Handler interface {
	// OnTimer runs on the lease owner when the room is due (server clock + grace of the type). done = the room ended and
	// is forgotten; an error keeps the room and retries at a later tick (the module makes it idempotent).
	OnTimer(ctx context.Context, room Room, at time.Time) (done bool, err error)
}

// Registry is where rooms, leases and the orphan index live (Valkey: ValkeyRegistry).
type Registry interface {
	// Announce writes the room record and puts it in the orphan index so that an owner picks it up.
	Announce(ctx context.Context, room Room) error
	// Acquire takes the lease of a room without a live owner (SET NX PX); true = owner now.
	Acquire(ctx context.Context, room, owner string, ttl time.Duration) (bool, error)
	// Renew extends the lease while owner holds it and the room exists, and scores the room in the index; false = lost.
	Renew(ctx context.Context, room, owner string, ttl time.Duration, now time.Time) (bool, error)
	// Release gives the lease back (graceful stop) and makes the room an orphan at once.
	Release(ctx context.Context, room, owner string) error
	// Room reads a room record (nil when unknown).
	Room(ctx context.Context, id string) (*Room, error)
	// Forget removes the room record, its lease and its index entry (the room ended).
	Forget(ctx context.Context, id string) error
	// Orphans lists rooms whose lease was not renewed since before.
	Orphans(ctx context.Context, before time.Time, limit int) ([]string, error)
}

// Grace is the grace per room type (CHALLENGE_BOSS_SYSTEM §4: deadline_grace of a challenge, 120 s of a party boss…).
type Grace map[string]time.Duration

// For returns the grace of a room type (0 when none).
func (g Grace) For(roomType string) time.Duration { return g[roomType] }

// Metrics observes the host; nil = not observed.
type Metrics interface {
	Takeover(roomType string)
	TimerLag(roomType string, seconds float64)
	LeaseLost(roomType string)
	Rooms(roomType string, n int)
	TimerFailed(roomType, reason string)
}

// Config of a host.
type Config struct {
	Owner      string        // instance id written in the lease
	LeaseTTL   time.Duration // lease TTL (10 s, VALKEY_DESIGN)
	RenewEvery time.Duration // lease renewal period (3 s)
	WatchEvery time.Duration // orphan scan period (2 s)
	TickEvery  time.Duration // timer resolution (250 ms)
	Batch      int           // orphans taken per scan
	Grace      Grace
}

type hosted struct {
	room      Room
	nextTryAt time.Time // a failed timer is retried from here
}

// Host hosts rooms on one node: it takes over orphan rooms (lease + checkpoint), keeps their leases and fires their
// timers on the server clock (F076-AC-17, AC-18). One node owns a room at a time; a node that loses the lease drops
// the room before firing anything, so a timer never fires twice. Business code is the Handler of each room type.
type Host struct {
	cfg      Config
	reg      Registry
	store    CheckpointStore
	clk      clock.Clock
	metrics  Metrics
	handlers map[string]Handler

	mu        sync.Mutex
	rooms     map[string]*hosted
	lastRenew time.Time
	lastWatch time.Time
}

// NewHost builds a host (clk nil = system clock).
func NewHost(cfg Config, reg Registry, store CheckpointStore, clk clock.Clock, m Metrics) *Host {
	if clk == nil {
		clk = clock.System()
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10 * time.Second
	}
	if cfg.RenewEvery <= 0 {
		cfg.RenewEvery = cfg.LeaseTTL / 3
	}
	if cfg.WatchEvery <= 0 {
		cfg.WatchEvery = 2 * time.Second
	}
	if cfg.TickEvery <= 0 {
		cfg.TickEvery = 250 * time.Millisecond
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 50
	}
	return &Host{cfg: cfg, reg: reg, store: store, clk: clk, metrics: m, handlers: map[string]Handler{}, rooms: map[string]*hosted{}}
}

// Handle registers the handler of a room type (before Run).
func (h *Host) Handle(roomType string, hd Handler) { h.handlers[roomType] = hd }

// Hosting is the number of rooms this node owns.
func (h *Host) Hosting() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

type ownerKey struct{}

// OwnerFrom returns the owner node a handler runs on (logs, tests).
func OwnerFrom(ctx context.Context) string {
	s, _ := ctx.Value(ownerKey{}).(string)
	return s
}

// Run steps the host every TickEvery (renewal and orphan scan at their own periods) until ctx ends, then releases the
// leases so that another node takes the rooms over at once.
func (h *Host) Run(ctx context.Context) error {
	t := time.NewTicker(h.cfg.TickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			h.handover()
			return nil
		case <-t.C:
			now := h.clk.Now()
			if now.Sub(h.lastRenew) >= h.cfg.RenewEvery {
				h.renew(ctx, now)
			}
			if now.Sub(h.lastWatch) >= h.cfg.WatchEvery {
				h.watch(ctx, now)
			}
			h.fire(ctx)
		}
	}
}

// Step runs one renewal, one orphan scan and one timer pass at once (tests, and the first pass of Run).
func (h *Host) Step(ctx context.Context) {
	now := h.clk.Now()
	h.renew(ctx, now)
	h.watch(ctx, now)
	h.fire(ctx)
}

// Close forgets a room this node or another hosts (the room ended outside its timer: goal reached, abandoned).
func (h *Host) Close(ctx context.Context, id string) error {
	h.mu.Lock()
	delete(h.rooms, id)
	h.mu.Unlock()
	return h.reg.Forget(ctx, id)
}

func (h *Host) local() []*hosted {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*hosted, 0, len(h.rooms))
	for _, r := range h.rooms {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].room.ID < out[j].room.ID })
	return out
}

func (h *Host) drop(id string) {
	h.mu.Lock()
	delete(h.rooms, id)
	h.mu.Unlock()
}

func (h *Host) observe(fn func(m Metrics)) {
	if h.metrics != nil {
		fn(h.metrics)
	}
}

// renew extends the lease of every local room; a room whose lease is lost (or which was forgotten) stops here.
func (h *Host) renew(ctx context.Context, now time.Time) {
	h.lastRenew = now
	for _, r := range h.local() {
		ok, err := h.reg.Renew(ctx, r.room.ID, h.cfg.Owner, h.cfg.LeaseTTL, now)
		if err != nil {
			// A short outage does not move the room: the lease expires in the registry only after its TTL.
			slog.WarnContext(ctx, "roomhost: renew failed", "room_type", r.room.Type, "error", err)
			continue
		}
		if !ok {
			h.drop(r.room.ID)
			h.observe(func(m Metrics) { m.LeaseLost(r.room.Type) })
		}
	}
	h.count()
}

func (h *Host) count() {
	if h.metrics == nil {
		return
	}
	by := map[string]int{}
	for t := range h.handlers {
		by[t] = 0
	}
	for _, r := range h.local() {
		by[r.room.Type]++
	}
	for t, n := range by {
		h.metrics.Rooms(t, n)
	}
}

// watch takes over the orphan rooms of the handled types: lease, then the checkpoint of the module.
func (h *Host) watch(ctx context.Context, now time.Time) {
	h.lastWatch = now
	ids, err := h.reg.Orphans(ctx, now.Add(-h.cfg.LeaseTTL), h.cfg.Batch)
	if err != nil {
		slog.WarnContext(ctx, "roomhost: orphan scan failed", "error", err)
		return
	}
	for _, id := range ids {
		h.mu.Lock()
		_, mine := h.rooms[id]
		h.mu.Unlock()
		if mine {
			continue
		}
		if err := h.takeover(ctx, id, now); err != nil {
			slog.WarnContext(ctx, "roomhost: takeover failed", "error", err)
		}
	}
}

func (h *Host) takeover(ctx context.Context, id string, now time.Time) error {
	rec, err := h.reg.Room(ctx, id)
	if err != nil {
		return err
	}
	if rec == nil {
		return h.reg.Forget(ctx, id) // a stale index entry
	}
	if _, ok := h.handlers[rec.Type]; !ok {
		return nil // another room type: another host takes it
	}
	ok, err := h.reg.Acquire(ctx, id, h.cfg.Owner, h.cfg.LeaseTTL)
	if err != nil || !ok {
		return err
	}
	cp, err := h.store.Load(ctx, *rec)
	if err != nil {
		_ = h.reg.Release(ctx, id, h.cfg.Owner)
		return err
	}
	if cp.Closed {
		return h.reg.Forget(ctx, id)
	}
	if _, err := h.reg.Renew(ctx, id, h.cfg.Owner, h.cfg.LeaseTTL, now); err != nil {
		_ = h.reg.Release(ctx, id, h.cfg.Owner)
		return err
	}
	room := cp.Room
	room.Owner = h.cfg.Owner
	h.mu.Lock()
	h.rooms[id] = &hosted{room: room}
	h.mu.Unlock()
	if rec.Owner != "" && rec.Owner != h.cfg.Owner {
		h.observe(func(m Metrics) { m.Takeover(rec.Type) })
	}
	return nil
}

// fire runs the due timers of the local rooms (server clock + grace of the type).
func (h *Host) fire(ctx context.Context) {
	now := h.clk.Now()
	for _, r := range h.local() {
		if r.room.DueAt == nil || now.Before(r.nextTryAt) {
			continue
		}
		due := r.room.DueAt.Add(h.cfg.Grace.For(r.room.Type))
		if now.Before(due) {
			continue
		}
		hd := h.handlers[r.room.Type]
		done, err := hd.OnTimer(context.WithValue(ctx, ownerKey{}, h.cfg.Owner), r.room, now)
		if err != nil {
			slog.WarnContext(ctx, "roomhost: timer failed", "room_type", r.room.Type, "error", err)
			h.observe(func(m Metrics) { m.TimerFailed(r.room.Type, "error") })
			h.mu.Lock()
			r.nextTryAt = now.Add(time.Second)
			h.mu.Unlock()
			continue
		}
		lag := now.Sub(due).Seconds()
		h.observe(func(m Metrics) { m.TimerLag(r.room.Type, lag) })
		if done {
			h.drop(r.room.ID)
			if err := h.reg.Forget(ctx, r.room.ID); err != nil {
				slog.WarnContext(ctx, "roomhost: forget failed", "room_type", r.room.Type, "error", err)
			}
			continue
		}
		h.reload(ctx, r, now)
	}
}

// reload reads the checkpoint again after a timer that did not end the room (F103: the next deadline of a boss phase
// or a presence check): the due time of the database wins; a room closed there is forgotten. A failed read keeps the
// room and retries at the next second.
func (h *Host) reload(ctx context.Context, r *hosted, now time.Time) {
	cp, err := h.store.Load(ctx, r.room)
	if err != nil {
		h.mu.Lock()
		r.nextTryAt = now.Add(time.Second)
		h.mu.Unlock()
		return
	}
	if cp.Closed {
		h.drop(r.room.ID)
		_ = h.reg.Forget(ctx, r.room.ID)
		return
	}
	h.mu.Lock()
	r.room.DueAt = cp.Room.DueAt
	if r.room.DueAt != nil && !r.room.DueAt.After(now) {
		r.nextTryAt = now.Add(time.Second) // a due time still past: no busy loop
	}
	h.mu.Unlock()
}

// handover releases every local lease (graceful stop): the rooms become orphans another node takes at once.
func (h *Host) handover() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, r := range h.local() {
		_ = h.reg.Release(ctx, r.room.ID, h.cfg.Owner)
		h.drop(r.room.ID)
	}
}
