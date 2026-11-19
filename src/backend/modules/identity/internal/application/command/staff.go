package command

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// InitialRole is one role of createTenantUser.
type InitialRole struct {
	RoleID    uuid.UUID
	ScopeType string
	ScopeID   *uuid.UUID
}

// CreateStaffInput is createTenantUser.
type CreateStaffInput struct {
	Username, Email, DisplayName, Locale string
	Roles                                []InitialRole
}

// UpdateStaffInput is updateTenantUser (JSON fields absent = unchanged).
type UpdateStaffInput struct {
	UserID      uuid.UUID
	IfMatch     int64
	Username    *string
	Email       *string
	DisplayName *string
	Locale      *string
	Status      *string
}

// StaffAdmin is the Keycloak part of staff creation.
type StaffAdmin interface {
	CreateUser(ctx context.Context, u port.KCNewUser) (string, error)
	DisableUser(ctx context.Context, userID string) error
	ExecuteActionsEmail(ctx context.Context, userID string, actions ...string) error
}

// Staff creates and updates the staff accounts of a tenant by hand (F012 US6: AC-25, AC-26).
type Staff struct {
	UoW         write.UnitOfWork
	Repo        write.StaffRepository
	Roles       UserRoles
	KC          StaffAdmin
	Events      write.EventPublisher
	Audit       write.Auditor
	LocalIssuer string
}

func (h Staff) session(ctx context.Context) (context.Context, authz.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, uuid.Nil, err
	}
	if p.TenantID == nil {
		return ctx, p, uuid.Nil, problem.New(problem.TenantHeaderRequired, "")
	}
	if err := authz.Require(ctx, p, authz.PermTenantUserWrite, authz.Resource{TenantID: p.TenantID}); err != nil {
		return ctx, p, uuid.Nil, err
	}
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, p, uuid.Nil, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), p, *p.TenantID, nil
}

func normalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	return s, err == nil && a.Address == s && len(s) <= 320
}

// Create creates the Keycloak user (no password; the action email asks for one), the staff account with the
// session tenant as home tenant and its initial roles in one use case; a database failure disables the
// Keycloak user it created.
func (h Staff) Create(ctx context.Context, in CreateStaffInput) (out write.StaffRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateTenantUser")
	defer func() { done(err) }()
	ctx, p, tenant, err := h.session(ctx)
	if err != nil {
		return out, err
	}
	pe := problem.New(problem.ValidationFailed, "")
	username, uerr := user.NormalizeUsername(in.Username)
	if uerr != nil {
		pe.AddField("username", problem.SubInvalid, "must match ^[A-Za-z0-9._-]{3,128}$")
	}
	email, ok := normalizeEmail(in.Email)
	if !ok {
		pe.AddField("email", problem.SubRequired, "a valid email is required for staff (ADR-0069)")
	}
	name, nerr := user.CheckDisplayName(in.DisplayName)
	if nerr != nil {
		pe.AddField("display_name", problem.SubInvalid, "1 to 128 characters")
	}
	if len(in.Roles) == 0 || len(in.Roles) > 20 {
		pe.AddField("initial_user_roles", problem.SubInvalid, "1 to 20 roles")
	}
	if len(pe.Fields) > 0 {
		return out, pe
	}
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if taken, err := h.Repo.UsernameTaken(ctx, username); err != nil || taken {
			return orTaken(err, taken, problem.UsernameTaken)
		}
		taken, err := h.Repo.EmailTaken(ctx, email, nil)
		return orTaken(err, taken, problem.EmailTaken)
	}); err != nil {
		return out, err
	}
	userID := ids.New()
	kcID, err := h.KC.CreateUser(ctx, port.KCNewUser{Username: username, Email: email, FirstName: name, Enabled: true,
		Attributes: map[string][]string{"eduworld_user_id": {userID.String()}, "ew_tenant": {tenant.String()}}})
	if errors.Is(err, port.ErrKCConflict) {
		return out, problem.New(problem.UsernameTaken, "") // the account exists on the SSO (same username or email)
	}
	if err != nil {
		return out, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Repo.CreateStaff(ctx, write.NewStaff{UserID: userID, Username: username, Email: email, DisplayName: name, Locale: in.Locale,
			TenantID: &tenant, Issuer: h.LocalIssuer, Subject: kcID}); err != nil {
			return err
		}
		for _, r := range in.Roles {
			if _, err := h.Roles.Grant(ctx, GrantUserRoleInput{Route: RouteTenant, UserID: userID, RoleID: r.RoleID, ScopeType: r.ScopeType,
				ScopeID: r.ScopeID, Reason: "initial role"}); err != nil {
				return err
			}
		}
		t, uid := tenant, userID.String()
		if err := h.Events.Publish(ctx, write.Event{Type: "user.provisioned", SubjectType: "user", SubjectID: uid, TenantID: &t,
			Payload: map[string]any{"user_id": uid, "user_type": "staff", "source": "manual"}}); err != nil {
			return err
		}
		created, err := h.Repo.Staff(ctx, userID, tenant, h.LocalIssuer) // version after the initial grants
		if err != nil {
			return err
		}
		out = *created
		if err := h.Audit.Record(ctx, audit.Event{Action: "user.created", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
			Metadata: map[string]any{"user_type": "staff", "roles": len(in.Roles), "actor": p.UserID.String()}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, StaffView(out)) // same Idempotency-Key → the stored response (R10)
	})
	if err != nil {
		_ = h.KC.DisableUser(context.WithoutCancel(ctx), kcID)
		return out, err
	}
	// The staff sets a password through the Keycloak action email [Proposed]; a failure is logged, the
	// account exists and the email can be sent again from the portal (password reset).
	if err := h.KC.ExecuteActionsEmail(ctx, kcID, port.KCActionUpdatePassword); err != nil {
		slog.WarnContext(ctx, "staff setup email not sent", "user_id", userID.String(), "error", err)
	}
	return out, nil
}

func orTaken(err error, taken bool, code problem.Code) error {
	if err != nil {
		return err
	}
	if taken {
		return problem.New(code, "")
	}
	return nil
}

var staffStatuses = map[string]bool{"active": true, "locked": true, "deactivated": true}

// Update changes the display name, locale and status of a staff of the session tenant (If-Match). Username
// and email changes need the Keycloak account to follow and are not supported yet [Proposed, OQ F012-1].
func (h Staff) Update(ctx context.Context, in UpdateStaffInput) (out write.StaffRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.UpdateTenantUser")
	defer func() { done(err) }()
	ctx, p, tenant, err := h.session(ctx)
	if err != nil {
		return out, err
	}
	if in.Username != nil || in.Email != nil {
		return out, problem.Invalid("username", problem.SubInvalid, "username and email cannot be changed yet")
	}
	if in.Status != nil && !staffStatuses[*in.Status] {
		return out, problem.Invalid("status", problem.SubInvalid, "active, locked or deactivated")
	}
	if in.DisplayName != nil {
		name, err := user.CheckDisplayName(*in.DisplayName)
		if err != nil {
			return out, problem.Invalid("display_name", problem.SubInvalid, "1 to 128 characters")
		}
		in.DisplayName = &name
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := h.Repo.Staff(ctx, in.UserID, tenant, h.LocalIssuer)
		if err != nil {
			return err
		}
		if row.Version != in.IfMatch {
			return problem.New(problem.VersionMismatch, "")
		}
		from := row.Status
		if in.DisplayName != nil {
			row.DisplayName = *in.DisplayName
		}
		if in.Locale != nil {
			row.Locale = in.Locale
		}
		if in.Status != nil {
			row.Status = *in.Status
		}
		if err := h.Repo.UpdateStaff(ctx, row); err != nil {
			return err
		}
		uid := row.UserID.String()
		if row.Status != from {
			t := tenant
			if err := h.Events.Publish(ctx, write.Event{Type: "user.status_changed", SubjectType: "user", SubjectID: uid, TenantID: &t,
				Payload: map[string]any{"user_id": uid, "from": from, "to": row.Status, "reason": "manual"}}); err != nil {
				return err
			}
			if row.Status != "active" {
				if err := h.Events.Publish(ctx, write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: uid, TenantID: &t,
					Payload: map[string]any{"user_id": uid, "reason": row.Status}}); err != nil {
					return err
				}
			}
		}
		out = *row
		return h.Audit.Record(ctx, audit.Event{Action: "user.updated", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
			Metadata: map[string]any{"status_from": from, "status_to": row.Status, "actor": p.UserID.String(),
				"display_name_changed": in.DisplayName != nil, "locale_changed": in.Locale != nil}})
	})
	return out, err
}

// StaffView is the User response of createTenantUser (stored for idempotent replays and written by the handler).
func StaffView(s write.StaffRow) read.UserView {
	v := read.UserView{UserID: s.UserID, UserType: "staff", PrimaryTenantID: s.PrimaryTenantID, Status: s.Status, Username: s.Username,
		Email: s.Email, DisplayName: s.DisplayName, Source: "manual", UserRoles: []read.UserRoleView{}, Version: s.Version}
	if s.Locale != nil {
		v.Locale = *s.Locale
	}
	return v
}
