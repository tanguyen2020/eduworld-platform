package devseed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Reason is the reason recorded by every write of the seed.
const Reason = "dev seed (LOCAL_DEVELOPMENT §5)"

// Use cases called by the seed (typed interfaces, no mediator: ADR-0087).
type (
	// Users finds a user of the local realm by username (Keycloak Admin port).
	Users interface {
		FindUserByUsername(ctx context.Context, username string) (port.KCUser, error)
	}
	// Lookup answers whether a row exists (command.SeedLookup).
	Lookup interface {
		Tenant(ctx context.Context, code string) (*uuid.UUID, error)
		School(ctx context.Context, tenantID uuid.UUID, code string) (*uuid.UUID, error)
		CsvConnection(ctx context.Context, tenantID uuid.UUID) (*uuid.UUID, error)
		Account(ctx context.Context, username string) (*write.SeedAccount, error)
		HasGrant(ctx context.Context, k write.RoleGrantKey) (bool, error)
		ConnectionSchools(ctx context.Context, tenantID, connectionID uuid.UUID) ([]write.SisConnectionSchool, error)
	}
	// PlatformStaff is command.PlatformStaffEnsure.
	PlatformStaff interface {
		Ensure(ctx context.Context, in command.EnsurePlatformStaffInput) (command.EnsurePlatformStaffResult, error)
	}
	// SisConnections is command.SisConnections.
	SisConnections interface {
		Create(ctx context.Context, tenantID uuid.UUID, providerKind, reason string) (write.SisConnection, error)
		SetSchools(ctx context.Context, id uuid.UUID, schoolIDs []uuid.UUID, reason string) ([]write.SisConnectionSchool, error)
	}
	// SisImport is the csv_import entry of command.SisRun (the one of the import job).
	SisImport interface {
		RunImportSystem(ctx context.Context, tenantID uuid.UUID, in command.RunImportInput) (command.RunImportResult, error)
	}
	// StaffCreator is command.Staff.
	StaffCreator interface {
		Create(ctx context.Context, in command.CreateStaffInput) (write.StaffRow, error)
	}
	// RoleGranter is command.UserRoles.
	RoleGranter interface {
		Grant(ctx context.Context, in command.GrantUserRoleInput) (write.UserRoleRow, error)
	}
	// Lifecycle is command.UserLifecycle.
	Lifecycle interface {
		Change(ctx context.Context, route, action string, userID uuid.UUID, reason string) error
	}
	// PrincipalOf resolves the Principal of a user (in a tenant, or platform when tenantID is nil) through the
	// resolvePrincipal read model, as the portal session of that user would get it (portal client, MFA).
	PrincipalOf func(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) (authz.Principal, error)
)

// Seeder runs a plan.
type Seeder struct {
	Users        Users
	Lookup       Lookup
	Platform     PlatformStaff
	CreateTenant command.CreateTenantHandler
	CreateSchool command.CreateAdminTenantSchoolHandler
	Connections  SisConnections
	Import       SisImport
	Parser       port.SisFileParser
	Provision    command.ProvisionStudentAccountHandler
	Staff        StaffCreator
	Roles        RoleGranter
	Lifecycle    Lifecycle
	Principal    PrincipalOf
	RoleID       func(code string) uuid.UUID // id of a system role (deterministic, cmd/migrate seed)
	Clock        func() time.Time
}

// Report counts what a run created; a second run reports zeros.
type Report struct {
	Tenants, Schools, Connections, Imports, Students, PlatformStaff, TenantStaff, Grants, Locked int
}

// Changed reports whether the run wrote anything.
func (r Report) Changed() bool { return r != Report{} }

func (s Seeder) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

type run struct {
	Seeder
	plan    Plan
	rep     Report
	admin   uuid.UUID
	tenants map[string]uuid.UUID
	schools map[string]uuid.UUID // school code → id
	actAs   map[string]context.Context
}

// Run applies the plan: bootstrap System Admin → platform staff → tenants and schools → SIS csv import and student
// accounts → tenant staff and roles → locked students.
func (s Seeder) Run(ctx context.Context, plan Plan) (Report, error) {
	r := &run{Seeder: s, plan: plan, tenants: map[string]uuid.UUID{}, schools: map[string]uuid.UUID{}, actAs: map[string]context.Context{}}
	for _, step := range []struct {
		name string
		fn   func(context.Context) error
	}{{"platform staff", r.platformStaff}, {"tenants", r.orgs}, {"sis", r.students}, {"tenant staff", r.tenantStaff}, {"locked", r.locked}} {
		if err := step.fn(ctx); err != nil {
			return r.rep, fmt.Errorf("identity seed: %s: %w", step.name, err)
		}
	}
	return r.rep, nil
}

// as returns ctx with the Principal of the System Admin, in tenant (code) or on the platform ("").
func (r *run) as(ctx context.Context, tenant string) (context.Context, error) {
	if c, ok := r.actAs[tenant]; ok {
		return c, nil
	}
	var tid *uuid.UUID
	if tenant != "" {
		id := r.tenants[tenant]
		tid = &id
	}
	p, err := r.Principal(ctx, r.admin, tid)
	if err != nil {
		return ctx, fmt.Errorf("principal of the System Admin (%q): %w", tenant, err)
	}
	c := authz.WithPrincipal(ctx, p)
	r.actAs[tenant] = c
	return c, nil
}

func (r *run) subject(ctx context.Context, username string) (string, error) {
	u, err := r.Users.FindUserByUsername(ctx, username)
	if errors.Is(err, port.ErrKCNotFound) {
		return "", fmt.Errorf("%s is not a user of the local realm: run make keycloak-import first", username)
	}
	return u.ID, err
}

func (r *run) platformStaff(ctx context.Context) error {
	for i, st := range r.plan.PlatformStaff {
		sub, err := r.subject(ctx, st.Username)
		if err != nil {
			return err
		}
		in := command.EnsurePlatformStaffInput{Username: st.Username, Email: st.Email, DisplayName: st.DisplayName, Subject: sub, Reason: Reason}
		for _, role := range st.Roles {
			pr := command.PlatformRoleInput{RoleID: r.RoleID(role.Role)}
			if role.Training {
				t := r.now()
				pr.Training = &t
			}
			in.Roles = append(in.Roles, pr)
		}
		actx := ctx // the first entry bootstraps the System Admin (no Principal); the others are granted by it
		if i > 0 {
			if actx, err = r.as(ctx, ""); err != nil {
				return err
			}
		}
		res, err := r.Platform.Ensure(actx, in)
		if err != nil {
			return fmt.Errorf("%s: %w", st.Username, err)
		}
		if i == 0 {
			r.admin = res.UserID
		}
		if res.Created {
			r.rep.PlatformStaff++
		}
		r.rep.Grants += res.Granted
	}
	return nil
}

func (r *run) orgs(ctx context.Context) error {
	sa, err := r.as(ctx, "")
	if err != nil {
		return err
	}
	for _, t := range r.plan.Tenants {
		id, err := r.Lookup.Tenant(ctx, t.Code)
		if err != nil {
			return err
		}
		if id == nil {
			created, err := r.CreateTenant.Handle(sa, dto.CreateTenant{Code: t.Code, Name: t.Name, TenantType: t.Type, Alias: t.Alias, Reason: Reason})
			if err != nil {
				return fmt.Errorf("tenant %s: %w", t.Code, err)
			}
			id = &created.TenantID
			r.rep.Tenants++
		}
		r.tenants[t.Code] = *id
		tctx, err := r.as(ctx, t.Code)
		if err != nil {
			return err
		}
		for _, s := range t.Schools {
			sid, err := r.Lookup.School(ctx, *id, s.Code)
			if err != nil {
				return err
			}
			if sid == nil {
				created, err := r.CreateSchool.Handle(tctx, dto.CreateSchool{TenantID: *id, Code: s.Code, Name: dto.LocalizedName{Vi: s.NameVi, En: s.NameEn},
					ProvinceCode: s.Province, SchoolLevel: s.Level, GradeMin: s.GradeMin, GradeMax: s.GradeMax, Reason: Reason})
				if err != nil {
					return fmt.Errorf("school %s: %w", s.Code, err)
				}
				sid = &created.SchoolID
				r.rep.Schools++
			}
			r.schools[s.Code] = *sid
		}
	}
	return nil
}

// students imports the SIS CSV files of every school through a csv_import connection of its tenant, then links each
// student account to the realm-import user with the same username (provisioning use case of the SIS pipeline).
func (r *run) students(ctx context.Context) error {
	for _, t := range r.plan.Tenants {
		var withSis []SchoolPlan
		for _, s := range t.Schools {
			if s.Sis != "" {
				withSis = append(withSis, s)
			}
		}
		if len(withSis) == 0 {
			continue
		}
		tid := r.tenants[t.Code]
		tctx, err := r.as(ctx, t.Code)
		if err != nil {
			return err
		}
		conn, err := r.connection(ctx, tctx, tid, withSis)
		if err != nil {
			return fmt.Errorf("tenant %s: %w", t.Code, err)
		}
		for _, s := range withSis {
			if err := r.importSchool(ctx, tid, conn, s); err != nil {
				return fmt.Errorf("school %s: %w", s.Code, err)
			}
		}
	}
	return nil
}

func (r *run) connection(ctx, tctx context.Context, tenantID uuid.UUID, schools []SchoolPlan) (uuid.UUID, error) {
	id, err := r.Lookup.CsvConnection(ctx, tenantID)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil {
		c, err := r.Connections.Create(tctx, tenantID, "csv_import", Reason)
		if err != nil {
			return uuid.Nil, err
		}
		id = &c.ID
		r.rep.Connections++
	}
	assigned, err := r.Lookup.ConnectionSchools(ctx, tenantID, *id)
	if err != nil {
		return uuid.Nil, err
	}
	want := make([]uuid.UUID, 0, len(schools))
	missing := false
	for _, s := range schools {
		sid := r.schools[s.Code]
		want = append(want, sid)
		if !slices.ContainsFunc(assigned, func(a write.SisConnectionSchool) bool { return a.SchoolID == sid && a.Active }) {
			missing = true
		}
	}
	if missing {
		if _, err := r.Connections.SetSchools(tctx, *id, want, Reason); err != nil {
			return uuid.Nil, err
		}
	}
	return *id, nil
}

// importSchool runs the csv_import of a school unless every student of its file already has an account, then
// provisions the accounts not linked to the local realm yet.
func (r *run) importSchool(ctx context.Context, tenantID, conn uuid.UUID, s SchoolPlan) error {
	files, usernames, err := r.read(r.plan.SisDir(s))
	if err != nil {
		return err
	}
	accounts := map[string]*write.SeedAccount{}
	pending := false
	for _, u := range usernames {
		a, err := r.Lookup.Account(ctx, u)
		if err != nil {
			return err
		}
		accounts[u], pending = a, pending || a == nil
	}
	if pending {
		res, err := r.Import.RunImportSystem(ctx, tenantID, command.RunImportInput{ConnectionID: conn, SchoolID: r.schools[s.Code], Files: files})
		if err != nil {
			return err
		}
		if res.Status != "succeeded" || len(res.Errors) > 0 || res.Issues > 0 {
			return fmt.Errorf("csv import %s: status %s, %d record errors, %d reconciliation issues (see sis.sync_runs)", res.RunID, res.Status,
				len(res.Errors), res.Issues)
		}
		r.rep.Imports++
	}
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return err
	}
	ectx := tenancy.WithScope(ctx, sc) // the scope of the consumer keycloak-provisioner
	for _, u := range usernames {
		a := accounts[u]
		if a == nil {
			if a, err = r.Lookup.Account(ctx, u); err != nil {
				return err
			}
		}
		if a == nil || a.UserType != "student" {
			return fmt.Errorf("%s: no student account after the import", u)
		}
		if a.Subject != nil {
			continue
		}
		if err := r.Provision.Handle(ectx, a.UserID); err != nil {
			return fmt.Errorf("provision %s: %w", u, err)
		}
		r.rep.Students++
	}
	return nil
}

// read parses the CSV files of a school directory with the contract parser of the import job and returns the
// login usernames of its students.
func (r *run) read(dir string) ([]command.SisFile, []string, error) {
	var files []command.SisFile
	var usernames []string
	for _, kind := range SisFiles {
		f, err := os.Open(filepath.Join(dir, kind+".csv")) //nolint:gosec // seed directory of the repository
		if err != nil {
			return nil, nil, err
		}
		pf, err := r.Parser.Parse(kind, f, r.now())
		_ = f.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("%s.csv: %w", kind, err)
		}
		if len(pf.Violations) > 0 {
			v := pf.Violations[0]
			return nil, nil, fmt.Errorf("%s.csv: %d violations, first: row %d %s %s", kind, len(pf.Violations), v.Row, v.Column, v.Code)
		}
		sf := command.SisFile{Kind: pf.Kind}
		for _, rec := range pf.Records {
			sf.Records = append(sf.Records, command.SisRecord{Row: rec.Row, Data: rec.Data})
			if pf.Kind == sis.KindStudent {
				usernames = append(usernames, sis.LoginUsername(rec.Data))
			}
		}
		files = append(files, sf)
	}
	return files, usernames, nil
}

func (r *run) tenantStaff(ctx context.Context) error {
	for _, st := range r.plan.TenantStaff {
		home := r.tenants[st.Tenant]
		hctx, err := r.as(ctx, st.Tenant)
		if err != nil {
			return err
		}
		a, err := r.Lookup.Account(ctx, st.Username)
		if err != nil {
			return err
		}
		var userID uuid.UUID
		switch {
		case a == nil:
			var initial []command.InitialRole
			for _, role := range st.Roles {
				if role.Tenant == "" || role.Tenant == st.Tenant {
					initial = append(initial, r.initial(role))
				}
			}
			row, err := r.Staff.Create(hctx, command.CreateStaffInput{Username: st.Username, Email: st.Email, DisplayName: st.DisplayName,
				Roles: initial})
			if err != nil {
				return fmt.Errorf("%s: %w", st.Username, err)
			}
			userID = row.UserID
			r.rep.TenantStaff++
			r.rep.Grants += len(initial)
		case a.UserType != "staff" || a.PrimaryTenantID == nil || *a.PrimaryTenantID != home:
			return fmt.Errorf("%s: the account exists but is not a staff of %s", st.Username, st.Tenant)
		default:
			userID = a.UserID
		}
		for _, role := range st.Roles {
			if err := r.grant(ctx, st, userID, role); err != nil {
				return fmt.Errorf("%s %s: %w", st.Username, role.Role, err)
			}
		}
	}
	return nil
}

func (r *run) initial(role RolePlan) command.InitialRole {
	in := command.InitialRole{RoleID: r.RoleID(role.Role), ScopeType: role.ScopeOf()}
	if role.School != "" {
		id := r.schools[role.School]
		in.ScopeID = &id
	}
	return in
}

func (r *run) grant(ctx context.Context, st StaffPlan, userID uuid.UUID, role RolePlan) error {
	tenant := role.Tenant
	if tenant == "" {
		tenant = st.Tenant
	}
	tid := r.tenants[tenant]
	in := r.initial(role)
	held, err := r.Lookup.HasGrant(ctx, write.RoleGrantKey{UserID: userID, RoleID: in.RoleID, TenantID: &tid, ScopeType: in.ScopeType, ScopeID: in.ScopeID})
	if err != nil || held {
		return err
	}
	tctx, err := r.as(ctx, tenant)
	if err != nil {
		return err
	}
	if _, err := r.Roles.Grant(tctx, command.GrantUserRoleInput{Route: command.RouteTenant, UserID: userID, RoleID: in.RoleID, ScopeType: in.ScopeType,
		ScopeID: in.ScopeID, Reason: Reason}); err != nil {
		return err
	}
	r.rep.Grants++
	return nil
}

func (r *run) locked(ctx context.Context) error {
	for _, l := range r.plan.LockedStudents {
		a, err := r.Lookup.Account(ctx, l.Username)
		if err != nil {
			return err
		}
		if a == nil || a.UserType != "student" {
			return fmt.Errorf("%s: no student account to lock", l.Username)
		}
		if a.Status == "locked" {
			continue
		}
		tctx, err := r.as(ctx, l.Tenant)
		if err != nil {
			return err
		}
		if err := r.Lifecycle.Change(tctx, command.LifecycleRouteTenant, command.LifecycleLock, a.UserID, Reason); err != nil {
			return fmt.Errorf("lock %s: %w", l.Username, err)
		}
		r.rep.Locked++
	}
	return nil
}

// LogReport logs the outcome of a run.
func LogReport(ctx context.Context, rep Report) {
	slog.InfoContext(ctx, "identity seed done", "tenants", rep.Tenants, "schools", rep.Schools, "sis_connections", rep.Connections,
		"csv_imports", rep.Imports, "students_linked", rep.Students, "platform_staff", rep.PlatformStaff, "tenant_staff", rep.TenantStaff,
		"grants", rep.Grants, "locked", rep.Locked, "changed", rep.Changed())
}
