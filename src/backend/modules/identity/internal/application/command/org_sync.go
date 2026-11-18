package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// SyncTenantOrganizationHandler makes the Keycloak Organization of a tenant match the tenant row
// (worker keycloak-membership on tenant.created / tenant.updated, F010-AC-01, F010-AC-03). It runs in the
// consumer transaction: a Keycloak failure returns an error so the event is retried, never acked.
type SyncTenantOrganizationHandler interface {
	Handle(ctx context.Context, tenantID uuid.UUID) error
}

// SyncTenantOrganization implements SyncTenantOrganizationHandler. It reads the current tenant (not the
// event payload) so out-of-order or repeated events converge to the same Organization state.
type SyncTenantOrganization struct {
	Tenants write.TenantRepository
	KC      port.KeycloakAdmin
}

// Handle runs the sync.
func (h SyncTenantOrganization) Handle(ctx context.Context, tenantID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SyncTenantOrganization")
	defer func() { done(err) }()
	t, err := h.Tenants.Get(ctx, tenantID)
	if err != nil {
		return err
	}
	org := port.KCOrganization{Alias: t.OrgAlias, Name: t.Name, Enabled: t.Status == tenant.StatusActive, Attributes: t.OrgAttributes()}
	if t.OrgID == nil {
		id, err := h.KC.EnsureOrganization(ctx, org)
		if err != nil {
			return fmt.Errorf("ensure organization %s: %w", t.OrgAlias, err)
		}
		orgID, err := uuid.Parse(id)
		if err != nil {
			return fmt.Errorf("organization id %q: %w", id, err)
		}
		if err := h.Tenants.SetOrgID(ctx, t.ID, orgID); err != nil {
			return err
		}
		t.OrgID = &orgID
		if org.Enabled { // created enabled with the current name: nothing else to do
			return nil
		}
	}
	org.ID = t.OrgID.String()
	if err := h.KC.UpdateOrganization(ctx, org); err != nil {
		return fmt.Errorf("update organization %s: %w", t.OrgAlias, err)
	}
	return nil
}
