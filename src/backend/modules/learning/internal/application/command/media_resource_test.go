package command

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// fakeMediaStore is a TEST double of write.MediaStore: Assets returns the assets of the map (the scope filter is the
// repository's job, covered by the integration test TestPlatformMediaAssignment).
type fakeMediaStore struct {
	write.MediaStore
	assets map[uuid.UUID]media.Asset
}

func (f fakeMediaStore) Assets(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]media.Asset, error) {
	out := map[uuid.UUID]media.Asset{}
	for _, id := range ids {
		if a, ok := f.assets[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

// F065 Clarifications 2026-11-28: media is assignable (owners of the visible, non-rejected assets) but has no trash or
// clone handler, so the registry hands it to assignment only.
func TestMediaResourceAssignable(t *testing.T) {
	tenant := uuid.New()
	ready, processing, rejected, tenantAsset := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	platform := contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	store := fakeMediaStore{assets: map[uuid.UUID]media.Asset{
		ready:       {ID: ready, Owner: platform, Status: media.StatusReady},
		processing:  {ID: processing, Owner: platform, Status: media.StatusProcessing},
		rejected:    {ID: rejected, Owner: platform, Status: media.StatusRejected},
		tenantAsset: {ID: tenantAsset, Owner: contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &tenant}, Status: media.StatusReady},
	}}
	r := MediaResource{Store: store}
	if r.ResourceType() != contentmodel.ResourceMedia {
		t.Fatalf("type %s", r.ResourceType())
	}
	owners, err := r.Owners(context.Background(), []uuid.UUID{ready, processing, rejected, tenantAsset, uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) != 3 || !owners[ready].Platform() || !owners[processing].Platform() || owners[tenantAsset].Platform() {
		t.Fatalf("owners %+v", owners)
	}
	if _, ok := owners[rejected]; ok {
		t.Fatal("a rejected asset is assignable")
	}
	if o, err := r.Owners(context.Background(), nil); err != nil || len(o) != 0 {
		t.Fatalf("no ids: %v %v", o, err)
	}

	reg, err := port.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterAssignable(r); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterAssignable(r); err == nil {
		t.Fatal("media registered twice")
	}
	if _, ok := reg.Assignable(contentmodel.ResourceMedia); !ok {
		t.Fatal("media is not assignable")
	}
	if _, ok := reg.Get(contentmodel.ResourceMedia); ok || len(reg.Types()) != 0 {
		t.Fatalf("media has a trash/clone handler: %v", reg.Types())
	}
	if _, ok := reg.Assignable(contentmodel.ResourceQuestion); ok {
		t.Fatal("unregistered type is assignable")
	}
	full := QuestionResource{}
	if err := reg.Register(full); err != nil {
		t.Fatal(err)
	}
	if a, ok := reg.Assignable(contentmodel.ResourceQuestion); !ok || a.ResourceType() != contentmodel.ResourceQuestion {
		t.Fatal("a full handler is not assignable")
	}
	if err := reg.RegisterAssignable(full); err == nil {
		t.Fatal("assign-only registration of a type with a full handler")
	}
}
