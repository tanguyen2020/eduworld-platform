// Package revocation decides what a connection does with a message received on Valkey Pub/Sub (F090 US4,
// REALTIME_ARCHITECTURE §3.1, research R1): deliver it or not, then close with 4403 (revoked), 4409 (replaced)
// or 4410 (session ended without revoking credentials), and whether the resume buffer is invalidated. Pure function:
// no I/O, unit tested for every case.
package revocation

import (
	"encoding/json"

	"github.com/google/uuid"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Action is the decision for one connection.
type Action struct {
	Deliver     bool   // deliver the message (seq assigned by the session)
	EndedReason string // send session.session.ended with this reason before closing (control access_revoked…)
	Close       int    // close code after delivery (0 = keep open)
	ClearResume bool   // delete the resume buffer and invalidate the resume token
}

// Conn is what the decision needs to know about the connection.
type Conn struct {
	UserID        uuid.UUID
	GameSessionID uuid.UUID
	ConnID        string
}

// Decide applies the target filter and the close rules. Session messages and controls are honored only on the
// user's own channel and on the node channel (userChannel = true); elsewhere they are ignored.
func Decide(e rt.Envelope, c Conn, userChannel bool) Action {
	if t := e.Target; t != nil {
		if t.GameSessionID != nil && *t.GameSessionID != c.GameSessionID {
			return Action{} // F090-AC-26: another session of the student, never close this connection
		}
		if t.ConnID != "" && t.ConnID != c.ConnID {
			return Action{}
		}
		if t.ExcludeConnID != "" && t.ExcludeConnID == c.ConnID {
			return Action{}
		}
		if len(t.UserIDs) > 0 && !contains(t.UserIDs, c.UserID) {
			return Action{}
		}
	}
	switch e.Kind {
	case rt.KindControl:
		if !userChannel || e.Control == nil {
			return Action{}
		}
		return control(*e.Control)
	case rt.KindMessage:
	default:
		return Action{}
	}
	if e.Message == nil {
		return Action{}
	}
	a := Action{Deliver: true}
	switch e.Message.Type {
	case "session.session.replaced", "session.session.ended":
		var p struct {
			GameSessionID string `json:"game_session_id"`
			Reason        string `json:"reason"`
		}
		if !userChannel || json.Unmarshal(e.Message.Payload, &p) != nil || p.GameSessionID != c.GameSessionID.String() {
			return Action{} // F090-AC-26, AC-32: only the connection holding that session is concerned
		}
		a.ClearResume = true
		switch {
		case e.Message.Type == "session.session.replaced":
			a.Close = rt.CloseReplaced // F090-AC-25
		case p.Reason == "curfew" || p.Reason == "daily_limit":
			a.Close = rt.CloseSessionDone // F090-AC-32: 4410, credentials kept
		default:
			a.Close = rt.CloseRevoked // F090-AC-24 reasons
		}
	}
	if userChannel && e.Control != nil && a.Close == 0 {
		ctl := control(*e.Control)
		a.Close, a.ClearResume = ctl.Close, ctl.ClearResume
	}
	return a
}

func control(c rt.Control) Action {
	if c.Action != "close" {
		return Action{}
	}
	switch {
	case c.Code == rt.CloseRevoked && rt.RevocationReason(c.Reason):
		reason := c.Reason
		if reason == "transferred" {
			reason = "access_revoked" // AsyncAPI SessionEnded.reason has no "transferred" (school of another tenant)
		}
		return Action{EndedReason: reason, Close: rt.CloseRevoked, ClearResume: true}
	case c.Code == rt.CloseReplaced:
		return Action{Close: rt.CloseReplaced}
	case c.Code == rt.CloseSessionDone && (c.Reason == "curfew" || c.Reason == "daily_limit"):
		return Action{EndedReason: c.Reason, Close: rt.CloseSessionDone, ClearResume: true}
	}
	return Action{}
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
