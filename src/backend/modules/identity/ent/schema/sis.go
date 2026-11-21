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

// F011 — SIS sync (DATABASE_DESIGN §2.3, DDL §3.14, ADR-0021, ADR-0046). Composite keys of the DDL
// (sis_connection_schools, synced_students) become a surrogate UUID key plus a unique index on the
// composite key (ent needs a single id); partitioned staging tables are created by hand in the migration.

func live() schema.Annotation { return entsql.IndexWhere("deleted_at IS NULL") }

// SisConnection — sis.sis_connections (T): one SIS source of a tenant (csv_import F011, contract_v1 F017).
type SisConnection struct{ ent.Schema }

func (SisConnection) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sis_connections"}}
}
func (SisConnection) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "sis_connection_id"}}}
}
func (SisConnection) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider_kind").MaxLen(64).Immutable(),
		field.String("contract_version").MaxLen(64).Optional().Nillable(),
		field.JSON("config", map[string]any{}).Optional(),
		field.String("secret_ref").MaxLen(512).Optional().Nillable().Sensitive(),
		field.String("status").MaxLen(64).Default("active"),
		field.String("delta_watermark").MaxLen(512).Optional().Nillable(),
		field.String("schedule").MaxLen(64).Optional().Nillable(),
		// F017 (data-model): webhook secret reference, Health.capabilities, health/success times, circuit counter.
		field.String("webhook_secret_ref").MaxLen(512).Optional().Nillable().Sensitive(),
		field.JSON("capabilities", []string{}).Optional(),
		field.Time("last_health_at").Optional().Nillable(),
		field.Time("last_success_at").Optional().Nillable(),
		field.Int32("consecutive_failed_runs").Default(0),
		field.String("status_reason").MaxLen(64).Optional().Nillable(),
	}
}
func (SisConnection) Indexes() []ent.Index {
	return []ent.Index{index.Fields("status", "provider_kind").StorageKey("sis_connections_schedule_idx").Annotations(live())}
}

// SisConnectionSchool — sis.sis_connection_schools (T): one active connection per school (ADR-0046).
type SisConnectionSchool struct{ ent.Schema }

func (SisConnectionSchool) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sis_connection_schools"}}
}
func (SisConnectionSchool) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "connection_school_id"}}}
}
func (SisConnectionSchool) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.Bool("active").Default(true),
		field.UUID("assigned_by", uuid.UUID{}),
		field.Time("assigned_at").Default(nowUTC),
	}
}
func (SisConnectionSchool) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sis_connection_id", "school_id").Unique().StorageKey("sis_connection_schools_pair_uq").Annotations(live()),
		index.Fields("school_id").Unique().StorageKey("sis_connection_schools_one_active").
			Annotations(entsql.IndexWhere("active AND deleted_at IS NULL")),
	}
}

// SyncRun — sis.sync_runs (T: carries tenant_id, so RLS applies — data-model listed G + tenant_id).
type SyncRun struct{ ent.Schema }

func (SyncRun) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sync_runs"}}
}
func (SyncRun) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "sync_run_id"}}} }
func (SyncRun) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.String("mode").MaxLen(64).Immutable(),
		field.String("trigger").MaxLen(64).Default("manual"),
		field.Bool("dry_run").Default(false),
		field.String("status").MaxLen(64).Default("running"),
		field.String("failure_code").MaxLen(64).Optional().Nillable(),
		field.JSON("stats", map[string]int{}).Default(map[string]int{}),
		field.Time("started_at").Default(nowUTC),
		field.Time("finished_at").Optional().Nillable(),
		// F017: who triggered a manual run, the webhook delivery of a webhook run.
		field.UUID("requested_by", uuid.UUID{}).Optional().Nillable(),
		field.UUID("delivery_id", uuid.UUID{}).Optional().Nillable(),
	}
}
func (SyncRun) Indexes() []ent.Index {
	return []ent.Index{index.Fields("sis_connection_id", "started_at").StorageKey("sync_runs_connection_idx").Annotations(live())}
}

// StagingRecord — sis.staging_records (GA, partitioned by month in the migration): normalized records of a run.
type StagingRecord struct{ ent.Schema }

func (StagingRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "staging_records"}}
}
func (StagingRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "staging_record_id"}}}
}
func (StagingRecord) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sync_run_id", uuid.UUID{}).Immutable(),
		field.String("entity_kind").MaxLen(64).Immutable(),
		field.String("external_id").MaxLen(64).Immutable(),
		field.Int("row_no").Optional().Nillable(),
		field.JSON("payload", map[string]any{}).Optional().Sensitive(),
		field.String("raw_hash").MaxLen(128),
		field.String("action").MaxLen(64),
		field.String("status").MaxLen(64).Default("pending"),
	}
}

// SyncRecordError — sis.sync_record_errors (GA, partitioned by month): per-record validation errors.
type SyncRecordError struct{ ent.Schema }

func (SyncRecordError) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sync_record_errors"}}
}
func (SyncRecordError) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.AppendOnly{PK: entx.PK{Column: "sync_record_error_id"}}}
}
func (SyncRecordError) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sync_run_id", uuid.UUID{}).Immutable(),
		field.String("entity_kind").MaxLen(64),
		field.String("external_id").MaxLen(64).Optional().Nillable(),
		field.Int("row_no").Optional().Nillable(),
		field.String("column_name").MaxLen(64).Optional().Nillable(),
		field.String("error_code").MaxLen(64),
		field.String("message").MaxLen(1000),
		field.Time("occurred_at").Default(nowUTC),
	}
}

// SisImport — sis.sis_imports (T): one CSV upload of a school.
type SisImport struct{ ent.Schema }

func (SisImport) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sis_imports"}}
}
func (SisImport) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "import_id"}}} }
func (SisImport) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Immutable(),
		field.UUID("requested_by", uuid.UUID{}).Immutable(),
		field.String("storage_key").MaxLen(512).Optional().Nillable(),
		field.String("file_sha256").MaxLen(128).Optional().Nillable(),
		field.Int64("size_bytes").Default(0),
		field.JSON("entity_kinds", []string{}),
		field.String("status").MaxLen(64).Default("awaiting_upload"),
		field.Bool("dry_run").Default(true),
		field.UUID("sync_run_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("stats", map[string]int{}).Default(map[string]int{}),
		field.String("idempotency_key").MaxLen(64).Optional().Nillable(),
	}
}

// SisLink — sis.sis_links (T): SIS external id ↔ school (class and enrollment links are history, ADR-0095 D6).
type SisLink struct{ ent.Schema }

func (SisLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "sis_links"}}
}
func (SisLink) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{PK: entx.PK{Column: "sis_link_id"}}} }
func (SisLink) Fields() []ent.Field {
	return []ent.Field{
		field.String("entity_type").MaxLen(64).Immutable(),
		field.UUID("entity_id", uuid.UUID{}),
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.String("external_id").MaxLen(64).Immutable(),
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.String("last_hash").MaxLen(128),
		field.Time("last_synced_at").Default(nowUTC),
	}
}
func (SisLink) Indexes() []ent.Index {
	return []ent.Index{index.Fields("sis_connection_id", "entity_type", "external_id").Unique().StorageKey("sis_links_source_uq").Annotations(live())}
}

// SyncedStudent — sis.synced_students (T): the SIS record of a student by source key (ADR-0046).
type SyncedStudent struct{ ent.Schema }

func (SyncedStudent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "synced_students"}}
}
func (SyncedStudent) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "synced_student_id"}}}
}
func (SyncedStudent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.String("student_external_id").MaxLen(64).Immutable(),
		field.UUID("school_id", uuid.UUID{}),
		field.String("student_code").MaxLen(32),
		field.String("record_status").MaxLen(32),
		field.String("national_student_id_hash").MaxLen(64).Optional().Nillable().Sensitive(),
		field.String("last_hash").MaxLen(64),
		field.Time("last_synced_at").Default(nowUTC),
		field.Int16("missing_runs").Default(0),
	}
}
func (SyncedStudent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sis_connection_id", "student_external_id").Unique().StorageKey("synced_students_source_uq").Annotations(live()),
		index.Fields("tenant_id", "school_id").StorageKey("synced_students_school_idx").Annotations(live()),
	}
}

// StudentLoginKey — sis.student_login_keys (T): login keys from the contract DTO (username, IdP subject).
type StudentLoginKey struct{ ent.Schema }

func (StudentLoginKey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "student_login_keys"}}
}
func (StudentLoginKey) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "login_key_id"}}}
}
func (StudentLoginKey) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.String("login_type").MaxLen(16),
		field.String("idp_alias").MaxLen(64).Optional().Nillable(),
		field.String("login_key").MaxLen(255),
		field.String("student_external_id").MaxLen(64),
	}
}

// ReconciliationIssue — sis.reconciliation_issues (T): conflicts waiting for a person (SIS_INTEGRATION §6.3).
type ReconciliationIssue struct{ ent.Schema }

func (ReconciliationIssue) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "sis", Table: "reconciliation_issues"}}
}
func (ReconciliationIssue) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "issue_id"}}}
}
func (ReconciliationIssue) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("school_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("sis_connection_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("sync_run_id", uuid.UUID{}).Optional().Nillable(),
		field.String("issue_type").MaxLen(64),
		field.JSON("entity_refs", map[string]any{}).Default(map[string]any{}),
		field.String("suggested_action").MaxLen(64).Optional().Nillable(),
		field.String("status").MaxLen(64).Default("open"),
		field.String("resolution").MaxLen(64).Optional().Nillable(),
		field.String("resolution_note").MaxLen(1000).Optional().Nillable(),
		field.UUID("resolved_by", uuid.UUID{}).Optional().Nillable(),
		field.Time("resolved_at").Optional().Nillable(),
	}
}
func (ReconciliationIssue) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "status", "issue_type").StorageKey("reconciliation_issues_open_idx").Annotations(live())}
}

// SisIdentityLink — identity.sis_identity_links (T): durable link user ↔ SIS source record (ADR-0046).
type SisIdentityLink struct{ ent.Schema }

func (SisIdentityLink) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Schema: "identity", Table: "sis_identity_links"}}
}
func (SisIdentityLink) Mixin() []ent.Mixin {
	return []ent.Mixin{entx.Base{PK: entx.PK{Column: "link_id"}}}
}
func (SisIdentityLink) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("school_id", uuid.UUID{}),
		field.UUID("sis_connection_id", uuid.UUID{}).Immutable(),
		field.String("student_external_id").MaxLen(64).Immutable(),
		field.String("status").MaxLen(16).Default("active"),
		field.Time("linked_at").Default(nowUTC),
		field.Time("superseded_at").Optional().Nillable(),
		field.UUID("superseded_by", uuid.UUID{}).Optional().Nillable(),
	}
}
func (SisIdentityLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique().StorageKey("sis_identity_links_one_active").
			Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL")),
		index.Fields("sis_connection_id", "student_external_id").Unique().StorageKey("sis_identity_links_source_active").
			Annotations(entsql.IndexWhere("status = 'active' AND deleted_at IS NULL")),
	}
}
