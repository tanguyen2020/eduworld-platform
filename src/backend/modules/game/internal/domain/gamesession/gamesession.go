// Package gamesession is the domain of the server-side game session (F014, ADR-0031, DATABASE_DESIGN §3.12): the
// GameSession aggregate, the one-device rule (TakeoverPolicy), end reasons, client versions and the curfew
// window. Pure: no I/O, the clock is a parameter.
package gamesession

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// End reasons of game.game_sessions.end_reason and game.login_events.end_reason (LoginEndReason).
const (
	ReasonLogout        = "logout"
	ReasonReplaced      = "replaced_by_other_device"
	ReasonTimeout       = "timeout"
	ReasonAccountLocked = "account_locked"
	ReasonAccessRevoked = "access_revoked"
	ReasonCleanup       = "cleanup"
	ReasonClientCrash   = "client_crash"
	ReasonCurfew        = "curfew"
)

// ValidEndReason reports whether r is an end reason.
func ValidEndReason(r string) bool {
	switch r {
	case ReasonLogout, ReasonReplaced, ReasonTimeout, ReasonAccountLocked, ReasonAccessRevoked, ReasonCleanup,
		ReasonClientCrash, ReasonCurfew:
		return true
	}
	return false
}

// ValidPlatform reports a client platform of the contract (GameSessionCreate.client_platform).
func ValidPlatform(p string) bool {
	switch p {
	case "windows", "android", "ios", "web", "macos", "linux":
		return true
	}
	return false
}

// MaxDeviceLabel is the length of OtherDevice.device_label and LoginEvent.device_label.
const MaxDeviceLabel = 64

// DeviceLabel shortens a device model to the label shown to the student and the school (never the device id).
func DeviceLabel(model *string) *string {
	if model == nil || *model == "" {
		return nil
	}
	r := []rune(*model)
	if len(r) > MaxDeviceLabel {
		r = r[:MaxDeviceLabel]
	}
	s := string(r)
	return &s
}

// ErrEnded: the session is no longer active (replaced, ended or timed out): heartbeat → 409.
var ErrEnded = errors.New("gamesession: session ended")

// Session is one game session of a student on one device.
type Session struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	StudentID       uuid.UUID
	SchoolID        *uuid.UUID
	DeviceID        uuid.UUID
	DeviceModel     *string
	Platform        string
	ClientVersion   string
	KeycloakSID     string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	EndedAt         *time.Time
	EndReason       string
	ReplacedBy      *uuid.UUID
	Version         int64
}

// Active reports a session that has not ended.
func (s *Session) Active() bool { return s.EndedAt == nil }

// Stale reports an active session without heartbeat for longer than timeout (game_session_heartbeat_timeout).
func (s *Session) Stale(now time.Time, timeout time.Duration) bool {
	return s.Active() && now.Sub(s.LastHeartbeatAt) > timeout
}

// End ends the session with reason at; false when it had already ended (idempotent).
func (s *Session) End(reason string, at time.Time) bool {
	if !s.Active() {
		return false
	}
	s.EndedAt = &at
	s.EndReason = reason
	return true
}

// Replace ends the session because by took over the account on another device.
func (s *Session) Replace(by uuid.UUID, at time.Time) bool {
	if !s.End(ReasonReplaced, at) {
		return false
	}
	s.ReplacedBy = &by
	return true
}

// Heartbeat keeps the session alive; ErrEnded when it is no longer active.
func (s *Session) Heartbeat(at time.Time) error {
	if !s.Active() {
		return ErrEnded
	}
	if at.After(s.LastHeartbeatAt) {
		s.LastHeartbeatAt = at
	}
	return nil
}

// Duration is the played time of an ended session (game_session.ended.duration_ms).
func (s *Session) Duration() time.Duration {
	if s.EndedAt == nil || s.EndedAt.Before(s.StartedAt) {
		return 0
	}
	return s.EndedAt.Sub(s.StartedAt)
}

// Decision is what StartGameSession does with the active session of the student (TakeoverPolicy).
type Decision int

const (
	// NoActive: the student has no active session.
	NoActive Decision = iota
	// Conflict: active on another device with a recent heartbeat and no takeover → 409 ACTIVE_SESSION_ON_OTHER_DEVICE.
	Conflict
	// TakeOver: the student confirmed; the old session ends replaced_by_other_device.
	TakeOver
	// EndTimedOut: the old session missed its heartbeats; it ends with timeout, no confirmation asked.
	EndTimedOut
	// EndSameDevice: the same installation starts again; the old session ends with logout.
	EndSameDevice
)

// Decide applies the one-device rule (ADR-0031, DATABASE_DESIGN §3.12 step 2).
func Decide(active *Session, device uuid.UUID, takeover bool, now time.Time, timeout time.Duration) Decision {
	switch {
	case active == nil || !active.Active():
		return NoActive
	case active.Stale(now, timeout):
		return EndTimedOut
	case active.DeviceID == device:
		return EndSameDevice
	case takeover:
		return TakeOver
	}
	return Conflict
}

// EndReason is the end reason of the old session for d ("" when nothing ends).
func (d Decision) EndReason() string {
	switch d {
	case TakeOver:
		return ReasonReplaced
	case EndTimedOut:
		return ReasonTimeout
	case EndSameDevice:
		return ReasonLogout
	}
	return ""
}
