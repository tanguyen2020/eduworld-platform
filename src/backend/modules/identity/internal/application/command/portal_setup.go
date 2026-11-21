package command

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// PortalAccountSetupHandler is the typed interface of setupPortalPassword, forgotPortalPassword and
// resetPortalPassword (F012 US8).
type PortalAccountSetupHandler interface {
	SetupPassword(ctx context.Context, in dto.PortalSetupPassword) (dto.PortalSetupResult, error)
	ForgotPassword(ctx context.Context, in dto.PortalForgotPassword) error
	ResetPassword(ctx context.Context, in dto.PortalResetPassword) error
}

var _ PortalAccountSetupHandler = PortalAuth{}

// SubPasswordPolicy is errors[].code when Keycloak refuses a password (realm password policy).
const SubPasswordPolicy = "PASSWORD_POLICY_VIOLATION"

func passwordPolicy() error {
	return problem.Invalid("new_password", SubPasswordPolicy, "the password does not meet the password policy")
}

// SetupPassword implements setupPortalPassword (F012-AC-34, AC-35): the one-time ticket (GETDEL) gives the
// Keycloak user; the new password is set through the Admin API (realm policy) and UPDATE_PASSWORD is removed
// (no OTP enrolment since ADR-0094). A policy violation puts the ticket back until its expiry; any other
// failure consumes it.
func (h PortalAuth) SetupPassword(ctx context.Context, in dto.PortalSetupPassword) (out dto.PortalSetupResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SetupPortalPassword")
	defer func() { done(err) }()
	defer clear(in.NewPassword)
	cfg := h.Cfg.withDefaults()
	hash := portalauth.HashToken(in.SetupTicket)
	if err := limit(ctx, h.Limiter, rateCheck{"setup:t:" + hash[:32], cfg.Limits.SetupPerTicket}); err != nil {
		return out, err
	}
	t, err := h.Tickets.Take(ctx, hash)
	switch {
	case errors.Is(err, port.ErrSetupTicketInvalid):
		return out, problem.New(problem.SetupTicketInvalid, "")
	case err != nil:
		return out, unavailable(err)
	}
	putBack := func() {
		if t.ExpiresAt.After(h.now()) {
			if err := h.Tickets.Save(ctx, hash, t); err != nil {
				slog.WarnContext(ctx, "portal: setup ticket could not be restored", "error", err)
			}
		}
	}
	acct, err := loadAccount(ctx, h.UoW, func(ctx context.Context) (*write.PortalAccount, error) { return h.Accounts.ByID(ctx, t.UserID) })
	if err != nil {
		putBack()
		return out, unavailable(err)
	}
	// A ticket dies with the account (locked/deactivated meanwhile) and serves the password step only.
	if acct == nil || acct.UserType != "staff" || decide(&write.PortalAccount{UserType: acct.UserType, Status: acct.Status, HasPortalRole: true}) != portalauth.Allowed ||
		!slices.Contains(t.Steps, portalauth.StepUpdatePassword) {
		return out, problem.New(problem.SetupTicketInvalid, "")
	}
	err = h.KC.ResetPassword(ctx, t.KCUserID, in.NewPassword)
	clear(in.NewPassword)
	switch {
	case errors.Is(err, port.ErrKCPasswordPolicy):
		putBack()
		return out, passwordPolicy()
	case err != nil:
		putBack()
		return out, unavailable(err)
	}
	if err := h.KC.RemoveRequiredAction(ctx, t.KCUserID, port.KCActionUpdatePassword); err != nil {
		slog.WarnContext(ctx, "portal: UPDATE_PASSWORD not removed", "error", err)
	}
	steps := slices.DeleteFunc(slices.Clone(t.Steps), func(s string) bool { return s == portalauth.StepUpdatePassword })
	if actions, err := h.KC.RequiredActions(ctx, t.KCUserID); err == nil {
		steps = portalauth.Steps(actions)
	}
	h.record(ctx, audit.Event{Action: AuditPortalSetupPassword, TargetType: "user", TargetID: t.UserID.String(), Result: audit.ResultSuccess,
		Metadata: map[string]any{"remaining_steps": len(steps)}})
	if steps == nil {
		steps = []string{}
	}
	return dto.PortalSetupResult{RemainingSteps: steps}, nil
}
