package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// forgotSlots bounds the background work of forgotPortalPassword; when full the request is dropped (the
// answer is 202 anyway) rather than queued without limit.
var forgotSlots = make(chan struct{}, 32)

// forgotTimeout bounds one background run (lookup, token, email).
const forgotTimeout = 30 * time.Second

// ForgotPassword implements forgotPortalPassword (F012-AC-36): after the rate limit it always answers
// 202 at once; the lookup, the token and the email run in the background, so the response time does not
// depend on whether an account exists. Students, unknown identifiers and inactive accounts get nothing.
func (h PortalAuth) ForgotPassword(ctx context.Context, in dto.PortalForgotPassword) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ForgotPortalPassword")
	defer func() { done(err) }()
	cfg := h.Cfg.withDefaults()
	login := portalauth.NormalizeLogin(in.Identifier)
	deviceKey := portalauth.Key("device", in.DeviceID.String())
	if err := limit(ctx, h.Limiter, rateCheck{"forgot:i:" + portalauth.Key("login", login), cfg.Limits.ForgotPerIdentifier},
		rateCheck{"forgot:d:" + deviceKey, cfg.Limits.ForgotPerDevice}); err != nil {
		return err
	}
	job := context.WithoutCancel(ctx)
	run := func() {
		jctx, cancel := context.WithTimeout(job, forgotTimeout)
		defer cancel()
		h.issueReset(jctx, cfg, login, in.Locale, deviceKey)
	}
	if h.Background != nil { // tests run the job synchronously
		h.Background(run)
		return nil
	}
	select {
	case forgotSlots <- struct{}{}:
		go func() {
			defer func() { <-forgotSlots }()
			run()
		}()
	default:
		slog.WarnContext(ctx, "portal: password reset request dropped (background capacity)")
	}
	return nil
}

// issueReset creates the token (only its SHA-256 is stored, older unused tokens are superseded) and sends
// the email with the link https://portal.<domain>/reset-password?token=… — nothing else.
func (h PortalAuth) issueReset(ctx context.Context, cfg PortalAuthConfig, login, locale, deviceKey string) {
	acct, err := loadAccount(ctx, h.UoW, func(ctx context.Context) (*write.PortalAccount, error) {
		return h.Accounts.ActiveStaffByLogin(ctx, login)
	})
	if err != nil {
		slog.ErrorContext(ctx, "portal: password reset lookup failed", "error", err)
		return
	}
	if acct == nil || acct.UserType != "staff" || acct.Status != "active" {
		return // never tell, never send (students included)
	}
	if acct.Email == "" {
		h.record(ctx, audit.Event{Action: AuditPortalPasswordResetRequested, TargetType: "user", TargetID: acct.UserID.String(),
			Result: audit.ResultFailure, Reason: "no_email"})
		return
	}
	token, err := portalauth.NewSecret(portalauth.SecretBytes)
	if err != nil {
		slog.ErrorContext(ctx, "portal: password reset token", "error", err)
		return
	}
	if locale == "" {
		locale = acct.Locale
	}
	if len(locale) > 8 {
		locale = locale[:8]
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		n, err := h.Resets.Issue(ctx, write.PortalResetToken{ID: ids.New(), UserID: acct.UserID, TokenHash: portalauth.HashToken(token),
			Locale: locale, ExpiresAt: h.now().Add(cfg.ResetTTL), DeviceHash: deviceKey})
		if err != nil {
			return err
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditPortalPasswordResetRequested, TargetType: "user", TargetID: acct.UserID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"superseded": n, "device_hash": deviceKey}})
	})
	if err != nil {
		slog.ErrorContext(ctx, "portal: password reset token not stored", "error", err)
		return
	}
	link := strings.TrimRight(cfg.PortalBaseURL, "/") + "/reset-password?token=" + token
	if err := h.Mailer.SendPasswordReset(ctx, port.PasswordResetMail{To: acct.Email, Locale: locale, Link: link}); err != nil {
		slog.ErrorContext(ctx, "portal: password reset email not sent", "error", err)
	}
}

// ResetPassword implements resetPortalPassword (F012-AC-37): in one transaction the token is used atomically
// (used_at), the password set through the Admin API, UPDATE_PASSWORD removed and every Keycloak session of
// the user revoked; a policy violation or a Keycloak failure rolls back, so the token stays usable.
func (h PortalAuth) ResetPassword(ctx context.Context, in dto.PortalResetPassword) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ResetPortalPassword")
	defer func() { done(err) }()
	defer clear(in.NewPassword)
	cfg := h.Cfg.withDefaults()
	deviceKey := portalauth.Key("device", in.DeviceID.String())
	if err := limit(ctx, h.Limiter, rateCheck{"reset:d:" + deviceKey, cfg.Limits.ResetPerDevice}); err != nil {
		return err
	}
	sc, err := authz.SystemPlatformScope("portal-auth")
	if err != nil {
		return err
	}
	invalid := problem.New(problem.PasswordResetTokenInvalid, "")
	return h.UoW.Do(tenancy.WithScope(ctx, sc), func(ctx context.Context) error {
		userID, ok, err := h.Resets.Use(ctx, portalauth.HashToken(in.Token), h.now())
		if err != nil {
			return unavailable(err)
		}
		if !ok {
			return invalid
		}
		acct, err := h.Accounts.ByID(ctx, userID)
		if err != nil {
			return unavailable(err)
		}
		if acct == nil || acct.UserType != "staff" || acct.Status != "active" {
			return invalid
		}
		kcID, err := h.Accounts.KeycloakSubject(ctx, userID, cfg.Issuers)
		if err != nil {
			return unavailable(err)
		}
		if kcID == "" {
			return unavailable(errors.New("portal: no Keycloak identity for the user"))
		}
		err = h.KC.ResetPassword(ctx, kcID, in.NewPassword)
		clear(in.NewPassword)
		switch {
		case errors.Is(err, port.ErrKCPasswordPolicy):
			return passwordPolicy()
		case err != nil:
			return unavailable(err)
		}
		if err := h.KC.RemoveRequiredAction(ctx, kcID, port.KCActionUpdatePassword); err != nil {
			slog.WarnContext(ctx, "portal: UPDATE_PASSWORD not removed", "error", err)
		}
		if err := h.KC.LogoutUser(ctx, kcID); err != nil {
			return unavailable(err) // every portal session must end; the token stays usable for a retry
		}
		return h.Audit.Record(ctx, audit.Event{Action: AuditPortalPasswordReset, TargetType: "user", TargetID: userID.String(),
			Result: audit.ResultSuccess, Metadata: map[string]any{"device_hash": deviceKey, "sessions_revoked": true}})
	})
}
