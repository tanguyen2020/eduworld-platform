-- F021 — curriculum structure (specs/021-curriculum: data-model §1, §3; CURRICULUM_MODEL §2–§7; DATABASE_DESIGN §2.4,
-- §2.16; ADR-0005, ADR-0015, ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089). Applied by `migrate --module=learning`
-- as learning_migrator. Order: ent tables (entddl block, golden test modules/learning/ent/ddl_test.go), then what ent
-- cannot express: CHECKs, FKs, expression unique indexes, RLS, _live views, grants, triggers.

-- BEGIN entddl
CREATE TABLE curriculum.curricula (
  curriculum_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  code varchar(64) NOT NULL,
  name jsonb NOT NULL,
  framework varchar(64),
  PRIMARY KEY (curriculum_id)
);

CREATE TABLE curriculum.curriculum_versions (
  curriculum_version_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  version_no bigint NOT NULL,
  version_label varchar(16) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  effective_from date,
  effective_to date,
  supersedes_version_id uuid,
  based_on_version_id uuid,
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  change_note varchar(1000),
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  PRIMARY KEY (curriculum_version_id)
);
CREATE UNIQUE INDEX curriculum_versions_label_uq ON curriculum.curriculum_versions (curriculum_id, version_label) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX curriculum_versions_no_uq ON curriculum.curriculum_versions (curriculum_id, version_no);

CREATE TABLE curriculum.subjects (
  subject_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name jsonb NOT NULL,
  grade_levels jsonb NOT NULL,
  icon_key varchar(64),
  PRIMARY KEY (subject_id)
);
CREATE UNIQUE INDEX subjects_code_uq ON curriculum.subjects (code) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.subject_grades (
  subject_grade_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  curriculum_version_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  periods_per_year smallint,
  PRIMARY KEY (subject_grade_id)
);
CREATE UNIQUE INDEX subject_grades_uq ON curriculum.subject_grades (curriculum_version_id, subject_id, grade_level) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.topics (
  topic_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  curriculum_version_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  parent_topic_id uuid,
  code varchar(64) NOT NULL,
  name varchar(255) NOT NULL,
  sort_order integer NOT NULL DEFAULT 1,
  PRIMARY KEY (topic_id)
);
CREATE UNIQUE INDEX topics_code_uq ON curriculum.topics (curriculum_version_id, code) WHERE deleted_at IS NULL;
CREATE INDEX topics_tree_idx ON curriculum.topics (curriculum_version_id, subject_id, grade_level, sort_order) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.learning_objectives (
  objective_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  curriculum_version_id uuid NOT NULL,
  topic_id uuid NOT NULL,
  code varchar(64) NOT NULL,
  stable_key varchar(64) NOT NULL,
  description varchar(1000) NOT NULL,
  bloom_level varchar(64),
  yccd_ref varchar(64),
  competency_ids jsonb NOT NULL,
  prerequisite_ids jsonb NOT NULL,
  sort_order integer NOT NULL DEFAULT 1,
  PRIMARY KEY (objective_id)
);
CREATE UNIQUE INDEX learning_objectives_code_uq ON curriculum.learning_objectives (curriculum_version_id, code) WHERE deleted_at IS NULL;
CREATE INDEX learning_objectives_topic_idx ON curriculum.learning_objectives (curriculum_version_id, topic_id) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.competencies (
  competency_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  curriculum_version_id uuid NOT NULL,
  code varchar(64) NOT NULL,
  name jsonb NOT NULL,
  kind varchar(64) NOT NULL,
  subject_id uuid,
  PRIMARY KEY (competency_id)
);
CREATE UNIQUE INDEX competencies_code_uq ON curriculum.competencies (curriculum_version_id, code) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.lo_version_mappings (
  mapping_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  curriculum_id uuid NOT NULL,
  curriculum_version_id uuid NOT NULL,
  from_lo_id uuid NOT NULL,
  to_lo_id uuid,
  relation varchar(64) NOT NULL,
  PRIMARY KEY (mapping_id)
);

CREATE TABLE curriculum.textbook_series (
  series_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name varchar(255) NOT NULL,
  publisher varchar(255),
  PRIMARY KEY (series_id)
);
CREATE UNIQUE INDEX textbook_series_code_uq ON curriculum.textbook_series (code) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.textbooks (
  textbook_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  series_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  volume smallint NOT NULL,
  PRIMARY KEY (textbook_id)
);
CREATE UNIQUE INDEX textbooks_uq ON curriculum.textbooks (series_id, subject_id, grade_level, volume) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.textbook_sections (
  section_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  textbook_id uuid NOT NULL,
  chapter varchar(128),
  lesson_no smallint NOT NULL,
  title varchar(255) NOT NULL,
  page_range varchar(16),
  PRIMARY KEY (section_id)
);
CREATE INDEX textbook_sections_textbook_idx ON curriculum.textbook_sections (textbook_id, lesson_no) WHERE deleted_at IS NULL;

CREATE TABLE curriculum.textbook_section_objectives (
  section_objective_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  section_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  PRIMARY KEY (section_objective_id)
);
CREATE INDEX textbook_section_objectives_lo_idx ON curriculum.textbook_section_objectives (objective_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX textbook_section_objectives_uq ON curriculum.textbook_section_objectives (section_id, objective_id) WHERE deleted_at IS NULL;

CREATE TABLE content.import_jobs (
  import_id uuid NOT NULL,
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
  asset_id uuid,
  source_ref varchar(512),
  dry_run boolean NOT NULL DEFAULT false,
  status varchar(64) NOT NULL DEFAULT 'queued',
  created_count integer NOT NULL DEFAULT 0,
  rejected_count integer NOT NULL DEFAULT 0,
  errors jsonb NOT NULL,
  result_refs jsonb NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  finished_at timestamptz,
  PRIMARY KEY (import_id)
);
CREATE UNIQUE INDEX import_jobs_idem_uq ON content.import_jobs (created_by, idempotency_key);
CREATE INDEX import_jobs_status_idx ON content.import_jobs (status, created_at);
-- END entddl

-- Hybrid tables (H): owner CHECK (F020-AC-08) and tenant FK to the projection.
ALTER TABLE curriculum.curricula
  ADD CONSTRAINT curricula_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT curricula_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT curricula_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT curricula_code_ck CHECK (code ~ '^[A-Z][A-Z0-9_]{1,63}$'),
  ADD CONSTRAINT curricula_name_ck CHECK (jsonb_typeof(name) = 'object' AND octet_length(name::text) <= 2048);
-- Code unique per owner scope (platform; each tenant): a tenant never learns another tenant's codes (spec Assumptions).
CREATE UNIQUE INDEX curricula_code_uq ON curriculum.curricula
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), code) WHERE deleted_at IS NULL;

ALTER TABLE curriculum.curriculum_versions
  ADD CONSTRAINT curriculum_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT curriculum_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT curriculum_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT curriculum_versions_curriculum_fk FOREIGN KEY (curriculum_id) REFERENCES curriculum.curricula (curriculum_id),
  ADD CONSTRAINT curriculum_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT curriculum_versions_label_ck CHECK (version_label ~ '^[0-9]{1,4}\.[0-9]{1,4}$'),
  ADD CONSTRAINT curriculum_versions_effective_ck CHECK (effective_to IS NULL OR effective_from IS NULL OR effective_to >= effective_from),
  ADD CONSTRAINT curriculum_versions_language_ck CHECK (content_language IN ('vi','en'));

ALTER TABLE curriculum.subject_grades
  ADD CONSTRAINT subject_grades_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT subject_grades_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT subject_grades_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT subject_grades_version_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT subject_grades_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT subject_grades_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT subject_grades_periods_ck CHECK (periods_per_year IS NULL OR periods_per_year > 0);

ALTER TABLE curriculum.topics
  ADD CONSTRAINT topics_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT topics_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT topics_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT topics_version_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT topics_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT topics_parent_fk FOREIGN KEY (parent_topic_id) REFERENCES curriculum.topics (topic_id),
  ADD CONSTRAINT topics_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT topics_sort_ck CHECK (sort_order >= 1),
  ADD CONSTRAINT topics_parent_self_ck CHECK (parent_topic_id IS NULL OR parent_topic_id <> topic_id);

ALTER TABLE curriculum.learning_objectives
  ADD CONSTRAINT learning_objectives_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT learning_objectives_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT learning_objectives_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT learning_objectives_version_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT learning_objectives_topic_fk FOREIGN KEY (topic_id) REFERENCES curriculum.topics (topic_id),
  ADD CONSTRAINT learning_objectives_code_ck CHECK (code ~ '^[A-Z]{2,8}\.G[1-9]\.[A-Z]{1,6}\.[A-Z][A-Z0-9_]{0,40}(\.[0-9]{1,2})?$'),
  ADD CONSTRAINT learning_objectives_bloom_ck CHECK (bloom_level IS NULL OR bloom_level IN ('remember','understand','apply','analyze','evaluate','create')),
  ADD CONSTRAINT learning_objectives_competencies_ck CHECK (jsonb_typeof(competency_ids) = 'array' AND jsonb_array_length(competency_ids) <= 20),
  ADD CONSTRAINT learning_objectives_prerequisites_ck CHECK (jsonb_typeof(prerequisite_ids) = 'array' AND jsonb_array_length(prerequisite_ids) <= 20),
  ADD CONSTRAINT learning_objectives_sort_ck CHECK (sort_order >= 1);
-- Reverse lookup of prerequisites ("which LOs depend on this one", deletion check, F021 edge case).
CREATE INDEX learning_objectives_prerequisites_gin ON curriculum.learning_objectives USING gin (prerequisite_ids jsonb_path_ops)
  WHERE deleted_at IS NULL;

ALTER TABLE curriculum.competencies
  ADD CONSTRAINT competencies_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT competencies_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT competencies_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT competencies_version_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT competencies_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT competencies_code_ck CHECK (code ~ '^[A-Z][A-Z0-9_]{1,63}$'),
  ADD CONSTRAINT competencies_kind_ck CHECK (kind IN ('general_competency','subject_competency','quality')),
  ADD CONSTRAINT competencies_subject_ck CHECK ((kind = 'subject_competency') = (subject_id IS NOT NULL)),
  ADD CONSTRAINT competencies_name_ck CHECK (jsonb_typeof(name) = 'object' AND octet_length(name::text) <= 2048);

ALTER TABLE curriculum.lo_version_mappings
  ADD CONSTRAINT lo_version_mappings_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT lo_version_mappings_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT lo_version_mappings_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT lo_version_mappings_version_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT lo_version_mappings_from_fk FOREIGN KEY (from_lo_id) REFERENCES curriculum.learning_objectives (objective_id),
  ADD CONSTRAINT lo_version_mappings_to_fk FOREIGN KEY (to_lo_id) REFERENCES curriculum.learning_objectives (objective_id),
  ADD CONSTRAINT lo_version_mappings_relation_ck CHECK (relation IN ('same','split','merge','replaces','removed')),
  ADD CONSTRAINT lo_version_mappings_removed_ck CHECK ((relation = 'removed') = (to_lo_id IS NULL));
CREATE UNIQUE INDEX lo_version_mappings_uq ON curriculum.lo_version_mappings
  (curriculum_version_id, from_lo_id, coalesce(to_lo_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE deleted_at IS NULL;

-- Global catalogs (G): platform data, no tenant, no RLS (ADR-0089 §4).
ALTER TABLE curriculum.subjects
  ADD CONSTRAINT subjects_code_ck CHECK (code ~ '^[a-z][a-z0-9_]{1,63}$'),
  ADD CONSTRAINT subjects_name_ck CHECK (jsonb_typeof(name) = 'object' AND octet_length(name::text) <= 2048),
  ADD CONSTRAINT subjects_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) BETWEEN 1 AND 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)'));
ALTER TABLE curriculum.textbook_series
  ADD CONSTRAINT textbook_series_code_ck CHECK (code ~ '^[A-Z][A-Z0-9_]{1,63}$');
ALTER TABLE curriculum.textbooks
  ADD CONSTRAINT textbooks_series_fk FOREIGN KEY (series_id) REFERENCES curriculum.textbook_series (series_id),
  ADD CONSTRAINT textbooks_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT textbooks_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT textbooks_volume_ck CHECK (volume BETWEEN 1 AND 9);
ALTER TABLE curriculum.textbook_sections
  ADD CONSTRAINT textbook_sections_textbook_fk FOREIGN KEY (textbook_id) REFERENCES curriculum.textbooks (textbook_id),
  ADD CONSTRAINT textbook_sections_lesson_ck CHECK (lesson_no >= 1),
  ADD CONSTRAINT textbook_sections_pages_ck CHECK (page_range IS NULL OR page_range ~ '^[0-9]{1,4}(-[0-9]{1,4})?$');
ALTER TABLE curriculum.textbook_section_objectives
  ADD CONSTRAINT textbook_section_objectives_section_fk FOREIGN KEY (section_id) REFERENCES curriculum.textbook_sections (section_id),
  ADD CONSTRAINT textbook_section_objectives_lo_fk FOREIGN KEY (objective_id) REFERENCES curriculum.learning_objectives (objective_id);
-- Platform catalogs are written only by platform staff (learning_platform under PlatformScope); learning_app reads.
REVOKE INSERT, UPDATE ON curriculum.subjects, curriculum.textbook_series, curriculum.textbooks, curriculum.textbook_sections,
  curriculum.textbook_section_objectives FROM learning_app;

-- content.import_jobs (T*): tenant jobs, or platform jobs (tenant_id NULL) written under PlatformScope.
ALTER TABLE content.import_jobs
  ADD CONSTRAINT import_jobs_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT import_jobs_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT import_jobs_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT import_jobs_kind_ck CHECK (kind IN ('curriculum_yaml','curriculum_csv')),
  ADD CONSTRAINT import_jobs_status_ck CHECK (status IN ('queued','running','succeeded','failed')),
  ADD CONSTRAINT import_jobs_counts_ck CHECK (created_count >= 0 AND rejected_count >= 0),
  ADD CONSTRAINT import_jobs_source_ck CHECK (asset_id IS NOT NULL OR source_ref IS NOT NULL),
  ADD CONSTRAINT import_jobs_errors_ck CHECK (jsonb_typeof(errors) = 'array' AND jsonb_array_length(errors) <= 500
    AND octet_length(errors::text) <= 262144),
  ADD CONSTRAINT import_jobs_result_ck CHECK (octet_length(result_refs::text) <= 65536);

-- RLS (ADR-0089 §1): tenant condition only, never deleted_at. H: tenant rows, or platform rows whose curriculum is
-- assigned to the session tenant (resource_type curriculum, matched on curriculum_id).
ALTER TABLE curriculum.curricula ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.curricula FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.curricula FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = curricula.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.curricula FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.curricula FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.curriculum_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.curriculum_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.curriculum_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = curriculum_versions.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.curriculum_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.curriculum_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.subject_grades ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.subject_grades FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.subject_grades FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = subject_grades.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.subject_grades FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.subject_grades FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.topics ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.topics FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.topics FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = topics.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.topics FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.topics FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.learning_objectives ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.learning_objectives FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.learning_objectives FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = learning_objectives.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.learning_objectives FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.learning_objectives FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.competencies ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.competencies FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.competencies FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = competencies.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.competencies FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.competencies FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE curriculum.lo_version_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE curriculum.lo_version_mappings FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON curriculum.lo_version_mappings FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'curriculum' AND a.resource_id = lo_version_mappings.curriculum_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON curriculum.lo_version_mappings FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON curriculum.lo_version_mappings FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.import_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.import_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.import_jobs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.import_jobs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.import_jobs FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows (ADR-0089 §1): learning_readonly reads the _live views only, never the tables.
CREATE OR REPLACE VIEW curriculum.curricula_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.curricula WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.curriculum_versions_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.curriculum_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.subjects_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.subjects WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.subject_grades_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.subject_grades WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.topics_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.topics WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.learning_objectives_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.learning_objectives WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.competencies_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.competencies WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.lo_version_mappings_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.lo_version_mappings WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.textbook_series_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.textbook_series WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.textbooks_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.textbooks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.textbook_sections_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.textbook_sections WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW curriculum.textbook_section_objectives_live WITH (security_barrier = true) AS
  SELECT * FROM curriculum.textbook_section_objectives WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.import_jobs_live WITH (security_barrier = true) AS
  SELECT * FROM content.import_jobs WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA curriculum, content TO learning_readonly;
GRANT SELECT ON curriculum.curricula_live, curriculum.curriculum_versions_live, curriculum.subjects_live,
  curriculum.subject_grades_live, curriculum.topics_live, curriculum.learning_objectives_live, curriculum.competencies_live,
  curriculum.lo_version_mappings_live, curriculum.textbook_series_live, curriculum.textbooks_live,
  curriculum.textbook_sections_live, curriculum.textbook_section_objectives_live, content.import_jobs_live TO learning_readonly;
REVOKE ALL ON curriculum.curricula, curriculum.curriculum_versions, curriculum.subjects, curriculum.subject_grades,
  curriculum.topics, curriculum.learning_objectives, curriculum.competencies, curriculum.lo_version_mappings,
  curriculum.textbook_series, curriculum.textbooks, curriculum.textbook_sections, curriculum.textbook_section_objectives,
  content.import_jobs FROM learning_readonly;

-- Immutability (ADR-0015, F021-AC-16): a published/archived version never changes except published → archived
-- (content.forbid_published_update of F020) …
CREATE TRIGGER curriculum_versions_immutable BEFORE UPDATE ON curriculum.curriculum_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();

-- … and the content of a version (subject × grade, topics, LOs, competencies, LO mappings) changes only while the
-- version is draft (F021-AC-07). The version row is read with the privileges of the caller: it has the owner of
-- the child row, so RLS shows it.
CREATE FUNCTION curriculum.forbid_child_change_when_published() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  s text;
BEGIN
  SELECT v.status INTO s FROM curriculum.curriculum_versions v WHERE v.curriculum_version_id = NEW.curriculum_version_id;
  IF s IS NOT NULL AND s <> 'draft' THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER subject_grades_draft_only BEFORE INSERT OR UPDATE ON curriculum.subject_grades
  FOR EACH ROW EXECUTE FUNCTION curriculum.forbid_child_change_when_published();
CREATE TRIGGER topics_draft_only BEFORE INSERT OR UPDATE ON curriculum.topics
  FOR EACH ROW EXECUTE FUNCTION curriculum.forbid_child_change_when_published();
CREATE TRIGGER learning_objectives_draft_only BEFORE INSERT OR UPDATE ON curriculum.learning_objectives
  FOR EACH ROW EXECUTE FUNCTION curriculum.forbid_child_change_when_published();
CREATE TRIGGER competencies_draft_only BEFORE INSERT OR UPDATE ON curriculum.competencies
  FOR EACH ROW EXECUTE FUNCTION curriculum.forbid_child_change_when_published();
CREATE TRIGGER lo_version_mappings_draft_only BEFORE INSERT OR UPDATE ON curriculum.lo_version_mappings
  FOR EACH ROW EXECUTE FUNCTION curriculum.forbid_child_change_when_published();
