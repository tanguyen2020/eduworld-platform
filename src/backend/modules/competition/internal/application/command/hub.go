// Package command holds the write use cases of module competition (ADR-0087): each loads what it needs through the
// ports, applies the domain and persists in one UnitOfWork transaction with the tenant scope of the caller.
package command

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/hub"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Error is a refusal of a hub command with its realtime error code (AsyncAPI ErrorCode). It never says where another
// student is (F091-AC-05).
type Error struct {
	Code       string
	Retryable  bool
	RetryAfter time.Duration
	Detail     string
}

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

func fail(code, detail string) *Error { return &Error{Code: code, Detail: detail} }

// Realtime error codes used by the hub (AsyncAPI ErrorCode).
const (
	CodeValidation    = "VALIDATION_FAILED"
	CodeNotFound      = "NOT_FOUND"
	CodeDenied        = "PERMISSION_DENIED"
	CodeConflict      = "CONFLICT"
	CodeRoomFull      = "ROOM_FULL"
	CodeRoomNotMember = "ROOM_NOT_MEMBER"
	CodeRateLimited   = "RATE_LIMITED"
	CodeUnavailable   = "UNAVAILABLE"
)

// ErrRoomGone means the room has no live owner and could not be taken over (closed meanwhile).
var ErrRoomGone = errors.New("hub: room gone")

// HubConfig holds the hub parameters (CONFIG_DEFAULTS: hub_instance_capacity, hub_sync_rate, disconnect_grace hub,
// hub_instance_idle_close, emote_rate).
type HubConfig struct {
	Capacity  int
	TickHz    int
	Grace     time.Duration
	IdleClose time.Duration
	EmoteRate time.Duration
}

// Hub runs the hub instance use cases (F091 US1–US2).
type Hub struct {
	UoW       port.UnitOfWork
	Directory port.Directory
	Store     port.RoomStore
	Registry  port.Registry
	Rooms     port.Rooms
	Presence  port.Presence
	Audit     port.Auditor
	Metrics   port.HubMetrics
	Config    HubConfig
	Now       func() time.Time
	NewID     func() uuid.UUID
}

func (h Hub) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func (h Hub) newID() uuid.UUID {
	if h.NewID != nil {
		return h.NewID()
	}
	return uuid.Must(uuid.NewV7())
}

// Caller is the identity of a realtime command, taken from the Ew-* headers set by the gateway (F091-AC-08).
type Caller struct {
	TenantID      uuid.UUID
	UserID        uuid.UUID
	GameSessionID uuid.UUID
	NodeID        string
}

// JoinInput is hub.instance.join.
type JoinInput struct {
	AreaID     string
	SpawnPoint string
	Follow     *uuid.UUID
}

// Assigned is the result of a join: the content of hub.instance.assigned and the room output to subscribe.
type Assigned struct {
	RoomID     string             `json:"room_id"`
	AreaID     string             `json:"area_id"`
	SelfEID    int                `json:"self_eid"`
	Spawn      hub.Vec3           `json:"spawn"`
	TickRateHz int                `json:"tick_rate_hz"`
	MaxSpeed   float64            `json:"max_speed_mps"`
	Players    []port.PlayerState `json:"players"`
	Previous   string             `json:"-"` // room left by this join (unsubscribe)
}

var idRe = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// context of one join after the reads.
type joinCtx struct {
	c         Caller
	in        JoinInput
	area      hub.Area
	partition hub.Partition
	player    hub.PublicPlayer
	friends   map[uuid.UUID]bool
	blocks    map[uuid.UUID]bool
	audit     bool
}

// JoinHubInstance assigns a hub instance (F091-AC-01…AC-06, AC-09, AC-11, AC-23).
func (h Hub) JoinHubInstance(ctx context.Context, c Caller, in JoinInput) (a *Assigned, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.JoinHubInstance")
	defer func() { done(err) }()
	start := time.Now()
	if !idRe.MatchString(in.AreaID) || len(in.SpawnPoint) > 64 || (in.SpawnPoint != "" && !idRe.MatchString(in.SpawnPoint)) {
		return nil, fail(CodeValidation, "area_id/spawn_point")
	}
	if in.Follow != nil && (*in.Follow == uuid.Nil || *in.Follow == c.UserID) {
		return nil, fail(CodeValidation, "follow_user_id")
	}
	j, err := h.load(ctx, c, in)
	if err != nil {
		return nil, err
	}
	seat := hub.Seat{Player: j.player, TenantID: c.TenantID, SchoolID: j.partition.SchoolID, GradeBand: j.partition.GradeBand,
		GameSessionID: c.GameSessionID, Node: c.NodeID}
	current, err := h.Registry.UserRoom(ctx, c.UserID)
	if err != nil {
		return nil, fmt.Errorf("hub: user room: %w", err)
	}
	if current != nil && current.AreaID == in.AreaID && in.Follow == nil {
		// Back to the held seat of the same area (F091-AC-09), even when the instance is full.
		if res, err := h.Rooms.Join(ctx, current.RoomID, port.JoinRequest{Seat: seat, Spawn: in.SpawnPoint}); err == nil {
			return h.assigned(ctx, j, current.RoomID, res, "", start)
		}
	}
	// The new seat is taken first; the previous one is left only once the student has somewhere to go, so a refused
	// join (follow denied, no seat) keeps the student where they were.
	var room string
	var res *port.JoinResult
	if in.Follow != nil {
		room, res, err = h.follow(ctx, j, seat, *in.Follow)
		if h.Metrics != nil {
			h.Metrics.JoinFriend(followResult(err))
		}
		if err != nil {
			return nil, err
		}
	} else {
		for attempt := 0; attempt < 3 && room == ""; attempt++ {
			room, res, err = h.place(ctx, j, seat)
			if errors.Is(err, hub.ErrFull) || errors.Is(err, ErrRoomGone) {
				room = "" // another join took the last seat (or the room closed): choose again
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		if room == "" {
			return nil, &Error{Code: CodeUnavailable, Retryable: true, RetryAfter: time.Second, Detail: "no seat"}
		}
	}
	previous := ""
	if current != nil && current.RoomID != room {
		if err := h.Rooms.Leave(ctx, current.RoomID, c.UserID, "left"); err != nil && !errors.Is(err, hub.ErrNotMember) && !errors.Is(err, ErrRoomGone) {
			slog.WarnContext(ctx, "hub: previous seat not released", "error", err) // its grace or idle close frees it
		}
		previous = current.RoomID
	}
	return h.assigned(ctx, j, room, res, previous, start)
}

func followResult(err error) string {
	var e *Error
	switch {
	case err == nil:
		return "joined"
	case errors.As(err, &e) && e.Code == CodeRoomFull:
		return "full"
	case errors.As(err, &e) && e.Code == CodeDenied:
		return "denied"
	}
	return "error"
}

// load reads the projections in one tenant transaction: student partition, area rules, unlock, public profile,
// friends and blocks (F091-AC-06).
func (h Hub) load(ctx context.Context, c Caller, in JoinInput) (*joinCtx, error) {
	sc, err := authz.EventScope(c.TenantID)
	if err != nil {
		return nil, fail("UNAUTHENTICATED", "tenant")
	}
	j := &joinCtx{c: c, in: in}
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		st, err := h.Directory.Student(ctx, c.TenantID, c.UserID)
		if err != nil {
			return err
		}
		if st == nil {
			return &Error{Code: CodeUnavailable, Retryable: true, RetryAfter: 2 * time.Second, Detail: "student not projected yet"}
		}
		band, ok := hub.BandOf(st.GradeLevel)
		if !st.Active || !ok {
			return fail(CodeDenied, "student")
		}
		j.partition = hub.Partition{TenantID: c.TenantID, SchoolID: st.SchoolID, GradeBand: band}
		area, err := h.Directory.Area(ctx, in.AreaID)
		if err != nil {
			return err
		}
		if area == nil || !area.Active {
			return fail(CodeNotFound, "area")
		}
		if !area.Admits(band) {
			return fail(CodeDenied, "area")
		}
		if area.RequiresUnlock {
			ok, err := h.Directory.Unlocked(ctx, c.TenantID, c.UserID, in.AreaID)
			if err != nil {
				return err
			}
			if !ok {
				return fail(CodeDenied, "area")
			}
		}
		j.area = *area
		p, err := h.Directory.Player(ctx, c.TenantID, c.UserID)
		if err != nil {
			return err
		}
		if p == nil || p.Nickname == "" {
			return &Error{Code: CodeConflict, Retryable: true, RetryAfter: 2 * time.Second, Detail: "profile incomplete"}
		}
		j.player = *p
		if j.friends, err = h.Directory.Friends(ctx, c.TenantID, c.UserID); err != nil {
			return err
		}
		if j.blocks, err = h.Directory.Blocks(ctx, c.TenantID, c.UserID); err != nil {
			return err
		}
		j.audit, err = h.Directory.AuditHubMembership(ctx, c.TenantID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return j, nil
}

// place chooses an instance of the partition (MULTIPLAYER_GAMEPLAY §6) or creates one, and takes a seat.
func (h Hub) place(ctx context.Context, j *joinCtx, seat hub.Seat) (string, *port.JoinResult, error) {
	infos, err := h.Registry.Instances(ctx, j.area.ID, j.partition)
	if err != nil {
		return "", nil, fmt.Errorf("hub: instances: %w", err)
	}
	cands := make([]hub.Candidate, 0, len(infos))
	for _, in := range infos {
		cand := hub.Candidate{RoomID: in.RoomID, Members: len(in.Members), Capacity: in.Capacity}
		for _, m := range in.Members {
			if j.friends[m] {
				cand.Friends++
			}
			if j.blocks[m] {
				cand.Blocked = true
			}
		}
		cands = append(cands, cand)
	}
	room := hub.Choose(cands).RoomID
	if room == "" {
		id, err := h.create(ctx, j)
		if err != nil {
			return "", nil, err
		}
		room = id
	}
	res, err := h.Rooms.Join(ctx, room, port.JoinRequest{Seat: seat, Spawn: j.in.SpawnPoint})
	if err != nil {
		return "", nil, err
	}
	return room, res, nil
}

// create writes the room summary and opens the instance on this room host.
func (h Hub) create(ctx context.Context, j *joinCtx) (string, error) {
	sc, err := authz.EventScope(j.c.TenantID)
	if err != nil {
		return "", err
	}
	rec := port.RoomRecord{RoomID: h.newID(), AreaID: j.area.ID, Partition: j.partition, Capacity: hub.Capacity(j.area.Capacity, h.Config.Capacity),
		CreatedAt: h.now()}
	if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.Store.CreateRoom(ctx, rec) }); err != nil {
		return "", fmt.Errorf("hub: create room: %w", err)
	}
	m := port.RoomMeta{RoomID: rec.RoomID.String(), AreaID: rec.AreaID, Partition: rec.Partition, Capacity: rec.Capacity}
	if err := h.Rooms.Open(ctx, m); err != nil {
		return "", fmt.Errorf("hub: open room: %w", err)
	}
	return m.RoomID, nil
}

// follow joins the instance of a friend (F091-AC-05): friends, no block either way, a public hub instance of the same
// partition and area, a free seat. Any other case answers PERMISSION_DENIED without saying why.
func (h Hub) follow(ctx context.Context, j *joinCtx, seat hub.Seat, friend uuid.UUID) (string, *port.JoinResult, error) {
	ur, err := h.Registry.UserRoom(ctx, friend)
	if err != nil {
		return "", nil, fmt.Errorf("hub: friend room: %w", err)
	}
	var meta *port.RoomMeta
	if ur != nil {
		if meta, err = h.Registry.Meta(ctx, ur.RoomID); err != nil {
			return "", nil, fmt.Errorf("hub: friend room meta: %w", err)
		}
	}
	samePartition := meta != nil && meta.Partition == j.partition && meta.AreaID == j.area.ID
	switch hub.Follow(j.friends[friend], j.blocks[friend], samePartition, meta != nil, true) {
	case hub.FollowDenied:
		return "", nil, fail(CodeDenied, "follow")
	}
	res, err := h.Rooms.Join(ctx, meta.RoomID, port.JoinRequest{Seat: seat, Spawn: j.in.SpawnPoint})
	switch {
	case errors.Is(err, hub.ErrFull):
		return "", nil, fail(CodeRoomFull, "follow")
	case errors.Is(err, hub.ErrWrongPartition), errors.Is(err, ErrRoomGone):
		return "", nil, fail(CodeDenied, "follow")
	case err != nil:
		return "", nil, err
	}
	return meta.RoomID, res, nil
}

// assigned records the membership (audit_hub_membership), tells the gateway the area/room and builds the result.
func (h Hub) assigned(ctx context.Context, j *joinCtx, room string, res *port.JoinResult, previous string, start time.Time) (*Assigned, error) {
	if j.audit && !res.Rejoined {
		roomID, err := uuid.Parse(room)
		if err == nil {
			sc, _ := authz.EventScope(j.c.TenantID)
			rec := port.MembershipRecord{MembershipID: h.newID(), TenantID: j.c.TenantID, RoomID: roomID, SchoolID: j.partition.SchoolID,
				StudentID: j.c.UserID, GameSessionID: j.c.GameSessionID, JoinedAt: h.now()}
			err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error { return h.Store.OpenMembership(ctx, rec) })
		}
		if err != nil {
			return nil, fmt.Errorf("hub: membership: %w", err)
		}
	}
	if h.Metrics != nil {
		h.Metrics.AssignDuration(time.Since(start))
	}
	hz := h.Config.TickHz
	if hz <= 0 {
		hz = 10
	}
	if previous == room {
		previous = ""
	}
	a := &Assigned{RoomID: room, AreaID: j.area.ID, SelfEID: res.SelfEID, Spawn: res.Spawn, TickRateHz: hz,
		MaxSpeed: j.area.MaxSpeedMPS, Players: res.Players, Previous: previous}
	if h.Presence != nil {
		// ephemeral (ADR-0004): a lost update is refreshed by the next join; hub.instance.assigned also answers the ack
		_ = h.Presence.Update(ctx, j.c.UserID, j.area.ID, room)
		_ = h.Presence.ToUser(ctx, j.c.UserID, j.c.GameSessionID, "hub.instance.assigned", a)
	}
	return a, nil
}

// EmitEmote checks and broadcasts hub.player.emote (F091-AC-07, AC-18).
func (h Hub) EmitEmote(ctx context.Context, c Caller, room, emote string) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.EmitEmote")
	defer func() { done(err) }()
	if _, perr := uuid.Parse(room); perr != nil || !idRe.MatchString(emote) {
		return fail(CodeValidation, "room_id/emote_id")
	}
	res, err := h.Rooms.Emote(ctx, room, c.TenantID, c.UserID, emote)
	switch {
	case errors.Is(err, hub.ErrNotMember), errors.Is(err, ErrRoomGone):
		return fail(CodeRoomNotMember, "")
	case err != nil:
		return err
	case !res.Owned:
		return fail(CodeDenied, "emote")
	case res.Wait > 0:
		return &Error{Code: CodeRateLimited, Retryable: true, RetryAfter: res.Wait}
	}
	return nil
}

// RemoveStudent takes a student out of their hub instance at once (user.access_revoked: no grace).
func (h Hub) RemoveStudent(ctx context.Context, user uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RemoveHubStudent")
	defer func() { done(err) }()
	ur, err := h.Registry.UserRoom(ctx, user)
	if err != nil || ur == nil {
		return err
	}
	if err := h.Rooms.Leave(ctx, ur.RoomID, user, reason); err != nil && !errors.Is(err, hub.ErrNotMember) && !errors.Is(err, ErrRoomGone) {
		return err
	}
	return nil
}

// ProfileChanged pushes a new public profile to the instance of the student (F091-AC-19).
func (h Hub) ProfileChanged(ctx context.Context, user uuid.UUID, p hub.PublicPlayer) (err error) {
	ur, err := h.Registry.UserRoom(ctx, user)
	if err != nil || ur == nil {
		return err
	}
	if err := h.Rooms.Appearance(ctx, ur.RoomID, user, p); err != nil && !errors.Is(err, hub.ErrNotMember) && !errors.Is(err, ErrRoomGone) {
		return err
	}
	return nil
}

var _ port.RoomEvents = Hub{}

// MemberRemoved closes the membership row and clears the presence area (F091-AC-09, AC-11, AC-23).
func (h Hub) MemberRemoved(ctx context.Context, m port.RoomMeta, user, _ uuid.UUID, reason string) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.LeaveHubInstance")
	defer func() { done(err) }()
	if h.Presence != nil {
		_ = h.Presence.Update(ctx, user, "", "")
	}
	room, err := uuid.Parse(m.RoomID)
	if err != nil {
		return err
	}
	sc, err := authz.EventScope(m.Partition.TenantID)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		_, err := h.Store.CloseMembership(ctx, m.Partition.TenantID, room, user, reason, h.now())
		return err
	})
}

// RoomClosed records the end of an empty instance (F091-AC-10).
func (h Hub) RoomClosed(ctx context.Context, m port.RoomMeta) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CloseHubInstance")
	defer func() { done(err) }()
	room, err := uuid.Parse(m.RoomID)
	if err != nil {
		return err
	}
	sc, err := authz.EventScope(m.Partition.TenantID)
	if err != nil {
		return err
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		_, err := h.Store.CloseRoom(ctx, m.Partition.TenantID, room, h.now())
		return err
	})
}

// Violation records an integrity signal and an audit event when a student goes over 20 violations per minute
// (F091-AC-13): never an automatic sanction.
func (h Hub) Violation(ctx context.Context, m port.RoomMeta, user, school uuid.UUID, reason string, count int, sample hub.Vec3) (err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.RecordHubViolation")
	defer func() { done(err) }()
	room, err := uuid.Parse(m.RoomID)
	if err != nil {
		return err
	}
	sc, err := authz.EventScope(m.Partition.TenantID)
	if err != nil {
		return err
	}
	typ := "hub_speed"
	if reason == hub.ReasonOutOfBounds {
		typ = "hub_bounds"
	}
	now := h.now()
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Store.RecordSignal(ctx, port.SignalRecord{SignalID: h.newID(), TenantID: m.Partition.TenantID, SchoolID: school,
			StudentID: user, RoomID: room, SignalType: typ, Score: float64(count), CreatedAt: now,
			Evidence: map[string]any{"room_id": m.RoomID, "area_id": m.AreaID, "violations_per_min": count, "reason": reason,
				"sample": []float64{sample[0], sample[1], sample[2]}}}); err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: "hub.violation_threshold_exceeded", TargetType: "student", TargetID: user.String(),
			Result: audit.ResultSuccess, Reason: reason, SchoolID: school.String(),
			Metadata: map[string]any{"room_id": m.RoomID, "violations_per_min": count, "signal_type": typ}})
	})
}

// CloseOrphanRooms is one run of the job close-idle-hub-instances: open hub rooms older than idle_close whose
// instance is not live in the registry any more (room host gone with Valkey state, ADR-0004) are closed.
func (h Hub) CloseOrphanRooms(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "competition.command.CloseIdleHubInstances")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("close-idle-hub-instances")
	if err != nil {
		return 0, err
	}
	var refs []port.RoomRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = h.Store.OpenHubRooms(ctx, h.now().Add(-h.Config.IdleClose), batch)
		return err
	}); err != nil {
		return 0, err
	}
	for _, r := range refs {
		live, err := h.Registry.Live(ctx, r.RoomID.String())
		if err != nil {
			return n, err
		}
		if live {
			continue
		}
		sc, err := authz.EventScope(r.TenantID)
		if err != nil {
			return n, err
		}
		var closed bool
		if err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
			var err error
			closed, err = h.Store.CloseRoom(ctx, r.TenantID, r.RoomID, h.now())
			return err
		}); err != nil {
			return n, err
		}
		if closed {
			n++
		}
	}
	return n, nil
}

// LoadArea reads the runtime data of an area for the room host (ref.world_areas, tenant scope of the room).
func (h Hub) LoadArea(ctx context.Context, tenant uuid.UUID, area string) (*hub.Area, error) {
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	var a *hub.Area
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		var err error
		a, err = h.Directory.Area(ctx, area)
		return err
	})
	return a, err
}
