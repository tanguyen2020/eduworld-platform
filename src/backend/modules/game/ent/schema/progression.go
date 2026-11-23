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

// F074 — XP ledger, level, XP curve, titles and the Reward Engine (PROGRESSION_SYSTEM §3–§4, ADR-0049, DATABASE_DESIGN
// §2.8a, §2.13). Reward rules and progression curves are platform game content on the F071 release core (G, ADR-0043);
// the ledger, the level history and the reward grants of a student are append-only tenant tables (TA, R21); the daily
// cap counters are a tenant table (T) locked FOR UPDATE by the award transaction.

func gamification(name string) []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "gamification", Table: name}}
}

// XpLedgerEntry — gamification.xp_ledger (TA): one XP posting of a student (amount after the multipliers and the caps,
// 0 when a cap cut it all), the source it comes from and the reward rule version applied. Never updated or deleted:
// xp_total = Σ amount. Idempotent per (student_id, source_type, source_id, rule_id) and per idempotency_key.
type XpLedgerEntry struct{ ent.Schema }

func (XpLedgerEntry) Annotations() []schema.Annotation { return gamification("xp_ledger") }
func (XpLedgerEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "xp_entry_id"}}}
}
func (XpLedgerEntry) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int("amount").Immutable(),
		field.String("reason_code").MaxLen(64).Default("reward").Immutable(),
		field.String("source_type").MaxLen(64).Immutable(),
		field.String("source_code").MaxLen(64).Immutable(),
		field.UUID("source_id", uuid.UUID{}).Immutable(),
		field.UUID("content_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("rule_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Int("rule_version").Optional().Nillable().Immutable(),
		field.Bool("capped").Default(false).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
	}
}
func (XpLedgerEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("student_id", "source_type", "source_id", "rule_id").Unique().StorageKey("xp_ledger_source_uq"),
		index.Fields("idempotency_key").Unique().StorageKey("xp_ledger_idem_uq"),
		index.Fields("tenant_id", "student_id", "created_at").StorageKey("xp_ledger_student_idx"),
		index.Fields("tenant_id", "student_id", "content_version_id", "created_at").StorageKey("xp_ledger_content_idx"),
	}
}

// XpDailyCap — gamification.xp_daily_caps (T): XP granted to a student on one calendar day (Asia/Ho_Chi_Minh) per bucket
// (xp:total, xp:src:<source_type>, a rule bucket; F075 adds currency:xu). A surrogate key plus a partial unique replaces
// the composite key of DATABASE_DESIGN (ent: no composite primary key).
type XpDailyCap struct{ ent.Schema }

func (XpDailyCap) Annotations() []schema.Annotation { return gamification("xp_daily_caps") }
func (XpDailyCap) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "xp_daily_cap_id"}}}
}
func (XpDailyCap) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Time("cap_date").SchemaType(map[string]string{"postgres": "date"}).Immutable(),
		field.String("bucket").MaxLen(64).Immutable(),
		field.Int("granted").Default(0),
	}
}
func (XpDailyCap) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "cap_date", "bucket").Unique().StorageKey("xp_daily_caps_uq").
		Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// RewardRule — gamification.reward_rules (G): identity of a platform reward rule and its release pointer.
type RewardRule struct{ ent.Schema }

func (RewardRule) Annotations() []schema.Annotation { return gamification("reward_rules") }
func (RewardRule) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "reward_rule_id"}}}
}
func (RewardRule) Fields() []ent.Field {
	return append([]ent.Field{field.String("code").MaxLen(64).Optional().Nillable()}, pointerFields()...)
}
func (RewardRule) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("reward_rules_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// RewardRuleVersion — gamification.reward_rule_versions (G): payload of specs/schemas/reward-rule.schema.json plus the
// columns the Reward Engine selects on (trigger, source code, effective time = coalesce(active_from, published_at)).
type RewardRuleVersion struct{ ent.Schema }

func (RewardRuleVersion) Annotations() []schema.Annotation {
	return gamification("reward_rule_versions")
}
func (RewardRuleVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "reward_rule_version_id"}}}
}
func (RewardRuleVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("reward_rule_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.String("trigger_event_type").MaxLen(64).Immutable(),
		field.String("source_code").MaxLen(64).Immutable(),
		field.Time("effective_from").Immutable(),
		field.Time("active_to").Optional().Nillable().Immutable(),
	)
}
func (RewardRuleVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("reward_rule_id", "version_no").Unique().StorageKey("reward_rule_versions_no_uq"),
		index.Fields("trigger_event_type", "effective_from").StorageKey("reward_rule_versions_trigger_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}

// RewardGrant — gamification.reward_grants (TA): one idempotent reward given to a student (level-up reward of F074;
// F075–F077 add achievements, collections, event milestones). Append-only: a grant is never changed or taken back.
type RewardGrant struct{ ent.Schema }

func (RewardGrant) Annotations() []schema.Annotation { return gamification("reward_grants") }
func (RewardGrant) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "reward_grant_id"}}}
}
func (RewardGrant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("rule_version_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("trigger_event_id", uuid.UUID{}).Immutable(),
		field.String("trigger_kind").MaxLen(64).Immutable(),
		field.Int("level").Optional().Nillable().Immutable(),
		field.UUID("curve_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("grants", []map[string]any{}).Immutable(),
		field.String("idempotency_key").MaxLen(64).Immutable(),
		field.Time("granted_at").Default(nowUTC).Immutable(),
	}
}
func (RewardGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("idempotency_key").Unique().StorageKey("reward_grants_idem_uq"),
		index.Fields("tenant_id", "student_id", "granted_at").StorageKey("reward_grants_student_idx"),
	}
}

// ProgressionCurve — game.progression_curves (G): identity of the platform XP curve and its release pointer (Release 1.0:
// one curve, a second one is refused with 409).
type ProgressionCurve struct{ ent.Schema }

func (ProgressionCurve) Annotations() []schema.Annotation { return table("progression_curves") }
func (ProgressionCurve) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "curve_id"}}}
}
func (ProgressionCurve) Fields() []ent.Field {
	return append([]ent.Field{field.String("code").MaxLen(64).Optional().Nillable()}, pointerFields()...)
}
func (ProgressionCurve) Indexes() []ent.Index {
	return []ent.Index{index.Fields("code").Unique().StorageKey("progression_curves_code_uq").Annotations(entsql.IndexWhere("deleted_at IS NULL"))}
}

// ProgressionCurveVersion — game.progression_curve_versions (G): payload of specs/schemas/progression-curve.schema.json
// (formula, overrides, titles, level rewards stay in the payload) plus max_level and level_floor_protection.
type ProgressionCurveVersion struct{ ent.Schema }

func (ProgressionCurveVersion) Annotations() []schema.Annotation {
	return table("progression_curve_versions")
}
func (ProgressionCurveVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.GlobalBase{PK: entx.PK{Column: "curve_version_id"}}}
}
func (ProgressionCurveVersion) Fields() []ent.Field {
	return append(versionFields(field.UUID("curve_id", uuid.UUID{}).Immutable()),
		field.String("code").MaxLen(64).Immutable(),
		field.Int("max_level").Immutable(),
		field.Bool("level_floor_protection").Default(true).Immutable(),
	)
}
func (ProgressionCurveVersion) Indexes() []ent.Index {
	return []ent.Index{index.Fields("curve_id", "version_no").Unique().StorageKey("progression_curve_versions_no_uq")}
}

// ProgressionLevel — game.progression_levels (GA): the level table generated when a curve version is published (one row
// per level): XP to the next level (NULL at max_level), cumulative XP to reach the level, title and level rewards.
// Immutable like the version it belongs to.
type ProgressionLevel struct{ ent.Schema }

func (ProgressionLevel) Annotations() []schema.Annotation { return table("progression_levels") }
func (ProgressionLevel) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "progression_level_id"}}}
}
func (ProgressionLevel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("curve_version_id", uuid.UUID{}).Immutable(),
		field.Int("level").Immutable(),
		field.Int64("xp_required").Optional().Nillable().Immutable(),
		field.Int64("xp_cumulative").Immutable(),
		field.String("title_code").MaxLen(64).Immutable(),
		field.JSON("rewards", []map[string]any{}).Immutable(),
	}
}
func (ProgressionLevel) Indexes() []ent.Index {
	return []ent.Index{index.Fields("curve_version_id", "level").Unique().StorageKey("progression_levels_uq")}
}

// LevelHistory — game.level_history (TA): every level a student reached (one row per level, also several at once or by
// the curve-rollout job). Append-only.
type LevelHistory struct{ ent.Schema }

func (LevelHistory) Annotations() []schema.Annotation { return table("level_history") }
func (LevelHistory) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "level_history_id"}}}
}
func (LevelHistory) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("student_id", uuid.UUID{}).Immutable(),
		field.Int("from_level").Immutable(),
		field.Int("to_level").Immutable(),
		field.UUID("curve_id", uuid.UUID{}).Immutable(),
		field.Int("curve_version").Immutable(),
		field.UUID("trigger_xp_entry_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Time("reached_at").Default(nowUTC).Immutable(),
	}
}
func (LevelHistory) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "student_id", "reached_at").StorageKey("level_history_student_idx")}
}
