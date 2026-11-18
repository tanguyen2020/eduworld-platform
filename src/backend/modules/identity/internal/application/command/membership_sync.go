package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// SyncUserMembershipHandler makes the Keycloak Organization membership of one user equal to the
// EduWorld memberships (identity.tenant_memberships_v): remove first, then add, then revoke the Keycloak
// sessions when a membership was removed and emit user.access_revoked (F010-AC-20). Reconciling the whole
// state makes the order of tenant_membership.changed deliveries irrelevant.
type SyncUserMembershipHandler interface {
	Handle(ctx context.Context, userID uuid.UUID) error
}

// SyncUserMembership implements SyncUserMembershipHandler.
type SyncUserMembership struct {
	Reader read.MembershipReader
	KC     port.KeycloakAdmin
	Events write.EventPublisher
	Audit  write.Auditor
}

// errOrgPending: a tenant of the user has no Organization yet (tenant.created not processed): retry later.
var errOrgPending = fmt.Errorf("keycloak organization of a tenant not created yet")

// Handle runs the sync for one user.
func (h SyncUserMembership) Handle(ctx context.Context, userID uuid.UUID) (err error) {
	ctx, done := observability.UseCase(ctx, "identity.command.SyncUserMembership")
	defer func() { done(err) }()
	m, err := h.Reader.MembershipOf(ctx, userID)
	if err != nil {
		return err
	}
	if m.KeycloakUserID == "" {
		return nil // no Keycloak account (yet): nothing to mirror
	}
	desired := map[string]bool{}
	for _, t := range m.Tenants {
		if t.OrgID == nil {
			return errOrgPending
		}
		desired[t.OrgID.String()] = true
	}
	current, err := h.KC.ListMemberOrganizations(ctx, m.KeycloakUserID)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	var removed []port.KCOrganization
	for _, o := range current {
		have[o.ID] = true
		if !desired[o.ID] {
			removed = append(removed, o)
		}
	}
	for _, o := range removed { // remove before add (F010-AC-20)
		if err := h.KC.RemoveOrganizationMember(ctx, o.ID, m.KeycloakUserID); err != nil {
			return err
		}
	}
	for id := range desired {
		if !have[id] {
			if err := h.KC.AddOrganizationMember(ctx, id, m.KeycloakUserID); err != nil {
				return err
			}
		}
	}
	if len(removed) == 0 {
		return nil
	}
	if err := h.KC.LogoutUser(ctx, m.KeycloakUserID); err != nil {
		return err
	}
	uid := userID.String()
	for _, o := range removed {
		var tenantID *string
		if v := o.Attributes["tenant_id"]; len(v) == 1 {
			tenantID = &v[0]
		}
		var envTenant *uuid.UUID
		if tenantID != nil {
			if t, err := uuid.Parse(*tenantID); err == nil {
				envTenant = &t
			}
		}
		e := write.Event{Type: "user.access_revoked", SubjectType: "user", SubjectID: uid, TenantID: envTenant, Platform: envTenant == nil,
			Payload: map[string]any{"user_id": uid, "reason": "transferred", "tenant_id": tenantID}}
		if err := h.Events.Publish(ctx, e); err != nil {
			return err
		}
	}
	return h.Audit.Record(ctx, audit.Event{Action: "tenant_membership.synced", TargetType: "user", TargetID: uid, Result: audit.ResultSuccess,
		Metadata: map[string]any{"removed_organizations": len(removed), "desired_organizations": len(desired)}})
}
