-- F024 — question sets, lessons, learning materials, assessment blueprints, full-text search (specs/024-question-set-
-- lesson-blueprint: data-model §1–§3; CONTENT_MODEL §2, §9, §9.2; EXAM_ENGINE §3; QUESTION_BANK §6; DATABASE_DESIGN §2.5;
-- ADR-0015, ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089). Applied by `migrate --module=learning` as learning_migrator.
-- Order: ent tables (entddl block, golden test modules/learning/ent/ddl_test.go), then unaccent and the search
-- functions, generated search columns + GIN, CHECKs, FKs, RLS H, _live views, immutability and release-target triggers,
-- the import kinds of F024.

-- BEGIN entddl
CREATE TABLE content.question_sets (
  question_set_id uuid NOT NULL,
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
  PRIMARY KEY (question_set_id)
);

CREATE TABLE content.question_set_versions (
  question_set_version_id uuid NOT NULL,
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
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  curriculum_version_id uuid,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  question_set_id uuid NOT NULL,
  title jsonb NOT NULL,
  summary jsonb,
  subject_id uuid,
  grade_levels jsonb NOT NULL,
  topic_ids jsonb NOT NULL,
  objective_ids jsonb NOT NULL,
  media_refs jsonb NOT NULL,
  worked_example_ids jsonb NOT NULL,
  body jsonb NOT NULL,
  PRIMARY KEY (question_set_version_id)
);
CREATE UNIQUE INDEX question_set_versions_no_uq ON content.question_set_versions (question_set_id, version_no);
CREATE UNIQUE INDEX question_set_versions_open_uq ON content.question_set_versions (question_set_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;
CREATE INDEX question_set_versions_subject_idx ON content.question_set_versions (tenant_id, subject_id) WHERE deleted_at IS NULL;

CREATE TABLE content.question_set_items (
  question_set_item_id uuid NOT NULL,
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
  question_set_id uuid NOT NULL,
  question_set_version_id uuid NOT NULL,
  position smallint NOT NULL,
  question_id uuid NOT NULL,
  question_version_id uuid NOT NULL,
  max_score numeric(7,2),
  PRIMARY KEY (question_set_item_id)
);
CREATE INDEX question_set_items_qv_idx ON content.question_set_items (question_version_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX question_set_items_uq ON content.question_set_items (question_set_version_id, question_version_id) WHERE deleted_at IS NULL;

CREATE TABLE content.lessons (
  lesson_id uuid NOT NULL,
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
  PRIMARY KEY (lesson_id)
);

CREATE TABLE content.lesson_versions (
  lesson_version_id uuid NOT NULL,
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
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  curriculum_version_id uuid,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  lesson_id uuid NOT NULL,
  title jsonb NOT NULL,
  summary jsonb,
  subject_id uuid,
  grade_levels jsonb NOT NULL,
  topic_ids jsonb NOT NULL,
  objective_ids jsonb NOT NULL,
  media_refs jsonb NOT NULL,
  worked_example_ids jsonb NOT NULL,
  body jsonb NOT NULL,
  PRIMARY KEY (lesson_version_id)
);
CREATE UNIQUE INDEX lesson_versions_no_uq ON content.lesson_versions (lesson_id, version_no);
CREATE UNIQUE INDEX lesson_versions_open_uq ON content.lesson_versions (lesson_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;
CREATE INDEX lesson_versions_subject_idx ON content.lesson_versions (tenant_id, subject_id) WHERE deleted_at IS NULL;

CREATE TABLE content.learning_materials (
  learning_material_id uuid NOT NULL,
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
  PRIMARY KEY (learning_material_id)
);

CREATE TABLE content.learning_material_versions (
  learning_material_version_id uuid NOT NULL,
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
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  curriculum_version_id uuid,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  learning_material_id uuid NOT NULL,
  title jsonb NOT NULL,
  summary jsonb,
  subject_id uuid,
  grade_levels jsonb NOT NULL,
  topic_ids jsonb NOT NULL,
  objective_ids jsonb NOT NULL,
  media_refs jsonb NOT NULL,
  worked_example_ids jsonb NOT NULL,
  body jsonb NOT NULL,
  PRIMARY KEY (learning_material_version_id)
);
CREATE UNIQUE INDEX learning_material_versions_no_uq ON content.learning_material_versions (learning_material_id, version_no);
CREATE UNIQUE INDEX learning_material_versions_open_uq ON content.learning_material_versions (learning_material_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;
CREATE INDEX learning_material_versions_subject_idx ON content.learning_material_versions (tenant_id, subject_id) WHERE deleted_at IS NULL;

CREATE TABLE content.assessment_blueprints (
  assessment_blueprint_id uuid NOT NULL,
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
  PRIMARY KEY (assessment_blueprint_id)
);

CREATE TABLE content.assessment_blueprint_versions (
  assessment_blueprint_version_id uuid NOT NULL,
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
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  curriculum_version_id uuid,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  assessment_blueprint_id uuid NOT NULL,
  title jsonb NOT NULL,
  summary jsonb,
  subject_id uuid,
  grade_levels jsonb NOT NULL,
  topic_ids jsonb NOT NULL,
  objective_ids jsonb NOT NULL,
  media_refs jsonb NOT NULL,
  worked_example_ids jsonb NOT NULL,
  body jsonb NOT NULL,
  PRIMARY KEY (assessment_blueprint_version_id)
);
CREATE UNIQUE INDEX assessment_blueprint_versions_no_uq ON content.assessment_blueprint_versions (assessment_blueprint_id, version_no);
CREATE UNIQUE INDEX assessment_blueprint_versions_open_uq ON content.assessment_blueprint_versions (assessment_blueprint_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;
CREATE INDEX assessment_blueprint_versions_subject_idx ON content.assessment_blueprint_versions (tenant_id, subject_id) WHERE deleted_at IS NULL;
-- END entddl

-- Full-text search without accents (research F024 R1, API_DESIGN §5): unaccent is a trusted extension; the IMMUTABLE
-- wrappers make it usable in generated columns. f_search_text keeps every string value of a JSON document.
CREATE EXTENSION IF NOT EXISTS unaccent WITH SCHEMA content;
CREATE FUNCTION content.f_unaccent(text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT content.unaccent('content.unaccent'::regdictionary, $1) $$;
CREATE FUNCTION content.f_search_text(VARIADIC docs jsonb[]) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE
  AS $$ SELECT content.f_unaccent(lower(left(coalesce(string_agg(v #>> '{}', ' '), ''), 20000)))
        FROM unnest(docs) AS d, jsonb_path_query(coalesce(d, '[]'::jsonb), 'strict $.** ? (@.type() == "string")') AS v $$;
GRANT EXECUTE ON FUNCTION content.f_unaccent(text), content.f_search_text(jsonb[]) TO learning_app, learning_platform, learning_readonly;

-- The archive transition of a published version compares the rows without bookkeeping columns: the generated search
-- column is bookkeeping too.
CREATE OR REPLACE FUNCTION content.forbid_published_update() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  bookkeeping text[] := ARRAY['status', 'updated_at', 'updated_by', 'version', 'search_tsv'];
BEGIN
  IF OLD.status IN ('published', 'archived') AND NOT (
       OLD.status = 'published' AND NEW.status = 'archived'
       AND (to_jsonb(NEW) - bookkeeping) = (to_jsonb(OLD) - bookkeeping)) THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;

-- searchQuestions q (F024-AC-17): stem and render payload of the question versions.
ALTER TABLE content.question_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(stem, render_payload))) STORED;
CREATE INDEX question_versions_search_gin ON content.question_versions USING gin (search_tsv) WHERE deleted_at IS NULL;

-- question_sets / question_set_versions (H).
ALTER TABLE content.question_sets
  ADD CONSTRAINT question_sets_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT question_sets_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT question_sets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT question_sets_latest_ck CHECK (latest_version_no >= 0);
ALTER TABLE content.question_set_versions
  ADD CONSTRAINT question_set_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT question_set_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT question_set_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT question_set_versions_item_fk FOREIGN KEY (question_set_id) REFERENCES content.question_sets (question_set_id),
  ADD CONSTRAINT question_set_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT question_set_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT question_set_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT question_set_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT question_set_versions_title_ck CHECK (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 2048),
  ADD CONSTRAINT question_set_versions_summary_ck CHECK (summary IS NULL OR (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 81920)),
  ADD CONSTRAINT question_set_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT question_set_versions_topics_ck CHECK (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50),
  ADD CONSTRAINT question_set_versions_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT question_set_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT question_set_versions_worked_examples_ck CHECK (jsonb_typeof(worked_example_ids) = 'array' AND jsonb_array_length(worked_example_ids) <= 100),
  ADD CONSTRAINT question_set_versions_body_ck CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 524288);
ALTER TABLE content.question_set_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(title, summary, body))) STORED;
CREATE INDEX question_set_versions_search_gin ON content.question_set_versions USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX question_set_versions_topics_gin ON content.question_set_versions USING gin (topic_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX question_set_versions_objectives_gin ON content.question_set_versions USING gin (objective_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX question_set_versions_worked_examples_gin ON content.question_set_versions USING gin (worked_example_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE content.question_sets ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_sets FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_sets FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question_set' AND a.resource_id = question_sets.question_set_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_sets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_sets FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.question_set_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_set_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_set_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question_set' AND a.resource_id = question_set_versions.question_set_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_set_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_set_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE TRIGGER question_set_versions_immutable BEFORE UPDATE ON content.question_set_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.question_sets_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.question_set_versions v
       WHERE v.question_set_version_id = NEW.current_version_id AND v.question_set_id = NEW.question_set_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER question_sets_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.question_sets
  FOR EACH ROW EXECUTE FUNCTION content.question_sets_release_target();
CREATE OR REPLACE VIEW content.question_sets_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_sets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.question_set_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_set_versions WHERE deleted_at IS NULL;
GRANT SELECT ON content.question_sets_live, content.question_set_versions_live TO learning_readonly;
REVOKE ALL ON content.question_sets, content.question_set_versions FROM learning_readonly;

-- lessons / lesson_versions (H).
ALTER TABLE content.lessons
  ADD CONSTRAINT lessons_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT lessons_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT lessons_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT lessons_latest_ck CHECK (latest_version_no >= 0);
ALTER TABLE content.lesson_versions
  ADD CONSTRAINT lesson_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT lesson_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT lesson_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT lesson_versions_item_fk FOREIGN KEY (lesson_id) REFERENCES content.lessons (lesson_id),
  ADD CONSTRAINT lesson_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT lesson_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT lesson_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT lesson_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT lesson_versions_title_ck CHECK (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 2048),
  ADD CONSTRAINT lesson_versions_summary_ck CHECK (summary IS NULL OR (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 81920)),
  ADD CONSTRAINT lesson_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT lesson_versions_topics_ck CHECK (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50),
  ADD CONSTRAINT lesson_versions_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT lesson_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT lesson_versions_worked_examples_ck CHECK (jsonb_typeof(worked_example_ids) = 'array' AND jsonb_array_length(worked_example_ids) <= 100),
  ADD CONSTRAINT lesson_versions_body_ck CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 524288);
ALTER TABLE content.lesson_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(title, summary, body))) STORED;
CREATE INDEX lesson_versions_search_gin ON content.lesson_versions USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX lesson_versions_topics_gin ON content.lesson_versions USING gin (topic_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX lesson_versions_objectives_gin ON content.lesson_versions USING gin (objective_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX lesson_versions_worked_examples_gin ON content.lesson_versions USING gin (worked_example_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE content.lessons ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.lessons FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.lessons FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'lesson' AND a.resource_id = lessons.lesson_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.lessons FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.lessons FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.lesson_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.lesson_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.lesson_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'lesson' AND a.resource_id = lesson_versions.lesson_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.lesson_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.lesson_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE TRIGGER lesson_versions_immutable BEFORE UPDATE ON content.lesson_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.lessons_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.lesson_versions v
       WHERE v.lesson_version_id = NEW.current_version_id AND v.lesson_id = NEW.lesson_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER lessons_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.lessons
  FOR EACH ROW EXECUTE FUNCTION content.lessons_release_target();
CREATE OR REPLACE VIEW content.lessons_live WITH (security_barrier = true) AS
  SELECT * FROM content.lessons WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.lesson_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.lesson_versions WHERE deleted_at IS NULL;
GRANT SELECT ON content.lessons_live, content.lesson_versions_live TO learning_readonly;
REVOKE ALL ON content.lessons, content.lesson_versions FROM learning_readonly;

-- learning_materials / learning_material_versions (H).
ALTER TABLE content.learning_materials
  ADD CONSTRAINT learning_materials_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT learning_materials_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT learning_materials_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT learning_materials_latest_ck CHECK (latest_version_no >= 0);
ALTER TABLE content.learning_material_versions
  ADD CONSTRAINT learning_material_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT learning_material_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT learning_material_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT learning_material_versions_item_fk FOREIGN KEY (learning_material_id) REFERENCES content.learning_materials (learning_material_id),
  ADD CONSTRAINT learning_material_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT learning_material_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT learning_material_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT learning_material_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT learning_material_versions_title_ck CHECK (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 2048),
  ADD CONSTRAINT learning_material_versions_summary_ck CHECK (summary IS NULL OR (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 81920)),
  ADD CONSTRAINT learning_material_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT learning_material_versions_topics_ck CHECK (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50),
  ADD CONSTRAINT learning_material_versions_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT learning_material_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT learning_material_versions_worked_examples_ck CHECK (jsonb_typeof(worked_example_ids) = 'array' AND jsonb_array_length(worked_example_ids) <= 100),
  ADD CONSTRAINT learning_material_versions_body_ck CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 524288);
ALTER TABLE content.learning_material_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(title, summary, body))) STORED;
CREATE INDEX learning_material_versions_search_gin ON content.learning_material_versions USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX learning_material_versions_topics_gin ON content.learning_material_versions USING gin (topic_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX learning_material_versions_objectives_gin ON content.learning_material_versions USING gin (objective_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX learning_material_versions_worked_examples_gin ON content.learning_material_versions USING gin (worked_example_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE content.learning_materials ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.learning_materials FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.learning_materials FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'learning_material' AND a.resource_id = learning_materials.learning_material_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.learning_materials FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.learning_materials FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.learning_material_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.learning_material_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.learning_material_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'learning_material' AND a.resource_id = learning_material_versions.learning_material_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.learning_material_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.learning_material_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE TRIGGER learning_material_versions_immutable BEFORE UPDATE ON content.learning_material_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.learning_materials_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.learning_material_versions v
       WHERE v.learning_material_version_id = NEW.current_version_id AND v.learning_material_id = NEW.learning_material_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER learning_materials_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.learning_materials
  FOR EACH ROW EXECUTE FUNCTION content.learning_materials_release_target();
CREATE OR REPLACE VIEW content.learning_materials_live WITH (security_barrier = true) AS
  SELECT * FROM content.learning_materials WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.learning_material_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.learning_material_versions WHERE deleted_at IS NULL;
GRANT SELECT ON content.learning_materials_live, content.learning_material_versions_live TO learning_readonly;
REVOKE ALL ON content.learning_materials, content.learning_material_versions FROM learning_readonly;

-- assessment_blueprints / assessment_blueprint_versions (H).
ALTER TABLE content.assessment_blueprints
  ADD CONSTRAINT assessment_blueprints_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT assessment_blueprints_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT assessment_blueprints_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT assessment_blueprints_latest_ck CHECK (latest_version_no >= 0);
ALTER TABLE content.assessment_blueprint_versions
  ADD CONSTRAINT assessment_blueprint_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT assessment_blueprint_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT assessment_blueprint_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT assessment_blueprint_versions_item_fk FOREIGN KEY (assessment_blueprint_id) REFERENCES content.assessment_blueprints (assessment_blueprint_id),
  ADD CONSTRAINT assessment_blueprint_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT assessment_blueprint_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT assessment_blueprint_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT assessment_blueprint_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT assessment_blueprint_versions_title_ck CHECK (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 2048),
  ADD CONSTRAINT assessment_blueprint_versions_summary_ck CHECK (summary IS NULL OR (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 81920)),
  ADD CONSTRAINT assessment_blueprint_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT assessment_blueprint_versions_topics_ck CHECK (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50),
  ADD CONSTRAINT assessment_blueprint_versions_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT assessment_blueprint_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT assessment_blueprint_versions_worked_examples_ck CHECK (jsonb_typeof(worked_example_ids) = 'array' AND jsonb_array_length(worked_example_ids) <= 100),
  ADD CONSTRAINT assessment_blueprint_versions_body_ck CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 524288);
ALTER TABLE content.assessment_blueprint_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(title, summary, body))) STORED;
CREATE INDEX assessment_blueprint_versions_search_gin ON content.assessment_blueprint_versions USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX assessment_blueprint_versions_topics_gin ON content.assessment_blueprint_versions USING gin (topic_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX assessment_blueprint_versions_objectives_gin ON content.assessment_blueprint_versions USING gin (objective_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX assessment_blueprint_versions_worked_examples_gin ON content.assessment_blueprint_versions USING gin (worked_example_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE content.assessment_blueprints ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.assessment_blueprints FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.assessment_blueprints FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'assessment_blueprint' AND a.resource_id = assessment_blueprints.assessment_blueprint_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.assessment_blueprints FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.assessment_blueprints FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.assessment_blueprint_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.assessment_blueprint_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.assessment_blueprint_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'assessment_blueprint' AND a.resource_id = assessment_blueprint_versions.assessment_blueprint_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.assessment_blueprint_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.assessment_blueprint_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE TRIGGER assessment_blueprint_versions_immutable BEFORE UPDATE ON content.assessment_blueprint_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.assessment_blueprints_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.assessment_blueprint_versions v
       WHERE v.assessment_blueprint_version_id = NEW.current_version_id AND v.assessment_blueprint_id = NEW.assessment_blueprint_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER assessment_blueprints_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.assessment_blueprints
  FOR EACH ROW EXECUTE FUNCTION content.assessment_blueprints_release_target();
CREATE OR REPLACE VIEW content.assessment_blueprints_live WITH (security_barrier = true) AS
  SELECT * FROM content.assessment_blueprints WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.assessment_blueprint_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.assessment_blueprint_versions WHERE deleted_at IS NULL;
GRANT SELECT ON content.assessment_blueprints_live, content.assessment_blueprint_versions_live TO learning_readonly;
REVOKE ALL ON content.assessment_blueprints, content.assessment_blueprint_versions FROM learning_readonly;

-- question_set_items (H through the set): fixed items of a draft only.
ALTER TABLE content.question_set_items
  ADD CONSTRAINT question_set_items_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT question_set_items_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT question_set_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT question_set_items_version_fk FOREIGN KEY (question_set_version_id) REFERENCES content.question_set_versions (question_set_version_id),
  ADD CONSTRAINT question_set_items_question_version_fk FOREIGN KEY (question_version_id) REFERENCES content.question_versions (question_version_id),
  ADD CONSTRAINT question_set_items_position_ck CHECK (position BETWEEN 1 AND 200),
  ADD CONSTRAINT question_set_items_score_ck CHECK (max_score IS NULL OR max_score > 0);
ALTER TABLE content.question_set_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_set_items FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_set_items FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question_set' AND a.resource_id = question_set_items.question_set_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_set_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_set_items FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW content.question_set_items_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_set_items WHERE deleted_at IS NULL;
GRANT SELECT ON content.question_set_items_live TO learning_readonly;
REVOKE ALL ON content.question_set_items FROM learning_readonly;
CREATE FUNCTION content.forbid_set_item_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  s text;
BEGIN
  SELECT v.status INTO s FROM content.question_set_versions v WHERE v.question_set_version_id = NEW.question_set_version_id;
  IF s IS NOT NULL AND s <> 'draft' THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER question_set_items_draft_only BEFORE INSERT OR UPDATE ON content.question_set_items
  FOR EACH ROW EXECUTE FUNCTION content.forbid_set_item_change();

-- Import kinds of F024 (questions_csv, questions_xlsx, questions_qti_lite, content_json).
ALTER TABLE content.import_jobs DROP CONSTRAINT import_jobs_kind_ck;
ALTER TABLE content.import_jobs ADD CONSTRAINT import_jobs_kind_ck CHECK (kind IN ('curriculum_yaml','curriculum_csv','questions_csv',
  'questions_xlsx','questions_qti_lite','content_json'));
