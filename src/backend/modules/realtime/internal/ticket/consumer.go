// Package ticket consumes realtime tickets (F090-AC-05, AC-06, ADR-0008): one atomic GETDEL of
// ew:game:rt:ticket:<sha256(ticket)> written by module game; the plain ticket is never stored nor logged.
package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"

	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// Rejection reasons (label of eduworld_rt_ticket_rejected_total).
const (
	ReasonMissing     = "missing"     // no ticket parameter
	ReasonMalformed   = "malformed"   // too long, JWT-shaped or not base64url
	ReasonUnknown     = "unknown"     // not found: never issued, expired or already used
	ReasonUnavailable = "unavailable" // Valkey error
)

// ErrRejected carries the reason of a refused ticket.
type ErrRejected struct{ Reason string }

func (e ErrRejected) Error() string { return "ticket rejected: " + e.Reason }

// Consumer reads tickets once.
type Consumer struct {
	C       valkey.Client
	Timeout time.Duration
}

// Consume returns the claims of ticket and deletes it in the same command (one connection per ticket, SC-001).
func (c Consumer) Consume(ctx context.Context, ticket string) (rt.TicketClaims, error) {
	switch {
	case ticket == "":
		return rt.TicketClaims{}, ErrRejected{ReasonMissing}
	case len(ticket) > 512 || strings.Count(ticket, ".") >= 2 || strings.Trim(ticket,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "":
		return rt.TicketClaims{}, ErrRejected{ReasonMalformed} // a JWT is refused here (F090-AC-07)
	}
	t := c.Timeout
	if t <= 0 {
		t = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	raw, err := c.C.Do(ctx, c.C.B().Getdel().Key(rt.TicketKey(rt.HashTicket(ticket))).Build()).ToString()
	if valkey.IsValkeyNil(err) {
		return rt.TicketClaims{}, ErrRejected{ReasonUnknown}
	}
	if err != nil {
		return rt.TicketClaims{}, errors.Join(ErrRejected{ReasonUnavailable}, err)
	}
	var claims rt.TicketClaims
	if err := json.Unmarshal([]byte(raw), &claims); err != nil || !claims.Valid() {
		return rt.TicketClaims{}, ErrRejected{ReasonMalformed}
	}
	return claims, nil
}
