-- F025 — content review workflow, bilingual content, g1_3 voice-over (specs/025-content-review-workflow: data-model
-- §1–§3; CONTENT_MODEL §6, §8, §8.1; ADMIN_PORTAL §4; DATABASE_DESIGN §2.5; ADR-0028, ADR-0041, ADR-0056, ADR-0089).
-- Applied by migrate --module=learning as learning_migrator. Order: ent tables and added columns (entddl block,
-- golden test modules/learning/ent/ddl_test.go), CHECKs, FKs, RLS T*, _live views (recreated for the version tables:
-- SELECT * picks up the added columns), grants of learning_readonly. Expand only (AGENTS R12): every added column is
-- nullable, rows written before F025 keep NULL.

-- BEGIN entddl
CREATE TABLE content.review_tasks (
  task_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  review_lane varchar(64) NOT NULL,
  content_type varchar(64) NOT NULL,
  content_id uuid NOT NULL,
  content_version_id uuid NOT NULL,
  version_no bigint NOT NULL,
  title varchar(255),
  school_id uuid,
  author_id uuid NOT NULL,
  assignee_id uuid,
  status varchar(64) NOT NULL DEFAULT 'open',
  comments jsonb,
  submitted_at timestamptz NOT NULL,
  due_at timestamptz,
  resolved_at timestamptz,
  resolved_by uuid,
  PRIMARY KEY (task_id)
);
CREATE INDEX review_tasks_assignee_idx ON content.review_tasks (assignee_id, status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX review_tasks_open_uq ON content.review_tasks (content_version_id) WHERE status = 'open' AND deleted_at IS NULL;
CREATE INDEX review_tasks_queue_idx ON content.review_tasks (tenant_id, review_lane, status, submitted_at) WHERE deleted_at IS NULL;

CREATE TABLE content.translation_jobs (
  job_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL,
  kind varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'queued',
  format varchar(64) NOT NULL,
  source_language varchar(64) NOT NULL,
  target_language varchar(64) NOT NULL,
  request jsonb NOT NULL,
  input_asset_id uuid,
  result_storage_key varchar(512),
  report jsonb,
  requested_by uuid NOT NULL,
  finished_at timestamptz,
  PRIMARY KEY (job_id)
);
CREATE INDEX translation_jobs_status_idx ON content.translation_jobs (status, created_at) WHERE deleted_at IS NULL;
ALTER TABLE content.content_transitions ADD COLUMN content_id uuid;
ALTER TABLE content.content_transitions ADD COLUMN action varchar(64);
ALTER TABLE content.content_transitions ADD COLUMN idempotency_key varchar(64);
CREATE UNIQUE INDEX content_transitions_idem_uq ON content.content_transitions (content_version_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE content.question_versions ADD COLUMN translation_status varchar(64);
ALTER TABLE content.question_versions ADD COLUMN translation_meta jsonb;
ALTER TABLE content.question_versions ADD COLUMN audio_exempt_reason varchar(255);
ALTER TABLE content.question_versions ADD COLUMN audio_exempt_approved_by uuid;
ALTER TABLE content.question_set_versions ADD COLUMN translation_status varchar(64);
ALTER TABLE content.question_set_versions ADD COLUMN translation_meta jsonb;
ALTER TABLE content.question_set_versions ADD COLUMN audio_exempt_reason varchar(255);
ALTER TABLE content.question_set_versions ADD COLUMN audio_exempt_approved_by uuid;
ALTER TABLE content.lesson_versions ADD COLUMN translation_status varchar(64);
ALTER TABLE content.lesson_versions ADD COLUMN translation_meta jsonb;
ALTER TABLE content.lesson_versions ADD COLUMN audio_exempt_reason varchar(255);
ALTER TABLE content.lesson_versions ADD COLUMN audio_exempt_approved_by uuid;
ALTER TABLE content.learning_material_versions ADD COLUMN translation_status varchar(64);
ALTER TABLE content.learning_material_versions ADD COLUMN translation_meta jsonb;
ALTER TABLE content.learning_material_versions ADD COLUMN audio_exempt_reason varchar(255);
ALTER TABLE content.learning_material_versions ADD COLUMN audio_exempt_approved_by uuid;
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN translation_status varchar(64);
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN translation_meta jsonb;
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN audio_exempt_reason varchar(255);
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN audio_exempt_approved_by uuid;
-- END entddl

-- content.review_tasks (T*): lane tenant iff tenant row; platform lane rows written under PlatformScope only.
ALTER TABLE content.review_tasks
  ADD CONSTRAINT review_tasks_lane_ck CHECK (review_lane IN ('tenant','platform_learning')),
  ADD CONSTRAINT review_tasks_lane_owner_ck CHECK ((review_lane = 'tenant') = (tenant_id IS NOT NULL)),
  ADD CONSTRAINT review_tasks_type_ck CHECK (content_type IN ('question','question_set','assessment_blueprint','lesson',
    'learning_material','learning_path','activity')),
  ADD CONSTRAINT review_tasks_status_ck CHECK (status IN ('open','approved','rejected','cancelled')),
  ADD CONSTRAINT review_tasks_comments_ck CHECK (comments IS NULL OR (jsonb_typeof(comments) = 'array'
    AND octet_length(comments::text) <= 65536)),
  ADD CONSTRAINT review_tasks_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- content.translation_jobs (T*).
ALTER TABLE content.translation_jobs
  ADD CONSTRAINT translation_jobs_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT translation_jobs_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT translation_jobs_kind_ck CHECK (kind IN ('export','import')),
  ADD CONSTRAINT translation_jobs_status_ck CHECK (status IN ('queued','running','succeeded','failed')),
  ADD CONSTRAINT translation_jobs_format_ck CHECK (format IN ('xliff','csv')),
  ADD CONSTRAINT translation_jobs_lang_ck CHECK (source_language IN ('vi','en') AND target_language IN ('vi','en')
    AND source_language <> target_language),
  ADD CONSTRAINT translation_jobs_import_ck CHECK (kind = 'export' OR input_asset_id IS NOT NULL),
  ADD CONSTRAINT translation_jobs_request_ck CHECK (octet_length(request::text) <= 65536),
  ADD CONSTRAINT translation_jobs_report_ck CHECK (report IS NULL OR octet_length(report::text) <= 65536),
  ADD CONSTRAINT translation_jobs_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- content.content_transitions: the workflow actions (F025 writes submit_review/approve/reject/revise, F026 the others).
ALTER TABLE content.content_transitions
  ADD CONSTRAINT content_transitions_action_ck CHECK (action IS NULL OR action IN ('submit_review','approve','reject',
    'revise','publish','archive','rollback','unpublish'));

-- Translation columns of the version tables (CONTENT_MODEL §8).
ALTER TABLE content.question_versions
  ADD CONSTRAINT question_versions_translation_ck CHECK (translation_status IS NULL OR translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT question_versions_translation_meta_ck CHECK (translation_meta IS NULL OR octet_length(translation_meta::text) <= 65536);
ALTER TABLE content.question_set_versions
  ADD CONSTRAINT question_set_versions_translation_ck CHECK (translation_status IS NULL OR translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT question_set_versions_translation_meta_ck CHECK (translation_meta IS NULL OR octet_length(translation_meta::text) <= 65536);
ALTER TABLE content.lesson_versions
  ADD CONSTRAINT lesson_versions_translation_ck CHECK (translation_status IS NULL OR translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT lesson_versions_translation_meta_ck CHECK (translation_meta IS NULL OR octet_length(translation_meta::text) <= 65536);
ALTER TABLE content.learning_material_versions
  ADD CONSTRAINT learning_material_versions_translation_ck CHECK (translation_status IS NULL OR translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT learning_material_versions_translation_meta_ck CHECK (translation_meta IS NULL OR octet_length(translation_meta::text) <= 65536);
ALTER TABLE content.assessment_blueprint_versions
  ADD CONSTRAINT assessment_blueprint_versions_translation_ck CHECK (translation_status IS NULL OR translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT assessment_blueprint_versions_translation_meta_ck CHECK (translation_meta IS NULL OR octet_length(translation_meta::text) <= 65536);

-- RLS T*: tenant condition only; platform rows (tenant_id NULL) are read and written by learning_platform (BYPASSRLS)
-- under PlatformScope (ADR-0089 §1–2).
ALTER TABLE content.review_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.review_tasks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.review_tasks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.review_tasks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.review_tasks FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.translation_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.translation_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.translation_jobs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.translation_jobs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.translation_jobs FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows (ADR-0089 §1): learning_readonly reads the _live views only. The version views are
-- recreated so that SELECT * includes the added columns.
CREATE OR REPLACE VIEW content.review_tasks_live WITH (security_barrier = true) AS
  SELECT * FROM content.review_tasks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.translation_jobs_live WITH (security_barrier = true) AS
  SELECT * FROM content.translation_jobs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.question_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.question_set_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_set_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.lesson_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.lesson_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.learning_material_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.learning_material_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.assessment_blueprint_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.assessment_blueprint_versions WHERE deleted_at IS NULL;
GRANT SELECT ON content.review_tasks_live, content.translation_jobs_live TO learning_readonly;
REVOKE ALL ON content.review_tasks, content.translation_jobs FROM learning_readonly;
