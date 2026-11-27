package command

import (
	"context"
	"errors"
	"log/slog"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/gamelogin"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Results of eduworld_auth_game_refresh_total{mode,result}.
const (
	refreshOK             = "ok"
	refreshInvalid        = "invalid"
	refreshReused         = "reused"
	refreshDeviceMismatch = "device_mismatch"
	refreshDenied         = "forbidden"
	refreshUnavailable    = "unavailable"
)

func (h GameAuth) refreshMetric(mode, result string) {
	if h.Metrics != nil {
		h.Metrics.Refresh(mode, result)
	}
}

// refreshOutcome is what the locked part of a refresh decided; side effects run after the commit.
type refreshOutcome struct {
	err       error
	result    string
	tokens    port.PortalTokens
	bootstrap *dto.PlayerBootstrap
	revoke    *gamelogin.Session // revoked in the transaction: end its Keycloak session after the commit
	audit     *audit.Event
}

// Refresh implements refreshGameToken (F013 US3, ADR-0086): the session is found by the SHA-256 of the
// presented token and locked (FOR UPDATE) so two concurrent refreshes cannot both succeed; a rotated token
// revokes the whole session (REFRESH_TOKEN_REUSED); another device is refused without calling Keycloak and
// without revoking (REFRESH_TOKEN_DEVICE_MISMATCH); resume re-checks account, tenant, enrollment, version
// and curfew and returns the bootstrap.
func (h GameAuth) Refresh(ctx context.Context, in dto.GameRefresh) (out dto.GameLoginResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RefreshGameToken")
	defer func() { done(err) }()
	cfg := h.Cfg.withDefaults()
	mode := in.Mode
	if mode == "" {
		mode = dto.RefreshRenew
	}
	if err := limit(ctx, h.Limiter, rateCheck{"game_refresh:s:" + h.IdP.SessionKey(in.RefreshToken), cfg.Limits.RefreshPerSession}); err != nil {
		return out, err
	}
	var o refreshOutcome
	err = h.inPlatform(ctx, func(ctx context.Context) error {
		o = h.refreshLocked(ctx, cfg, in, mode)
		if o.revoke != nil {
			return h.Sessions.Save(ctx, o.revoke)
		}
		return o.err
	})
	if o.revoke != nil && err == nil {
		h.afterRevoke(ctx, o.revoke)
		err = o.err
	}
	if o.audit != nil {
		h.record(ctx, *o.audit)
	}
	if err != nil {
		res := o.result
		if res == "" || o.err == nil {
			res = refreshUnavailable
			if problem.From(err, "").Code == problem.Internal {
				err = unavailable(err)
			}
		}
		h.refreshMetric(mode, res)
		return out, err
	}
	h.refreshMetric(mode, refreshOK)
	return dto.GameLoginResult{Tokens: tokenSet(o.tokens), Bootstrap: o.bootstrap}, nil
}

func (h GameAuth) refreshLocked(ctx context.Context, cfg GameAuthConfig, in dto.GameRefresh, mode string) refreshOutcome {
	invalid := refreshOutcome{err: problem.New(problem.RefreshTokenInvalid, ""), result: refreshInvalid}
	s, err := h.Sessions.ByTokenHash(ctx, gamelogin.HashToken(in.RefreshToken))
	if err != nil {
		return refreshOutcome{err: unavailable(err), result: refreshUnavailable}
	}
	if s == nil {
		return invalid
	}
	now := h.now()
	switch s.Classify(gamelogin.HashToken(in.RefreshToken), in.DeviceID) {
	case gamelogin.Revoked:
		return invalid
	case gamelogin.DeviceMismatch:
		return refreshOutcome{err: problem.New(problem.RefreshTokenDeviceMismatch, ""), result: refreshDeviceMismatch,
			audit: &audit.Event{Action: AuditRefreshDeviceMismatch, TargetType: "game_auth_session", TargetID: s.ID.String(),
				Result: audit.ResultDenied, Reason: string(problem.RefreshTokenDeviceMismatch), Metadata: map[string]any{"user_id": s.UserID.String()}}}
	case gamelogin.Reused:
		s.Revoke(gamelogin.RevokeRefreshReuse, now)
		return refreshOutcome{err: problem.New(problem.RefreshTokenReused, ""), result: refreshReused, revoke: s,
			audit: &audit.Event{Action: AuditRefreshTokenReused, TargetType: "game_auth_session", TargetID: s.ID.String(),
				Result: audit.ResultDenied, Reason: string(problem.RefreshTokenReused), Metadata: map[string]any{"user_id": s.UserID.String()}}}
	}
	if s.TenantID != in.TenantID {
		authz.RecordDenied(ctx, string(problem.TenantMismatch), map[string]any{"operation": "refreshGameToken"})
		return refreshOutcome{err: problem.New(problem.TenantMismatch, ""), result: refreshDenied,
			audit: &audit.Event{Action: AuditGameTenantMismatch, TargetType: "user", TargetID: s.UserID.String(),
				Result: audit.ResultDenied, Reason: string(problem.TenantMismatch)}}
	}
	if mode == dto.RefreshResume && in.ClientVersion != "" && !gamelogin.Supported(in.ClientVersion, cfg.MinClientVersion) {
		return refreshOutcome{err: problem.New(problem.ClientVersionUnsupported, ""), result: refreshDenied} // session kept
	}
	if o, done := h.recheck(ctx, cfg, s, mode); done {
		return o
	}
	var b *dto.PlayerBootstrap
	if mode == dto.RefreshResume {
		sc, err := authz.EventScope(s.TenantID)
		if err != nil {
			return refreshOutcome{err: unavailable(err), result: refreshUnavailable}
		}
		boot, err := h.Bootstrap.Bootstrap(tenancy.WithScope(ctx, sc), s.UserID)
		if err != nil {
			return refreshOutcome{err: unavailable(err), result: refreshUnavailable}
		}
		if !boot.IsCommonTenant && !boot.Placed {
			s.Revoke(gamelogin.RevokeAccessRevoked, now)
			return refreshOutcome{err: problem.New(problem.NoActiveEnrollment, ""), result: refreshDenied, revoke: s}
		}
		if err := h.applyHours(ctx, &boot); err != nil {
			return refreshOutcome{err: err, result: refreshDenied} // outside the hours: the session stays
		}
		if err := h.applyCurfew(ctx, cfg, &boot); err != nil {
			return refreshOutcome{err: err, result: refreshDenied} // curfew never revokes the session (F013-AC-32)
		}
		h.applyChat(ctx, &boot)
		h.applySocial(ctx, &boot)
		if err := h.applyDailyLimit(ctx, &boot, s.UserID); err != nil {
			return refreshOutcome{err: err, result: refreshDenied} // neither does the daily play limit (F013-AC-33)
		}
		b = &boot
	}
	tok, err := h.IdP.Refresh(ctx, in.RefreshToken)
	switch {
	case errors.Is(err, port.ErrPortalRefreshInvalid):
		return invalid
	case err != nil:
		return refreshOutcome{err: unavailable(err), result: refreshUnavailable}
	}
	s.Rotate(gamelogin.HashToken(tok.RefreshToken), now)
	if err := h.Sessions.Save(ctx, s); err != nil {
		return refreshOutcome{err: err, result: refreshUnavailable}
	}
	return refreshOutcome{tokens: tok, bootstrap: b}
}

// recheck verifies that the account may still play in the session tenant (IDENTITY_MAPPING §4.1): on
// failure the game session is revoked and the matching 403 returned (F013-AC-28).
func (h GameAuth) recheck(ctx context.Context, cfg GameAuthConfig, s *gamelogin.Session, mode string) (refreshOutcome, bool) {
	_ = cfg
	acct, err := h.Accounts.AccountByID(ctx, s.UserID)
	if err != nil {
		return refreshOutcome{err: unavailable(err), result: refreshUnavailable}, true
	}
	now := h.now()
	if code, _ := accountCheck(acct); code != "" {
		reason := gamelogin.RevokeAccessRevoked
		if code == problem.AccountLocked {
			reason = gamelogin.RevokeAccountLocked
		}
		s.Revoke(reason, now)
		return refreshOutcome{err: problem.New(code, ""), result: refreshDenied, revoke: s}, true
	}
	ms, err := h.Memberships.LoginMemberships(ctx, s.UserID)
	if err != nil {
		return refreshOutcome{err: unavailable(err), result: refreshUnavailable}, true
	}
	if _, ok := gamelogin.Find(ms, s.TenantID); !ok {
		if mode == dto.RefreshResume {
			s.Revoke(gamelogin.RevokeAccessRevoked, now)
			return refreshOutcome{err: problem.New(problem.TenantMismatch, ""), result: refreshDenied, revoke: s}, true
		}
		return refreshOutcome{err: problem.New(problem.TenantMismatch, ""), result: refreshDenied}, true
	}
	return refreshOutcome{}, false
}

// afterRevoke ends the Keycloak session of a revoked game session and counts it (best effort: Keycloak
// rotation and the revoked row already refuse the next refresh).
func (h GameAuth) afterRevoke(ctx context.Context, s *gamelogin.Session) {
	if h.KC != nil {
		if err := h.KC.DeleteSession(ctx, s.KeycloakSID); err != nil {
			slog.WarnContext(ctx, "game auth: deleting the Keycloak session failed", "error", err)
		}
	}
	if h.Metrics != nil {
		h.Metrics.Revoked(s.RevokeReason)
	}
}

// Logout implements gameLogout (F013-AC-14): the game session of the caller's Keycloak session (sid of the
// access token, or of the refresh token in the body) is revoked with reason logout and the Keycloak session
// ends; always 204 (idempotent). Module game is never called (ADR-0083).
func (h GameAuth) Logout(ctx context.Context, refreshToken string) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.GameLogout")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	var revoked *gamelogin.Session
	err = h.inPlatform(ctx, func(ctx context.Context) error {
		var s *gamelogin.Session
		var err error
		if p.SessionID != "" {
			s, err = h.Sessions.BySID(ctx, p.SessionID)
		}
		if err == nil && s == nil && refreshToken != "" {
			s, err = h.Sessions.ByTokenHash(ctx, gamelogin.HashToken(refreshToken))
		}
		if err != nil || s == nil || s.UserID != p.UserID || !s.Revoke(gamelogin.RevokeLogout, h.now()) {
			return err
		}
		revoked = s
		return h.Sessions.Save(ctx, s)
	})
	if err != nil {
		return unavailable(err)
	}
	if refreshToken != "" {
		h.revokeKC(ctx, refreshToken)
	}
	if revoked != nil {
		h.afterRevoke(ctx, revoked)
		h.record(ctx, audit.Event{Action: AuditGameLogout, TargetType: "game_auth_session", TargetID: revoked.ID.String(), Result: audit.ResultSuccess})
	} else if refreshToken == "" && p.SessionID != "" && h.KC != nil {
		if err := h.KC.DeleteSession(ctx, p.SessionID); err != nil {
			slog.WarnContext(ctx, "game auth: deleting the Keycloak session failed", "error", err)
		}
	}
	return nil
}

// RevokeSessions implements GameAuthSessionRevoker (F013-AC-29): every live game session matching f is
// revoked with reason, its Keycloak session deleted and audited. Revoking twice has no effect.
func (h GameAuth) RevokeSessions(ctx context.Context, f write.SessionFilter, reason string) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RevokeGameAuthSessions")
	defer func() { done(err) }()
	if !gamelogin.ValidRevokeReason(reason) {
		return 0, problem.New(problem.Internal, "unknown revoke reason")
	}
	var revoked []*gamelogin.Session
	err = h.inPlatform(ctx, func(ctx context.Context) error {
		list, err := h.Sessions.Live(ctx, f)
		if err != nil {
			return err
		}
		now := h.now()
		for _, s := range list {
			if s.Revoke(reason, now) {
				if err := h.Sessions.Save(ctx, s); err != nil {
					return err
				}
				revoked = append(revoked, s)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, s := range revoked {
		h.afterRevoke(ctx, s)
		h.record(ctx, audit.Event{Action: AuditGameSessionRevoked, TargetType: "game_auth_session", TargetID: s.ID.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"user_id": s.UserID.String()}})
	}
	return len(revoked), nil
}
