package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
)

// F116 — moderation pipeline, sensitive-term catalogue, moderation switches (specs/116-chat-moderation-pipeline
// data-model §1, DATABASE_DESIGN §2.12, MODERATION §7, §11, §12). The partitioned decision log
// chat.moderation_decisions is hand-written in the migration (composite key of a partitioned table).

// SensitiveTermCategory — chat.sensitive_term_categories (T*, platform rows only: CHECK tenant_id IS NULL). Codes are
// category names (profanity, insult, …), never terms. Created/edited by System Admin only (chat.sensitive_terms.write).
type SensitiveTermCategory struct{ ent.Schema }

func (SensitiveTermCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "sensitive_term_categories"}}
}
func (SensitiveTermCategory) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "category_id"}}}
}
func (SensitiveTermCategory) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64),
		field.String("name").MaxLen(128),
		field.String("description").MaxLen(1000).Optional().Nillable(),
		field.String("severity").MaxLen(64),
		field.String("default_action").MaxLen(64),
		field.Bool("enabled").Default(true),
		field.Bool("allow_tenant_toggle").Default(false),
	}
}
func (SensitiveTermCategory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique().StorageKey("sensitive_term_categories_code_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// SensitiveTerm — chat.sensitive_terms (T*): platform terms (tenant_id NULL, System Admin) and the additional terms of
// one tenant (Safety Team, ADR-0072). Edits stay draft until the next publish; the catalogue in force is the snapshot
// of the latest chat.sensitive_catalog_versions row. Content is read only with chat.sensitive_terms.read and never
// logged (R14, R15).
type SensitiveTerm struct{ ent.Schema }

func (SensitiveTerm) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "sensitive_terms"}}
}
func (SensitiveTerm) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "term_id"}}}
}
func (SensitiveTerm) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("category_id", uuid.UUID{}),
		field.String("term").MaxLen(255),
		field.String("match_type").MaxLen(64),
		field.String("locale").MaxLen(64),
		field.JSON("variants", []string{}).Optional(),
		field.String("severity_override").MaxLen(64).Optional().Nillable(),
		field.String("action_override").MaxLen(64).Optional().Nillable(),
		field.Bool("enabled").Default(true),
		field.String("status").MaxLen(64).Default("draft"),
		field.String("note").MaxLen(1000).Optional().Nillable(),
		field.Int64("published_in_version").Optional().Nillable(),
	}
}
func (SensitiveTerm) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "category_id").StorageKey("sensitive_terms_tenant_category_idx"),
	}
}

// SensitiveAllowlistEntry — chat.sensitive_allowlist (T*): phrases excluded before matching (MODERATION §11.3).
type SensitiveAllowlistEntry struct{ ent.Schema }

func (SensitiveAllowlistEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "sensitive_allowlist"}}
}
func (SensitiveAllowlistEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "entry_id"}}}
}
func (SensitiveAllowlistEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("phrase").MaxLen(255),
		field.String("locale").MaxLen(64),
		field.String("reason").MaxLen(1000).Optional().Nillable(),
	}
}

// SensitiveCatalogVersion — chat.sensitive_catalog_versions (TA*, append-only: a trigger refuses UPDATE/DELETE). One
// row per publish of a scope (platform, or one tenant); snapshot is the immutable catalogue of that version (categories,
// enabled terms, allowlist) compiled by every instance of module chat (MODERATION §11.6).
type SensitiveCatalogVersion struct{ ent.Schema }

func (SensitiveCatalogVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "sensitive_catalog_versions"}}
}
func (SensitiveCatalogVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptionalAppendOnly{PK: entx.PK{Column: "catalog_version_id"}}}
}
func (SensitiveCatalogVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("catalog_version").Immutable(),
		field.String("scope").MaxLen(64).Immutable(),
		field.Time("published_at").Immutable(),
		field.UUID("published_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("change_summary").MaxLen(1000).Immutable(),
		field.Int("term_count").Immutable(),
		field.String("checksum").MaxLen(128).Immutable(),
		field.JSON("snapshot", json.RawMessage{}).Immutable(),
	}
}

// TenantCategorySetting — chat.tenant_category_settings (T): a category switched on/off for one tenant by the Safety
// Team or System Admin (chat.tenant_category.toggle), only when the category allows it (ADR-0072).
type TenantCategorySetting struct{ ent.Schema }

func (TenantCategorySetting) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "tenant_category_settings"}}
}
func (TenantCategorySetting) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "setting_id"}}}
}
func (TenantCategorySetting) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("category_id", uuid.UUID{}).Immutable(),
		field.Bool("enabled"),
		field.String("reason").MaxLen(1000),
	}
}
func (TenantCategorySetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "category_id").Unique().StorageKey("tenant_category_settings_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// ModerationSwitch — chat.moderation_switches (T*): the configured switches of the platform (tenant_id NULL) or of
// one tenant. steps holds sensitive_terms, spam, bullying, async_analysis; pii and grooming have no switch (ADR-0078).
// Only System Admin changes them (chat.policy.toggle); a disabling change needs a second System Admin (requests).
type ModerationSwitch struct{ ent.Schema }

func (ModerationSwitch) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_switches"}}
}
func (ModerationSwitch) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "switch_id"}}}
}
func (ModerationSwitch) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").MaxLen(64).Immutable(),
		field.Bool("enabled_all").Default(true),
		field.JSON("steps", map[string]bool{}),
		field.String("reason").MaxLen(1000).Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
	}
}
func (ModerationSwitch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").StorageKey("moderation_switches_tenant_idx"),
	}
}

// ModerationSwitchRequest — chat.moderation_switch_requests (T*): a request to disable steps (or enabled_all) waiting
// for the approval of a second System Admin (ADR-0078, F116-AC-32); pending requests expire after 24 hours.
type ModerationSwitchRequest struct{ ent.Schema }

func (ModerationSwitchRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_switch_requests"}}
}
func (ModerationSwitchRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptional{PK: entx.PK{Column: "request_id"}}}
}
func (ModerationSwitchRequest) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").MaxLen(64).Immutable(),
		field.Bool("enabled_all").Immutable(),
		field.JSON("steps", map[string]bool{}).Immutable(),
		field.String("reason").MaxLen(1000).Immutable(),
		field.Time("switch_expires_at").Immutable(),
		field.String("status").MaxLen(64).Default("pending_approval"),
		field.UUID("proposed_by", uuid.UUID{}).Immutable(),
		field.UUID("approved_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("decided_at").Optional().Nillable(),
		field.Time("request_expires_at").Immutable(),
	}
}
func (ModerationSwitchRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "request_expires_at").StorageKey("moderation_switch_requests_status_idx"),
	}
}

// ModerationSwitchChange — chat.moderation_switch_changes (TA*, append-only): before/after of every effective switch
// change (manual, approved request, auto_expire).
type ModerationSwitchChange struct{ ent.Schema }

func (ModerationSwitchChange) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_switch_changes"}}
}
func (ModerationSwitchChange) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantOptionalAppendOnly{PK: entx.PK{Column: "change_id"}}}
}
func (ModerationSwitchChange) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope").MaxLen(64).Immutable(),
		field.Bool("enabled_all_before").Immutable(),
		field.Bool("enabled_all_after").Immutable(),
		field.JSON("before", map[string]bool{}).Immutable(),
		field.JSON("after", map[string]bool{}).Immutable(),
		field.String("reason").MaxLen(1000).Optional().Nillable().Immutable(),
		field.Time("expires_at").Optional().Nillable().Immutable(),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("approved_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("request_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("source").MaxLen(64).Immutable(),
	}
}
func (ModerationSwitchChange) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at").StorageKey("moderation_switch_changes_tenant_idx"),
	}
}

// Sanction — chat.sanctions (T, MODERATION §7): a restriction of one student. F116 creates the system mutes
// (auto_spam, block_and_alert_hold); F121 adds the case hold (case_hold), the sanctions of the Safety Team, revocation
// and expiry (reason_code, issued_by_kind, revoked_by, revoke_reason, lifted_at: added by 20261231000001_f121).
type Sanction struct{ ent.Schema }

func (Sanction) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "sanctions"}}
}
func (Sanction) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "sanction_id"}}}
}
func (Sanction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.String("type").MaxLen(64).Immutable(),
		field.String("scope").MaxLen(64).Immutable(),
		field.UUID("channel_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(1000),
		field.UUID("case_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("issued_by", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("starts_at"),
		field.Time("ends_at").Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.String("reason_code").MaxLen(64).Optional().Nillable(),
		field.String("issued_by_kind").MaxLen(16).Default("system"),
		field.UUID("revoked_by", uuid.UUID{}).Optional().Nillable(),
		field.String("revoke_reason").MaxLen(1000).Optional().Nillable(),
		field.Time("lifted_at").Optional().Nillable(),
	}
}
func (Sanction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "user_id", "ends_at").StorageKey("sanctions_user_active_idx").
			Annotations(entsql.IndexWhere("revoked_at IS NULL AND deleted_at IS NULL")),
		index.Fields("ends_at").StorageKey("sanctions_expiry_idx").
			Annotations(entsql.IndexWhere("lifted_at IS NULL AND ends_at IS NOT NULL AND deleted_at IS NULL")),
	}
}

// QuickMessage — chat.quick_messages (G): the approved bilingual quick messages (ADR-0056, CHAT_SYSTEM §8.5); the same
// list for every grade (R15). The key is the contract code (varchar(64)).
type QuickMessage struct{ ent.Schema }

func (QuickMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "quick_messages"}}
}
func (QuickMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "quick_message_id", StringLen: 64}}}
}
func (QuickMessage) Fields() []ent.Field {
	return []ent.Field{
		field.String("category").MaxLen(64),
		field.String("text_vi").MaxLen(255),
		field.String("text_en").MaxLen(255),
		field.String("emoji").MaxLen(64).Optional().Nillable(),
		field.Int("sort_order").Default(0),
		field.String("status").MaxLen(64).Default("active"),
	}
}
