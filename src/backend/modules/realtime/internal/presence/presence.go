// Package presence keeps the ephemeral presence of the students connected to the gateway (F091 US3,
// MULTIPLAYER_GAMEPLAY §5, VALKEY_DESIGN §4.2): Valkey HASH ew:gateway:presence:user:<user_id> {status, activity,
// area_id, room_id, hidden, updated_at} with TTL presence_ttl, the ZSET ew:gateway:presence:tenant:<tenant_id>:online,
// and one NATS core signal ew.rt.presence.<tenant_id> per change (presence-changed.schema.json) for the modules that
// show it (F095) or react to it (hub seat, F091-AC-09). Presence is never written to PostgreSQL and never sent to a
// client by this package (F091-AC-25, AGENTS R9).
package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/valkey-io/valkey-go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Keys of the gateway (real prefix ew:gateway:, VALKEY_DESIGN §1 rule 4).
func userKey(user uuid.UUID) string { return "ew:gateway:presence:user:" + user.String() }
func onlineKey(tenant uuid.UUID) string {
	return "ew:gateway:presence:tenant:" + tenant.String() + ":online"
}

// State is the presence of one student.
type State struct {
	Status    string
	Activity  string
	AreaID    string
	RoomID    string
	Hidden    bool
	UpdatedAt int64
}

// Holder tells whether a student still has a live connection on any node (registry of F090).
type Holder func(ctx context.Context, user uuid.UUID) (bool, error)

// Service records presence changes and publishes their signal.
type Service struct {
	C       valkey.Client
	NC      *nats.Conn // nil: no signal (Valkey state only)
	TTL     time.Duration
	Timeout time.Duration // ws_timeout: away → offline when no new connection
	Now     func() time.Time
	Holder  Holder

	mu     sync.Mutex
	timers map[uuid.UUID]*time.Timer
}

// New builds the service.
func New(c valkey.Client, nc *nats.Conn, ttl, timeout time.Duration, now func() time.Time, holder Holder) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{C: c, NC: nc, TTL: ttl, Timeout: timeout, Now: now, Holder: holder, timers: map[uuid.UUID]*time.Timer{}}
}

// Caller is the identity of the connection (from the ticket, never from the client payload).
type Caller struct {
	UserID, TenantID, SchoolID uuid.UUID
}

// Get returns the stored presence (nil when none).
func (s *Service) Get(ctx context.Context, user uuid.UUID) (*State, error) {
	m, err := s.C.Do(ctx, s.C.B().Hgetall().Key(userKey(user)).Build()).AsStrMap()
	if err != nil || len(m) == 0 {
		return nil, err
	}
	at, _ := strconv.ParseInt(m["updated_at"], 10, 64)
	return &State{Status: m["status"], Activity: m["activity"], AreaID: m["area_id"], RoomID: m["room_id"],
		Hidden: m["hidden"] == "1", UpdatedAt: at}, nil
}

// apply writes the fields of fn's result, refreshes the TTL and the online index and publishes the signal.
func (s *Service) apply(ctx context.Context, c Caller, reason string, fn func(st *State)) (State, error) {
	cur, err := s.Get(ctx, c.UserID)
	if err != nil {
		return State{}, fmt.Errorf("presence: read: %w", err)
	}
	st := State{Status: rt.PresenceOffline, Activity: "idle"}
	if cur != nil {
		st = *cur
	}
	fn(&st)
	now := s.Now()
	st.UpdatedAt = now.UnixMilli()
	hidden := "0"
	if st.Hidden {
		hidden = "1"
	}
	key := userKey(c.UserID)
	cmds := valkey.Commands{
		s.C.B().Hset().Key(key).FieldValue().FieldValue("status", st.Status).FieldValue("activity", st.Activity).
			FieldValue("area_id", st.AreaID).FieldValue("room_id", st.RoomID).FieldValue("hidden", hidden).
			FieldValue("updated_at", strconv.FormatInt(st.UpdatedAt, 10)).Build(),
		s.C.B().Expire().Key(key).Seconds(int64(s.TTL / time.Second)).Build(),
	}
	if st.Status == rt.PresenceOffline {
		cmds = append(cmds, s.C.B().Zrem().Key(onlineKey(c.TenantID)).Member(c.UserID.String()).Build())
	} else {
		cmds = append(cmds, s.C.B().Zadd().Key(onlineKey(c.TenantID)).ScoreMember().ScoreMember(float64(st.UpdatedAt), c.UserID.String()).Build(),
			// trim members not seen for presence_ttl (VALKEY_DESIGN §3.1)
			s.C.B().Zremrangebyscore().Key(onlineKey(c.TenantID)).Min("-inf").Max(strconv.FormatInt(st.UpdatedAt-s.TTL.Milliseconds(), 10)).Build())
	}
	for _, r := range s.C.DoMulti(ctx, cmds...) {
		if err := r.Error(); err != nil {
			return st, fmt.Errorf("presence: write: %w", err)
		}
	}
	s.publish(c, st, reason, now)
	return st, nil
}

func (s *Service) publish(c Caller, st State, reason string, now time.Time) {
	if s.NC == nil {
		return
	}
	p := rt.PresenceChanged{V: 1, UserID: c.UserID, TenantID: c.TenantID, Status: st.Status, Activity: st.Activity,
		AreaID: st.AreaID, RoomID: st.RoomID, Hidden: st.Hidden, Reason: reason, Ts: now.UnixMilli()}
	if c.SchoolID != uuid.Nil {
		school := c.SchoolID
		p.SchoolID = &school
	}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	if err := s.NC.Publish(rt.PresenceSubject(c.TenantID), b); err != nil {
		slog.Warn("realtime: presence signal not published", "error", err) // ephemeral: accepted loss (ADR-0004)
	}
}

// Connected marks the student online (F091-AC-21) and cancels a pending offline timer.
func (s *Service) Connected(ctx context.Context, c Caller) error {
	s.cancelTimer(c.UserID)
	_, err := s.apply(ctx, c, rt.PresenceConnected, func(st *State) {
		if st.Status == rt.PresenceOffline || st.Status == rt.PresenceAway || st.Status == "" {
			st.Status = rt.PresenceOnline
		}
	})
	return err
}

// Disconnected applies the end of a connection (F091-AC-21, AC-24): 4409 (replaced) changes nothing, the new
// connection holds the presence; a timeout close (4408) or a gateway restart (1012) goes away and turns offline when
// no new connection exists after ws_timeout; any other end is offline at once.
func (s *Service) Disconnected(ctx context.Context, c Caller, code int) error {
	switch code {
	case rt.CloseReplaced:
		return nil
	case 4408, 1012:
		if _, err := s.apply(ctx, c, rt.PresenceTimeout, func(st *State) { st.Status = rt.PresenceAway }); err != nil {
			return err
		}
		s.armTimer(c)
		return nil
	}
	if s.connected(ctx, c.UserID) {
		return nil // another connection (other node) already holds the student
	}
	_, err := s.apply(ctx, c, rt.PresenceDisconnected, func(st *State) { st.Status = rt.PresenceOffline })
	return err
}

func (s *Service) connected(ctx context.Context, user uuid.UUID) bool {
	if s.Holder == nil {
		return false
	}
	ok, err := s.Holder(ctx, user)
	return err == nil && ok
}

func (s *Service) armTimer(c Caller) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.timers[c.UserID]; t != nil {
		t.Stop()
	}
	s.timers[c.UserID] = time.AfterFunc(s.Timeout, func() {
		s.mu.Lock()
		delete(s.timers, c.UserID)
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if s.connected(ctx, c.UserID) {
			return // reconnected (maybe on another node): that connection set online
		}
		cur, err := s.Get(ctx, c.UserID)
		if err != nil || cur == nil || cur.Status != rt.PresenceAway {
			return
		}
		if _, err := s.apply(ctx, c, rt.PresenceTimeout, func(st *State) { st.Status = rt.PresenceOffline }); err != nil {
			slog.Warn("realtime: presence offline failed", "error", err)
		}
	})
}

func (s *Service) cancelTimer(user uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.timers[user]; t != nil {
		t.Stop()
		delete(s.timers, user)
	}
}

// Statuses a client may set (presence.status.set); offline is set by the gateway only (F091-AC-22).
var settable = map[string]bool{rt.PresenceOnline: true, rt.PresenceInActivity: true, rt.PresenceInMatch: true, rt.PresenceAway: true}

var activities = map[string]bool{"idle": true, "learning": true, "quest": true, "battle": true, "party_lobby": true, "exam": true}

// ErrInvalid is a status, activity or flag outside the presence enum.
var ErrInvalid = fmt.Errorf("presence: invalid status")

// SetStatus applies presence.status.set (F091-AC-22). hidden nil keeps the current flag.
func (s *Service) SetStatus(ctx context.Context, c Caller, status, activity string, hidden *bool) error {
	if !settable[status] || (activity != "" && !activities[activity]) {
		return ErrInvalid
	}
	_, err := s.apply(ctx, c, rt.PresenceStatusSet, func(st *State) {
		st.Status = status
		if activity != "" {
			st.Activity = activity
		}
		if hidden != nil {
			st.Hidden = *hidden
		}
	})
	return err
}

// SetArea applies the control presence.update of the room host (F091-AC-23).
func (s *Service) SetArea(ctx context.Context, c Caller, area, room string) error {
	_, err := s.apply(ctx, c, rt.PresenceAreaChanged, func(st *State) {
		st.AreaID, st.RoomID = area, room
		if st.Status == rt.PresenceOffline {
			st.Status = rt.PresenceOnline
		}
	})
	return err
}

// Refresh extends the TTL and the last-seen score while the connection lives (gateway heartbeat).
func (s *Service) Refresh(ctx context.Context, c Caller) error {
	now := s.Now().UnixMilli()
	for _, r := range s.C.DoMulti(ctx, s.C.B().Expire().Key(userKey(c.UserID)).Seconds(int64(s.TTL/time.Second)).Build(),
		s.C.B().Zadd().Key(onlineKey(c.TenantID)).Xx().ScoreMember().ScoreMember(float64(now), c.UserID.String()).Build()) {
		if err := r.Error(); err != nil {
			return err
		}
	}
	return nil
}
