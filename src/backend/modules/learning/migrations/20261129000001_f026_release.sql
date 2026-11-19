-- F026 — content release: scheduled publications, published_by, release CHECKs (specs/026-content-release: data-model
-- §1–§3; CONTENT_MODEL §5, §6; DATABASE_DESIGN §3.13; ADR-0015, ADR-0043, ADR-0089). Applied by migrate --module=learning
-- as learning_migrator. Order: ent tables and added columns (entddl block, golden test modules/learning/ent/ddl_test.go),
-- CHECKs, FKs, RLS T*, _live views, grants. The immutability (forbid_published_update) and release-target triggers of
-- the item tables exist since F022/F024. Expand only (AGENTS R12).

-- BEGIN entddl
CREATE TABLE content.scheduled_publications (
  schedule_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  item_type varchar(64) NOT NULL,
  item_id uuid NOT NULL,
  version_id uuid NOT NULL,
  version_no bigint NOT NULL,
  publish_at timestamptz NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'scheduled',
  failure_code varchar(64),
  applied_release_id uuid,
  change_note varchar(500),
  requested_by uuid NOT NULL,
  cancelled_by uuid,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (schedule_id)
);
CREATE INDEX scheduled_publications_due_idx ON content.scheduled_publications (status, publish_at) WHERE status = 'scheduled' AND deleted_at IS NULL;
CREATE UNIQUE INDEX scheduled_publications_idem_uq ON content.scheduled_publications (item_type, item_id, idempotency_key);
CREATE UNIQUE INDEX scheduled_publications_pending_uq ON content.scheduled_publications (item_type, item_id) WHERE status = 'scheduled' AND deleted_at IS NULL;
ALTER TABLE content.question_versions ADD COLUMN published_by uuid;
ALTER TABLE content.question_set_versions ADD COLUMN published_by uuid;
ALTER TABLE content.lesson_versions ADD COLUMN published_by uuid;
ALTER TABLE content.learning_material_versions ADD COLUMN published_by uuid;
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN published_by uuid;
-- END entddl

-- content.scheduled_publications (T*): platform rows (tenant_id NULL) under PlatformScope only.
ALTER TABLE content.scheduled_publications
  ADD CONSTRAINT scheduled_publications_type_ck CHECK (item_type IN ('question','question_set','assessment_blueprint','lesson',
    'learning_material','learning_path','activity')),
  ADD CONSTRAINT scheduled_publications_status_ck CHECK (status IN ('scheduled','applied','cancelled','failed')),
  ADD CONSTRAINT scheduled_publications_applied_ck CHECK ((status = 'applied') = (applied_release_id IS NOT NULL)),
  ADD CONSTRAINT scheduled_publications_failed_ck CHECK ((status = 'failed') = (failure_code IS NOT NULL)),
  ADD CONSTRAINT scheduled_publications_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE content.scheduled_publications ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.scheduled_publications FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.scheduled_publications FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.scheduled_publications FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.scheduled_publications FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- _live views: the version views again (published_by added), the new table.
CREATE OR REPLACE VIEW content.scheduled_publications_live WITH (security_barrier = true) AS
  SELECT * FROM content.scheduled_publications WHERE deleted_at IS NULL;
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
GRANT SELECT ON content.scheduled_publications_live TO learning_readonly;
REVOKE ALL ON content.scheduled_publications FROM learning_readonly;
