// Package battlehost is the room host of the match rooms of module competition (F105, REALTIME_ARCHITECTURE §5.1, §7):
// the core-api replica holding the lease of a match room keeps the room (domain battle.Room) in memory, ticks it,
// writes its checkpoint to Valkey after every state transition and publishes its messages; commands of the realtime
// gateway reach the owner through ew.int.competition.battle.<room>.<op>. A replica that finds a room without owner takes
// it over from its checkpoint (the question in progress is played again); without a checkpoint the match aborts.
package battlehost

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
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/battle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/roomhost"
)

// Config of the host (CONFIG_DEFAULTS through internal/infrastructure/config).
type Config struct {
	InstanceID      string
	TickHz          int           // ticks per second of a match room (10)
	LeaseTTL        time.Duration // 10 s
	LeaseEvery      time.Duration // 3 s
	WatchEvery      time.Duration // failover watcher period
	CheckpointEvery time.Duration // EW_BATTLE_ROOM_CHECKPOINT: refresh of the checkpoint besides every transition
	CheckpointTTL   time.Duration // 1 h
	OpTimeout       time.Duration // NATS request to the owner
	// ClientOnly: this process never hosts a room (worker): it asks a core-api replica to open the room.
	ClientOnly bool
}

// App is the application of the battle engine (command.Battles).
type App interface {
	Open(ctx context.Context, matchID, tenant uuid.UUID) (command.BattleInfo, *battle.Room, error)
	Started(ctx context.Context, info command.BattleInfo, room *battle.Room, at time.Time) error
	Score(ctx context.Context, info command.BattleInfo, plan uuid.UUID, p battle.Pending) (bool, error)
	Reveal(ctx context.Context, info command.BattleInfo, plan uuid.UUID, index int, closedAt time.Time) (battle.Reveal, error)
	Finish(ctx context.Context, info command.BattleInfo, room *battle.Room) error
	Encouragements(ctx context.Context, info command.BattleInfo, room *battle.Room) (map[uuid.UUID]battle.Encouragement, error)
	Abort(ctx context.Context, info command.BattleInfo, room *battle.Room, reason string) error
	AbortUnhosted(ctx context.Context, matchID, tenant uuid.UUID) (command.BattleInfo, error)
	Memberships(ctx context.Context, info command.BattleInfo, room *battle.Room, ms []battle.Membership, at time.Time) error
	Signals(ctx context.Context, info command.BattleInfo, room *battle.Room, ss []battle.Signal, at time.Time) error
}

// Errors of the room operations (mapped to the realtime ErrorCode by natscmd).
var (
	ErrNoRoom      = errors.New("battlehost: no live room for this match")          // MATCH_NOT_ACTIVE
	ErrWrongTenant = errors.New("battlehost: the room belongs to another tenant")   // PERMISSION_DENIED
	ErrNoActions   = errors.New("battlehost: a co-op challenge takes answers only") // VALIDATION_FAILED
)

// checkpoint is what the host writes in Valkey: the match info and the room.
type checkpoint struct {
	Info command.BattleInfo `json:"info"`
	Room json.RawMessage    `json:"room"`
}

// Host hosts the match rooms of this replica.
type Host struct {
	cfg   Config
	nc    *nats.Conn
	store cache.BattleStore
	out   port.RoomOutput
	app   App
	now   func() time.Time

	mu      sync.Mutex
	base    context.Context
	rooms   map[uuid.UUID]*hosted
	crashed bool
}

// New builds a host; Run must be started for the open requests, the presence signals and the failover watcher.
func New(cfg Config, nc *nats.Conn, store cache.BattleStore, out port.RoomOutput, app App, now func() time.Time) *Host {
	if now == nil {
		now = time.Now
	}
	if cfg.TickHz <= 0 {
		cfg.TickHz = 10
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10 * time.Second
	}
	if cfg.LeaseEvery <= 0 {
		cfg.LeaseEvery = 3 * time.Second
	}
	if cfg.OpTimeout <= 0 {
		cfg.OpTimeout = 3 * time.Second
	}
	if cfg.CheckpointTTL <= 0 {
		cfg.CheckpointTTL = time.Hour
	}
	if cfg.CheckpointEvery <= 0 {
		cfg.CheckpointEvery = 5 * time.Second
	}
	return &Host{cfg: cfg, nc: nc, store: store, out: out, app: app, now: now, base: context.Background(), rooms: map[uuid.UUID]*hosted{}}
}

const (
	openSubject = "ew.int.competition.battle.open"
	queueGroup  = "competition"
)

// opSubject is the internal subject of an operation on a room (outside ew.cmd.> and ew.in.>: the gateway never
// reaches it).
func opSubject(room uuid.UUID, op string) string {
	return "ew.int.competition.battle." + room.String() + "." + op
}

// Run serves the open requests, follows the presence signals and takes over orphan rooms until ctx ends, then hands
// the hosted rooms over (their checkpoints stay for the next owner).
func (h *Host) Run(ctx context.Context) error {
	h.mu.Lock()
	h.base = ctx
	h.mu.Unlock()
	if h.nc == nil {
		return errors.New("battlehost: no NATS connection")
	}
	open, err := h.nc.QueueSubscribe(openSubject, queueGroup, h.onOpen)
	if err != nil {
		return fmt.Errorf("battlehost: open: %w", err)
	}
	defer func() { _ = open.Unsubscribe() }()
	presence, err := h.nc.Subscribe(rt.PresenceSubjectAll, h.onPresence)
	if err != nil {
		return fmt.Errorf("battlehost: presence: %w", err)
	}
	defer func() { _ = presence.Unsubscribe() }()
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
			h.Watch(ctx)
		}
	}
}

func (h *Host) ctx() context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.base
}

func (h *Host) get(id uuid.UUID) *hosted {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rooms[id]
}

func (h *Host) local() []*hosted {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*hosted, 0, len(h.rooms))
	for _, r := range h.rooms {
		out = append(out, r)
	}
	return out
}

// Hosted returns the match ids hosted here (tests, diagnostics).
func (h *Host) Hosted() []uuid.UUID {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]uuid.UUID, 0, len(h.rooms))
	for id := range h.rooms {
		out = append(out, id)
	}
	return out
}

type openReq struct {
	MatchID uuid.UUID `json:"match_id"`
	Tenant  uuid.UUID `json:"tenant_id"`
}

type opReply struct {
	Code   string          `json:"code,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

func (h *Host) onOpen(m *nats.Msg) {
	var req openReq
	reply := opReply{}
	if err := json.Unmarshal(m.Data, &req); err != nil {
		reply.Code = "invalid"
	} else if err := h.open(h.ctx(), req.MatchID, req.Tenant); err != nil {
		slog.Warn("battlehost: room not opened", "error", err)
		reply.Code = "error"
	}
	b, _ := json.Marshal(reply)
	_ = m.Respond(b)
}

// Open opens the room of a match (match.created): here when this replica hosts rooms, otherwise on a core-api replica
// through NATS. An error means the open must be retried (the consumer redelivers).
func (h *Host) Open(ctx context.Context, matchID, tenant uuid.UUID) error {
	if !h.cfg.ClientOnly {
		return h.open(ctx, matchID, tenant)
	}
	b, _ := json.Marshal(openReq{MatchID: matchID, Tenant: tenant})
	rctx, cancel := context.WithTimeout(ctx, 3*h.cfg.OpTimeout)
	defer cancel()
	msg, err := h.nc.RequestWithContext(rctx, openSubject, b)
	if err != nil {
		return fmt.Errorf("battlehost: open on core-api: %w", err)
	}
	var rep opReply
	if err := json.Unmarshal(msg.Data, &rep); err != nil {
		return err
	}
	if rep.Code != "" {
		return fmt.Errorf("battlehost: open failed on core-api (%s)", rep.Code)
	}
	return nil
}

func (h *Host) open(ctx context.Context, matchID, tenant uuid.UUID) error {
	if h.dead() {
		return errors.New("battlehost: host stopped")
	}
	if h.get(matchID) != nil {
		return nil
	}
	id := matchID.String()
	ok, err := h.store.Acquire(ctx, id, h.cfg.InstanceID, h.cfg.LeaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		return nil // another replica opens or hosts it
	}
	if err := h.store.Register(ctx, id, tenant, "battle", h.cfg.InstanceID, h.now()); err != nil {
		_ = h.store.Release(ctx, id, h.cfg.InstanceID)
		return err
	}
	info, room, err := h.app.Open(ctx, matchID, tenant)
	switch {
	case errors.Is(err, command.ErrMatchAborted), errors.Is(err, command.ErrMatchOver), errors.Is(err, command.ErrMatchUnknown):
		_ = h.store.Remove(ctx, id)
		if errors.Is(err, command.ErrMatchAborted) {
			h.publishAbort(ctx, matchID, battle.EndPlanFailure)
		}
		return nil // nothing to host (aborted, over, or not a match of this tenant): acknowledged
	case errors.Is(err, command.ErrMatchRunning):
		_ = h.store.Release(ctx, id, h.cfg.InstanceID)
		return nil // the watcher restores it from its checkpoint, or aborts it
	case err != nil:
		_ = h.store.Release(ctx, id, h.cfg.InstanceID)
		return err
	}
	r := h.start(info, room)
	r.checkpoint(ctx)
	return nil
}

// publishAbort tells the players of a match that could not be played (battle.match.ended aborted).
func (h *Host) publishAbort(ctx context.Context, matchID uuid.UUID, reason string) {
	if err := h.out.Room(ctx, matchID, "battle.match.ended", map[string]any{"match_id": matchID.String(), "status": "aborted",
		"results": []map[string]any{}, "rated": false, "end_reason": reason, "review_available": false}, nil); err != nil {
		slog.WarnContext(ctx, "battlehost: abort not published", "error", err)
	}
}

// Watch takes over the battle rooms whose lease was not renewed (owner dead, F105-AC-15).
func (h *Host) Watch(ctx context.Context) {
	if h.cfg.ClientOnly || h.dead() {
		return
	}
	orphans, err := h.store.Orphans(ctx, h.now().Add(-h.cfg.LeaseTTL), 20)
	if err != nil {
		slog.WarnContext(ctx, "battlehost: orphan scan failed", "error", err)
		return
	}
	for _, s := range orphans {
		id, err := uuid.Parse(s)
		if err != nil || h.get(id) != nil {
			continue
		}
		if _, err := h.takeover(ctx, id); err != nil {
			slog.WarnContext(ctx, "battlehost: takeover failed", "error", err)
		}
	}
}

// takeover acquires the lease of a room without owner and restores it from its checkpoint (the question in progress
// is played again); without a checkpoint the match aborts without result (server_failure).
func (h *Host) takeover(ctx context.Context, id uuid.UUID) (*hosted, error) {
	s := id.String()
	ok, err := h.store.Acquire(ctx, s, h.cfg.InstanceID, h.cfg.LeaseTTL)
	if err != nil || !ok {
		return nil, err
	}
	raw, err := h.store.Checkpoint(ctx, s)
	if err != nil {
		_ = h.store.Release(ctx, s, h.cfg.InstanceID)
		return nil, err
	}
	var cp checkpoint
	var room *battle.Room
	if raw != nil {
		if err := json.Unmarshal(raw, &cp); err == nil {
			room, err = battle.Restore(cp.Room)
			if err != nil {
				room = nil
			}
		}
	}
	if room == nil || room.Done() {
		tenant, err := h.store.Tenant(ctx, s)
		if err != nil {
			_ = h.store.Release(ctx, s, h.cfg.InstanceID)
			return nil, err
		}
		if tenant != nil && room == nil {
			if _, err := h.app.AbortUnhosted(ctx, id, *tenant); err != nil {
				_ = h.store.Release(ctx, s, h.cfg.InstanceID)
				return nil, err
			}
			h.publishAbort(ctx, id, battle.EndServerFailure)
		}
		return nil, h.store.Remove(ctx, s)
	}
	r := h.start(cp.Info, room)
	r.mu.Lock()
	step := room.Resume(h.now())
	r.mu.Unlock()
	r.exec(ctx, step)
	slog.InfoContext(ctx, "battlehost: room taken over")
	return r, nil
}

// dead reports a host stopped by Crash (it hosts nothing any more).
func (h *Host) dead() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.crashed
}

// Crash stops hosting without giving the leases back, like a killed process (failover tests, F105-AC-15).
func (h *Host) Crash() {
	h.mu.Lock()
	h.crashed = true
	h.mu.Unlock()
	for _, r := range h.local() {
		r.stop(false)
	}
}

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
		r.checkpoint(ctx)
		r.stop(false)
		_ = h.store.Release(ctx, r.info.MatchID.String(), h.cfg.InstanceID)
	}
}

func (h *Host) onPresence(m *nats.Msg) {
	var p rt.PresenceChanged
	if json.Unmarshal(m.Data, &p) != nil || !p.Gone() {
		return
	}
	for _, r := range h.local() {
		r.mu.Lock()
		member := r.room.Member(p.UserID) && r.info.Tenant == p.TenantID
		var step battle.Step
		if member {
			step = r.room.Disconnect(p.UserID, h.now())
		}
		r.mu.Unlock()
		if member {
			r.exec(h.ctx(), step)
		}
	}
}

// AnswerIn is an answer of the realtime gateway (battle.answer.submit, coop.answer.submit).
type AnswerIn struct {
	Instance string          `json:"question_instance_id"`
	Response json.RawMessage `json:"response"`
}

type op struct {
	Tenant  uuid.UUID `json:"tenant_id"`
	User    uuid.UUID `json:"user_id"`
	Session uuid.UUID `json:"game_session_id"`
	Answer  *AnswerIn `json:"answer,omitempty"`
}

var opErrors = map[string]error{"not_member": battle.ErrNotMember, "not_active": battle.ErrNotActive, "not_open": battle.ErrNotOpen,
	"late": battle.ErrLate, "already": battle.ErrAlreadySubmitted, "out": battle.ErrOut, "no_room": ErrNoRoom,
	"wrong_tenant": ErrWrongTenant, "no_actions": ErrNoActions, "invalid": battle.ErrInvalidResponse}

func codeOf(err error) string {
	for code, e := range opErrors {
		if errors.Is(err, e) {
			return code
		}
	}
	return "error"
}

// Answer submits an answer to the room of match (F105-AC-03) on its owner.
func (h *Host) Answer(ctx context.Context, match uuid.UUID, c rt.Caller, in AnswerIn) error {
	_, err := h.call(ctx, match, "answer", op{Tenant: c.TenantID, User: c.UserID, Session: c.GameSessionID, Answer: &in})
	return err
}

// State (re)joins the room of match and returns the snapshot of the caller (F105-AC-11, AC-14, AC-22).
func (h *Host) State(ctx context.Context, match uuid.UUID, c rt.Caller) (json.RawMessage, error) {
	return h.call(ctx, match, "state", op{Tenant: c.TenantID, User: c.UserID, Session: c.GameSessionID})
}

// Action is coop.action.perform: a co-op challenge has no puzzle actions (its tasks are questions).
func (h *Host) Action(ctx context.Context, match uuid.UUID, c rt.Caller) error {
	_, err := h.call(ctx, match, "action", op{Tenant: c.TenantID, User: c.UserID, Session: c.GameSessionID})
	return err
}

func (h *Host) call(ctx context.Context, match uuid.UUID, kind string, o op) (json.RawMessage, error) {
	if r := h.get(match); r != nil {
		return r.apply(ctx, kind, o)
	}
	if h.nc == nil {
		return nil, ErrNoRoom
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, h.cfg.OpTimeout)
	defer cancel()
	msg, err := h.nc.RequestWithContext(rctx, opSubject(match, kind), b)
	if errors.Is(err, nats.ErrNoResponders) {
		if h.cfg.ClientOnly || h.dead() {
			return nil, ErrNoRoom
		}
		r, terr := h.takeover(ctx, match)
		if terr != nil || r == nil {
			return nil, ErrNoRoom
		}
		return r.apply(ctx, kind, o)
	}
	if err != nil {
		return nil, fmt.Errorf("battlehost: %s on owner: %w", kind, err)
	}
	var rep opReply
	if err := json.Unmarshal(msg.Data, &rep); err != nil {
		return nil, err
	}
	if rep.Code != "" {
		if e, ok := opErrors[rep.Code]; ok {
			return nil, e
		}
		return nil, fmt.Errorf("battlehost: %s failed on owner", kind)
	}
	return rep.Result, nil
}

// hosted is one match room hosted here.
type hosted struct {
	h        *Host
	info     command.BattleInfo
	mu       sync.Mutex
	room     *battle.Room
	cancel   context.CancelFunc
	subs     []*nats.Subscription
	once     sync.Once
	busy     sync.Mutex // one finish/abort at a time
	finished bool
	lastCkpt time.Time

	// effects run the persistence of the steps in order (a membership is joined, then disconnected, then left), off the
	// tick goroutine.
	qmu     sync.Mutex
	queue   []func()
	running bool
}

// enqueue runs fn after the effects already queued, on one background goroutine per room.
func (r *hosted) enqueue(fn func()) {
	r.qmu.Lock()
	r.queue = append(r.queue, fn)
	if r.running {
		r.qmu.Unlock()
		return
	}
	r.running = true
	r.qmu.Unlock()
	go func() {
		for {
			r.qmu.Lock()
			if len(r.queue) == 0 {
				r.running = false
				r.qmu.Unlock()
				return
			}
			next := r.queue[0]
			r.queue = r.queue[1:]
			r.qmu.Unlock()
			next()
		}
	}()
}

func (h *Host) start(info command.BattleInfo, room *battle.Room) *hosted {
	h.mu.Lock()
	if r, ok := h.rooms[info.MatchID]; ok {
		h.mu.Unlock()
		return r
	}
	base := h.base
	h.mu.Unlock()
	rctx, cancel := context.WithCancel(context.WithoutCancel(base))
	r := &hosted{h: h, info: info, room: room, cancel: cancel}
	if h.nc != nil {
		if sub, err := h.nc.Subscribe(opSubject(info.MatchID, "*"), r.onOp); err == nil {
			r.subs = append(r.subs, sub)
			_ = h.nc.Flush()
		} else {
			slog.Warn("battlehost: op subscription failed", "error", err)
		}
	}
	h.mu.Lock()
	h.rooms[info.MatchID] = r
	h.mu.Unlock()
	go roomhost.Tick(rctx, h.cfg.TickHz, h.now, r.tick, nil)
	go roomhost.KeepLease(rctx, h.cfg.LeaseEvery, h.cfg.LeaseTTL, func(ctx context.Context) (bool, error) {
		return h.store.Renew(ctx, info.MatchID.String(), h.cfg.InstanceID, h.cfg.LeaseTTL, h.now())
	}, func() {
		slog.Warn("battlehost: lease lost, room stopped here")
		r.stop(false)
	})
	return r
}

func (r *hosted) stop(closed bool) {
	r.once.Do(func() {
		r.cancel()
		for _, s := range r.subs {
			_ = s.Unsubscribe()
		}
		r.h.mu.Lock()
		delete(r.h.rooms, r.info.MatchID)
		r.h.mu.Unlock()
		if closed {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := r.h.store.Remove(ctx, r.info.MatchID.String()); err != nil {
				slog.Warn("battlehost: registry remove failed", "error", err)
			}
		}
	})
}

// snapshot is a copy of the room for the use cases (never the live room outside its lock).
func (r *hosted) snapshot() *battle.Room {
	r.mu.Lock()
	b, err := json.Marshal(r.room)
	r.mu.Unlock()
	if err != nil {
		return nil
	}
	c, err := battle.Restore(b)
	if err != nil {
		return nil
	}
	return c
}

func (r *hosted) checkpoint(ctx context.Context) {
	r.mu.Lock()
	room, err := json.Marshal(r.room)
	r.mu.Unlock()
	if err != nil {
		return
	}
	b, err := json.Marshal(checkpoint{Info: r.info, Room: room})
	if err != nil {
		return
	}
	if err := r.h.store.SaveCheckpoint(ctx, r.info.MatchID.String(), b, r.h.cfg.CheckpointTTL); err != nil {
		slog.WarnContext(ctx, "battlehost: checkpoint not written", "error", err)
		return
	}
	r.lastCkpt = r.h.now()
}

func (r *hosted) tick(now time.Time) {
	r.mu.Lock()
	step := r.room.Tick(now)
	r.mu.Unlock()
	if !step.Checkpoint && now.Sub(r.lastCkpt) >= r.h.cfg.CheckpointEvery {
		step.Checkpoint = true
	}
	r.exec(r.h.ctx(), step)
}

// exec runs the effects of a step: messages and checkpoint at once, persistence and module learning in the
// background (the tick never waits for the database).
func (r *hosted) exec(ctx context.Context, step battle.Step) {
	for _, m := range step.Messages {
		var err error
		if len(m.To) == 1 {
			err = r.h.out.User(ctx, m.To[0], m.Type, m.Payload)
		} else {
			err = r.h.out.Room(ctx, r.info.MatchID, m.Type, m.Payload, m.To)
		}
		if err != nil {
			slog.WarnContext(ctx, "battlehost: publish failed", "type", m.Type, "error", err)
		}
	}
	if step.Checkpoint {
		r.checkpoint(ctx)
	}
	now := r.h.now()
	if len(step.Memberships) > 0 || len(step.Signals) > 0 || step.Started {
		snap := r.snapshot()
		r.enqueue(func() {
			bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if err := r.h.app.Memberships(bctx, r.info, snap, step.Memberships, now); err != nil {
				slog.Warn("battlehost: memberships not recorded", "error", err)
			}
			if err := r.h.app.Signals(bctx, r.info, snap, step.Signals, now); err != nil {
				slog.Warn("battlehost: integrity signals not recorded", "error", err)
			}
			if step.Started {
				if err := r.h.app.Started(bctx, r.info, snap, now); err != nil {
					slog.Warn("battlehost: match start not recorded", "error", err)
				}
			}
		})
	}
	if step.Reveal != nil {
		idx := *step.Reveal
		go r.reveal(ctx, idx)
	}
	if step.Finish {
		r.enqueue(func() { r.finish(ctx) }) // after the memberships already queued
	}
	if step.Abort != "" {
		reason := step.Abort
		r.enqueue(func() { r.abort(ctx, reason) })
	}
}

func (r *hosted) reveal(ctx context.Context, idx int) {
	r.mu.Lock()
	plan, closed := r.room.PlanID, r.room.Closed
	r.mu.Unlock()
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	rv, err := r.h.app.Reveal(rctx, r.info, plan, idx, closed)
	if err != nil {
		slog.Warn("battlehost: reveal unavailable, revealing without the answer", "error", err)
	}
	rv.Question = nil // the review is served by getMatch after the match
	r.mu.Lock()
	step := r.room.Revealed(idx, rv, r.h.now())
	r.mu.Unlock()
	r.exec(ctx, step)
}

// finish records the result once (F105-AC-07) and only then sends battle.match.ended.
func (r *hosted) finish(ctx context.Context) {
	if !r.busy.TryLock() {
		return
	}
	defer r.busy.Unlock()
	if r.finished {
		return
	}
	snap := r.snapshot()
	if snap == nil {
		return
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := r.h.app.Finish(fctx, r.info, snap)
	var pe *problem.Error
	if err != nil && (!errors.As(err, &pe) || pe.Code != problem.MatchResultAlreadyRecorded) {
		slog.Warn("battlehost: result not recorded, retrying", "error", err)
		return // the room asks again (finalizing retry)
	}
	r.finished = true
	enc, err := r.h.app.Encouragements(fctx, r.info, snap)
	if err != nil { // best effort: the match ends without the encouragement
		slog.Warn("battlehost: encouragements unavailable", "error", err)
		enc = nil
	}
	r.mu.Lock()
	step := r.room.EndedWith(r.h.now(), enc)
	r.mu.Unlock()
	r.exec(ctx, step)
	r.stop(true)
}

func (r *hosted) abort(ctx context.Context, reason string) {
	if !r.busy.TryLock() {
		return
	}
	defer r.busy.Unlock()
	snap := r.snapshot()
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if snap != nil {
		if err := r.h.app.Abort(actx, r.info, snap, reason); err != nil {
			slog.Warn("battlehost: abort not recorded", "error", err)
			return // the checkpoint stays: the next owner aborts again
		}
	}
	r.finished = true
	r.stop(true)
}

func (r *hosted) onOp(m *nats.Msg) {
	kind := m.Subject[len(opSubject(r.info.MatchID, "")):]
	reply := opReply{}
	var o op
	res, err := func() (json.RawMessage, error) {
		if err := json.Unmarshal(m.Data, &o); err != nil {
			return nil, err
		}
		return r.apply(r.h.ctx(), kind, o)
	}()
	if err != nil {
		reply.Code = codeOf(err)
	} else {
		reply.Result = res
	}
	b, _ := json.Marshal(reply)
	_ = m.Respond(b)
}

// apply runs an operation of the gateway on the room: the tenant of the ticket must be the tenant of the room
// (PERMISSION_DENIED), then the caller must be a player (ROOM_NOT_MEMBER).
func (r *hosted) apply(ctx context.Context, kind string, o op) (json.RawMessage, error) {
	if o.Tenant != r.info.Tenant {
		return nil, ErrWrongTenant
	}
	now := r.h.now()
	switch kind {
	case "state":
		r.mu.Lock()
		step, err := r.room.Join(o.User, o.Session, now)
		var snap battle.Message
		if err == nil {
			snap = r.room.Snapshot(o.User)
		}
		r.mu.Unlock()
		if err != nil {
			return nil, err
		}
		r.exec(ctx, step)
		return json.Marshal(map[string]any{"type": snap.Type, "payload": snap.Payload})
	case "answer":
		if o.Answer == nil {
			return nil, battle.ErrInvalidResponse
		}
		r.mu.Lock()
		step, pend, err := r.room.Submit(o.User, o.Answer.Instance, o.Answer.Response, now)
		plan := r.room.PlanID
		r.mu.Unlock()
		r.exec(ctx, step) // signals of refused answers too
		if err != nil {
			return nil, err
		}
		go r.score(ctx, plan, pend)
		return nil, nil
	case "action":
		r.mu.Lock()
		member := r.room.Member(o.User)
		r.mu.Unlock()
		if !member {
			return nil, battle.ErrNotMember
		}
		return nil, ErrNoActions
	}
	return nil, fmt.Errorf("battlehost: unknown op %s", kind)
}

// score grades an accepted answer with module learning (outside the room lock and any transaction): a refusal (the
// response does not fit the question) is a wrong answer, an unavailable module replays the question.
func (r *hosted) score(ctx context.Context, plan uuid.UUID, p battle.Pending) {
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	correct, err := r.h.app.Score(sctx, r.info, plan, p)
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case err == nil:
		r.room.Scored(p.Index, p.Student, correct)
	case errors.Is(err, port.ErrPlanRefused):
		r.room.Scored(p.Index, p.Student, false)
	default:
		slog.Warn("battlehost: answer not graded", "question_index", p.Index, "error", err)
		r.room.ScoreFailed(p.Index, p.Student)
	}
}
