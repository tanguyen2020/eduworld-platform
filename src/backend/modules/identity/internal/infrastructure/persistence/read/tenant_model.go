// Package read implements the read ports of module identity in entx.ReadSession (READ ONLY); the global
// filter of the ent mixins applies (deleted_at IS NULL, tenant scope for T tables).
package read

import (
	"context"

	"entgo.io/ent/dialect/sql"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/predicate"
	enttenant "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/tenant"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// TenantModel implements read.TenantReadModel.
type TenantModel struct{ Session *entx.ReadSession }

var _ read.TenantReadModel = TenantModel{}

// ListTenants returns up to f.Limit+1 tenants ordered by (code, tenant_id) after the cursor.
func (m TenantModel) ListTenants(ctx context.Context, f read.TenantFilter) (out []dto.Tenant, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		q := ent.NewClient(ent.Driver(entx.Driver(ctx))).Tenant.Query()
		if f.Q != "" {
			q = q.Where(enttenant.Or(enttenant.CodeContainsFold(f.Q), enttenant.NameContainsFold(f.Q)))
		}
		if f.TenantType != "" {
			q = q.Where(enttenant.TenantType(f.TenantType))
		}
		if f.After != nil {
			after := f.After
			q = q.Where(predicate.Tenant(func(s *sql.Selector) {
				s.Where(sql.Or(sql.GT(s.C(enttenant.FieldCode), after.Key),
					sql.And(sql.EQ(s.C(enttenant.FieldCode), after.Key), sql.GT(s.C(enttenant.FieldID), after.ID))))
			}))
		}
		rows, err := q.Order(ent.Asc(enttenant.FieldCode), ent.Asc(enttenant.FieldID)).Limit(f.Limit + 1).All(ctx)
		if err != nil {
			return err
		}
		out = make([]dto.Tenant, 0, len(rows))
		for _, r := range rows {
			t := dto.Tenant{TenantID: r.ID, Code: r.Code, Name: r.Name, TenantType: r.TenantType, Status: r.Status,
				IsQA: r.IsQa, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version}
			if r.KeycloakOrgAlias != nil {
				t.Alias = *r.KeycloakOrgAlias
			}
			out = append(out, t)
		}
		return nil
	})
	return out, err
}
