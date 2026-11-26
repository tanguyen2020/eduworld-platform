package command

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/analytics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// ProtocolVersion is the WebSocket protocol version of subprotocol eduworld.v1.
const ProtocolVersion = 1

// ticketBytes is the entropy of a realtime ticket: 32 random bytes = 256 bit (F090-AC-01).
const ticketBytes = 32

// TicketConfig is the configuration of createRealtimeTicket (CONFIG_DEFAULTS realtime_ticket_ttl, ws_ticket_rate).
type TicketConfig struct {
	URL        string        // wss://…/realtime/v1 of the environment (EW_REALTIME_PUBLIC_URL); empty = not configured
	TTL        time.Duration // realtime_ticket_ttl
	Rate       int           // ws_ticket_rate per RateWindow and student
	RateWindow time.Duration
}

// TicketResult is the RealtimeTicket of the 201 response.
type TicketResult struct {
	Ticket          string
	ExpiresAt       time.Time
	URL             string
	ProtocolVersion int
}

// RetryAfter wraps a 429/503 problem with the delay of the Retry-After header.
type RetryAfter struct {
	Err   *problem.Error
	After time.Duration
}

func (e RetryAfter) Error() string { return e.Err.Error() }
func (e RetryAfter) Unwrap() error { return e.Err }

// RealtimeTickets implements createRealtimeTicket (F090 US1, ADR-0008): a one-time ticket for the realtime gateway,
// bound to the active game session of the calling student, its tenant, school and device.
type RealtimeTickets struct {
	Sessions Sessions
	Store    port.TicketStore
	Config   TicketConfig
	Random   io.Reader // nil = crypto/rand
}

// Issue checks the caller (student of the game channel, R23), the rate limit, the session (own, active), the curfew
// (ADR-0078), then stores sha256(ticket) with the claims for realtime_ticket_ttl (F090-AC-01..04, AC-31).
func (h RealtimeTickets) Issue(ctx context.Context, sessionID uuid.UUID) (res TicketResult, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.IssueRealtimeTicket")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return res, err
	}
	if p.UserType != authz.UserTypeStudent || p.Channel != authz.ChannelGame || p.TenantID == nil {
		return res, h.Sessions.deny(ctx, p, "not_student")
	}
	if h.Store == nil || h.Config.URL == "" {
		return res, problem.New(problem.Unavailable, "realtime is not configured").Retry()
	}
	ok, after, err := h.Store.Allow(ctx, p.UserID, h.Config.Rate, h.Config.RateWindow)
	if err != nil {
		return res, RetryAfter{Err: problem.New(problem.Unavailable, "realtime ticket store unavailable").Retry(), After: 5 * time.Second}
	}
	if !ok {
		return res, RetryAfter{Err: problem.New(problem.RateLimited, ""), After: after}
	}
	var claims rt.TicketClaims
	err = h.Sessions.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.Sessions.own(ctx, sessionID)
		if err != nil {
			return err
		}
		if !s.Active() {
			return problem.New(problem.Conflict, "the session has ended")
		}
		st, err := h.Sessions.Directory.Student(ctx, s.TenantID, s.StudentID)
		if err != nil {
			return err
		}
		if st == nil {
			return problem.New(problem.Unavailable, "student not projected yet").Retry()
		}
		cf, err := h.Sessions.curfew(ctx, s.TenantID, st.SchoolID)
		if err != nil {
			return err
		}
		if active, reopen := cf.Active(h.Sessions.now()); active {
			return problem.New(problem.CurfewActive, "").With("reopen_at", reopen.UTC())
		}
		claims = rt.TicketClaims{UserID: s.StudentID, TenantID: s.TenantID, SchoolID: st.SchoolID, GameSessionID: s.ID,
			DeviceID: s.DeviceID, IssuedAt: h.Sessions.now().UnixMilli(), Platform: s.Platform, GradeBand: ticketGradeBand(st.GradeLevel)}
		return nil
	})
	if err != nil {
		return res, err
	}
	ticket, err := h.newTicket()
	if err != nil {
		return res, fmt.Errorf("realtime ticket: %w", err)
	}
	if err := h.Store.Save(ctx, rt.HashTicket(ticket), claims, h.Config.TTL); err != nil {
		return res, RetryAfter{Err: problem.New(problem.Unavailable, "realtime ticket store unavailable").Retry(), After: 5 * time.Second}
	}
	return TicketResult{Ticket: ticket, ExpiresAt: h.Sessions.now().Add(h.Config.TTL), URL: h.Config.URL, ProtocolVersion: ProtocolVersion}, nil
}

// ticketGradeBand is the grade band of the ticket (label of eduworld_players_online, MONITORING §3): the band only, never
// the grade itself; "" when the grade is not projected.
func ticketGradeBand(grade *int) string {
	if grade == nil {
		return ""
	}
	if b := analytics.GradeBand(*grade); b != analytics.Unknown {
		return b
	}
	return ""
}

func (h RealtimeTickets) newTicket() (string, error) {
	r := h.Random
	if r == nil {
		r = rand.Reader
	}
	b := make([]byte, ticketBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RealtimeHeartbeat implements the NATS command ew.cmd.game.session.heartbeat of the gateway (F090-AC-12): like
// heartbeatGameSession, last_heartbeat_at moves forward; the identity comes from the ticket-bound Ew-* headers
// (tenant scope of the event, MULTITENANCY §4.1). Another student's or an unknown session is NOT_FOUND; an ended,
// replaced or other-device session is CONFLICT (the gateway closes 4409).
func (h Sessions) RealtimeHeartbeat(ctx context.Context, tenant, student, session, device uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.RealtimeHeartbeat")
	defer func() { done(err) }()
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return problem.New(problem.Unauthenticated, "")
	}
	return h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		s, err := h.Sessions.ByID(ctx, tenant, session)
		if err != nil {
			return err
		}
		if s == nil || s.StudentID != student {
			return problem.New(problem.NotFound, "")
		}
		if s.DeviceID != device {
			return problem.New(problem.Conflict, "the session belongs to another installation")
		}
		if err := s.Heartbeat(h.now()); err != nil {
			if errors.Is(err, gamesession.ErrEnded) {
				return problem.New(problem.Conflict, "the session has ended")
			}
			return err
		}
		return h.Sessions.Save(ctx, s)
	})
}
