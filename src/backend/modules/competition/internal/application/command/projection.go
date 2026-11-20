package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
)

// ErrProjectionTenantMismatch: the event's tenant differs from the tenant of the projection row it references
// (MULTITENANCY §4.1); retrying cannot fix it.
var ErrProjectionTenantMismatch = errors.New("projection: event tenant differs from the referenced row")

// Projection applies the events of other modules to the ref.* projections of module competition, inside the
// transaction the consumer opened with the event scope. Each method returns whether a row changed (older
// source_version ignored).
type Projection struct {
	Refs port.ProjectionWriter
}

func (p Projection) owned(ctx context.Context, tenant uuid.UUID, table string, id uuid.UUID) error {
	owner, err := p.Refs.OwnerTenant(ctx, table, id)
	if err != nil {
		return err
	}
	if owner != nil && *owner != tenant {
		return fmt.Errorf("%w: %s", ErrProjectionTenantMismatch, table)
	}
	return nil
}

// Tenant applies tenant.created/updated (the envelope tenant is the tenant itself).
func (p Projection) Tenant(ctx context.Context, tenant uuid.UUID, code *string, typ, status string, isQA *bool, settings []byte, version int64) (bool, error) {
	return p.Refs.Tenant(ctx, tenant, code, typ, status, isQA, settings, version)
}

// Student applies student.synced.
func (p Projection) Student(ctx context.Context, tenant, student, school uuid.UUID, grade *int, status *string, version int64) (bool, error) {
	if err := p.owned(ctx, tenant, "students", student); err != nil {
		return false, err
	}
	return p.Refs.Student(ctx, student, tenant, school, grade, status, version)
}

// StudentStatus applies user.status_changed.
func (p Projection) StudentStatus(ctx context.Context, tenant *uuid.UUID, student uuid.UUID, status string) (bool, error) {
	return p.Refs.StudentStatus(ctx, student, tenant, status)
}

// StudentDeleted applies student.cleaned_up/restored.
func (p Projection) StudentDeleted(ctx context.Context, tenant, student uuid.UUID, deleted bool, at time.Time) (bool, error) {
	return p.Refs.StudentDeleted(ctx, student, tenant, deleted, at)
}

// PlayerProfile applies game_profile.updated.
func (p Projection) PlayerProfile(ctx context.Context, tenant, student uuid.UUID, nickname *string, level int, avatar []byte, version int64) (bool, error) {
	if err := p.owned(ctx, tenant, "player_profiles", student); err != nil {
		return false, err
	}
	return p.Refs.PlayerProfile(ctx, student, tenant, nickname, level, avatar, version)
}
