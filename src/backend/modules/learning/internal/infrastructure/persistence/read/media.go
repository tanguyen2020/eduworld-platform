package read

import (
	"context"
	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entma "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/mediaasset"
	entmu "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/mediausage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
)

// MediaModel implements read.MediaReadModel with ent (entx.HybridContent filter of the scope of ctx).
type MediaModel struct{}

var _ read.MediaReadModel = MediaModel{}

func int32p(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

// assetOf maps an ent media asset (read side copy of the write mapping: the read model never imports the write
// repositories).
func assetOf(r *ent.MediaAsset) media.Asset {
	variants := map[string]media.Variant{}
	if b, err := json.Marshal(r.Variants); err == nil {
		_ = json.Unmarshal(b, &variants)
	}
	return media.Asset{ID: r.ID, Owner: ownerOf(r.OwnerScope, r.TenantID), Kind: media.Kind(r.Kind), OriginalFilename: r.OriginalFilename,
		MIME: r.MimeType, Size: r.SizeBytes, Width: int32p(r.Width), Height: int32p(r.Height), DurationMS: int32p(r.DurationMs), SHA256: r.Sha256,
		Bucket: r.Bucket, Variants: variants, AltText: r.AltText, License: r.License, Status: media.Status(r.Status), RejectReason: r.RejectReason,
		QuarantineKey: r.QuarantineKey, ExamMaterial: r.IsExamMaterial, UploadCompletedAt: r.UploadCompletedAt, ProcessedAt: r.ProcessedAt,
		UploadedBy: r.UploadedBy, RowVersion: r.Version, CreatedAt: r.CreatedAt}
}

// List implements read.MediaReadModel: newest first (UUIDv7 keys), keyset after f.After.
func (MediaModel) List(ctx context.Context, f read.MediaFilter) ([]media.Asset, error) {
	q := client(ctx).MediaAsset.Query()
	if f.Kind != "" {
		q = q.Where(entma.Kind(f.Kind))
	}
	if f.Status != "" {
		q = q.Where(entma.Status(f.Status))
	}
	if f.Q != "" {
		q = q.Where(entma.Or(entma.OriginalFilenameContainsFold(f.Q), entma.AltTextContainsFold(f.Q)))
	}
	if f.After != nil {
		q = q.Where(entma.IDLT(*f.After))
	}
	rows, err := q.Order(entma.ByID(entsql.OrderDesc())).Limit(f.Limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]media.Asset, len(rows))
	for i, r := range rows {
		out[i] = assetOf(r)
	}
	return out, nil
}

// Asset implements read.MediaReadModel.
func (MediaModel) Asset(ctx context.Context, id uuid.UUID) (media.Asset, error) {
	r, err := client(ctx).MediaAsset.Query().Where(entma.ID(id)).Only(ctx)
	if err != nil {
		return media.Asset{}, notFound(err)
	}
	return assetOf(r), nil
}

// Usages implements read.MediaReadModel: the live usages visible in the scope, newest version first.
func (MediaModel) Usages(ctx context.Context, assetID uuid.UUID, limit int) ([]read.MediaUsage, error) {
	rows, err := client(ctx).MediaUsage.Query().Where(entmu.AssetID(assetID)).
		Order(ent.Asc(entmu.FieldItemType), ent.Asc(entmu.FieldItemID), ent.Desc(entmu.FieldVersionNo)).Limit(limit).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]read.MediaUsage, len(rows))
	for i, r := range rows {
		out[i] = read.MediaUsage{ItemType: r.ItemType, ItemID: r.ItemID, VersionID: r.VersionID, VersionNo: r.VersionNo}
	}
	return out, nil
}
