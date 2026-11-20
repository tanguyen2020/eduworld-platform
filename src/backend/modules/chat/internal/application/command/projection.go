package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// ErrProjectionTenantMismatch: the event's tenant differs from the tenant of the projection row it references
// (MULTITENANCY §4.1); retrying cannot fix it.
var ErrProjectionTenantMismatch = errors.New("projection: event tenant differs from the referenced row")

// Projection applies the events of other modules to the ref.* projections of module chat, inside the transaction the
// consumer opened with the event scope. Each method returns whether a row changed (older source_version ignored). A
// student who leaves (cleaned up, deactivated) gets the pending friend requests sent or received cancelled (F095 edge
// case).
type Projection struct {
	Refs   write.ProjectionWriter
	Social write.SocialRepository
	Now    func() time.Time
}

func (p Projection) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
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

// Student applies student.synced; a deactivated student gets its pending requests cancelled.
func (p Projection) Student(ctx context.Context, tenant, student, school uuid.UUID, grade *int, status *string, version int64) (bool, error) {
	if err := p.owned(ctx, tenant, "students", student); err != nil {
		return false, err
	}
	ok, err := p.Refs.Student(ctx, student, tenant, school, grade, status, version)
	if err != nil || !ok || status == nil || *status == "active" {
		return ok, err
	}
	_, err = p.Social.CancelOfStudent(ctx, tenant, student, p.now())
	return ok, err
}

// StudentStatus applies user.status_changed.
func (p Projection) StudentStatus(ctx context.Context, tenant *uuid.UUID, student uuid.UUID, status string) (bool, error) {
	return p.Refs.StudentStatus(ctx, student, tenant, status)
}

// StudentDeleted applies student.cleaned_up/restored; a cleaned-up student gets its pending requests cancelled.
func (p Projection) StudentDeleted(ctx context.Context, tenant, student uuid.UUID, deleted bool, at time.Time) (bool, error) {
	ok, err := p.Refs.StudentDeleted(ctx, student, tenant, deleted, at)
	if err != nil || !deleted {
		return ok, err
	}
	_, err = p.Social.CancelOfStudent(ctx, tenant, student, p.now())
	return ok, err
}

// PlayerProfile applies game_profile.updated.
func (p Projection) PlayerProfile(ctx context.Context, tenant, student uuid.UUID, nickname *string, level int, avatar []byte, version int64) (bool, error) {
	if err := p.owned(ctx, tenant, "player_profiles", student); err != nil {
		return false, err
	}
	return p.Refs.PlayerProfile(ctx, student, tenant, nickname, level, avatar, version)
}

// School applies school.synced/school.updated (curfew_window of the school, F115-AC-22).
func (p Projection) School(ctx context.Context, tenant, school uuid.UUID, status *string, settings []byte, version int64) (bool, error) {
	if err := p.owned(ctx, tenant, "schools", school); err != nil {
		return false, err
	}
	return p.Refs.School(ctx, school, tenant, status, settings, version)
}

// PlatformCurfew applies platform_settings.changed key platform_curfew (scope platform): value {start, end}.
func (p Projection) PlatformCurfew(ctx context.Context, value []byte, version int64) (bool, error) {
	return p.Refs.PlatformSetting(ctx, "platform_curfew", value, version)
}

// Expiry is the job expire-friend-requests (F095-AC-07): pending requests past friend_request_ttl become expired
// without any notification; expired friend codes are soft-deleted (never physically deleted, ADR-0028).
type Expiry struct {
	UoW   write.UnitOfWork
	Repo  write.ExpiryRepository
	Batch int
	Now   func() time.Time
}

// Run is one run of the job; it returns the number of requests and codes handled.
func (h Expiry) Run(ctx context.Context) (n int, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.ExpireFriendRequests")
	defer func() { done(err) }()
	ps, err := authz.SystemPlatformScope("expire-friend-requests")
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	if h.Now != nil {
		now = h.Now().UTC()
	}
	batch := h.Batch
	if batch <= 0 {
		batch = 500
	}
	err = h.UoW.InTenantTx(ctx, ps, func(ctx context.Context) error {
		r, err := h.Repo.ExpireRequests(ctx, now, batch)
		if err != nil {
			return err
		}
		c, err := h.Repo.ExpireCodes(ctx, now, batch)
		n = r + c
		return err
	})
	return n, err
}
