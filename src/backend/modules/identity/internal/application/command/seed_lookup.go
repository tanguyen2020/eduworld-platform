package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SeedJob is the name of the system component of the dev seed (modules/identity/cmd/seed).
const SeedJob = "identity-seed"

// SeedLookup answers the idempotency questions of the dev seed before it calls a use case: does the tenant, school,
// csv connection, account or grant already exist? Reads only, in the scope of the row it asks about: platform rows
// under the system platform scope identity-seed, tenant rows under the TenantScope of their tenant (RLS applies).
type SeedLookup struct {
	UoW      write.UnitOfWork
	Repo     write.SeedLookupRepository
	Accounts write.PlatformStaffRepository
	Schools  write.SeedSchoolsRepository
	Issuer   string // local issuer whose subject Account returns
}

func (h SeedLookup) platform(ctx context.Context, fn func(ctx context.Context) error) error {
	sc, err := authz.SystemPlatformScope(SeedJob)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

func (h SeedLookup) tenant(ctx context.Context, tenantID uuid.UUID, fn func(ctx context.Context) error) error {
	sc, err := authz.EventScope(tenantID)
	if err != nil {
		return err
	}
	return h.UoW.Do(tenancy.WithScope(ctx, sc), fn)
}

// Tenant returns the id of the tenant with the code (nil when absent).
func (h SeedLookup) Tenant(ctx context.Context, code string) (id *uuid.UUID, err error) {
	err = h.platform(ctx, func(ctx context.Context) error {
		id, err = h.Repo.TenantByCode(ctx, code)
		return err
	})
	return id, err
}

// School returns the id of the school of the tenant with the code (nil when absent).
func (h SeedLookup) School(ctx context.Context, tenantID uuid.UUID, code string) (id *uuid.UUID, err error) {
	err = h.tenant(ctx, tenantID, func(ctx context.Context) error {
		id, err = h.Repo.SchoolByCode(ctx, tenantID, code)
		return err
	})
	return id, err
}

// CsvConnection returns the csv_import connection of the tenant (nil when absent).
func (h SeedLookup) CsvConnection(ctx context.Context, tenantID uuid.UUID) (id *uuid.UUID, err error) {
	err = h.tenant(ctx, tenantID, func(ctx context.Context) error {
		id, err = h.Repo.CsvConnection(ctx, tenantID)
		return err
	})
	return id, err
}

// Account returns the account with the username and its subject of the local issuer (nil when absent).
func (h SeedLookup) Account(ctx context.Context, username string) (a *write.SeedAccount, err error) {
	err = h.platform(ctx, func(ctx context.Context) error {
		a, err = h.Accounts.AccountByUsername(ctx, username, h.Issuer)
		return err
	})
	return a, err
}

// HasGrant reports whether the grant is active (tenant grants are read in their tenant).
func (h SeedLookup) HasGrant(ctx context.Context, k write.RoleGrantKey) (ok bool, err error) {
	fn := func(ctx context.Context) error {
		ok, err = h.Accounts.HasGrant(ctx, k)
		return err
	}
	if k.TenantID == nil {
		return ok, h.platform(ctx, fn)
	}
	return ok, h.tenant(ctx, *k.TenantID, fn)
}

// ConnectionSchools lists the schools of a csv connection of the tenant.
func (h SeedLookup) ConnectionSchools(ctx context.Context, tenantID, connectionID uuid.UUID) (out []write.SisConnectionSchool, err error) {
	err = h.tenant(ctx, tenantID, func(ctx context.Context) error {
		out, err = h.Schools.Schools(ctx, connectionID)
		return err
	})
	return out, err
}
