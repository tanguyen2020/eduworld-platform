package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CurriculumResource is the ResourceHandler of resource_type curriculum (F021 T008): trash and restore of curricula,
// topics and objectives, assignment (owners) and the deep clone of a curriculum into another tenant (ADR-0028,
// ADR-0029). Register it in the port.Registry of module learning.
type CurriculumResource struct {
	Store write.CurriculumStore
	Trash write.CurriculumTrashStore
}

var _ port.ResourceHandler = CurriculumResource{}

// ResourceType implements port.ResourceHandler.
func (CurriculumResource) ResourceType() contentmodel.ResourceType {
	return contentmodel.ResourceCurriculum
}

// WritePermission implements port.ResourceHandler.
func (CurriculumResource) WritePermission() authz.Permission { return authz.PermCurriculumWrite }

// Owners implements port.ResourceHandler: curricula, and under the trash bypass also deleted topics and
// objectives (their owner is the owner of their curriculum).
func (r CurriculumResource) Owners(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error) {
	out := make(map[uuid.UUID]contentmodel.Owner, len(idList))
	for _, id := range idList {
		c, err := r.Store.Curriculum(ctx, id)
		if err == nil {
			out[id] = c.Owner
			continue
		}
		if pe, ok := problemOf(err); !ok || pe.Code != problem.NotFound {
			return nil, err
		}
		if t, err := r.Store.Topic(ctx, id); err == nil {
			out[id] = t.Owner
			continue
		}
		if o, err := r.Store.Objective(ctx, id); err == nil {
			out[id] = o.Owner
		}
	}
	return out, nil
}

// SoftDelete implements port.ResourceHandler: a curriculum with a current release is not deleted
// (contentmodel.ErrItemPublished → 409).
func (r CurriculumResource) SoftDelete(ctx context.Context, id uuid.UUID, reason string) (port.Deleted, error) {
	c, err := r.Store.Curriculum(ctx, id)
	if err != nil {
		return port.Deleted{}, err
	}
	if err := contentmodel.CanDeleteItem(c.CurrentVersionID); err != nil {
		return port.Deleted{}, err
	}
	if err := r.Trash.SoftDeleteCurriculum(ctx, id, reason); err != nil {
		return port.Deleted{}, err
	}
	return port.Deleted{Owner: c.Owner, Title: "curriculum " + c.Code}, nil
}

// ListDeleted implements port.ResourceHandler.
func (r CurriculumResource) ListDeleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	return r.Trash.DeletedEntries(ctx, f)
}

// Restore implements port.ResourceHandler (a topic or LO of a version that is no longer draft cannot come back:
// the draft-only trigger answers 409 CONTENT_NOT_EDITABLE).
func (r CurriculumResource) Restore(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	return r.Trash.RestoreEntry(ctx, id)
}

// Clone implements port.ResourceHandler (ctx carries tenancy.Bypass(tenant_clone)): the curriculum of the source
// tenant and its latest version become a new curriculum of the target tenant with version 1 in draft (structure
// deep-copied); student data is never copied (R22).
func (r CurriculumResource) Clone(ctx context.Context, in port.CloneInput) (uuid.UUID, error) {
	src, err := r.Store.Curriculum(ctx, in.SourceID)
	if err != nil || src.Owner.TenantID == nil || *src.Owner.TenantID != in.SourceTenant {
		return uuid.Nil, port.ErrNotInSourceTenant
	}
	vs, err := r.Store.Versions(ctx, src.ID)
	if err != nil {
		return uuid.Nil, err
	}
	target := in.TargetTenant
	o := contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &target}
	srcTenant, at, by := in.SourceTenant, in.At, in.Actor
	c := dto.Curriculum{ID: ids.New(), Code: src.Code, Name: src.Name, Framework: src.Framework, Owner: o, ClonedFromID: &src.ID,
		ClonedFromTenantID: &srcTenant, ClonedAt: &at, ClonedBy: &by}
	if len(vs) > 0 {
		c.LatestVersionNo = 1
	}
	if err := r.Store.InsertCurriculum(ctx, c); err != nil {
		return uuid.Nil, err
	}
	if len(vs) == 0 {
		return c.ID, nil
	}
	latest := vs[0] // newest first
	v := dto.CurriculumVersion{ID: ids.New(), CurriculumID: c.ID, VersionNo: 1, Label: latest.Label, Status: contentmodel.StatusDraft,
		EffectiveFrom: latest.EffectiveFrom, EffectiveTo: latest.EffectiveTo, BasedOnID: &latest.ID, ChangeNote: latest.ChangeNote,
		ContentLanguage: latest.ContentLanguage, AuthorID: in.Actor, Owner: o}
	if err := r.Store.InsertVersion(ctx, v); err != nil {
		return uuid.Nil, err
	}
	if err := (Curriculum{Store: r.Store}).copyStructure(ctx, latest, v); err != nil {
		return uuid.Nil, err
	}
	return c.ID, nil
}

func problemOf(err error) (*problem.Error, bool) {
	var pe *problem.Error
	ok := errors.As(err, &pe)
	return pe, ok
}
