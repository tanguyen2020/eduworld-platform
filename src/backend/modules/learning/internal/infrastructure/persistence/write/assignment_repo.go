package write

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	enttra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/tenantresourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/resourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

func client(ctx context.Context) *ent.Client { return ent.NewClient(ent.Driver(entx.Driver(ctx))) }

// AssignmentRepo implements write.AssignmentRepository with ent (entx.Base: tenant + soft-delete filter of the
// scope of ctx — the CrossTenantScope of the requested tenants).
type AssignmentRepo struct{}

var _ write.AssignmentRepository = AssignmentRepo{}

func assignmentOf(r *ent.TenantResourceAssignment) resourceassignment.Assignment {
	return resourceassignment.Assignment{ID: r.ID, ResourceType: contentmodel.ResourceType(r.ResourceType), ResourceID: r.ResourceID,
		TenantID: r.TenantID, AssignedBy: r.AssignedBy, AssignedAt: r.AssignedAt.UTC(), RevokedAt: r.DeletedAt}
}

// Active implements write.AssignmentRepository.
func (AssignmentRepo) Active(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID, tenants []uuid.UUID) ([]resourceassignment.Assignment, error) {
	rows, err := client(ctx).TenantResourceAssignment.Query().
		Where(enttra.ResourceType(string(rt)), enttra.ResourceID(id), enttra.TenantIDIn(tenants...)).
		Order(enttra.ByTenantID()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]resourceassignment.Assignment, len(rows))
	for i, r := range rows {
		out[i] = assignmentOf(r)
	}
	return out, nil
}

// Create implements write.AssignmentRepository (tra_active_uq makes a concurrent duplicate 409 CONFLICT).
func (AssignmentRepo) Create(ctx context.Context, a resourceassignment.Assignment) error {
	err := client(ctx).TenantResourceAssignment.Create().SetID(a.ID).SetTenantID(a.TenantID).SetResourceType(string(a.ResourceType)).
		SetResourceID(a.ResourceID).SetAssignedBy(a.AssignedBy).SetAssignedAt(a.AssignedAt).Exec(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "the resource was assigned concurrently").Wrap(err)
	}
	return err
}

// Revoke implements write.AssignmentRepository: ent delete → soft-delete by the entx hook (deleted_at,
// deleted_by = the System Admin, delete_reason ≤ 64).
func (AssignmentRepo) Revoke(ctx context.Context, a resourceassignment.Assignment, reason string) (resourceassignment.Assignment, error) {
	if err := client(ctx).TenantResourceAssignment.DeleteOneID(a.ID).Exec(entx.WithDeleteReason(ctx, "unassigned: "+reason)); err != nil {
		if ent.IsNotFound(err) {
			return a, problem.New(problem.NotFound, "")
		}
		return a, err
	}
	at := time.Now().UTC()
	a.RevokedAt = &at
	return a, nil
}

// Publisher implements write.EventPublisher with the outbox writer (one idempotency key per event).
type Publisher struct{ W *outbox.Writer }

var _ write.EventPublisher = Publisher{}

// Publish writes e into the outbox of the transaction.
func (p Publisher) Publish(ctx context.Context, e write.Event) error {
	_, err := p.W.Publish(ctx, outbox.Message{Type: e.Type, Subject: outbox.Subject{Type: e.SubjectType, ID: e.SubjectID},
		PartitionKey: e.SubjectID, TenantID: e.TenantID, Platform: e.Platform, Payload: e.Payload})
	return err
}

// Auditor implements write.Auditor with the outbox audit recorder.
type Auditor struct{ R audit.Recorder }

var _ write.Auditor = Auditor{}

// Record records e in the transaction.
func (a Auditor) Record(ctx context.Context, e audit.Event) error { return a.R.Record(ctx, e) }
