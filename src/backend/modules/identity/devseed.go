package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/keycloak"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/seed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/csvreader"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/devseed"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DevSeedDeps are the resources of the dev seed (modules/identity/cmd/seed and its integration test).
type DevSeedDeps struct {
	UoW      *entx.UnitOfWork
	Read     *entx.ReadSession
	Writer   *outbox.Writer
	Recorder audit.Recorder
	// KC is the Keycloak Admin adapter of the module (client svc-identity); the seed wraps it in
	// keycloak.DevAdopter so the demo users of the realm import are adopted, never created or rewritten.
	KC      port.KeycloakAdmin
	Issuers []string // EW_OIDC_ISSUERS: the first one is the local issuer of the tokens the binaries verify
	IDHash  []byte   // EW_SIS_ID_HASH_KEY (optional, as in the binaries)
	Clock   func() time.Time
}

// NewDevSeed builds the dev seed on the use cases of module identity, wired like the binaries (wire.go).
func NewDevSeed(d DevSeedDeps) (devseed.Seeder, error) {
	if d.UoW == nil || d.Read == nil || d.Writer == nil || d.Recorder == nil || d.KC == nil || len(d.Issuers) == 0 {
		return devseed.Seeder{}, errors.New("identity seed: database, outbox, Keycloak and EW_OIDC_ISSUERS are required")
	}
	local := d.Issuers[0]
	kc := keycloak.DevAdopter{KeycloakAdmin: d.KC}
	events, auditor := write.Publisher{W: d.Writer}, write.Auditor{R: d.Recorder}
	tenants := command.TenantDeps{UoW: d.UoW, Tenants: write.TenantRepo{}, Events: events, Audit: auditor}
	schools := command.SchoolDeps{UoW: d.UoW, Tenants: write.TenantRepo{}, Schools: write.SchoolRepo{}, Catalog: write.CatalogRepo{},
		Events: events, Audit: auditor}
	roles := command.UserRoles{UoW: d.UoW, Repo: write.UserRoleRepo{}, Events: events, Audit: auditor, Clock: d.Clock}
	validator, err := contract.New()
	if err != nil {
		return devseed.Seeder{}, err
	}
	run := command.SisRun{UoW: d.UoW, Savepoints: d.UoW, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Rules: write.SisSyncRepo{}, Events: events, Audit: auditor, Observer: metrics.Runs{}, History: write.LifecycleRepo{}, IDHashKey: d.IDHash}
	resolve := query.ResolvePrincipal{Model: read.PrincipalModel{Session: d.Read}}
	return devseed.Seeder{
		Users: kc,
		Lookup: command.SeedLookup{UoW: d.UoW, Repo: write.SeedLookupRepo{}, Accounts: write.PlatformStaffRepo{}, Schools: write.SisConnectionRepo{},
			Issuer: local},
		Platform: command.PlatformStaffEnsure{UoW: d.UoW, Repo: write.PlatformStaffRepo{}, Roles: roles, LocalIssuer: local,
			Issuers: d.Issuers},
		CreateTenant: command.CreateTenant{TenantDeps: tenants},
		CreateSchool: command.CreateAdminTenantSchool{SchoolDeps: schools},
		Connections:  command.SisConnections{UoW: d.UoW, Connections: write.SisConnectionRepo{}, Audit: auditor},
		Import:       run,
		Parser:       csvreader.Parser{V: validator},
		Provision: command.ProvisionStudentAccount{UoW: d.UoW, KC: kc, Accounts: write.SisSyncRepo{}, Sync: write.SisSyncRepo{}, Events: events,
			Audit: auditor, LocalIssuer: local, Issuers: d.Issuers},
		Staff: command.Staff{UoW: d.UoW, Repo: write.StaffRepo{}, Roles: roles, KC: kc, Events: events, Audit: auditor,
			LocalIssuer: local},
		Roles:     roles,
		Lifecycle: command.UserLifecycle{UoW: d.UoW, Repo: write.LifecycleRepo{}, Events: events, Audit: auditor},
		Principal: func(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) (authz.Principal, error) {
			p, err := resolve.Handle(ctx, query.ResolvePrincipalInput{UserID: &userID, TenantID: tenantID})
			if err != nil {
				return authz.Principal{}, err
			}
			return portalPrincipal(p)
		},
		RoleID: seed.RoleID,
		Clock:  d.Clock,
	}, nil
}

// portalPrincipal builds the Principal a portal session of the user would carry (portal client), the
// way the principal middleware maps resolvePrincipal (cmd/internal/wiring).
func portalPrincipal(p dto.Principal) (authz.Principal, error) {
	if p.Status != "active" {
		return authz.Principal{}, problem.New(problem.AccountDeactivated, "")
	}
	d := authz.PrincipalData{UserID: p.UserID, UserType: authz.UserType(p.UserType), TenantID: p.TenantID, IsSystemAdmin: p.IsSystemAdmin,
		ClientID: "eduworld-portal", Channel: authz.ChannelPortal}
	if p.TenantType != nil {
		d.TenantType = authz.TenantType(*p.TenantType)
	}
	for _, g := range p.Grants {
		gr := authz.Grant{Role: g.RoleCode, ScopeKind: authz.RoleScopeKind(g.RoleScopeKind), SchoolID: g.SchoolID, SubjectID: g.SubjectID}
		for _, perm := range g.Permissions {
			if pp := authz.Permission(perm); pp.Known() {
				gr.Permissions = append(gr.Permissions, pp)
			}
		}
		d.Grants = append(d.Grants, gr)
	}
	return authz.NewPrincipal(d), nil
}
