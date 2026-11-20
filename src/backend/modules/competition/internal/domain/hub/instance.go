package hub

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// PublicPlayer is what other players may see of a member (AsyncAPI PublicPlayer, AGENTS R16, F091-AC-15): never the
// real name, username, school or class.
type PublicPlayer struct {
	UserID       uuid.UUID      `json:"user_id"`
	Nickname     string         `json:"nickname"`
	AvatarConfig map[string]any `json:"avatar_config,omitempty"`
	Level        int            `json:"level,omitempty"`
}

// Emotes returns the emote ids of the avatar (AvatarConfig.emote_slots).
func (p PublicPlayer) Emotes() []string {
	raw, _ := p.AvatarConfig["emote_slots"].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Appearance returns the slot → item summary of the avatar (hub.player.appearance_changed: string values only,
// ≤ 16 slots, ≤ 64 characters each).
func (p PublicPlayer) Appearance() map[string]string {
	out := map[string]string{}
	for k, v := range p.AvatarConfig {
		if s, ok := v.(string); ok && len(s) <= 64 && len(k) <= 64 && len(out) < 16 {
			out[k] = s
		}
	}
	return out
}

// Seat is what a member brings into an instance.
type Seat struct {
	Player        PublicPlayer
	TenantID      uuid.UUID
	SchoolID      uuid.UUID
	GradeBand     string
	GameSessionID uuid.UUID
	Node          string // gateway node of the connection (Ew-Node-Id): snapshots are packed per node
}

// Member is one student in the instance.
type Member struct {
	Seat
	EID       int
	Mover     Mover
	R         float64
	A         string
	ClientSeq int64
	LastInput time.Time
	HeldUntil time.Time // non-zero: disconnected, seat held until then (F091-AC-09)
	view      map[int]Entity
	lastAck   int64
	lastEmote time.Time
}

// Held reports a disconnected member whose seat is held.
func (m *Member) Held() bool { return !m.HeldUntil.IsZero() }

// Entity returns the snapshot state of the member (a held avatar stands still, idle).
func (m *Member) Entity() Entity {
	a := m.A
	if m.Held() {
		a = "idle"
	}
	return Entity{EID: m.EID, P: m.Mover.Pos, R: m.R, A: a}
}

// Instance is the in-memory state of one hub instance, owned by the room host holding its lease.
type Instance struct {
	RoomID     string
	Area       Area
	Partition  Partition
	Capacity   int
	members    map[uuid.UUID]*Member
	nextEID    int
	tick       int64
	emptySince time.Time
}

// NewInstance creates the state of an instance.
func NewInstance(room string, area Area, p Partition, capacity int, now time.Time) *Instance {
	return &Instance{RoomID: room, Area: area, Partition: p, Capacity: capacity, members: map[uuid.UUID]*Member{}, emptySince: now}
}

// Size is the number of seats taken (held seats included).
func (in *Instance) Size() int { return len(in.members) }

// Member returns the member of user.
func (in *Instance) Member(user uuid.UUID) (*Member, bool) {
	m, ok := in.members[user]
	return m, ok
}

// Members returns the members ordered by eid.
func (in *Instance) Members() []*Member {
	out := make([]*Member, 0, len(in.members))
	for _, m := range in.members {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EID < out[j].EID })
	return out
}

// Join seats a student (F091-AC-01, AC-02, AC-04, AC-09): a member coming back takes back the held seat even when the
// instance is full; a new member needs a free seat and the partition of the instance.
func (in *Instance) Join(s Seat, spawn Vec3, now time.Time) (m *Member, rejoined bool, err error) {
	if s.TenantID != in.Partition.TenantID || s.SchoolID != in.Partition.SchoolID || s.GradeBand != in.Partition.GradeBand {
		return nil, false, ErrWrongPartition
	}
	if m, ok := in.members[s.Player.UserID]; ok {
		m.HeldUntil, m.LastInput = time.Time{}, now
		m.Player, m.GameSessionID, m.Node = s.Player, s.GameSessionID, s.Node
		m.view = nil // the next snapshot is complete
		return m, true, nil
	}
	if len(in.members) >= in.Capacity {
		return nil, false, ErrFull
	}
	in.nextEID++
	m = &Member{Seat: s, EID: in.nextEID, Mover: NewMover(spawn, now, in.Area.MaxSpeedMPS), A: "idle", LastInput: now}
	in.members[s.Player.UserID] = m
	return m, false, nil
}

// Restore seats a member known from the registry after a failover (F091-AC-20), keeping its eid.
func (in *Instance) Restore(s Seat, eid int, pos Vec3, now time.Time) *Member {
	m := &Member{Seat: s, EID: eid, Mover: NewMover(pos, now, in.Area.MaxSpeedMPS), A: "idle", LastInput: now}
	in.members[s.Player.UserID] = m
	if eid > in.nextEID {
		in.nextEID = eid
	}
	return m
}

// Leave removes a member and returns it.
func (in *Instance) Leave(user uuid.UUID, now time.Time) (*Member, bool) {
	m, ok := in.members[user]
	if !ok {
		return nil, false
	}
	delete(in.members, user)
	if len(in.members) == 0 {
		in.emptySince = now
	}
	return m, true
}

// Hold marks a member disconnected and keeps the seat until now + grace (MULTIPLAYER_GAMEPLAY §9.2: avatar stands still).
func (in *Instance) Hold(user uuid.UUID, now time.Time, grace time.Duration) bool {
	m, ok := in.members[user]
	if !ok || m.Held() {
		return false
	}
	m.HeldUntil = now.Add(grace)
	return true
}

// Expired removes the members whose held seat ran out, or who sent no input for silence (fallback when the
// disconnection signal was lost), returning them.
func (in *Instance) Expired(now time.Time, grace, silence time.Duration) []*Member {
	var out []*Member
	for user, m := range in.members {
		if !m.Held() && silence > 0 && now.Sub(m.LastInput) > silence {
			m.HeldUntil = m.LastInput.Add(silence + grace)
		}
		if m.Held() && !now.Before(m.HeldUntil) {
			out = append(out, m)
			delete(in.members, user)
		}
	}
	if len(out) > 0 && len(in.members) == 0 {
		in.emptySince = now
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EID < out[j].EID })
	return out
}

// IdleFor reports how long the instance has been empty (0 when it has members).
func (in *Instance) IdleFor(now time.Time) time.Duration {
	if len(in.members) > 0 {
		return 0
	}
	return now.Sub(in.emptySince)
}

// Input is one hub.player.move.
type Input struct {
	ClientSeq int64
	P         Vec3
	R         float64
	A         string
}

// MoveResult is the outcome of an input.
type MoveResult struct {
	Corrected  bool
	Reason     string
	Valid      Vec3 // last valid position (the correction target)
	Violations int  // in the last minute
	Signal     bool // an integrity signal is due (F091-AC-13)
}

// Move applies an input of a member (F091-AC-12). An input of a held member means it is back: the seat is no longer
// held. Out-of-order inputs (client_seq not increasing) are ignored.
func (in *Instance) Move(user uuid.UUID, i Input, now time.Time) (MoveResult, error) {
	m, ok := in.members[user]
	if !ok {
		return MoveResult{}, ErrNotMember
	}
	if m.Held() {
		m.view = nil // back after a resume: the next snapshot is complete
	}
	m.HeldUntil, m.LastInput = time.Time{}, now
	if i.ClientSeq <= m.ClientSeq && m.ClientSeq != 0 {
		return MoveResult{Valid: m.Mover.Pos}, nil
	}
	m.ClientSeq = i.ClientSeq
	if ok, reason := m.Mover.Move(i.P, now, in.Area); !ok {
		n, sig := m.Mover.Violation(now)
		return MoveResult{Corrected: true, Reason: reason, Valid: m.Mover.Pos, Violations: n, Signal: sig}, nil
	}
	m.R = i.R
	if i.A != "" {
		m.A = i.A
	}
	return MoveResult{Valid: m.Mover.Pos}, nil
}

// Emote checks an emote of a member (F091-AC-18): owned by the avatar and not within rate of the previous one.
// It returns ok, or owned = false / wait > 0.
func (in *Instance) Emote(user uuid.UUID, emote string, now time.Time, rate time.Duration) (eid int, owned bool, wait time.Duration, err error) {
	m, ok := in.members[user]
	if !ok {
		return 0, false, 0, ErrNotMember
	}
	found := false
	for _, e := range m.Player.Emotes() {
		if e == emote {
			found = true
			break
		}
	}
	if !found {
		return m.EID, false, 0, nil
	}
	if d := now.Sub(m.lastEmote); !m.lastEmote.IsZero() && d < rate {
		return m.EID, true, rate - d, nil
	}
	m.lastEmote = now
	m.A = "emote:" + emote
	return m.EID, true, 0, nil
}

// SetPlayer replaces the public profile of a member (game_profile.updated, F091-AC-19).
func (in *Instance) SetPlayer(user uuid.UUID, p PublicPlayer) (*Member, bool) {
	m, ok := in.members[user]
	if !ok {
		return nil, false
	}
	p.UserID = user
	m.Player = p
	return m, true
}

// ViewerSnapshot is the hub.snapshot.updated of one viewer.
type ViewerSnapshot struct {
	UserID   uuid.UUID
	Node     string
	Tick     int64
	AckSeq   int64
	Entities []Entity
	Removed  []int
}

// Tick advances the instance and returns one snapshot per connected viewer that has something new (entities,
// removals or a newer acknowledged client_seq), grouped later by gateway node (F091-AC-14).
func (in *Instance) Tick() []ViewerSnapshot {
	in.tick++
	members := in.Members()
	all := make([]Entity, len(members))
	for i, m := range members {
		all[i] = m.Entity()
	}
	var out []ViewerSnapshot
	for i, m := range members {
		if m.Held() {
			continue
		}
		changed, removed, next := Delta(all[i], all, m.view)
		m.view = next
		if len(changed) == 0 && len(removed) == 0 && m.ClientSeq == m.lastAck {
			continue
		}
		m.lastAck = m.ClientSeq
		sort.Ints(removed)
		out = append(out, ViewerSnapshot{UserID: m.Player.UserID, Node: m.Node, Tick: in.tick, AckSeq: m.ClientSeq,
			Entities: changed, Removed: removed})
	}
	return out
}

// TickNo returns the current tick.
func (in *Instance) TickNo() int64 { return in.tick }
