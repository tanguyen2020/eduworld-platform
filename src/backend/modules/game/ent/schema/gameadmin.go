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

// F078 — Game Operations Console backend (specs/078-game-ops-console-backend: data-model §1; DATABASE_DESIGN §2.16,
// §3.13; ADR-0027, ADR-0043, ADR-0044, ADR-0047). The workflow of every game config type lives in two generic tables:
// game.config_items gives each item a UUID config_id (the API key; also for the items keyed by a code: area_id,
// chapter_code, item_id) and game.config_versions holds every version of the workflow (draft → in_review → approved →
// published → archived, rejected). Only published versions are copied into the version table of their type (F071 release
// core, read by the players through the release pointer); a published version is never changed (R11, R31).

// GameConfigItem — game.config_items (G): one game config item of the console and the key of its type table.
type GameConfigItem struct{ ent.Schema }

func (GameConfigItem) Annotations() []schema.Annotation { return table("config_items") }
func (GameConfigItem) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "config_id"}}}
}
func (GameConfigItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_type").MaxLen(64).Immutable(),
		field.String("item_key").MaxLen(64).Immutable(),
		field.Int("latest_version_no").Default(0),
		// current_version_id mirrors the release pointer of the type table (written with it, read by the console).
		field.UUID("current_version_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (GameConfigItem) Indexes() []ent.Index {
	return []ent.Index{index.Fields("item_type", "item_key").Unique().StorageKey("config_items_key_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// GameConfigVersion — game.config_versions (G): one version of a game config item in the review workflow. The payload of
// a draft/rejected version can change; from in_review on it is frozen (trigger game.config_version_guard).
type GameConfigVersion struct{ ent.Schema }

func (GameConfigVersion) Annotations() []schema.Annotation { return table("config_versions") }
func (GameConfigVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "config_version_id"}}}
}
func (GameConfigVersion) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.String("item_type").MaxLen(64).Immutable(),
		field.Int("version_no").Immutable(),
		field.String("status").MaxLen(64),
		field.String("title").MaxLen(255).Optional().Nillable(),
		field.JSON("payload", map[string]any{}),
		field.String("payload_sha256").MaxLen(64),
		field.UUID("author_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("reviewer_id", uuid.UUID{}).Optional().Nillable(),
		field.String("review_note").MaxLen(2000).Optional().Nillable(),
		field.String("change_note").MaxLen(500).Optional().Nillable(),
		field.UUID("based_on_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("submitted_at").Optional().Nillable(),
		field.Time("reviewed_at").Optional().Nillable(),
		field.Time("published_at").Optional().Nillable(),
		field.UUID("published_by", uuid.UUID{}).Optional().Nillable(),
	}
}
func (GameConfigVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_id", "version_no").Unique().StorageKey("config_versions_no_uq"),
		index.Fields("item_type", "status", "config_id").StorageKey("config_versions_list_idx").Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// ScheduledPublication — game.scheduled_publications (G): a publication of an approved version at publish_at (F078-AC-12);
// the job scheduled-publish applies it once (status scheduled → applied/failed) unless it is cancelled before.
type ScheduledPublication struct{ ent.Schema }

func (ScheduledPublication) Annotations() []schema.Annotation { return table("scheduled_publications") }
func (ScheduledPublication) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "schedule_id"}}}
}
func (ScheduledPublication) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("config_version_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Immutable(),
		field.Time("publish_at").Immutable(),
		field.String("status").MaxLen(16),
		field.String("failure_code").MaxLen(64).Optional().Nillable(),
		field.String("change_note").MaxLen(500).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(500).Optional().Nillable(),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.UUID("cancelled_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("finished_at").Optional().Nillable(),
	}
}
func (ScheduledPublication) Indexes() []ent.Index {
	scheduled := entsql.IndexWhere("status = 'scheduled' AND deleted_at IS NULL")
	return []ent.Index{
		index.Fields("publish_at").StorageKey("scheduled_publications_due_idx").Annotations(scheduled),
		index.Fields("config_id").Unique().StorageKey("scheduled_publications_one_uq").Annotations(scheduled),
	}
}

// PreviewSession — game.preview_sessions (G, DATABASE_DESIGN §2.16): a short-lived read of one version for the Godot Web
// preview (preview_token_ttl); only the SHA-256 of the token is stored. A preview never creates a game session (ADR-0031).
type PreviewSession struct{ ent.Schema }

func (PreviewSession) Annotations() []schema.Annotation { return table("preview_sessions") }
func (PreviewSession) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "preview_session_id"}}}
}
func (PreviewSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("token_hash").MaxLen(128).Immutable(),
		field.String("config_type").MaxLen(64).Immutable(),
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.UUID("config_version_id", uuid.UUID{}).Immutable(),
		field.Int("version_no").Immutable(),
		field.Time("expires_at").Immutable(),
	}
}
func (PreviewSession) Indexes() []ent.Index {
	return []ent.Index{index.Fields("token_hash").Unique().StorageKey("preview_sessions_token_uq")}
}

// RenderJob — game.render_jobs (G, DATABASE_DESIGN §2.16): one render of a 3D asset of a published version (thumbnail
// 512×512 or turntable 24 frames, FR-CMS-017); unique per (version, source asset, kind) so a replay creates nothing.
type RenderJob struct{ ent.Schema }

func (RenderJob) Annotations() []schema.Annotation { return table("render_jobs") }
func (RenderJob) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "render_job_id"}}}
}
func (RenderJob) Fields() []ent.Field {
	return []ent.Field{
		field.String("config_type").MaxLen(64).Immutable(),
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("config_version_id", uuid.UUID{}).Immutable(),
		field.UUID("source_asset_id", uuid.UUID{}).Immutable(),
		field.String("kind").MaxLen(64).Immutable(),
		field.String("status").MaxLen(16),
		field.Int("attempts").Default(0),
		field.String("last_error").MaxLen(1000).Optional().Nillable(),
		field.UUID("result_asset_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("next_attempt_at").Optional().Nillable(),
	}
}
func (RenderJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_version_id", "source_asset_id", "kind").Unique().StorageKey("render_jobs_uq"),
		index.Fields("next_attempt_at").StorageKey("render_jobs_retry_idx").Annotations(entsql.IndexWhere("status = 'failed' AND deleted_at IS NULL")),
	}
}

// MediaAsset — game.media_assets (G, DATABASE_DESIGN §3.13 as content.media_assets): a game content file in object
// storage (never in the database, ADR-0044); render_thumbnail/render_turntable are produced by the renderer.
type MediaAsset struct{ ent.Schema }

func (MediaAsset) Annotations() []schema.Annotation { return table("media_assets") }
func (MediaAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "asset_id"}}}
}
func (MediaAsset) Fields() []ent.Field {
	return []ent.Field{
		field.String("kind").MaxLen(64).Immutable(),
		field.String("mime_type").MaxLen(127).Immutable(),
		field.String("original_filename").MaxLen(255).Immutable(),
		field.Int64("size_bytes").Immutable(),
		field.String("sha256").MaxLen(64).Immutable(),
		field.String("bucket").MaxLen(64),
		field.String("quarantine_key").MaxLen(512).Optional().Nillable().Immutable(),
		field.String("status").MaxLen(16),
		field.String("reject_reason").MaxLen(255).Optional().Nillable(),
		field.JSON("variants", []map[string]any{}).Optional(),
		field.Int("width").Optional().Nillable(),
		field.Int("height").Optional().Nillable(),
		field.String("license").MaxLen(255).Optional().Nillable().Immutable(),
		field.String("alt_text").MaxLen(500).Optional().Nillable(),
		field.UUID("uploaded_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("upload_expires_at").Optional().Nillable(),
		field.Time("processed_at").Optional().Nillable(),
	}
}
func (MediaAsset) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("sha256").Unique().StorageKey("media_assets_sha_uq").Annotations(live),
		index.Fields("status", "created_at").StorageKey("media_assets_pipeline_idx").
			Annotations(entsql.IndexWhere("status IN ('quarantined', 'processing') AND deleted_at IS NULL")),
	}
}

// MediaUsage — game.media_usages (G): which version uses an asset ("đang dùng ở version nào"), and the render of a
// published version (usage render_thumbnail/render_turntable).
type MediaUsage struct{ ent.Schema }

func (MediaUsage) Annotations() []schema.Annotation { return table("media_usages") }
func (MediaUsage) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "usage_id"}}}
}
func (MediaUsage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("asset_id", uuid.UUID{}).Immutable(),
		field.String("item_type").MaxLen(64).Immutable(),
		field.UUID("config_id", uuid.UUID{}).Immutable(),
		field.UUID("config_version_id", uuid.UUID{}).Immutable(),
		field.String("usage").MaxLen(64).Immutable(),
	}
}
func (MediaUsage) Indexes() []ent.Index {
	live := entsql.IndexWhere("deleted_at IS NULL")
	return []ent.Index{
		index.Fields("asset_id", "config_version_id", "usage").Unique().StorageKey("media_usages_uq").Annotations(live),
		index.Fields("config_version_id", "usage").StorageKey("media_usages_version_idx").Annotations(live),
	}
}

// ClientManifest — game.client_manifests (G, DATABASE_DESIGN §2.16): the area/NPC/scene/interactable ids of one Godot
// build per platform; submit_review checks the references of a game config against the newest one (CMS-D-009).
type ClientManifest struct{ ent.Schema }

func (ClientManifest) Annotations() []schema.Annotation { return table("client_manifests") }
func (ClientManifest) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "client_manifest_id"}}}
}
func (ClientManifest) Fields() []ent.Field {
	return []ent.Field{
		field.String("client_version").MaxLen(32).Immutable(),
		field.String("platform").MaxLen(32).Immutable(),
		field.JSON("manifest", map[string]any{}).Immutable(),
		field.Time("published_at").Immutable(),
	}
}
func (ClientManifest) Indexes() []ent.Index {
	return []ent.Index{index.Fields("client_version", "platform").Unique().StorageKey("client_manifests_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}
