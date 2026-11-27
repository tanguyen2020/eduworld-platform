-- F028 — Word import lessons_docx (specs/028-docx-math-import: data-model §1; DATABASE_DESIGN §3.3; CONTENT_PRODUCTION_PLAN §8).
-- Applied by `migrate --module=learning` as learning_migrator. Expand only:
--   * content.import_items (T*, like content.import_jobs): one question of the Word file of a lessons_docx job with its
--     analysis (status, issues, formulas, pictures, parsed authoring fields) and the draft created from it; tenant rows
--     for tenant jobs, tenant_id NULL for platform jobs (PlatformScope). Soft-delete only; application roles have no
--     DELETE/TRUNCATE (baseline default privileges); learning_readonly reads the _live view only.
--   * content.import_jobs: the CHECK of the kinds accepts lessons_docx.
-- BEGIN entddl
CREATE TABLE content.import_items (
  import_item_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  import_id uuid NOT NULL,
  item_no integer NOT NULL,
  source_label varchar(32) NOT NULL,
  status varchar(32) NOT NULL,
  question_type varchar(32),
  stem_preview varchar(200) NOT NULL,
  issues jsonb NOT NULL,
  formulas jsonb NOT NULL,
  images jsonb NOT NULL,
  payload jsonb NOT NULL,
  question_id uuid,
  question_version_id uuid,
  PRIMARY KEY (import_item_id)
);
CREATE UNIQUE INDEX import_items_no_uq ON content.import_items (import_id, item_no) WHERE deleted_at IS NULL;
CREATE INDEX import_items_status_idx ON content.import_items (import_id, status, item_no) WHERE deleted_at IS NULL;
-- END entddl

ALTER TABLE content.import_items
  ADD CONSTRAINT import_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT import_items_job_fk FOREIGN KEY (import_id) REFERENCES content.import_jobs (import_id),
  ADD CONSTRAINT import_items_no_ck CHECK (item_no BETWEEN 1 AND 10000),
  ADD CONSTRAINT import_items_status_ck CHECK (status IN ('ready','warning','error','created','draft_failed')),
  ADD CONSTRAINT import_items_type_ck CHECK (question_type IS NULL OR question_type IN ('single_choice','multiple_choice')),
  ADD CONSTRAINT import_items_created_ck CHECK ((status = 'created') = (question_id IS NOT NULL AND question_version_id IS NOT NULL)),
  ADD CONSTRAINT import_items_issues_ck CHECK (jsonb_typeof(issues) = 'array' AND jsonb_array_length(issues) <= 50),
  ADD CONSTRAINT import_items_formulas_ck CHECK (jsonb_typeof(formulas) = 'array' AND jsonb_array_length(formulas) <= 60),
  ADD CONSTRAINT import_items_images_ck CHECK (jsonb_typeof(images) = 'array' AND jsonb_array_length(images) <= 30),
  ADD CONSTRAINT import_items_payload_ck CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 262144);

ALTER TABLE content.import_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.import_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.import_items FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.import_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.import_items FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

CREATE OR REPLACE VIEW content.import_items_live WITH (security_barrier = true) AS
  SELECT * FROM content.import_items WHERE deleted_at IS NULL;
GRANT SELECT ON content.import_items_live TO learning_readonly;
REVOKE ALL ON content.import_items FROM learning_readonly;

ALTER TABLE content.import_jobs DROP CONSTRAINT import_jobs_kind_ck;
ALTER TABLE content.import_jobs ADD CONSTRAINT import_jobs_kind_ck CHECK (kind IN ('curriculum_yaml','curriculum_csv','questions_csv',
  'questions_xlsx','questions_qti_lite','content_json','lessons_docx'));
