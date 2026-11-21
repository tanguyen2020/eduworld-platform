// Package cache holds the Valkey adapters of module identity. Keys use the module prefix ew:identity:
// (Valkey ACL per module, VALKEY_DESIGN §1). Valkey is never the only copy of important data (AGENTS R9):
// what lives here is short-lived (setup tickets, portal session times, rate-limit counters).
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

const prefix = "ew:identity:"

// SetupTickets implements port.SetupTicketStore (ADR-0091 §3): tickets are keyed by their SHA-256 and hold
// no password; GETDEL makes them single use.
type SetupTickets struct{ C valkey.Client }

// PortalSessions implements port.PortalSessionStore: the password-entry time of a portal session keyed
// by a hash of its sid (no token).
type PortalSessions struct{ C valkey.Client }

// RateLimiter implements port.RateLimiter with fixed windows.
type RateLimiter struct{ C valkey.Client }

var (
	_ port.SetupTicketStore   = SetupTickets{}
	_ port.PortalSessionStore = PortalSessions{}
	_ port.RateLimiter        = RateLimiter{}
)

func ticketKey(hash string) string { return prefix + "portal_setup:" + hash }
func sessionKey(key string) string { return prefix + "portal_session:" + key }

// Save implements port.SetupTicketStore (SET … PXAT expires_at).
func (s SetupTickets) Save(ctx context.Context, ticketHash string, t port.SetupTicket) error {
	if !t.ExpiresAt.After(time.Now()) {
		return errors.New("cache: setup ticket already expired")
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.C.Do(ctx, s.C.B().Set().Key(ticketKey(ticketHash)).Value(valkey.BinaryString(raw)).
		Pxat(t.ExpiresAt).Build()).Error()
}

// Take implements port.SetupTicketStore (GETDEL).
func (s SetupTickets) Take(ctx context.Context, ticketHash string) (port.SetupTicket, error) {
	raw, err := s.C.Do(ctx, s.C.B().Getdel().Key(ticketKey(ticketHash)).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return port.SetupTicket{}, port.ErrSetupTicketInvalid
	}
	if err != nil {
		return port.SetupTicket{}, fmt.Errorf("cache: setup ticket: %w", err)
	}
	var t port.SetupTicket
	if err := json.Unmarshal(raw, &t); err != nil || !t.ExpiresAt.After(time.Now()) {
		return port.SetupTicket{}, port.ErrSetupTicketInvalid
	}
	return t, nil
}

// Save implements port.PortalSessionStore.
func (s PortalSessions) Save(ctx context.Context, key string, v port.PortalSession, ttl time.Duration) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.C.Do(ctx, s.C.B().Set().Key(sessionKey(key)).Value(valkey.BinaryString(raw)).Px(ttl).Build()).Error()
}

// Get implements port.PortalSessionStore.
func (s PortalSessions) Get(ctx context.Context, key string) (port.PortalSession, bool, error) {
	raw, err := s.C.Do(ctx, s.C.B().Get().Key(sessionKey(key)).Build()).AsBytes()
	if valkey.IsValkeyNil(err) {
		return port.PortalSession{}, false, nil
	}
	if err != nil {
		return port.PortalSession{}, false, fmt.Errorf("cache: portal session: %w", err)
	}
	var v port.PortalSession
	if json.Unmarshal(raw, &v) != nil {
		return port.PortalSession{}, false, nil //nolint:nilerr // an unreadable record counts as absent (fail-safe for platform sessions)
	}
	return v, true, nil
}

// Delete implements port.PortalSessionStore.
func (s PortalSessions) Delete(ctx context.Context, key string) error {
	return s.C.Do(ctx, s.C.B().Del().Key(sessionKey(key)).Build()).Error()
}

// Allow implements port.RateLimiter: SET NX PX opens the window before INCR, so a counter never lives
// without an expiry.
func (s RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	k := prefix + key
	res := s.C.DoMulti(ctx,
		s.C.B().Set().Key(k).Value("0").Nx().Px(window).Build(),
		s.C.B().Incr().Key(k).Build(),
		s.C.B().Pttl().Key(k).Build())
	if err := res[0].Error(); err != nil && !valkey.IsValkeyNil(err) {
		return false, 0, fmt.Errorf("cache: rate limit: %w", err)
	}
	n, err := res[1].AsInt64()
	if err != nil {
		return false, 0, fmt.Errorf("cache: rate limit: %w", err)
	}
	if n <= int64(limit) {
		return true, 0, nil
	}
	ms, err := res[2].AsInt64()
	if err != nil || ms < 0 {
		ms = window.Milliseconds()
	}
	return false, time.Duration(ms) * time.Millisecond, nil
}
