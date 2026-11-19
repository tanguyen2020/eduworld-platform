package write

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entms "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/mathsvg"
	entma "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/mediaasset"
	entmu "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/mediausage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
)

// MediaRepo implements write.MediaStore with ent (entx.HybridContent filter of the scope of ctx).
type MediaRepo struct{}

var _ write.MediaStore = MediaRepo{}

func intPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func int32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p) //nolint:gosec // dimensions and durations are bounded by the pipeline limits
	return &v
}

// VariantsOf decodes media_assets.variants.
func VariantsOf(m map[string]any) map[string]media.Variant {
	out := map[string]media.Variant{}
	b, err := json.Marshal(m)
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func variantsJSON(v map[string]media.Variant) map[string]any {
	out := map[string]any{}
	b, err := json.Marshal(v)
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// AssetOf maps an ent media asset.
func AssetOf(r *ent.MediaAsset) media.Asset {
	return media.Asset{ID: r.ID, Owner: owner(r.OwnerScope, r.TenantID), Kind: media.Kind(r.Kind), OriginalFilename: r.OriginalFilename,
		MIME: r.MimeType, Size: r.SizeBytes, Width: intPtr(r.Width), Height: intPtr(r.Height), DurationMS: intPtr(r.DurationMs), SHA256: r.Sha256,
		Bucket: r.Bucket, Variants: VariantsOf(r.Variants), AltText: r.AltText, License: r.License, Status: media.Status(r.Status),
		RejectReason: r.RejectReason, QuarantineKey: r.QuarantineKey, ExamMaterial: r.IsExamMaterial, UploadCompletedAt: r.UploadCompletedAt,
		ProcessedAt: r.ProcessedAt, UploadedBy: r.UploadedBy, RowVersion: r.Version, CreatedAt: r.CreatedAt}
}

// InsertAsset implements write.MediaStore.
func (MediaRepo) InsertAsset(ctx context.Context, a media.Asset) error {
	c := client(ctx).MediaAsset.Create().SetID(a.ID).SetOwnerScope(string(a.Owner.Scope)).SetKind(string(a.Kind)).
		SetOriginalFilename(a.OriginalFilename).SetMimeType(a.MIME).SetSizeBytes(a.Size).SetSha256(a.SHA256).SetBucket(a.Bucket).
		SetVariants(variantsJSON(a.Variants)).SetNillableAltText(a.AltText).SetLicense(a.License).SetStatus(string(a.Status)).
		SetQuarantineKey(a.QuarantineKey).SetIsExamMaterial(a.ExamMaterial).SetUploadedBy(a.UploadedBy)
	if a.Owner.TenantID != nil {
		c.SetTenantID(*a.Owner.TenantID)
	}
	return MapCurriculumErr(c.Exec(ctx))
}

// Asset implements write.MediaStore.
func (MediaRepo) Asset(ctx context.Context, id uuid.UUID) (media.Asset, error) {
	r, err := client(ctx).MediaAsset.Query().Where(entma.ID(id)).Only(ctx)
	if err != nil {
		return media.Asset{}, MapCurriculumErr(err)
	}
	return AssetOf(r), nil
}

// AssetBySHA implements write.MediaStore.
func (MediaRepo) AssetBySHA(ctx context.Context, o contentmodel.Owner, sha string) (media.Asset, bool, error) {
	q := client(ctx).MediaAsset.Query().Where(entma.Sha256(sha))
	if o.TenantID != nil {
		q = q.Where(entma.TenantID(*o.TenantID))
	} else {
		q = q.Where(entma.TenantIDIsNil())
	}
	r, err := q.First(ctx)
	if ent.IsNotFound(err) {
		return media.Asset{}, false, nil
	}
	if err != nil {
		return media.Asset{}, false, MapCurriculumErr(err)
	}
	return AssetOf(r), true, nil
}

// SaveAsset implements write.MediaStore.
func (MediaRepo) SaveAsset(ctx context.Context, a media.Asset) error {
	u := client(ctx).MediaAsset.Update().Where(entma.ID(a.ID), entma.Version(a.RowVersion)).SetStatus(string(a.Status)).
		SetVariants(variantsJSON(a.Variants)).SetNillableUploadCompletedAt(a.UploadCompletedAt).SetNillableProcessedAt(a.ProcessedAt).AddVersion(1)
	if a.RejectReason != nil {
		u.SetRejectReason(*a.RejectReason)
	} else {
		u.ClearRejectReason()
	}
	for _, d := range []struct {
		v     *int
		set   func(int32) *ent.MediaAssetUpdate
		clear func() *ent.MediaAssetUpdate
	}{{a.Width, u.SetWidth, u.ClearWidth}, {a.Height, u.SetHeight, u.ClearHeight}, {a.DurationMS, u.SetDurationMs, u.ClearDurationMs}} {
		if p := int32Ptr(d.v); p != nil {
			d.set(*p)
		} else {
			d.clear()
		}
	}
	n, err := u.Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return mismatchOrMissing(client(ctx).MediaAsset.Query().Where(entma.ID(a.ID)).Exist(ctx))
	}
	return nil
}

// Assets implements write.MediaStore.
func (MediaRepo) Assets(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]media.Asset, error) {
	rows, err := client(ctx).MediaAsset.Query().Where(entma.IDIn(idList...)).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make(map[uuid.UUID]media.Asset, len(rows))
	for _, r := range rows {
		out[r.ID] = AssetOf(r)
	}
	return out, nil
}

// ReplaceUsages implements write.MediaStore.
func (MediaRepo) ReplaceUsages(ctx context.Context, u write.UsageSet) error {
	c := client(ctx)
	have, err := c.MediaUsage.Query().Where(entmu.VersionID(u.VersionID), entmu.ItemType(string(u.ItemType))).All(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	kept := map[uuid.UUID]bool{}
	for _, h := range have {
		if slices.Contains(u.AssetIDs, h.AssetID) {
			kept[h.AssetID] = true
			continue
		}
		if _, err := c.MediaUsage.Delete().Where(entmu.ID(h.ID)).Exec(entx.WithDeleteReason(ctx, "media_reference_removed")); err != nil {
			return MapCurriculumErr(err)
		}
	}
	for _, id := range u.AssetIDs {
		if kept[id] {
			continue
		}
		cr := c.MediaUsage.Create().SetID(ids.New()).SetOwnerScope(string(u.Owner.Scope)).SetAssetID(id).SetItemType(string(u.ItemType)).
			SetItemID(u.ItemID).SetVersionID(u.VersionID).SetVersionNo(u.VersionNo)
		if u.Owner.TenantID != nil {
			cr.SetTenantID(*u.Owner.TenantID)
		}
		if err := cr.Exec(ctx); err != nil {
			return MapCurriculumErr(err)
		}
		kept[id] = true
	}
	return nil
}

// MathRepo implements write.MathStore (content.math_svgs, G).
type MathRepo struct{}

var _ write.MathStore = MathRepo{}

// MathSvg implements write.MathStore.
func (MathRepo) MathSvg(ctx context.Context, sha string) (write.MathSvg, bool, error) {
	r, err := client(ctx).MathSvg.Query().Where(entms.ID(sha)).Only(ctx)
	if ent.IsNotFound(err) {
		return write.MathSvg{}, false, nil
	}
	if err != nil {
		return write.MathSvg{}, false, MapCurriculumErr(err)
	}
	m := write.MathSvg{SHA256: r.ID, StorageKey: r.StorageKey, Latex: r.Latex, SizeBytes: int(r.SizeBytes), SanitizedAt: r.SanitizedAt}
	if r.SpeechText != nil {
		m.SpeechText = *r.SpeechText
	}
	return m, true, nil
}

// InsertMathSvg implements write.MathStore: content-addressed rows are insert-only, so a concurrent save of the same
// formula is a no-op (ON CONFLICT DO NOTHING keeps the transaction usable).
func (MathRepo) InsertMathSvg(ctx context.Context, m write.MathSvg) error {
	var speech any
	if m.SpeechText != "" {
		speech = m.SpeechText
	}
	// tenancyguard:ok content.math_svgs is a G table (no tenant_id, insert-only, never soft-deleted): ADR-0055 §9
	_, err := exec(ctx, `INSERT INTO content.math_svgs (sha256, storage_key, latex, speech_text, size_bytes, sanitized_at, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now(), 1) ON CONFLICT (sha256) DO NOTHING`, m.SHA256, m.StorageKey, m.Latex, speech, m.SizeBytes, m.SanitizedAt.UTC())
	return MapCurriculumErr(err)
}
