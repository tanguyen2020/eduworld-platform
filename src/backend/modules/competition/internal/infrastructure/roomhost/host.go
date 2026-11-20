// Package roomhost is the room host of the hub instances of module competition (F091, REALTIME_ARCHITECTURE §5.1,
// §6): the instance holding the lease of a room keeps its state in memory, subscribes its input ew.in.room.<room_id>
// and its internal operations ew.int.competition.room.<room_id>.<op>, ticks at hub_sync_rate and publishes the
// snapshots packed per gateway node on ew:rt:room:<room_id>:out. Any instance reaches a room through NATS; an
// instance that finds a room without owner takes it over (failover, F091-AC-20). Valkey holds the registry, the lease
// and the last positions only — never the simulation (FR-MP-006, ADR-0004).
package roomhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/observability"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// Config of the room host (CONFIG_DEFAULTS through internal/infrastructure/config).
type Config struct {
	InstanceID string        // owner id written in the lease (EW_INSTANCE_ID)
	TickHz     int           // hub_sync_rate
	Grace      time.Duration // disconnect_grace hub
	IdleClose  time.Duration // hub_instance_idle_close
	Silence    time.Duration // no input at all: the member is treated as disconnected [Proposed]
	EmoteRate  time.Duration // emote_rate
	LeaseTTL   time.Duration // 10 s
	LeaseEvery time.Duration // 3 s
	WatchEvery time.Duration // failover watcher period
	OpTimeout  time.Duration // NATS request to the owner
	// ClientOnly: this process never hosts a room (worker): it only calls the owners and never takes a room over.
	ClientOnly bool
}

// Areas loads the runtime data of an area (ref.world_areas) for a room of tenant.
type Areas func(ctx context.Context, tenant uuid.UUID, area string) (*hub.Area, error)

// Host hosts the hub instances of this instance and implements port.Rooms for the whole cluster.
type Host struct {
	cfg     Config
	nc      *nats.Conn
	reg     cache.HubRegistry
	pub     rt.Publisher
	areas   Areas
	events  port.RoomEvents
	metrics observability.Hub
	now     func() time.Time

	base    context.Context
	mu      sync.Mutex
	rooms   map[string]*room
	crashed bool
}

var _ port.Rooms = (*Host)(nil)

// New builds a host; Run must be started for the failover watcher and the presence signals.
func New(cfg Config, nc *nats.Conn, reg cache.HubRegistry, pub rt.Publisher, areas Areas, events port.RoomEvents, now func() time.Time) *Host {
	if now == nil {
		now = time.Now
	}
	if cfg.OpTimeout <= 0 {
		cfg.OpTimeout = 2 * time.Second
	}
	return &Host{cfg: cfg, nc: nc, reg: reg, pub: pub, areas: areas, events: events, now: now, base: context.Background(),
		rooms: map[string]*room{}}
}

// SetApp sets the application callbacks and the area loader (wiring breaks the Hub ↔ Host cycle).
func (h *Host) SetApp(e port.RoomEvents, areas Areas) { h.events, h.areas = e, areas }

// opSubject is the internal subject of an operation on a room. It is outside ew.cmd.> and ew.in.> so the NATS user
// of the gateway can never reach it [Proposed F091 A-5].
func opSubject(room, op string) string { return "ew.int.competition.room." + room + "." + op }

// Run watches orphan rooms (failover) and the presence signals until ctx ends, then hands the hosted rooms over.
func (h *Host) Run(ctx context.Context) error {
	h.mu.Lock()
	h.base = ctx
	h.mu.Unlock()
	if h.nc == nil {
		return errors.New("roomhost: no NATS connection")
	}
	sub, err := h.nc.Subscribe(rt.PresenceSubjectAll, h.onPresence)
	if err != nil {
		return fmt.Errorf("roomhost: presence: %w", err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	every := h.cfg.WatchEvery
	if every <= 0 {
		every = 2 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			h.handover()
			return nil
		case <-t.C:
			h.watch(ctx)
		}
	}
}

// onPresence holds the seat of a student whose connection ended (F091-AC-09): avatar stands still.
func (h *Host) onPresence(m *nats.Msg) {
	var p rt.PresenceChanged
	if json.Unmarshal(m.Data, &p) != nil || !p.Gone() {
		return
	}
	for _, r := range h.local() {
		r.mu.Lock()
		if mm, ok := r.in.Member(p.UserID); ok && mm.TenantID == p.TenantID {
			r.in.Hold(p.UserID, h.now(), h.cfg.Grace)
		}
		r.mu.Unlock()
	}
}

func (h *Host) local() []*room {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*room, 0, len(h.rooms))
	for _, r := range h.rooms {
		out = append(out, r)
	}
	return out
}

func (h *Host) get(id string) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rooms[id]
}

// watch takes over the rooms whose lease was not renewed (owner dead): F091-AC-20.
func (h *Host) watch(ctx context.Context) {
	orphans, err := h.reg.Orphans(ctx, h.now().Add(-h.cfg.LeaseTTL), 20)
	if err != nil {
		slog.WarnContext(ctx, "roomhost: orphan scan failed", "error", err)
		return
	}
	for _, id := range orphans {
		if h.get(id) != nil {
			continue
		}
		if _, err := h.takeover(ctx, id); err != nil {
			slog.WarnContext(ctx, "roomhost: takeover failed", "error", err)
		}
	}
}

// takeover acquires the lease of a room without owner and restores it from the registry.
func (h *Host) takeover(ctx context.Context, id string) (*room, error) {
	ok, err := h.reg.Acquire(ctx, id, h.cfg.InstanceID, h.cfg.LeaseTTL)
	if err != nil || !ok {
		return nil, err
	}
	meta, err := h.reg.Meta(ctx, id)
	if err != nil || meta == nil {
		_ = h.reg.Release(ctx, id, h.cfg.InstanceID)
		if meta == nil && err == nil {
			err = command.ErrRoomGone
		}
		return nil, err
	}
	members, err := h.reg.Members(ctx, id)
	if err != nil {
		_ = h.reg.Release(ctx, id, h.cfg.InstanceID)
		return nil, err
	}
	r, err := h.start(ctx, *meta)
	if err != nil {
		_ = h.reg.Release(ctx, id, h.cfg.InstanceID)
		return nil, err
	}
	now := h.now()
	r.mu.Lock()
	for user, rec := range members {
		pos := r.in.Area.Spawn("")
		if p, err := h.reg.Position(ctx, id, user); err == nil && p != nil {
			pos = *p
		}
		r.in.Restore(hub.Seat{Player: rec.Player, TenantID: meta.Partition.TenantID, SchoolID: rec.SchoolID, GradeBand: rec.GradeBand,
			GameSessionID: rec.GameSessionID, Node: rec.Node}, rec.EID, pos, now)
	}
	n := r.in.Size()
	r.mu.Unlock()
	h.metrics.Players(hub.RoomTypeHub, float64(n))
	// Every member resyncs: positions come back with the next inputs (hub state is rebuildable).
	h.publish(ctx, id, "hub.instance.migrated", map[string]any{"old_room_id": id, "room_id": id, "reason": "failover"}, nil)
	slog.InfoContext(ctx, "roomhost: room taken over", "members", n)
	return r, nil
}

// Crash stops hosting without giving the leases back, like a killed process (tests of the failover, F091-AC-20).
func (h *Host) Crash() {
	h.mu.Lock()
	h.crashed = true
	h.mu.Unlock()
	for _, r := range h.local() {
		r.stop(false)
	}
}

// handover releases every hosted room at shutdown so another instance takes it over at its next watch.
func (h *Host) handover() {
	h.mu.Lock()
	crashed := h.crashed
	h.mu.Unlock()
	if crashed {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, r := range h.local() {
		r.stop(false)
		_ = h.reg.Release(ctx, r.meta.RoomID, h.cfg.InstanceID)
	}
}

// Open implements port.Rooms: register a new room and host it here (owner = the instance that created it).
func (h *Host) Open(ctx context.Context, m port.RoomMeta) error {
	if err := h.reg.Register(ctx, m, h.cfg.InstanceID, h.now()); err != nil {
		return err
	}
	ok, err := h.reg.Acquire(ctx, m.RoomID, h.cfg.InstanceID, h.cfg.LeaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		return command.ErrRoomGone
	}
	_, err = h.start(ctx, m)
	return err
}

// room is one hosted instance.
type room struct {
	h       *Host
	meta    port.RoomMeta
	mu      sync.Mutex
	in      *hub.Instance
	cancel  context.CancelFunc
	subs    []*nats.Subscription
	notices map[uuid.UUID]time.Time // ROOM_NOT_MEMBER notices (1 per 5 s per sender)
	once    sync.Once
}

func (h *Host) start(ctx context.Context, m port.RoomMeta) (*room, error) {
	area, err := h.areas(ctx, m.Partition.TenantID, m.AreaID)
	if err != nil {
		return nil, err
	}
	if area == nil {
		return nil, fmt.Errorf("roomhost: area %s not projected", m.AreaID)
	}
	h.mu.Lock()
	if r, ok := h.rooms[m.RoomID]; ok {
		h.mu.Unlock()
		return r, nil
	}
	base := h.base
	h.mu.Unlock()
	rctx, cancel := context.WithCancel(context.WithoutCancel(base))
	r := &room{h: h, meta: m, in: hub.NewInstance(m.RoomID, *area, m.Partition, m.Capacity, h.now()), cancel: cancel,
		notices: map[uuid.UUID]time.Time{}}
	in, err := h.nc.Subscribe(rt.RoomInputSubject(m.RoomID), r.onInput)
	if err != nil {
		cancel()
		return nil, err
	}
	ops, err := h.nc.Subscribe(opSubject(m.RoomID, "*"), r.onOp)
	if err != nil {
		_ = in.Unsubscribe()
		cancel()
		return nil, err
	}
	r.subs = []*nats.Subscription{in, ops}
	_ = h.nc.Flush()
	h.mu.Lock()
	h.rooms[m.RoomID] = r
	h.mu.Unlock()
	h.metrics.Rooms(hub.RoomTypeHub, 1)
	go roomhost.Tick(rctx, h.cfg.TickHz, h.now, r.tick, func(d time.Duration) { h.metrics.Tick(hub.RoomTypeHub, d) })
	go roomhost.KeepLease(rctx, h.cfg.LeaseEvery, h.cfg.LeaseTTL, func(ctx context.Context) (bool, error) {
		return h.reg.Renew(ctx, m.RoomID, h.cfg.InstanceID, h.cfg.LeaseTTL, h.now())
	}, func() {
		slog.Warn("roomhost: lease lost, room stopped here")
		r.stop(false)
	})
	return r, nil
}

// stop ends hosting (closed: the room is closed for good).
func (r *room) stop(closed bool) {
	r.once.Do(func() {
		r.cancel()
		for _, s := range r.subs {
			_ = s.Unsubscribe()
		}
		r.h.mu.Lock()
		delete(r.h.rooms, r.meta.RoomID)
		r.h.mu.Unlock()
		r.mu.Lock()
		n := r.in.Size()
		r.mu.Unlock()
		r.h.metrics.Rooms(hub.RoomTypeHub, -1)
		r.h.metrics.Players(hub.RoomTypeHub, -float64(n))
		if closed {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := r.h.reg.Remove(ctx, r.meta, r.h.cfg.InstanceID); err != nil {
				slog.Warn("roomhost: registry remove failed", "error", err)
			}
			if r.h.events != nil {
				if err := r.h.events.RoomClosed(ctx, r.meta); err != nil {
					slog.Warn("roomhost: room close not recorded", "error", err) // the job close-idle-hub-instances retries
				}
			}
		}
	})
}

// publish sends a message on the room output, to every member or to target only.
func (h *Host) publish(ctx context.Context, roomID, typ string, payload any, target []uuid.UUID) {
	e, err := rt.NewMessage(typ, payload, h.now())
	if err != nil {
		slog.ErrorContext(ctx, "roomhost: invalid message", "type", typ, "error", err)
		return
	}
	if target != nil {
		if len(target) == 0 {
			return
		}
		e.Target = &rt.Target{UserIDs: target}
	}
	if err := h.pub.Publish(ctx, rt.RoomOutChannel(roomID), e); err != nil {
		slog.WarnContext(ctx, "roomhost: publish failed", "type", typ, "error", err)
	}
}

func (h *Host) toUser(ctx context.Context, user uuid.UUID, typ string, payload any) {
	e, err := rt.NewMessage(typ, payload, h.now())
	if err == nil {
		err = h.pub.ToUser(ctx, user, e)
	}
	if err != nil {
		slog.WarnContext(ctx, "roomhost: user publish failed", "type", typ, "error", err)
	}
}

func others(in *hub.Instance, except uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{}
	for _, m := range in.Members() {
		if m.Player.UserID != except && !m.Held() {
			out = append(out, m.Player.UserID)
		}
	}
	return out
}

// moveIn is the payload of hub.player.move (AsyncAPI HubPlayerMove).
type moveIn struct {
	RoomID    string    `json:"room_id"`
	ClientSeq int64     `json:"client_seq"`
	P         *hub.Vec3 `json:"p"`
	R         float64   `json:"r"`
	A         string    `json:"a"`
}

// onInput applies hub.player.move (F091-AC-07, AC-08, AC-12, AC-13): identity and tenant from the Ew-* headers only.
func (r *room) onInput(m *nats.Msg) {
	c, err := rt.CallerFrom(m.Header)
	if err != nil {
		return // not from the gateway (F091-AC-08)
	}
	var in moveIn
	if json.Unmarshal(m.Data, &in) != nil || in.P == nil || len(in.A) > 255 || in.RoomID != r.meta.RoomID {
		return
	}
	now := r.h.now()
	r.mu.Lock()
	mm, ok := r.in.Member(c.UserID)
	if !ok || c.TenantID != r.meta.Partition.TenantID {
		notify := now.Sub(r.notices[c.UserID]) > 5*time.Second
		if notify {
			r.notices[c.UserID] = now
		}
		r.mu.Unlock()
		if notify { // F091-AC-07: the room is untouched, the sender learns it is not a member
			r.h.toUser(r.h.base, c.UserID, "system.command.failed", map[string]any{"code": command.CodeRoomNotMember, "retryable": false})
		}
		return
	}
	school := mm.SchoolID
	res, _ := r.in.Move(c.UserID, hub.Input{ClientSeq: in.ClientSeq, P: *in.P, R: in.R, A: in.A}, now)
	r.mu.Unlock()
	if !res.Corrected {
		return
	}
	r.h.metrics.Violation(res.Reason)
	ctx := r.h.base
	r.h.toUser(ctx, c.UserID, "hub.player.corrected", map[string]any{"room_id": r.meta.RoomID, "client_seq": in.ClientSeq,
		"p": res.Valid, "reason": res.Reason})
	if res.Signal && r.h.events != nil {
		go func(user uuid.UUID, reason string, count int, sample hub.Vec3) {
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := r.h.events.Violation(sctx, r.meta, user, school, reason, count, sample); err != nil {
				slog.Warn("roomhost: integrity signal not recorded", "error", err)
			}
		}(c.UserID, res.Reason, res.Violations, *in.P)
	}
}

// tick advances the instance (F091-AC-09, AC-10, AC-14): expired seats leave, snapshots go out packed per node, the
// last positions are written about once per second and an instance empty for idle_close closes.
func (r *room) tick(now time.Time) {
	ctx := r.h.base
	r.mu.Lock()
	gone := r.in.Expired(now, r.h.cfg.Grace, r.h.cfg.Silence)
	snaps := r.in.Tick()
	var pos map[uuid.UUID]hub.Vec3
	hz := int64(r.h.cfg.TickHz)
	if hz <= 0 {
		hz = 10
	}
	if r.in.TickNo()%hz == 0 {
		pos = map[uuid.UUID]hub.Vec3{}
		for _, m := range r.in.Members() {
			pos[m.Player.UserID] = m.Mover.Pos
		}
	}
	remaining := others(r.in, uuid.Nil)
	seats := r.in.Size()
	idle := r.h.cfg.IdleClose > 0 && r.in.IdleFor(now) >= r.h.cfg.IdleClose
	r.mu.Unlock()

	for _, m := range gone {
		r.removed(ctx, m, "disconnected", seats, remaining)
	}
	r.sendSnapshots(ctx, snaps, now)
	if pos != nil {
		if err := r.h.reg.Positions(ctx, r.meta.RoomID, pos); err != nil {
			slog.Debug("roomhost: positions not written", "error", err)
		}
	}
	if idle {
		r.stop(true)
	}
}

// removed publishes hub.player.left and reports the removal (membership row, presence).
func (r *room) removed(ctx context.Context, m *hub.Member, reason string, seats int, remaining []uuid.UUID) {
	r.h.metrics.Players(hub.RoomTypeHub, -1)
	if err := r.h.reg.Unseat(ctx, r.meta, m.Player.UserID, seats); err != nil {
		slog.WarnContext(ctx, "roomhost: unseat failed", "error", err)
	}
	r.h.publish(ctx, r.meta.RoomID, "hub.player.left", map[string]any{"room_id": r.meta.RoomID, "eid": m.EID}, remaining)
	if r.h.events != nil {
		if err := r.h.events.MemberRemoved(ctx, r.meta, m.Player.UserID, m.SchoolID, reason); err != nil {
			slog.WarnContext(ctx, "roomhost: removal not recorded", "error", err)
		}
	}
}

type snapshotOut struct {
	RoomID   string       `json:"room_id"`
	Tick     int64        `json:"tick"`
	AckSeq   int64        `json:"ack_client_seq"`
	Entities []hub.Entity `json:"entities"`
	Removed  []int        `json:"removed,omitempty"`
}

// sendSnapshots packs the viewers' snapshots per gateway node, one droppable bundle each (research R4).
func (r *room) sendSnapshots(ctx context.Context, snaps []hub.ViewerSnapshot, now time.Time) {
	if len(snaps) == 0 {
		return
	}
	byNode := map[string][]rt.BundleItem{}
	total := 0
	for _, s := range snaps {
		ents := s.Entities
		if ents == nil {
			ents = []hub.Entity{}
		}
		b, err := json.Marshal(snapshotOut{RoomID: r.meta.RoomID, Tick: s.Tick, AckSeq: s.AckSeq, Entities: ents, Removed: s.Removed})
		if err != nil {
			continue
		}
		total += len(b)
		byNode[s.Node] = append(byNode[s.Node], rt.BundleItem{UserID: s.UserID, Payload: b})
	}
	r.h.metrics.SnapshotBytes(hub.RoomTypeHub, total)
	for _, items := range byNode {
		e := rt.Envelope{V: 1, Kind: rt.KindBundle, Droppable: true, Bundle: &rt.Bundle{Type: "hub.snapshot.updated", Ts: now.UnixMilli(), Items: items}}
		if err := r.h.pub.Publish(ctx, rt.RoomOutChannel(r.meta.RoomID), e); err != nil {
			slog.Debug("roomhost: snapshot not published", "error", err) // droppable
		}
	}
}

// op is an operation on a room (internal NATS request/reply between instances).
type op struct {
	TenantID uuid.UUID         `json:"tenant_id"`
	UserID   uuid.UUID         `json:"user_id"`
	Join     *port.JoinRequest `json:"join,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	Emote    string            `json:"emote,omitempty"`
	Player   *hub.PublicPlayer `json:"player,omitempty"`
}

type opReply struct {
	Code   string          `json:"code,omitempty"` // full, not_member, wrong_partition, gone, error
	Result json.RawMessage `json:"result,omitempty"`
}

var opErrors = map[string]error{"full": hub.ErrFull, "not_member": hub.ErrNotMember, "wrong_partition": hub.ErrWrongPartition,
	"gone": command.ErrRoomGone}

func codeOf(err error) string {
	for code, e := range opErrors {
		if errors.Is(err, e) {
			return code
		}
	}
	return "error"
}

func (r *room) onOp(m *nats.Msg) {
	var o op
	reply := opReply{}
	kind := m.Subject[len(opSubject(r.meta.RoomID, "")):]
	res, err := func() (any, error) {
		if err := json.Unmarshal(m.Data, &o); err != nil {
			return nil, err
		}
		return r.apply(r.h.base, kind, o)
	}()
	if err != nil {
		reply.Code = codeOf(err)
	} else if res != nil {
		reply.Result, _ = json.Marshal(res)
	}
	b, _ := json.Marshal(reply)
	_ = m.Respond(b)
}

// apply runs an operation on the hosted instance.
func (r *room) apply(ctx context.Context, kind string, o op) (any, error) {
	now := r.h.now()
	switch kind {
	case "join":
		if o.Join == nil {
			return nil, errors.New("join without seat")
		}
		return r.join(ctx, *o.Join, now)
	case "leave":
		r.mu.Lock()
		m, ok := r.in.Leave(o.UserID, now)
		remaining, seats := others(r.in, uuid.Nil), r.in.Size()
		r.mu.Unlock()
		if !ok {
			return nil, hub.ErrNotMember
		}
		r.removed(ctx, m, o.Reason, seats, remaining)
		return nil, nil
	case "emote":
		r.mu.Lock()
		mm, ok := r.in.Member(o.UserID)
		if !ok || mm.TenantID != o.TenantID {
			r.mu.Unlock()
			return nil, hub.ErrNotMember
		}
		eid, owned, wait, err := r.in.Emote(o.UserID, o.Emote, now, r.h.cfg.EmoteRate)
		targets := others(r.in, uuid.Nil)
		r.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if owned && wait == 0 {
			r.h.publish(ctx, r.meta.RoomID, "hub.player.emoted", map[string]any{"room_id": r.meta.RoomID, "eid": eid, "emote_id": o.Emote}, targets)
		}
		return port.EmoteResult{Owned: owned, Wait: wait}, nil
	case "appearance":
		if o.Player == nil {
			return nil, errors.New("appearance without player")
		}
		r.mu.Lock()
		mm, ok := r.in.SetPlayer(o.UserID, *o.Player)
		var rec cache.MemberRecord
		var targets []uuid.UUID
		var eid, seats int
		if ok {
			rec = cache.MemberRecord{EID: mm.EID, Player: mm.Player, SchoolID: mm.SchoolID, GradeBand: mm.GradeBand,
				GameSessionID: mm.GameSessionID, Node: mm.Node}
			targets, eid, seats = others(r.in, o.UserID), mm.EID, r.in.Size()
		}
		r.mu.Unlock()
		if !ok {
			return nil, hub.ErrNotMember
		}
		_ = r.h.reg.Seat(ctx, r.meta, o.UserID, rec, seats)
		r.h.publish(ctx, r.meta.RoomID, "hub.player.appearance_changed", map[string]any{"room_id": r.meta.RoomID, "eid": eid,
			"appearance": o.Player.Appearance()}, targets)
		return nil, nil
	}
	return nil, fmt.Errorf("unknown op %s", kind)
}

// join seats a student (F091-AC-01, AC-02, AC-04, AC-09): hub.player.joined to the others, players[] to the joiner.
func (r *room) join(ctx context.Context, req port.JoinRequest, now time.Time) (*port.JoinResult, error) {
	user := req.Seat.Player.UserID
	r.mu.Lock()
	m, rejoined, err := r.in.Join(req.Seat, r.in.Area.Spawn(req.Spawn), now)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	res := &port.JoinResult{SelfEID: m.EID, Spawn: m.Mover.Pos, Rejoined: rejoined, Players: []port.PlayerState{}}
	for _, o := range r.in.Members() {
		if o.Player.UserID != user {
			res.Players = append(res.Players, port.PlayerState{EID: o.EID, Player: o.Player, State: o.Entity()})
		}
	}
	rec := cache.MemberRecord{EID: m.EID, Player: m.Player, SchoolID: m.SchoolID, GradeBand: m.GradeBand, GameSessionID: m.GameSessionID, Node: m.Node}
	targets, seats, state := others(r.in, user), r.in.Size(), m.Entity()
	r.mu.Unlock()
	if err := r.h.reg.Seat(ctx, r.meta, user, rec, seats); err != nil {
		slog.WarnContext(ctx, "roomhost: seat not registered", "error", err)
	}
	if !rejoined {
		r.h.metrics.Players(hub.RoomTypeHub, 1)
		r.h.publish(ctx, r.meta.RoomID, "hub.player.joined", map[string]any{"room_id": r.meta.RoomID, "eid": m.EID, "player": req.Seat.Player,
			"state": state}, targets)
	}
	return res, nil
}

// call runs an operation on the owner of a room: here when hosted locally, over NATS otherwise; a room without owner
// is taken over first (F091-AC-20).
func (h *Host) call(ctx context.Context, roomID, kind string, o op) (json.RawMessage, error) {
	if r := h.get(roomID); r != nil {
		res, err := r.apply(ctx, kind, o)
		if err != nil || res == nil {
			return nil, err
		}
		return json.Marshal(res)
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, h.cfg.OpTimeout)
	defer cancel()
	msg, err := h.nc.RequestWithContext(rctx, opSubject(roomID, kind), b)
	if errors.Is(err, nats.ErrNoResponders) {
		if h.cfg.ClientOnly {
			return nil, command.ErrRoomGone
		}
		r, terr := h.takeover(ctx, roomID)
		if terr != nil {
			return nil, terr
		}
		if r == nil {
			return nil, command.ErrRoomGone // another instance owns it (or took it over meanwhile): retry
		}
		res, err := r.apply(ctx, kind, o)
		if err != nil || res == nil {
			return nil, err
		}
		return json.Marshal(res)
	}
	if err != nil {
		return nil, fmt.Errorf("roomhost: %s on owner: %w", kind, err)
	}
	var rep opReply
	if err := json.Unmarshal(msg.Data, &rep); err != nil {
		return nil, err
	}
	if rep.Code != "" {
		if e, ok := opErrors[rep.Code]; ok {
			return nil, e
		}
		return nil, fmt.Errorf("roomhost: %s failed on owner", kind)
	}
	return rep.Result, nil
}

// Join implements port.Rooms.
func (h *Host) Join(ctx context.Context, roomID string, req port.JoinRequest) (*port.JoinResult, error) {
	b, err := h.call(ctx, roomID, "join", op{TenantID: req.Seat.TenantID, UserID: req.Seat.Player.UserID, Join: &req})
	if err != nil {
		return nil, err
	}
	var res port.JoinResult
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Leave implements port.Rooms.
func (h *Host) Leave(ctx context.Context, roomID string, user uuid.UUID, reason string) error {
	_, err := h.call(ctx, roomID, "leave", op{UserID: user, Reason: reason})
	return err
}

// Emote implements port.Rooms.
func (h *Host) Emote(ctx context.Context, roomID string, tenant, user uuid.UUID, emote string) (*port.EmoteResult, error) {
	b, err := h.call(ctx, roomID, "emote", op{TenantID: tenant, UserID: user, Emote: emote})
	if err != nil {
		return nil, err
	}
	var res port.EmoteResult
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Appearance implements port.Rooms.
func (h *Host) Appearance(ctx context.Context, roomID string, user uuid.UUID, p hub.PublicPlayer) error {
	_, err := h.call(ctx, roomID, "appearance", op{UserID: user, Player: &p})
	return err
}

// Hosted returns the ids of the rooms hosted here (tests, diagnostics).
func (h *Host) Hosted() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.rooms))
	for id := range h.rooms {
		out = append(out, id)
	}
	return out
}
