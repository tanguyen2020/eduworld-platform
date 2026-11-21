package command

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	domainauthz "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/portalauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// KCRealmRolePlatformStaff is the Keycloak realm role of platform staff: the portal flow admits it and module
// identity bounds its session age (KEYCLOAK_SSO §3.3, ADR-0094 §2). No OTP since ADR-0094.
const KCRealmRolePlatformStaff = portalauth.RealmRolePlatformStaff

// CreatePlatformStaffInput is createPlatformUser (F059-1).
type CreatePlatformStaffInput struct {
	Username, Email, DisplayName, Locale string
	RoleIDs                              []uuid.UUID
	Reason                               string
}

// PlatformStaffAdmin is the Keycloak part of platform staff creation.
type PlatformStaffAdmin interface {
	StaffAdmin
	AddRealmRoles(ctx context.Context, userID string, roles ...string) error
}

// PlatformStaff creates platform staff accounts (createPlatformUser, F059-1, owner F012): staff without a home
// tenant holding platform roles only (ADR-0081), through the same Keycloak admin path as tenant staff.
type PlatformStaff struct {
	UoW         write.UnitOfWork
	Repo        write.StaffRepository
	Roles       UserRoles
	KC          PlatformStaffAdmin
	Events      write.EventPublisher
	Audit       write.Auditor
	LocalIssuer string
}

func (h PlatformStaff) session(ctx context.Context) (context.Context, authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, err
	}
	if p.TenantID != nil {
		return ctx, p, problem.New(problem.PermissionDenied, "")
	}
	// Creating the account and granting its roles: both permissions up front, before Keycloak is called.
	for _, perm := range []authz.Permission{authz.PermPlatformUserManage, authz.PermPlatformUserRoleWrite} {
		if err := authz.Require(ctx, p, perm, authz.Resource{}); err != nil {
			return ctx, p, err
		}
	}
	sc, err := authz.PlatformScope(p)
	if err != nil {
		return ctx, p, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	return tenancy.WithScope(ctx, sc), p, nil
}

func (h PlatformStaff) validate(in CreatePlatformStaffInput) (username, email, name, reason string, err error) {
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
	if in.Locale != "" && in.Locale != "vi-VN" && in.Locale != "en-US" {
		pe.AddField("locale", problem.SubInvalid, "vi-VN or en-US")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range in.RoleIDs {
		if seen[id] {
			pe.AddField("initial_role_ids", problem.SubInvalid, "duplicate role")
			break
		}
		seen[id] = true
	}
	if len(in.RoleIDs) == 0 || len(in.RoleIDs) > 20 {
		pe.AddField("initial_role_ids", problem.SubInvalid, "1 to 20 platform roles")
	}
	reason = strings.TrimSpace(in.Reason)
	if n := utf8.RuneCountInString(reason); n < 3 || n > 1000 {
		pe.AddField("reason", problem.SubInvalid, "3 to 1000 characters")
	}
	if len(pe.Fields) > 0 {
		return "", "", "", "", pe
	}
	return username, email, name, reason, nil
}

// checkRoles verifies that every initial role is a platform role a platform staff may receive at platform scope
// without further data (safety_moderator needs training_completed_at: granted later by grantPlatformUserRole).
func (h PlatformStaff) checkRoles(ctx context.Context, roleIDs []uuid.UUID) error {
	for _, id := range roleIDs {
		role, err := h.Roles.Repo.Role(ctx, id, nil)
		if err != nil {
			return err
		}
		if role == nil {
			return problem.New(problem.RoleNotFound, "")
		}
		err = domainauthz.CheckGrant(domainauthz.Grant{RoleCode: role.Code, RoleKind: role.ScopeKind, ScopeType: domainauthz.ScopePlatform,
			UserType: "staff", ValidityOrder: true})
		if err != nil {
			msg := "not grantable to a new platform staff"
			for e, f := range grantErrors {
				if errors.Is(err, e) {
					msg = f.msg
				}
			}
			return problem.Invalid("initial_role_ids", problem.SubInvalid, msg)
		}
	}
	return nil
}

// PlatformStaffView is the User response of createPlatformUser (also the stored idempotent response): no home
// tenant, no user role list (listAllUserRoles?user_id= gives them), never a secret.
func PlatformStaffView(s write.StaffRow) read.UserView {
	v := read.UserView{UserID: s.UserID, UserType: "staff", Status: s.Status, Username: s.Username, Email: s.Email, DisplayName: s.DisplayName,
		Source: "manual", UserRoles: []read.UserRoleView{}, Version: s.Version}
	if s.Locale != nil {
		v.Locale = *s.Locale
	}
	return v
}

// Create creates the Keycloak user (realm role platform_staff, password only — ADR-0094; no password yet, the
// UPDATE_PASSWORD action email sets it), the staff account without a home tenant and its initial platform roles in
// one use case; a database failure disables the Keycloak user it created (same compensation as createTenantUser).
func (h PlatformStaff) Create(ctx context.Context, in CreatePlatformStaffInput) (out write.StaffRow, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreatePlatformUser")
	defer func() { done(err) }()
	ctx, p, err := h.session(ctx)
	if err != nil {
		return out, err
	}
	username, email, name, reason, err := h.validate(in)
	if err != nil {
		return out, err
	}
	if err := h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.checkRoles(ctx, in.RoleIDs); err != nil {
			return err
		}
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
		Attributes: map[string][]string{"eduworld_user_id": {userID.String()}}})
	if errors.Is(err, port.ErrKCConflict) {
		return out, problem.New(problem.UsernameTaken, "") // the account exists on the SSO (same username or email)
	}
	if err != nil {
		return out, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	compensate := func() { _ = h.KC.DisableUser(context.WithoutCancel(ctx), kcID) }
	if err := h.KC.AddRealmRoles(ctx, kcID, KCRealmRolePlatformStaff); err != nil {
		compensate()
		return out, problem.New(problem.Unavailable, "").Retry().Wrap(err)
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Repo.CreateStaff(ctx, write.NewStaff{UserID: userID, Username: username, Email: email, DisplayName: name, Locale: in.Locale,
			Issuer: h.LocalIssuer, Subject: kcID}); err != nil {
			return err
		}
		for _, id := range in.RoleIDs {
			if _, err := h.Roles.Grant(ctx, GrantUserRoleInput{Route: RoutePlatform, UserID: userID, RoleID: id,
				ScopeType: domainauthz.ScopePlatform, Reason: "initial role"}); err != nil {
				return err
			}
		}
		uid := userID.String()
		if err := h.Events.Publish(ctx, write.Event{Type: "user.provisioned", SubjectType: "user", SubjectID: uid, Platform: true,
			Payload: map[string]any{"user_id": uid, "user_type": "staff", "source": "manual"}}); err != nil {
			return err
		}
		created, err := h.Repo.PlatformStaff(ctx, userID, h.LocalIssuer) // version after the initial grants
		if err != nil {
			return err
		}
		out = *created
		if err := h.Audit.Record(ctx, audit.Event{Action: "user.created", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
			Reason: reason, Metadata: map[string]any{"user_type": "staff", "platform_staff": true, "roles": len(in.RoleIDs), "actor": p.UserID.String()}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, PlatformStaffView(out)) // same Idempotency-Key → the stored response (R10)
	})
	if err != nil {
		compensate()
		return out, err
	}
	if err := h.KC.ExecuteActionsEmail(ctx, kcID, port.KCActionUpdatePassword); err != nil {
		slog.WarnContext(ctx, "platform staff setup email not sent", "user_id", userID.String(), "error", err)
	}
	return out, nil
}
