package read

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	enttra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/tenantresourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/resourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

func client(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }

// AssignmentReadModel implements read.AssignmentReadModel with ent (global filter of the scope of ctx: the
// System Admin read runs under tenancy.Bypass(system_admin_cross_tenant), a reference check under the
// tenant's scope).
type AssignmentReadModel struct{}

var _ read.AssignmentReadModel = AssignmentReadModel{}

// ByResource implements read.AssignmentReadModel.
func (AssignmentReadModel) ByResource(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID, after uuid.UUID, limit int) ([]resourceassignment.Assignment, error) {
	q := client(ctx).TenantResourceAssignment.Query().Where(enttra.ResourceType(string(rt)), enttra.ResourceID(id))
	if after != uuid.Nil {
		q = q.Where(enttra.IDGT(after))
	}
	rows, err := q.Order(enttra.ByID()).Limit(limit + 1).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]resourceassignment.Assignment, len(rows))
	for i, r := range rows {
		out[i] = resourceassignment.Assignment{ID: r.ID, ResourceType: rt, ResourceID: r.ResourceID, TenantID: r.TenantID,
			AssignedBy: r.AssignedBy, AssignedAt: r.AssignedAt.UTC()}
	}
	return out, nil
}

// Assigned implements read.AssignmentReadModel.
func (AssignmentReadModel) Assigned(ctx context.Context, rt contentmodel.ResourceType, id, tenant uuid.UUID) (bool, error) {
	return client(ctx).TenantResourceAssignment.Query().
		Where(enttra.ResourceType(string(rt)), enttra.ResourceID(id), enttra.TenantID(tenant)).Exist(ctx)
}
