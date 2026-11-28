package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// MediaResource makes media assets assignable to tenants (ADR-0029, resource_type media; F065 Clarifications
// 2026-11-28): a platform video, audio or subtitle referenced by a platform activity is read by the students of a
// tenant only through an active assignment of that asset (entx.HybridContent of content.media_assets), exactly like
// the platform questions of a platform quiz. Assigning content never assigns its media implicitly: System Admin
// assigns each asset. Trash and clone of media stay out (F027 spec: trash of media → F045), so this is registered with
// port.Registry.RegisterAssignable only.
type MediaResource struct {
	Store write.MediaStore
}

var _ port.Assignable = MediaResource{}

// ResourceType implements port.Assignable.
func (MediaResource) ResourceType() contentmodel.ResourceType { return contentmodel.ResourceMedia }

// Owners implements port.Assignable: the live assets visible to the scope of ctx; a rejected asset is never served,
// so it counts as absent (not assignable).
func (r MediaResource) Owners(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error) {
	out := map[uuid.UUID]contentmodel.Owner{}
	if len(idList) == 0 {
		return out, nil
	}
	assets, err := r.Store.Assets(ctx, idList)
	if err != nil {
		return nil, err
	}
	for id, a := range assets {
		if a.Status != media.StatusRejected {
			out[id] = a.Owner
		}
	}
	return out, nil
}
