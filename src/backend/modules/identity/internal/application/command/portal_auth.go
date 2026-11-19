package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Audit actions of the portal login (plan F012 "Bảo mật, tenant, idempotency, audit").
const (
	AuditPortalLogin                  = "auth.portal_login"
	AuditPortalLogout                 = "auth.portal_logout"
	AuditPortalSetupPassword          = "auth.portal_setup_password"
	AuditPortalPasswordResetRequested = "auth.portal_password_reset_requested"
	AuditPortalPasswordReset          = "auth.portal_password_reset"
	AuditPortalOTPSetupEmailSent      = "auth.portal_otp_setup_email_sent"
)

// PortalSessionHandler is the typed interface of portalLogin, refreshPortalToken and portalLogout (ADR-0087:
// no mediator).
type PortalSessionHandler interface {
	Login(ctx context.Context, in dto.PortalLogin) (dto.PortalSession, error)
	Refresh(ctx context.Context, refreshToken string) (dto.PortalSession, error)
	Logout(ctx context.Context, refreshToken string) error
}

// PortalAuth implements the Admin Portal login through the backend (ADR-0091, F012 US7/US8). Only Keycloak
// verifies passwords and OTP codes (AGENTS R30); module identity checks the EduWorld side (staff, status,
// roles), keeps no refresh token and audits without credentials or raw usernames (F012-AC-28, AC-31).
type PortalAuth struct {
	UoW      write.UnitOfWork
	Accounts write.PortalAccountRepository
	Audit    write.Auditor
	IdP      port.PortalIdP
	KC       port.PortalAccountAdmin
	Tickets  port.SetupTicketStore
	Sessions port.PortalSessionStore
	Limiter  port.RateLimiter
	Metrics  port.PortalAuthMetrics
	Cfg      PortalAuthConfig
	Clock    func() time.Time
}

var _ PortalSessionHandler = PortalAuth{}

func (h PortalAuth) now() time.Time {
	if h.Clock != nil {
		return h.Clock().UTC()
	}
	return time.Now().UTC()
}

func (h PortalAuth) loginResult(r string) {
	if h.Metrics != nil {
		h.Metrics.Login(r)
	}
}

// record writes one audit event in its own transaction (durable for denials too); a failed audit write is
// logged and does not change the answer to the user.
func (h PortalAuth) record(ctx context.Context, e audit.Event) {
	if err := h.Audit.Record(ctx, e); err != nil {
		slog.ErrorContext(ctx, "portal audit failed", "action", e.Action, "error", err)
	}
}

// revoke ends the Keycloak session just created when EduWorld refuses it (F012-AC-29); best effort.
func (h PortalAuth) revoke(ctx context.Context, refreshToken string) {
	if err := h.IdP.Logout(ctx, refreshToken); err != nil {
		slog.WarnContext(ctx, "portal: revoking the Keycloak session failed", "error", err)
	}
}

func targetUser(id uuid.UUID) (string, string) {
	if id == uuid.Nil {
		return "", ""
	}
	return "user", id.String()
}

// Login implements portalLogin (F012-AC-27..AC-29, AC-32, AC-33).
func (h PortalAuth) Login(ctx context.Context, in dto.PortalLogin) (out dto.PortalSession, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PortalLogin")
	defer func() { done(err) }()
	defer clear(in.Password)
	defer clear(in.OTP)
	cfg := h.Cfg.withDefaults()
	login := portalauth.NormalizeLogin(in.Username)
	userKey := portalauth.Key("login", login)
	deviceKey := portalauth.Key("device", in.DeviceID.String())
	meta := map[string]any{"username_hash": userKey, "device_hash": deviceKey}
	if err := limit(ctx, h.Limiter, rateCheck{"login:g", cfg.Limits.LoginGlobal},
		rateCheck{"login:u:" + userKey, cfg.Limits.LoginPerUsername}, rateCheck{"login:d:" + deviceKey, cfg.Limits.LoginPerDevice}); err != nil {
		h.loginResult(port.LoginRateLimited)
		return out, err
	}
	tok, err := h.IdP.PasswordGrant(ctx, login, in.Password, in.OTP)
	clear(in.Password)
	clear(in.OTP)
	deny := func(code problem.Code, result string, userID uuid.UUID) error {
		h.loginResult(result)
		tt, tid := targetUser(userID)
		h.record(ctx, audit.Event{Action: AuditPortalLogin, TargetType: tt, TargetID: tid, Result: audit.ResultDenied,
			Reason: string(code), Metadata: meta})
		return problem.New(code, "")
	}
	switch {
	case errors.Is(err, port.ErrPortalInvalidCredentials):
		// One status, code and message for every credential failure; nothing tells whether OTP is needed.
		return out, deny(problem.InvalidCredentials, port.LoginInvalidCredentials, uuid.Nil)
	case errors.Is(err, port.ErrPortalAccessDenied):
		return out, deny(problem.PortalAccessStaffOnly, port.LoginStaffOnly, uuid.Nil)
	case errors.Is(err, port.ErrPortalSetupRequired):
		return out, h.setupRequired(ctx, cfg, login, meta)
	case err != nil:
		h.loginResult(port.LoginUnavailable)
		return out, unavailable(err)
	}
	acct, err := loadAccount(ctx, h.UoW, func(ctx context.Context) (*write.PortalAccount, error) {
		return h.Accounts.BySubject(ctx, tok.Issuer, tok.Subject)
	})
	if err != nil {
		h.revoke(ctx, tok.RefreshToken)
		h.loginResult(port.LoginUnavailable)
		return out, unavailable(err)
	}
	userID := uuid.Nil
	if acct != nil {
		userID = acct.UserID
	}
	switch decide(acct) {
	case portalauth.StaffOnly:
		h.revoke(ctx, tok.RefreshToken)
		return out, deny(problem.PortalAccessStaffOnly, port.LoginStaffOnly, userID)
	case portalauth.Deactivated:
		h.revoke(ctx, tok.RefreshToken)
		return out, deny(problem.AccountDeactivated, port.LoginDeactivated, userID)
	}
	mfa := portalauth.MFA(tok.AMR, cfg.MFAAMR)
	authTime := h.now()
	key := portalauth.SessionKey(tok.SessionID)
	if err := h.Sessions.Save(ctx, key, port.PortalSession{UserID: acct.UserID, AuthTime: authTime, MFA: mfa}, ttl(tok)); err != nil {
		// Without it a refresh of an MFA session is refused (fail-safe: the user signs in again).
		slog.WarnContext(ctx, "portal: session store unavailable", "error", err)
	}
	out, err = session(tok, authTime, mfa)
	if err != nil {
		h.revoke(ctx, tok.RefreshToken)
		return dto.PortalSession{}, unavailable(err)
	}
	meta["mfa"] = mfa
	meta["session_key"] = key
	h.loginResult(port.LoginSuccess)
	h.record(ctx, audit.Event{Action: AuditPortalLogin, TargetType: "user", TargetID: acct.UserID.String(), Result: audit.ResultSuccess, Metadata: meta})
	return out, nil
}

func decide(a *write.PortalAccount) portalauth.Decision {
	if a == nil {
		return portalauth.Decide(nil)
	}
	return portalauth.Decide(&portalauth.Account{UserType: a.UserType, Status: a.Status, HasPortalRole: a.HasPortalRole})
}

// ttl of the session record: the refresh lifetime Keycloak announced (idle, capped by the maximum).
func ttl(t port.PortalTokens) time.Duration {
	if t.RefreshExpiresIn > 0 {
		return time.Duration(t.RefreshExpiresIn) * time.Second
	}
	return 30 * time.Minute
}

func session(t port.PortalTokens, authTime time.Time, mfa bool) (dto.PortalSession, error) {
	csrf, err := portalauth.NewSecret(portalauth.SecretBytes)
	if err != nil {
		return dto.PortalSession{}, err
	}
	exp, rexp := t.ExpiresIn, t.RefreshExpiresIn
	if exp < 1 {
		exp = 1
	}
	if rexp < 1 {
		rexp = int(ttl(t) / time.Second)
	}
	return dto.PortalSession{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, CSRF: csrf, ExpiresIn: exp,
		RefreshExpiresIn: rexp, AuthTime: authTime, MFA: mfa}, nil
}

// Refresh implements refreshPortalToken (F012-AC-30, AC-32). The CSRF double-submit is checked by the
// presentation before this use case runs. Refreshes of MFA sessions stop after MaxAuthAge counted from the
// password entry (direct grant tokens have no auth_time — F004 research R5).
func (h PortalAuth) Refresh(ctx context.Context, refreshToken string) (out dto.PortalSession, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.RefreshPortalToken")
	defer func() { done(err) }()
	cfg := h.Cfg.withDefaults()
	if err := limit(ctx, h.Limiter, rateCheck{"refresh:s:" + h.IdP.SessionKey(refreshToken), cfg.Limits.RefreshPerSession}); err != nil {
		return out, err
	}
	tok, err := h.IdP.Refresh(ctx, refreshToken)
	switch {
	case errors.Is(err, port.ErrPortalRefreshInvalid):
		return out, problem.New(problem.RefreshTokenInvalid, "")
	case err != nil:
		return out, unavailable(err)
	}
	acct, err := loadAccount(ctx, h.UoW, func(ctx context.Context) (*write.PortalAccount, error) {
		return h.Accounts.BySubject(ctx, tok.Issuer, tok.Subject)
	})
	if err != nil {
		return out, unavailable(err)
	}
	key := portalauth.SessionKey(tok.SessionID)
	end := func(code problem.Code) error {
		h.revoke(ctx, tok.RefreshToken)
		if err := h.Sessions.Delete(ctx, key); err != nil {
			slog.WarnContext(ctx, "portal: session store unavailable", "error", err)
		}
		return problem.New(code, "")
	}
	switch decide(acct) {
	case portalauth.StaffOnly:
		return out, end(problem.PortalAccessStaffOnly)
	case portalauth.Deactivated:
		return out, end(problem.AccountDeactivated)
	}
	s, found, err := h.Sessions.Get(ctx, key)
	if err != nil {
		slog.WarnContext(ctx, "portal: session store unavailable", "error", err)
		found = false
	}
	mfa := portalauth.MFA(tok.AMR, cfg.MFAAMR) || (found && s.MFA)
	authTime := tok.IssuedAt
	if found {
		authTime = s.AuthTime
	}
	if authTime.IsZero() {
		authTime = h.now()
	}
	if mfa && (!found || h.now().Sub(authTime) > cfg.MaxAuthAge) {
		return out, end(problem.RefreshTokenInvalid) // platform sessions re-enter password + OTP (ADR-0091 §4)
	}
	if err := h.Sessions.Save(ctx, key, port.PortalSession{UserID: acct.UserID, AuthTime: authTime, MFA: mfa}, ttl(tok)); err != nil {
		slog.WarnContext(ctx, "portal: session store unavailable", "error", err)
	}
	out, err = session(tok, authTime, mfa)
	if err != nil {
		return dto.PortalSession{}, unavailable(err)
	}
	return out, nil
}

// Logout implements portalLogout (F012-AC-30): ends the Keycloak session of the refresh token; always
// succeeds for the browser (cookies are cleared by the presentation), the outcome is audited.
func (h PortalAuth) Logout(ctx context.Context, refreshToken string) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.PortalLogout")
	defer func() { done(err) }()
	if refreshToken == "" {
		return nil
	}
	key := h.IdP.SessionKey(refreshToken)
	var userID uuid.UUID
	if s, found, gerr := h.Sessions.Get(ctx, key); gerr == nil && found {
		userID = s.UserID
	}
	result, reason := audit.ResultSuccess, ""
	if lerr := h.IdP.Logout(ctx, refreshToken); lerr != nil {
		slog.WarnContext(ctx, "portal: Keycloak logout failed", "error", lerr)
		result, reason = audit.ResultFailure, "keycloak_unavailable"
	}
	if derr := h.Sessions.Delete(ctx, key); derr != nil {
		slog.WarnContext(ctx, "portal: session store unavailable", "error", derr)
	}
	tt, tid := targetUser(userID)
	h.record(ctx, audit.Event{Action: AuditPortalLogout, TargetType: tt, TargetID: tid, Result: result, Reason: reason,
		Metadata: map[string]any{"session_key": key}})
	return nil
}

// setupRequired answers 403 ACCOUNT_SETUP_REQUIRED with a one-time setup_ticket (F012-AC-33) after
// Keycloak confirmed the password; when configure_otp is the only step left the CONFIGURE_TOTP email is
// sent at most once per window (F012-AC-35).
func (h PortalAuth) setupRequired(ctx context.Context, cfg PortalAuthConfig, login string, meta map[string]any) error {
	kcUser, err := h.KC.FindUserByLogin(ctx, login)
	if err != nil {
		h.loginResult(port.LoginUnavailable)
		return unavailable(err)
	}
	acct, err := loadAccount(ctx, h.UoW, func(ctx context.Context) (*write.PortalAccount, error) {
		return h.Accounts.ByKeycloakID(ctx, cfg.Issuers, kcUser.ID)
	})
	if err != nil {
		h.loginResult(port.LoginUnavailable)
		return unavailable(err)
	}
	deny := func(code problem.Code, result string) error {
		h.loginResult(result)
		var id uuid.UUID
		if acct != nil {
			id = acct.UserID
		}
		tt, tid := targetUser(id)
		h.record(ctx, audit.Event{Action: AuditPortalLogin, TargetType: tt, TargetID: tid, Result: audit.ResultDenied, Reason: string(code), Metadata: meta})
		return problem.New(code, "")
	}
	switch {
	case acct == nil || acct.UserType != "staff":
		return deny(problem.PortalAccessStaffOnly, port.LoginStaffOnly)
	case decide(&write.PortalAccount{UserType: acct.UserType, Status: acct.Status, HasPortalRole: true}) == portalauth.Deactivated:
		return deny(problem.AccountDeactivated, port.LoginDeactivated)
	}
	actions, err := h.KC.RequiredActions(ctx, kcUser.ID)
	if err != nil {
		h.loginResult(port.LoginUnavailable)
		return unavailable(err)
	}
	steps := portalauth.Steps(actions)
	sent := false
	if portalauth.OnlyOTPLeft(steps) {
		sent = h.sendOTPEmail(ctx, cfg, acct.UserID, kcUser.ID)
	}
	ticket, err := portalauth.NewSecret(portalauth.SecretBytes)
	if err != nil {
		h.loginResult(port.LoginUnavailable)
		return unavailable(err)
	}
	t := port.SetupTicket{UserID: acct.UserID, KCUserID: kcUser.ID, Steps: steps, OTPEmailSent: sent, ExpiresAt: h.now().Add(cfg.SetupTicketTTL)}
	if err := h.Tickets.Save(ctx, portalauth.HashToken(ticket), t); err != nil {
		h.loginResult(port.LoginUnavailable)
		return unavailable(err)
	}
	if steps == nil {
		steps = []string{}
	}
	h.loginResult(port.LoginSetupRequired)
	meta["required_steps"] = len(steps)
	h.record(ctx, audit.Event{Action: AuditPortalLogin, TargetType: "user", TargetID: acct.UserID.String(), Result: audit.ResultDenied,
		Reason: string(problem.AccountSetupRequired), Metadata: meta})
	return problem.New(problem.AccountSetupRequired, "").With("setup_ticket", ticket).With("required_steps", steps).
		With("otp_setup_email_sent", sent)
}

// sendOTPEmail asks Keycloak for the CONFIGURE_TOTP action-token email at most once per SetupTicketTTL
// (F012-AC-35). Module identity never generates, stores or checks an OTP secret. It reports whether the
// email was sent in the current window.
func (h PortalAuth) sendOTPEmail(ctx context.Context, cfg PortalAuthConfig, userID uuid.UUID, kcUserID string) bool {
	first, err := h.Tickets.MarkOTPEmail(ctx, userID, cfg.SetupTicketTTL)
	if err != nil {
		slog.WarnContext(ctx, "portal: OTP email window unavailable", "error", err)
		return false
	}
	if !first {
		return true // already sent in this window
	}
	if err := h.KC.ExecuteActionsEmail(ctx, kcUserID, port.KCActionConfigureTOTP); err != nil {
		slog.WarnContext(ctx, "portal: CONFIGURE_TOTP email failed", "error", err)
		if uerr := h.Tickets.UnmarkOTPEmail(ctx, userID); uerr != nil {
			slog.WarnContext(ctx, "portal: OTP email window unavailable", "error", uerr)
		}
		return false
	}
	h.record(ctx, audit.Event{Action: AuditPortalOTPSetupEmailSent, TargetType: "user", TargetID: userID.String(), Result: audit.ResultSuccess})
	return true
}
