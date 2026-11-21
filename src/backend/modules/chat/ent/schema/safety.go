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

// F121 — reports, moderation cases, evidence and school safety notifications of the Platform Safety Team
// (specs/121-moderation-safety-team data-model §1, DATABASE_DESIGN §2.12, §2.16, MODERATION §5–§7, §10). Cross-tenant
// reads happen only in the moderation use cases through tenancy.Bypass(ctx, "moderation_case") (ADR-0072).

// Report — chat.reports (T): one student's report of a message or a student, linked to one case. The reporter is never
// shown to the reported student (MODERATION §5); note is normalised text written by a child (≤ 500).
type Report struct{ ent.Schema }

func (Report) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "reports"}}
}
func (Report) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "report_id"}}}
}
func (Report) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("reporter_id", uuid.UUID{}).Immutable(),
		field.String("target_type").MaxLen(16).Immutable(),
		field.UUID("target_id", uuid.UUID{}).Immutable(),
		field.UUID("target_user_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("reason").MaxLen(32).Immutable(),
		field.String("note").MaxLen(500).Optional().Nillable().Immutable(),
		field.JSON("context", map[string]string{}).Optional(),
		field.UUID("case_id", uuid.UUID{}),
		field.Float("weight").Default(1).SchemaType(map[string]string{"postgres": "numeric(3,2)"}),
		field.String("status").MaxLen(16).Default("open"),
		field.UUID("evidence_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (Report) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "target_type", "target_id", "created_at").StorageKey("reports_target_idx"),
		index.Fields("reporter_id", "target_type", "target_id", "case_id").Unique().StorageKey("reports_reporter_case_uq").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("case_id").StorageKey("reports_case_idx"),
	}
}

// ModerationCase — chat.moderation_cases (T): one incident of one tenant handled by the Platform Safety Team
// (MODERATION §6). school_ids and subject_user_ids are jsonb arrays of ≤ 20 ids (CHECK in the migration).
type ModerationCase struct{ ent.Schema }

func (ModerationCase) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_cases"}}
}
func (ModerationCase) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "case_id"}}}
}
func (ModerationCase) Fields() []ent.Field {
	return []ent.Field{
		field.JSON("school_ids", []uuid.UUID{}),
		field.String("status").MaxLen(16).Default("open"),
		field.String("severity").MaxLen(4),
		field.String("source").MaxLen(32).Immutable(),
		field.String("category_code").MaxLen(64).Optional().Nillable(),
		field.UUID("channel_id", uuid.UUID{}).Optional().Nillable(),
		field.String("channel_type").MaxLen(16).Optional().Nillable(),
		field.String("target_type").MaxLen(16).Immutable(),
		field.UUID("target_id", uuid.UUID{}).Immutable(),
		field.JSON("subject_user_ids", []uuid.UUID{}),
		field.UUID("assignee_moderator_id", uuid.UUID{}).Optional().Nillable(),
		field.String("assigned_via").MaxLen(16).Optional().Nillable(),
		field.Int("report_count").Default(0),
		field.Time("first_response_due_at"),
		field.Time("sla_due_at"),
		field.Time("first_response_at").Optional().Nillable(),
		field.Time("escalated_at").Optional().Nillable(),
		field.UUID("hold_sanction_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("school_notified_at").Optional().Nillable(),
		field.Time("school_ack_at").Optional().Nillable(),
		field.UUID("acknowledged_by", uuid.UUID{}).Optional().Nillable(),
		field.String("contact_outcome").MaxLen(32).Optional().Nillable(),
		field.String("contact_path").MaxLen(32).Default("school"),
		field.Time("resolved_at").Optional().Nillable(),
		field.UUID("source_event_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
func (ModerationCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "severity", "sla_due_at").StorageKey("moderation_cases_queue_idx"),
		index.Fields("tenant_id", "target_type", "target_id").StorageKey("moderation_cases_target_open_idx").
			Annotations(entsql.IndexWhere("status IN ('open','in_review') AND deleted_at IS NULL")),
		index.Fields("source_event_id").Unique().StorageKey("moderation_cases_source_event_uq").
			Annotations(entsql.IndexWhere("source_event_id IS NOT NULL")),
	}
}

// ModerationAction — chat.moderation_actions (TA, append-only): the immutable history of a case (MODERATION §9).
type ModerationAction struct{ ent.Schema }

func (ModerationAction) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_actions"}}
}
func (ModerationAction) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "action_id"}}}
}
func (ModerationAction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("case_id", uuid.UUID{}).Immutable(),
		field.String("action").MaxLen(32).Immutable(),
		field.UUID("actor_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("actor_kind").MaxLen(16).Immutable(),
		field.String("note").MaxLen(1000).Optional().Nillable().Immutable(),
		field.UUID("sanction_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("message_id", uuid.UUID{}).Optional().Nillable().Immutable(),
	}
}
func (ModerationAction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("case_id", "created_at").StorageKey("moderation_actions_case_idx"),
	}
}

// ModerationEvidence — chat.moderation_evidence (TA, immutable; DATABASE_DESIGN §2.16): a copy of the reported message
// and its ±10 context taken when the report or signal is received (original content and revision state). Kept until the
// case is closed + moderation_snapshot_retention; never removed by the chat retention or the gameplay purge (ADR-0033).
type ModerationEvidence struct{ ent.Schema }

func (ModerationEvidence) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "moderation_evidence"}}
}
func (ModerationEvidence) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.TenantAppendOnly{PK: entx.PK{Column: "evidence_id"}}}
}
func (ModerationEvidence) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("case_id", uuid.UUID{}).Immutable(),
		field.UUID("report_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.UUID("channel_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.JSON("snapshot", json.RawMessage{}).Immutable(),
		field.String("sha256").MaxLen(64).Immutable(),
	}
}
func (ModerationEvidence) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("case_id", "created_at").StorageKey("moderation_evidence_case_idx"),
	}
}

// SchoolSafetyNotification — chat.school_safety_notifications (T): the summary sent to one school about one case
// (MODERATION §6.4): template summary, students of that school only (ids and role), never chat content.
type SchoolSafetyNotification struct{ ent.Schema }

func (SchoolSafetyNotification) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "chat", Table: "school_safety_notifications"}}
}
func (SchoolSafetyNotification) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "notification_id"}}}
}
func (SchoolSafetyNotification) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("case_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.String("severity").MaxLen(4),
		field.String("category_code").MaxLen(64).Optional().Nillable(),
		field.JSON("students", []map[string]string{}),
		field.String("measures_taken").MaxLen(500).Optional().Nillable(),
		field.String("summary").MaxLen(1000),
		field.Time("school_notified_at"),
		field.Time("ack_due_at"),
		field.Int16("reminder_count").Default(0),
		field.Time("last_reminded_at").Optional().Nillable(),
		field.Time("school_ack_at").Optional().Nillable(),
		field.UUID("acknowledged_by", uuid.UUID{}).Optional().Nillable(),
		field.String("contact_outcome").MaxLen(32).Optional().Nillable(),
		field.String("ack_note").MaxLen(500).Optional().Nillable(),
	}
}
func (SchoolSafetyNotification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "school_id", "school_ack_at").StorageKey("school_safety_notifications_school_idx"),
		index.Fields("ack_due_at").StorageKey("school_safety_notifications_due_idx").
			Annotations(entsql.IndexWhere("school_ack_at IS NULL AND deleted_at IS NULL")),
		index.Fields("case_id").StorageKey("school_safety_notifications_case_idx"),
	}
}
