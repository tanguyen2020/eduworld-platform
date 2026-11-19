// Package command holds the write use cases of module game (ADR-0087): each loads the aggregate through a write
// port, calls the domain, saves in one entx.UnitOfWork transaction with its outbox events and returns ids.
package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func principal(ctx context.Context) (authz.Principal, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return authz.Principal{}, problem.New(problem.Unauthenticated, "")
	}
	return p, nil
}

// Maintenance is the maintenance flag of client-config (FR-GAME-008).
type Maintenance struct {
	Active  bool
	Message string
	Until   *time.Time
}

// SessionConfig is the configuration of the game session (CONFIG_DEFAULTS: min_client_version,
// game_session_heartbeat_timeout, platform_curfew); read by infrastructure/config, never hard-coded (R3).
type SessionConfig struct {
	MinClientVersion string
	HeartbeatTimeout time.Duration
	Maintenance      Maintenance
	PlatformCurfew   gamesession.Window
	// CurfewLocation is the time zone of school curfew windows (Asia/Ho_Chi_Minh).
	CurfewLocation *time.Location
}

// Sessions are the game session use cases (createGameSession, endGameSession, heartbeatGameSession, the jobs
// session-timeout and curfew-session-closer, and the consumer that ends sessions on revocation).
type Sessions struct {
	UoW         write.UnitOfWork
	AfterCommit write.AfterCommit
	Sessions    write.SessionRepository
	Logins      write.LoginEventRepository
	Directory   write.Directory
	Events      write.EventPublisher
	Audit       write.Auditor
	Realtime    port.RealtimePublisher // nil: no realtime push (tests without Valkey)
	Metrics     port.SessionMetrics    // nil: no metrics
	Config      SessionConfig
	Now         func() time.Time
}

func (h Sessions) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

// curfew returns the effective curfew of a student of school (platform ∪ school window, ADR-0078).
func (h Sessions) curfew(ctx context.Context, tenant uuid.UUID, school uuid.UUID) (gamesession.Curfew, error) {
	c := gamesession.Curfew{h.Config.PlatformCurfew}
	w, err := h.Directory.SchoolCurfew(ctx, tenant, school)
	if err != nil || w == nil {
		return c, err
	}
	// A malformed projected window is ignored: the platform curfew still applies.
	if sw, werr := gamesession.NewWindow(w.Start, w.End, h.Config.CurfewLocation); werr == nil {
		c = append(c, sw)
	}
	return c, nil
}

// endSession ends s with reason at inside the transaction of ctx: session, login event, outbox events
// game_session.ended and student.game_logged_out (F014-AC-14, AC-16, AC-24, AC-26).
func (h Sessions) endSession(ctx context.Context, s *gamesession.Session, reason string, at time.Time, replacedBy *uuid.UUID) (bool, error) {
	var ended bool
	if replacedBy != nil {
		ended = s.Replace(*replacedBy, at)
	} else {
		ended = s.End(reason, at)
	}
	if !ended {
		return false, nil
	}
	if err := h.Sessions.Save(ctx, s); err != nil {
		return false, err
	}
	if err := h.Logins.Close(ctx, s.TenantID, s.ID, at, s.EndReason); err != nil {
		return false, err
	}
	tenant := s.TenantID
	id := s.ID.String()
	if err := h.Events.Publish(ctx, write.Event{Type: "game_session.ended", SubjectType: "game_session", SubjectID: id,
		PartitionKey: s.StudentID.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{
			"game_session_id": id, "student_id": s.StudentID.String(), "platform": s.Platform, "client_version": s.ClientVersion,
			"duration_ms": s.Duration().Milliseconds(), "end_reason": s.EndReason, "ended_at": at}}); err != nil {
		return false, err
	}
	if err := h.Events.Publish(ctx, write.Event{Type: "student.game_logged_out", SubjectType: "game_session", SubjectID: id,
		PartitionKey: s.StudentID.String(), TenantID: &tenant, OccurredAt: at, Payload: map[string]any{
			"student_id": s.StudentID.String(), "game_session_id": id, "logout_at": at, "end_reason": s.EndReason}}); err != nil {
		return false, err
	}
	h.afterCommit(ctx, func(context.Context) {
		if h.Metrics != nil {
			h.Metrics.Ended(s.EndReason)
		}
	})
	return true, nil
}

// afterCommit runs fn after the commit of the transaction of ctx (best effort: metrics, realtime push).
func (h Sessions) afterCommit(ctx context.Context, fn func(ctx context.Context)) {
	if h.AfterCommit == nil {
		return
	}
	_ = h.AfterCommit(ctx, fn)
}
