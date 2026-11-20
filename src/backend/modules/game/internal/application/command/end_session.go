package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/gamesession"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// own loads a session of the calling student: another student's session (or an unknown one) is 404 NOT_FOUND
// (F014-AC-17, no existence leak).
func (h Sessions) own(ctx context.Context, id uuid.UUID) (*gamesession.Session, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.UserType != authz.UserTypeStudent || p.TenantID == nil {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	s, err := h.Sessions.ByID(ctx, *p.TenantID, id)
	if err != nil {
		return nil, err
	}
	if s == nil || s.StudentID != p.UserID {
		return nil, problem.New(problem.NotFound, "")
	}
	return s, nil
}

// End implements endGameSession (F014-AC-14): the session ends with logout, its login event records the end once
// and game_session.ended / student.game_logged_out are published. Ending an ended session is a no-op (204).
func (h Sessions) End(ctx context.Context, id uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.EndGameSession")
	defer func() { done(err) }()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.own(ctx, id)
		if err != nil {
			return err
		}
		_, err = h.endSession(ctx, s, gamesession.ReasonLogout, h.now(), nil)
		return err
	})
}

// Heartbeat implements heartbeatGameSession (F014-AC-15): last_heartbeat_at moves forward; a replaced or ended
// session (or one held by another installation) is 409 CONFLICT so the old device stops.
func (h Sessions) Heartbeat(ctx context.Context, id, device uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "game.command.HeartbeatGameSession")
	defer func() { done(err) }()
	return h.UoW.Do(ctx, func(ctx context.Context) error {
		s, err := h.own(ctx, id)
		if err != nil {
			return err
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

// endRef ends one session found by a platform job in its own tenant transaction (RLS of the tenant); a session
// that ended meanwhile (other instance, logout) is skipped, so two instances never end it twice.
func (h Sessions) endRef(ctx context.Context, ref write.SessionRef, decide func(ctx context.Context, s *gamesession.Session, now time.Time) (string, bool, error)) (bool, *gamesession.Session, error) {
	sc, err := authz.EventScope(ref.TenantID)
	if err != nil {
		return false, nil, err
	}
	var ended *gamesession.Session
	err = h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		s, err := h.Sessions.ByID(ctx, ref.TenantID, ref.ID)
		if err != nil || s == nil || !s.Active() {
			return err
		}
		now := h.now()
		reason, ok, err := decide(ctx, s, now)
		if err != nil || !ok {
			return err
		}
		done, err := h.endSession(ctx, s, reason, now, nil)
		if done {
			ended = s
		}
		return err
	})
	return ended != nil, ended, err
}

// TimeoutStaleSessions is one run of the job session-timeout (F014-AC-16): every active session without a
// heartbeat (REST or WebSocket) for longer than game_session_heartbeat_timeout ends with timeout, also when the
// client only called gameLogout (ADR-0083). Returns the number of sessions ended.
func (h Sessions) TimeoutStaleSessions(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.TimeoutStaleSessions")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("session-timeout")
	if err != nil {
		return 0, err
	}
	var refs []write.SessionRef
	if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		var err error
		refs, err = h.Sessions.Stale(ctx, h.now().Add(-h.Config.HeartbeatTimeout), batch)
		if err != nil || h.Metrics == nil {
			return err
		}
		active, err := h.Sessions.ActiveByPlatform(ctx)
		if err == nil {
			h.Metrics.Active(active) // before this run ends the stale ones: refreshed again at the next tick
		}
		return err
	}); err != nil {
		return 0, err
	}
	var errs []error
	for _, ref := range refs {
		ok, _, err := h.endRef(ctx, ref, func(_ context.Context, s *gamesession.Session, now time.Time) (string, bool, error) {
			return gamesession.ReasonTimeout, s.Stale(now, h.Config.HeartbeatTimeout), nil
		})
		if err != nil {
			errs = append(errs, err)
		}
		if ok {
			n++
		}
	}
	return n, errors.Join(errs...)
}

// CloseSessionsAtCurfew is one run of the job curfew-session-closer (F014-AC-26, AC-27): every active session of a
// student inside the effective curfew ends with curfew, and every session of a student who reached the daily play
// limit of the school ends with daily_limit (warned with session.session.ending_soon curfew_warning_minutes
// before); the client gets session.session.ended with reopen_at (gateway F090 closes 4410). Progress recorded on
// the server is untouched.
func (h Sessions) CloseSessionsAtCurfew(ctx context.Context, batch int) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.CloseSessionsAtCurfew")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("curfew-session-closer")
	if err != nil {
		return 0, err
	}
	var errs []error
	after := uuid.Nil
	for {
		var refs []write.SessionRef
		var platform gamesession.Window
		if err := h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
			var err error
			if refs, err = h.Sessions.ActivePage(ctx, after, batch); err != nil {
				return err
			}
			// platform_curfew in force for this page (projected platform setting, else the start-up default).
			platform, err = h.platformCurfew(ctx)
			return err
		}); err != nil {
			return n, err
		}
		for _, ref := range refs {
			after = ref.ID
			var reopen time.Time
			var reason string
			var warnAt *time.Time
			ok, s, err := h.endRef(ctx, ref, func(ctx context.Context, s *gamesession.Session, now time.Time) (string, bool, error) {
				school := uuid.Nil
				if s.SchoolID != nil {
					school = *s.SchoolID
				}
				cf, err := h.curfewWith(ctx, platform, s.TenantID, school)
				if err != nil {
					return "", false, err
				}
				var active bool
				if active, reopen = cf.Active(now); active {
					reason = gamesession.ReasonCurfew
					return reason, true, nil
				}
				// Daily play limit of the school (OQ-17, F014-AC-27): end at the limit, warn before it.
				a, err := h.dailyAllowance(ctx, s.TenantID, s.StudentID, school, now)
				if err != nil {
					return "", false, err
				}
				if a.Reached() {
					reason, reopen = gamesession.ReasonDailyLimit, a.Reopen
					return reason, true, nil
				}
				if h.warnNow(a, s.StartedAt, now) {
					endsAt := now.Add(a.Remaining)
					warnAt = &endsAt
				}
				return "", false, nil
			})
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if ok {
				n++
				if h.Realtime != nil {
					at := *s.EndedAt
					_ = h.Realtime.SessionEnded(ctx, s.StudentID, s.ID, reason, at, &reopen)
				}
			} else if warnAt != nil && h.Realtime != nil {
				_ = h.Realtime.SessionEndingSoon(ctx, ref.StudentID, ref.ID, gamesession.ReasonDailyLimit, *warnAt)
			}
		}
		if len(refs) < batch {
			return n, errors.Join(errs...)
		}
	}
}

// Revocation is an identity event that ends the sessions of a student (F014-AC-24).
type Revocation struct {
	StudentID uuid.UUID
	TenantID  *uuid.UUID // nil: every tenant (platform event)
	Reason    string     // gamesession end reason
	Realtime  string     // session.session.ended reason (account_locked, access_revoked, cleanup, password_changed…)
}

// EndSessionsForUser ends the active sessions of a student on user.access_revoked, user.status_changed (locked,
// deactivated) or student.cleaned_up, inside the consumer's transaction (scope of the event; idempotency per
// event_id by the consumer). An ended session is never ended again.
func (h Sessions) EndSessionsForUser(ctx context.Context, r Revocation) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "game.command.EndSessionsForUser")
	defer func() { done(err) }()
	if !gamesession.ValidEndReason(r.Reason) {
		return 0, problem.New(problem.Internal, "unknown end reason")
	}
	list, err := h.Sessions.ActiveOfStudent(ctx, r.StudentID, r.TenantID)
	if err != nil {
		return 0, err
	}
	now := h.now()
	for _, s := range list {
		ok, err := h.endSession(ctx, s, r.Reason, now, nil)
		if err != nil {
			return n, err
		}
		if ok {
			n++
			student, id, reason := s.StudentID, s.ID, r.Realtime
			h.afterCommit(ctx, func(ctx context.Context) {
				if h.Realtime != nil {
					_ = h.Realtime.SessionEnded(ctx, student, id, reason, now, nil)
				}
			})
		}
	}
	return n, nil
}
