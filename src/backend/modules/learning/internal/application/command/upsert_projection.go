package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ErrProjectionTenantMismatch: the event's tenant differs from the tenant of the projection row it references
// (F020-AC-04, MULTITENANCY §4.1); retrying cannot fix it.
var ErrProjectionTenantMismatch = errors.New("projection: event tenant differs from the referenced row")

// ErrProjectionNoTenant: a tenant event without a tenant in its envelope.
var ErrProjectionNoTenant = errors.New("projection: tenant event without envelope tenant")

// UpsertProjectionHandler applies identity events to the ref.* projections (DATABASE_DESIGN §2.17). It runs
// inside the transaction the consumer learning.projection_identity opened with the event scope, and is the
// only writer of ref.* (F020-AC-05). Each method returns whether the row changed (an older or equal
// source_version is ignored, F020-AC-01).
type UpsertProjectionHandler interface {
	Tenant(ctx context.Context, envTenant *uuid.UUID, r write.RefTenant) (bool, error)
	Province(ctx context.Context, r write.RefProvince) (bool, error)
	School(ctx context.Context, envTenant *uuid.UUID, r write.RefSchool) (bool, error)
	Student(ctx context.Context, envTenant *uuid.UUID, r write.RefStudent) (bool, error)
	StudentDeleted(ctx context.Context, envTenant *uuid.UUID, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error)
}

// UpsertProjection implements UpsertProjectionHandler.
type UpsertProjection struct {
	Refs write.ProjectionWriter
}

var _ UpsertProjectionHandler = UpsertProjection{}

// sameTenant returns the envelope tenant, which must equal want when want is set.
func sameTenant(envTenant *uuid.UUID, want uuid.UUID) (uuid.UUID, error) {
	if envTenant == nil || *envTenant == uuid.Nil {
		return uuid.Nil, ErrProjectionNoTenant
	}
	if want != uuid.Nil && want != *envTenant {
		return uuid.Nil, fmt.Errorf("%w: payload tenant", ErrProjectionTenantMismatch)
	}
	return *envTenant, nil
}

// owned checks that every referenced projection row, when known, belongs to tenant.
func (h UpsertProjection) owned(ctx context.Context, tenant uuid.UUID, refs map[write.RefTable]uuid.UUID) error {
	for table, id := range refs {
		if id == uuid.Nil {
			continue
		}
		owner, err := h.Refs.OwnerTenant(ctx, table, id)
		if err != nil {
			return err
		}
		if owner != nil && *owner != tenant {
			return fmt.Errorf("%w: %s %s", ErrProjectionTenantMismatch, table, id)
		}
	}
	return nil
}

func run(ctx context.Context, name string, fn func(ctx context.Context) (bool, error)) (applied bool, err error) {
	ctx, done := observability.UseCase(ctx, "learning.command.UpsertProjection"+name)
	defer func() { done(err) }()
	return fn(ctx)
}

// Tenant applies tenant.created/updated: the envelope tenant is the tenant itself.
func (h UpsertProjection) Tenant(ctx context.Context, envTenant *uuid.UUID, r write.RefTenant) (bool, error) {
	return run(ctx, "Tenant", func(ctx context.Context) (bool, error) {
		if _, err := sameTenant(envTenant, r.TenantID); err != nil {
			return false, err
		}
		return h.Refs.UpsertTenant(ctx, r)
	})
}

// Province applies province.updated (platform event).
func (h UpsertProjection) Province(ctx context.Context, r write.RefProvince) (bool, error) {
	return run(ctx, "Province", func(ctx context.Context) (bool, error) { return h.Refs.UpsertProvince(ctx, r) })
}

// School applies school.synced/updated.
func (h UpsertProjection) School(ctx context.Context, envTenant *uuid.UUID, r write.RefSchool) (bool, error) {
	return run(ctx, "School", func(ctx context.Context) (bool, error) {
		t, err := sameTenant(envTenant, r.TenantID)
		if err != nil {
			return false, err
		}
		r.TenantID = t
		if err := h.owned(ctx, t, map[write.RefTable]uuid.UUID{write.RefSchools: r.SchoolID}); err != nil {
			return false, err
		}
		return h.Refs.UpsertSchool(ctx, r)
	})
}

// Student applies student.synced.
func (h UpsertProjection) Student(ctx context.Context, envTenant *uuid.UUID, r write.RefStudent) (bool, error) {
	return run(ctx, "Student", func(ctx context.Context) (bool, error) {
		t, err := sameTenant(envTenant, r.TenantID)
		if err != nil {
			return false, err
		}
		r.TenantID = t
		if err := h.owned(ctx, t, map[write.RefTable]uuid.UUID{write.RefSchools: r.SchoolID, write.RefStudents: r.StudentID}); err != nil {
			return false, err
		}
		return h.Refs.UpsertStudent(ctx, r)
	})
}

// StudentDeleted applies student.cleaned_up (deleted) and student.restored.
func (h UpsertProjection) StudentDeleted(ctx context.Context, envTenant *uuid.UUID, student, tenant uuid.UUID, deleted bool, at time.Time) (bool, error) {
	return run(ctx, "StudentDeleted", func(ctx context.Context) (bool, error) {
		t, err := sameTenant(envTenant, tenant)
		if err != nil {
			return false, err
		}
		if err := h.owned(ctx, t, map[write.RefTable]uuid.UUID{write.RefStudents: student}); err != nil {
			return false, err
		}
		return h.Refs.MarkStudentDeleted(ctx, student, t, deleted, at)
	})
}
