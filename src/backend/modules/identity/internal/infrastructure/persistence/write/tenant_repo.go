// Package write implements the write ports of module identity with ent inside entx.UnitOfWork, and the
// outbox adapters (events, audit) of the same transaction.
package write

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	enttenant "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func client(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }

// TenantRepo implements write.TenantRepository (platform.tenants).
type TenantRepo struct{}

var _ write.TenantRepository = TenantRepo{}

const settingDefaultLocale = "default_locale"

func tenantToDomain(r *ent.Tenant) *tenant.Tenant {
	t := &tenant.Tenant{ID: r.ID, Code: r.Code, Name: r.Name, Type: tenant.Type(r.TenantType), Status: tenant.Status(r.Status),
		OrgID: r.KeycloakOrgID, Version: r.Version, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.KeycloakOrgAlias != nil {
		t.OrgAlias = *r.KeycloakOrgAlias
	}
	if v, ok := r.Settings[settingDefaultLocale].(string); ok {
		t.DefaultLocale = v
	}
	return t
}

// Create inserts the tenant. A duplicate code/alias or a second common tenant violates a unique index →
// 409 CONFLICT (F010-AC-02).
func (TenantRepo) Create(ctx context.Context, t *tenant.Tenant) error {
	row, err := client(ctx).Tenant.Create().SetID(t.ID).SetCode(t.Code).SetName(t.Name).SetTenantType(string(t.Type)).
		SetStatus(string(t.Status)).SetKeycloakOrgAlias(t.OrgAlias).
		SetSettings(map[string]any{settingDefaultLocale: t.DefaultLocale}).Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "a tenant with this code or alias exists, or the common tenant already exists").Wrap(err)
	}
	if err != nil {
		return err
	}
	*t = *tenantToDomain(row)
	return nil
}

// Get loads a live tenant.
func (TenantRepo) Get(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error) {
	row, err := client(ctx).Tenant.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return nil, err
	}
	return tenantToDomain(row), nil
}

// Save updates name and status where version = t.Version (a concurrent change → 409 CONFLICT).
func (TenantRepo) Save(ctx context.Context, t *tenant.Tenant) error {
	n, err := client(ctx).Tenant.Update().Where(enttenant.ID(t.ID), enttenant.Version(t.Version)).
		SetName(t.Name).SetStatus(string(t.Status)).AddVersion(1).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.Conflict, "tenant was modified")
	}
	t.Version++
	return nil
}

// SetOrgID records the Organization id (worker keycloak-membership); it does not bump the version.
func (TenantRepo) SetOrgID(ctx context.Context, id, orgID uuid.UUID) error {
	n, err := client(ctx).Tenant.Update().Where(enttenant.ID(id)).SetKeycloakOrgID(orgID).Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return problem.New(problem.NotFound, "")
	}
	return nil
}

// Publisher implements write.EventPublisher with the outbox writer (one idempotency key per event).
type Publisher struct{ W *outbox.Writer }

// Publish writes e into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, e write.Event) error {
	_, err := p.W.Publish(ctx, outbox.Message{Type: e.Type, Subject: outbox.Subject{Type: e.SubjectType, ID: e.SubjectID},
		PartitionKey: e.SubjectID, TenantID: e.TenantID, Platform: e.Platform, Payload: e.Payload})
	return err
}

// Auditor implements write.Auditor with the outbox audit recorder.
type Auditor struct{ R audit.Recorder }

// Record records e in the transaction.
func (a Auditor) Record(ctx context.Context, e audit.Event) error { return a.R.Record(ctx, e) }
