package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	domainauthz "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// BootstrapJob is the name of the system component that creates the first System Admin.
const BootstrapJob = "identity-bootstrap"

// PlatformRoleInput is one platform role of EnsurePlatformStaff.
type PlatformRoleInput struct {
	RoleID   uuid.UUID
	Training *time.Time // training_completed_at (safety_moderator)
}

// EnsurePlatformStaffInput is a platform staff account linked to an existing subject of the local realm.
type EnsurePlatformStaffInput struct {
	Username, Email, DisplayName string
	Subject                      string // Keycloak user id of the local realm (LocalIssuer)
	Roles                        []PlatformRoleInput
	Reason                       string
}

// EnsurePlatformStaffResult tells what Ensure changed.
type EnsurePlatformStaffResult struct {
	UserID  uuid.UUID
	Created bool
	Granted int
}

// PlatformStaffEnsure provisions platform staff accounts (user_type staff, no home tenant, ADR-0081) that already exist on
// the local realm and grants them platform roles; running it again changes nothing [Proposed, F010 Clarifications
// "seed identity"]; the dev seed is its only caller. Unlike createPlatformUser (PlatformStaff.Create, F059) it links an
// existing subject instead of creating a Keycloak user, grants safety_moderator with its training date and bootstraps
// the first System Admin, which no API can create.
//
//   - With a Principal in ctx: the caller needs platform.user_role.write as platform staff; roles go through
//     UserRoles.Grant (route platform), so audit, events and the grant rules are those of grantPlatformUserRole.
//   - Without a Principal: bootstrap of the first System Admin (system component identity-bootstrap). Allowed only
//     while no live system_admin grant exists and only for the role system_admin; the grant names the new account as
//     its own grantor. Once a System Admin exists the call is a no-op for that same account and a denial otherwise.
type PlatformStaffEnsure struct {
	UoW         write.UnitOfWork
	Repo        write.PlatformStaffRepository
	Roles       UserRoles
	LocalIssuer string
	Issuers     []string
}

type platformStaffRow struct {
	username, email, name string
	subject               string
}

func (h PlatformStaffEnsure) validate(in EnsurePlatformStaffInput) (platformStaffRow, error) {
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
	key, kerr := user.NewExternalIdentityKey(h.Issuers, h.LocalIssuer, in.Subject)
	if kerr != nil {
		pe.AddField("subject", problem.SubInvalid, "a subject of the local issuer (EW_OIDC_ISSUERS) is required")
	}
	if len(in.Roles) == 0 || len(in.Roles) > 20 {
		pe.AddField("roles", problem.SubInvalid, "1 to 20 roles")
	}
	if len(pe.Fields) > 0 {
		return platformStaffRow{}, pe
	}
	return platformStaffRow{username: username, email: email, name: name, subject: key.Subject}, nil
}

// Ensure creates the account when missing and grants the roles it does not hold yet.
func (h PlatformStaffEnsure) Ensure(ctx context.Context, in EnsurePlatformStaffInput) (out EnsurePlatformStaffResult, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.EnsurePlatformStaff")
	defer func() { done(err) }()
	row, err := h.validate(in)
	if err != nil {
		return out, err
	}
	p, acting := authz.PrincipalFrom(ctx)
	var sc tenancy.Scope
	if acting {
		if err := authz.Require(ctx, p, authz.PermPlatformUserRoleWrite, authz.Resource{}); err != nil {
			return out, err
		}
		ps, err := authz.PlatformScope(p)
		if err != nil {
			return out, problem.New(problem.PermissionDenied, "").Wrap(err)
		}
		sc = ps
	} else {
		ps, err := authz.SystemPlatformScope(BootstrapJob)
		if err != nil {
			return out, err
		}
		sc = ps
	}
	sctx := tenancy.WithScope(ctx, sc)
	var missing []PlatformRoleInput
	err = h.UoW.Do(sctx, func(ctx context.Context) error {
		acct, err := h.Repo.AccountByUsername(ctx, row.username, h.LocalIssuer)
		if err != nil {
			return err
		}
		if acct != nil {
			if acct.UserType != "staff" || acct.PrimaryTenantID != nil {
				return problem.Invalid("username", problem.SubInvalid, "the username belongs to a student or a tenant staff (ADR-0081)")
			}
			if acct.Subject == nil || *acct.Subject != row.subject {
				return problem.New(problem.IdentityConflict, "the account is linked to another subject of the local realm")
			}
			out.UserID = acct.UserID
		}
		if out.UserID != uuid.Nil {
			for _, r := range in.Roles {
				held, err := h.Repo.HasGrant(ctx, write.RoleGrantKey{UserID: out.UserID, RoleID: r.RoleID, ScopeType: domainauthz.ScopePlatform})
				if err != nil {
					return err
				}
				if !held {
					missing = append(missing, r)
				}
			}
		} else {
			missing = in.Roles
		}
		if acting {
			return h.create(ctx, acct, row, &out, p.UserID.String())
		}
		return h.bootstrap(ctx, acct, row, missing, &out, in.Reason)
	})
	if err != nil || !acting {
		return out, err
	}
	for _, r := range missing {
		if _, err := h.Roles.Grant(ctx, GrantUserRoleInput{Route: RoutePlatform, UserID: out.UserID, RoleID: r.RoleID,
			ScopeType: domainauthz.ScopePlatform, Training: r.Training, Reason: in.Reason}); err != nil {
			return out, err
		}
		out.Granted++
	}
	return out, nil
}

// create inserts the account when it does not exist (user.provisioned + audit user.created).
func (h PlatformStaffEnsure) create(ctx context.Context, acct *write.SeedAccount, row platformStaffRow, out *EnsurePlatformStaffResult, actor string) error {
	if acct != nil {
		return nil
	}
	out.UserID, out.Created = ids.New(), true
	if err := h.Repo.CreatePlatformStaff(ctx, write.NewPlatformStaff{UserID: out.UserID, Username: row.username, Email: row.email,
		DisplayName: row.name, Issuer: h.LocalIssuer, Subject: row.subject}); err != nil {
		return err
	}
	uid := out.UserID.String()
	if err := h.Roles.Events.Publish(ctx, write.Event{Type: "user.provisioned", SubjectType: "user", SubjectID: uid, Platform: true,
		Payload: map[string]any{"user_id": uid, "user_type": "staff", "source": "manual"}}); err != nil {
		return err
	}
	return h.Roles.Audit.Record(ctx, audit.Event{Action: "user.created", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
		Metadata: map[string]any{"user_type": "staff", "platform": true, "actor": actor}})
}

// bootstrap creates the first System Admin; afterwards it only accepts that account with roles it already holds.
func (h PlatformStaffEnsure) bootstrap(ctx context.Context, acct *write.SeedAccount, row platformStaffRow, missing []PlatformRoleInput,
	out *EnsurePlatformStaffResult, reason string) error {
	if len(missing) == 0 {
		return nil // the account exists with every role: nothing to do
	}
	exists, err := h.Repo.SystemAdminExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return problem.New(problem.PermissionDenied, "a System Admin exists: platform staff are granted by a System Admin")
	}
	roles := make([]*write.RoleInfo, 0, len(missing))
	for _, r := range missing {
		info, err := h.Roles.Repo.Role(ctx, r.RoleID, nil)
		if err != nil {
			return err
		}
		if info == nil {
			return problem.New(problem.RoleNotFound, "")
		}
		if info.Code != "system_admin" || info.ScopeKind != domainauthz.KindPlatform {
			return problem.Invalid("roles", problem.SubInvalid, "the bootstrap grants system_admin only")
		}
		roles = append(roles, info)
	}
	if err := h.create(ctx, acct, row, out, "system:"+BootstrapJob); err != nil {
		return err
	}
	now := h.Roles.now()
	for _, info := range roles {
		g := write.UserRoleRow{ID: ids.New(), UserID: out.UserID, RoleID: info.ID, RoleCode: info.Code, ScopeType: domainauthz.ScopePlatform,
			ValidFrom: now, Source: "manual", GrantedBy: out.UserID}
		if err := h.Roles.Repo.Insert(ctx, &g); err != nil {
			return err
		}
		if err := h.Roles.Repo.BumpUserVersion(ctx, out.UserID); err != nil {
			return err
		}
		if err := h.Roles.changed(ctx, g, "granted", out.UserID, reason); err != nil {
			return err
		}
		out.Granted++
	}
	return nil
}
