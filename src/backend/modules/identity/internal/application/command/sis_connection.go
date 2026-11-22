package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/appscope"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// SisConnections implements the SIS connection operations of F011 (createSisConnection,
// setSisConnectionSchools, listSisConnectionSchools; F011-AC-26). F011 accepts provider csv_import only;
// contract_v1 (config, secret_ref) comes with F017.
type SisConnections struct {
	UoW         write.UnitOfWork
	Connections write.SisConnectionRepository
	Audit       write.Auditor
	Contract    *SisContract // F017: contract_v1 connections (nil = csv_import only)
	Cursors     *page.Codec  // listSisConnectionSchools paging
}

// platformTarget authorizes platform.sis_connection.manage and returns ctx scoped to the target tenant.
func platformTarget(ctx context.Context, target uuid.UUID) (context.Context, authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{}); err != nil {
		return ctx, p, err
	}
	sc, err := appscope.TargetTenant(p, target)
	if err != nil {
		return ctx, p, err
	}
	return tenancy.WithScope(ctx, sc), p, nil
}

// Create creates a csv_import connection of the target tenant (X-Tenant-Id).
func (h SisConnections) Create(ctx context.Context, tenantID uuid.UUID, providerKind, reason string) (out write.SisConnection, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.CreateSisConnection")
	defer func() { done(err) }()
	if ctx, _, err = platformTarget(ctx, tenantID); err != nil {
		return out, err
	}
	if providerKind != "csv_import" {
		return out, problem.Invalid("provider_kind", problem.SubInvalid, "F011 supports csv_import only (contract_v1: F017)")
	}
	c := write.SisConnection{ID: ids.New(), TenantID: tenantID, ProviderKind: providerKind, Status: "active"}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		if err := h.Connections.Create(ctx, &c); err != nil {
			return err
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "sis_connection.created", TargetType: "sis_connection", TargetID: c.ID.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"provider_kind": providerKind}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, c)
	})
	return c, err
}

// connectionScope resolves the tenant of a connection (System Admin bypass, audited) and scopes ctx to it.
func (h SisConnections) connectionScope(ctx context.Context, id uuid.UUID) (context.Context, authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return ctx, p, err
	}
	if err := authz.Require(ctx, p, authz.PermPlatformSisConnectionManage, authz.Resource{}); err != nil {
		return ctx, p, err
	}
	bctx, err := tenancy.Bypass(ctx, tenancy.ReasonSystemAdminCrossTenant)
	if err != nil {
		return ctx, p, err
	}
	var tenant uuid.UUID
	if err := h.UoW.Do(bctx, func(ctx context.Context) error {
		var err error
		tenant, err = h.Connections.TenantOf(ctx, id)
		return err
	}); err != nil {
		return ctx, p, err
	}
	return platformTarget(ctx, tenant)
}

// SetSchools replaces the active schools of a connection (one active connection per school, ADR-0046).
func (h SisConnections) SetSchools(ctx context.Context, id uuid.UUID, schoolIDs []uuid.UUID, reason string) (out []write.SisConnectionSchool, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SetSisConnectionSchools")
	defer func() { done(err) }()
	ctx, p, err := h.connectionScope(ctx, id)
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	for _, s := range schoolIDs {
		if seen[s] {
			return nil, problem.Invalid("school_ids", problem.SubInvalid, "duplicate school")
		}
		seen[s] = true
	}
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := h.Connections.Get(ctx, id)
		if err != nil {
			return err
		}
		if out, err = h.Connections.SetSchools(ctx, c, schoolIDs, p.UserID); err != nil {
			return err
		}
		ss := make([]string, 0, len(schoolIDs))
		for _, s := range schoolIDs {
			ss = append(ss, s.String())
		}
		if err := h.Audit.Record(ctx, audit.Event{Action: "sis_connection.schools_set", TargetType: "sis_connection", TargetID: id.String(),
			Result: audit.ResultSuccess, Reason: reason, Metadata: map[string]any{"school_ids": ss}}); err != nil {
			return err
		}
		return idempotency.SetResult(ctx, map[string]any{"data": out})
	})
	return out, err
}

// SisConnectionSchoolPage is one cursor page of listSisConnectionSchools.
type SisConnectionSchoolPage struct {
	Data []write.SisConnectionSchool
	Page page.Info
}

// Schools lists the schools of a connection (listSisConnectionSchools), keyset paginated (API_DESIGN §5).
func (h SisConnections) Schools(ctx context.Context, id uuid.UUID, limit int, cursor string) (out SisConnectionSchoolPage, err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.ListSisConnectionSchools")
	defer func() { done(err) }()
	limit = page.Limit(limit)
	out.Page.Limit = limit
	var after *uuid.UUID
	if cursor != "" {
		if h.Cursors == nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		c, err := h.Cursors.Decode(cursor)
		if err != nil {
			return out, problem.New(problem.BadRequest, "invalid cursor")
		}
		after = &c.ID
	}
	ctx, _, err = h.connectionScope(ctx, id)
	if err != nil {
		return out, err
	}
	var rows []write.SisConnectionSchool
	var ids []uuid.UUID
	err = h.UoW.Do(ctx, func(ctx context.Context) error {
		rows, ids, err = h.Connections.SchoolPage(ctx, id, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
		out.Page.HasMore = true
		if h.Cursors != nil {
			next := h.Cursors.Encode(page.Cursor{ID: ids[limit-1]})
			out.Page.NextCursor = &next
		}
	}
	out.Data = rows
	return out, nil
}

// CreateContract is the contract_v1 branch of createSisConnection, served by SisContract (F017).
func (h SisConnections) CreateContract(ctx context.Context, tenantID uuid.UUID, in ContractCreateInput) (dto.SisConnection, error) {
	if h.Contract == nil {
		return dto.SisConnection{}, problem.Invalid("provider_kind", problem.SubInvalid, "contract_v1 is not enabled on this server")
	}
	return h.Contract.Create(ctx, tenantID, in)
}
