package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// Media tables (F027, specs/027-media-math-preview/data-model.md, DATABASE_DESIGN §3.13, §2.6a G, ADR-0044, ADR-0055).
// The files live in object storage only (R31); the CHECKs (kind, status, bucket, owner, variants size), the unique
// index per scope and sha256 (expression on coalesce(tenant_id)) and RLS are hand-written in the migration.

const mediaResource = "media"

// MediaAsset — content.media_assets (H): metadata of one uploaded file and its content-addressed variants.
type MediaAsset struct{ ent.Schema }

func (MediaAsset) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "media_assets"}}
}
func (MediaAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: mediaResource, PK: entx.PK{Column: "asset_id"}}}
}
func (MediaAsset) Fields() []ent.Field {
	return []ent.Field{
		field.String("kind").MaxLen(16).Immutable(),
		field.String("original_filename").MaxLen(255).Immutable(),
		field.String("mime_type").MaxLen(127).Immutable(),
		field.Int64("size_bytes").Immutable(),
		field.Int32("width").Optional().Nillable(),
		field.Int32("height").Optional().Nillable(),
		field.Int32("duration_ms").Optional().Nillable(),
		field.String("sha256").MaxLen(64).Immutable(),
		field.String("bucket").MaxLen(64).Immutable(),
		field.JSON("variants", map[string]any{}).Default(map[string]any{}),
		field.String("alt_text").MaxLen(500).Optional().Nillable(),
		field.String("license").MaxLen(255),
		field.String("status").MaxLen(16).Default("quarantined"),
		field.String("reject_reason").MaxLen(255).Optional().Nillable(),
		field.String("quarantine_key").MaxLen(512).Immutable(),
		field.Bool("is_exam_material").Default(false).Immutable(),
		field.Time("upload_completed_at").Optional().Nillable(),
		field.Time("processed_at").Optional().Nillable(),
		field.UUID("uploaded_by", uuid.UUID{}).Immutable(),
	}
}
func (MediaAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_scope", "tenant_id", "kind", "status").StorageKey("media_assets_library_idx").Annotations(live()),
	}
}

// MediaUsage — content.media_usages (H): which content version references an asset (ADR-0044 §4, F027-AC-10,
// AC-11). A surrogate key replaces the composite key of DATABASE_DESIGN §3.13 (soft-delete when a draft drops a
// reference); tenant_id is the owner of the version, visible to a tenant through the assignment of the asset.
type MediaUsage struct{ ent.Schema }

func (MediaUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "media_usages"}}
}
func (MediaUsage) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.HybridContent{ResourceType: mediaResource, PK: entx.PK{Column: "media_usage_id"}, ResourceIDColumn: "asset_id"}}
}
func (MediaUsage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("asset_id", uuid.UUID{}).Immutable(),
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("item_id", uuid.UUID{}).Immutable(),
		field.UUID("version_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Positive().Immutable(),
	}
}
func (MediaUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("asset_id", "item_type", "version_id").Unique().StorageKey("media_usages_uq").Annotations(live()),
		index.Fields("version_id").StorageKey("media_usages_version_idx").Annotations(live()),
	}
}

// MathSvg — content.math_svgs (G): a sanitized formula SVG stored at math/<sha256>.svg, immutable (ADR-0055 §4, §9).
type MathSvg struct{ ent.Schema }

func (MathSvg) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "content", Table: "math_svgs"}}
}
func (MathSvg) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "sha256", StringLen: 64}}}
}
func (MathSvg) Fields() []ent.Field {
	return []ent.Field{
		field.String("storage_key").MaxLen(128).Immutable(),
		field.String("latex").MaxLen(2000).Immutable(),
		field.String("speech_text").MaxLen(1000).Optional().Nillable().Immutable(),
		field.Int32("size_bytes").Immutable(),
		field.Time("sanitized_at").Immutable(),
	}
}
