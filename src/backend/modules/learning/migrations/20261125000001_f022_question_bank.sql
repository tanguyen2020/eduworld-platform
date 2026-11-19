-- F022 — question bank core (specs/022-question-bank-core: data-model §1, §3; QUESTION_BANK §2–§8; DATABASE_DESIGN §2.5,
-- §3.3; ADR-0015, ADR-0016, ADR-0041, ADR-0043, ADR-0089). Applied by `migrate --module=learning` as learning_migrator.
-- Order: ent tables (entddl block, golden test modules/learning/ent/ddl_test.go), then CHECKs, FKs, RLS, _live views,
-- triggers and the delivery view without answer columns.

-- BEGIN entddl
CREATE TABLE content.questions (
  question_id uuid NOT NULL,
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
  PRIMARY KEY (question_id)
);
CREATE INDEX questions_owner_idx ON content.questions (owner_scope, tenant_id) WHERE deleted_at IS NULL;

CREATE TABLE content.question_versions (
  question_version_id uuid NOT NULL,
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
  curriculum_version_id uuid NOT NULL,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  question_id uuid NOT NULL,
  type varchar(64) NOT NULL,
  stem jsonb NOT NULL,
  render_payload jsonb NOT NULL,
  answer_key jsonb NOT NULL,
  scoring_rule jsonb NOT NULL,
  explanation jsonb,
  option_feedback jsonb,
  hints jsonb,
  worked_example_ref jsonb,
  min_response_ms integer,
  max_score numeric(7,2) NOT NULL DEFAULT 1,
  exam_only boolean NOT NULL DEFAULT false,
  efficacy_instrument boolean NOT NULL DEFAULT false,
  math_assets jsonb,
  tags jsonb NOT NULL,
  difficulty smallint NOT NULL,
  grade_levels jsonb NOT NULL,
  subject_id uuid NOT NULL,
  topic_id uuid NOT NULL,
  media_refs jsonb NOT NULL,
  content_hash bytea,
  PRIMARY KEY (question_version_id)
);
CREATE INDEX question_versions_delivery_idx ON content.question_versions (tenant_id, subject_id, topic_id, difficulty) WHERE status = 'published' AND deleted_at IS NULL;
CREATE UNIQUE INDEX question_versions_no_uq ON content.question_versions (question_id, version_no);
CREATE UNIQUE INDEX question_versions_open_uq ON content.question_versions (question_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;

CREATE TABLE content.question_version_objectives (
  question_version_objective_id uuid NOT NULL,
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
  question_id uuid NOT NULL,
  question_version_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  PRIMARY KEY (question_version_objective_id)
);
CREATE INDEX question_version_objectives_lo_idx ON content.question_version_objectives (objective_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX question_version_objectives_uq ON content.question_version_objectives (question_version_id, objective_id) WHERE deleted_at IS NULL;
-- END entddl

ALTER TABLE content.questions
  ADD CONSTRAINT questions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT questions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT questions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT questions_latest_ck CHECK (latest_version_no >= 0);

ALTER TABLE content.question_versions
  ADD CONSTRAINT question_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT question_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT question_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT question_versions_question_fk FOREIGN KEY (question_id) REFERENCES content.questions (question_id),
  ADD CONSTRAINT question_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT question_versions_topic_fk FOREIGN KEY (topic_id) REFERENCES curriculum.topics (topic_id),
  ADD CONSTRAINT question_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT question_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT question_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT question_versions_type_ck CHECK (type IN ('single_choice','multiple_choice','true_false','fill_in_blank','short_answer',
    'numeric','matching','ordering','drag_drop','image_based','interactive')),
  ADD CONSTRAINT question_versions_stem_ck CHECK (octet_length(stem::text) <= 65536),
  ADD CONSTRAINT question_versions_render_ck CHECK (octet_length(render_payload::text) <= 65536),
  ADD CONSTRAINT question_versions_key_ck CHECK (octet_length(answer_key::text) <= 65536),
  ADD CONSTRAINT question_versions_scoring_ck CHECK (octet_length(scoring_rule::text) <= 4096),
  ADD CONSTRAINT question_versions_explanation_ck CHECK (explanation IS NULL OR octet_length(explanation::text) <= 65536),
  ADD CONSTRAINT question_versions_feedback_ck CHECK (option_feedback IS NULL OR octet_length(option_feedback::text) <= 65536),
  ADD CONSTRAINT question_versions_hints_ck CHECK (hints IS NULL OR (jsonb_typeof(hints) = 'array' AND jsonb_array_length(hints) <= 5
    AND octet_length(hints::text) <= 32768)),
  ADD CONSTRAINT question_versions_worked_example_ck CHECK (worked_example_ref IS NULL OR octet_length(worked_example_ref::text) <= 1024),
  ADD CONSTRAINT question_versions_min_response_ck CHECK (min_response_ms IS NULL OR min_response_ms BETWEEN 0 AND 600000),
  ADD CONSTRAINT question_versions_max_score_ck CHECK (max_score > 0),
  ADD CONSTRAINT question_versions_math_ck CHECK (math_assets IS NULL OR (jsonb_typeof(math_assets) = 'array'
    AND jsonb_array_length(math_assets) <= 50 AND octet_length(math_assets::text) <= 65536)),
  ADD CONSTRAINT question_versions_tags_ck CHECK (jsonb_typeof(tags) = 'array' AND jsonb_array_length(tags) <= 20),
  ADD CONSTRAINT question_versions_difficulty_ck CHECK (difficulty BETWEEN 1 AND 5),
  ADD CONSTRAINT question_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) BETWEEN 1 AND 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT question_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT question_versions_hash_ck CHECK (content_hash IS NULL OR octet_length(content_hash) = 32);
CREATE INDEX question_versions_grades_gin ON content.question_versions USING gin (grade_levels jsonb_path_ops) WHERE deleted_at IS NULL;

ALTER TABLE content.question_version_objectives
  ADD CONSTRAINT question_version_objectives_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT question_version_objectives_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT question_version_objectives_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT question_version_objectives_version_fk FOREIGN KEY (question_version_id) REFERENCES content.question_versions (question_version_id),
  ADD CONSTRAINT question_version_objectives_lo_fk FOREIGN KEY (objective_id) REFERENCES curriculum.learning_objectives (objective_id);

-- RLS H (ADR-0089 §1): tenant rows, or platform rows whose question is assigned to the session tenant.
ALTER TABLE content.questions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.questions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.questions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question' AND a.resource_id = questions.question_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.questions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.questions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.question_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question' AND a.resource_id = question_versions.question_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.question_version_objectives ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_version_objectives FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_version_objectives FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question' AND a.resource_id = question_version_objectives.question_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_version_objectives FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_version_objectives FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows (ADR-0089 §1).
CREATE OR REPLACE VIEW content.questions_live WITH (security_barrier = true) AS
  SELECT * FROM content.questions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.question_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.question_version_objectives_live WITH (security_barrier = true) AS
  SELECT * FROM content.question_version_objectives WHERE deleted_at IS NULL;
GRANT SELECT ON content.questions_live, content.question_versions_live, content.question_version_objectives_live TO learning_readonly;
REVOKE ALL ON content.questions, content.question_versions, content.question_version_objectives FROM learning_readonly;

-- Delivery view (ADR-0016, F022-AC-24): published versions without any answer column — answer_key, scoring_rule,
-- explanation, option_feedback, hints, min_response_ms never appear. Owner privileges + FORCE RLS of the base table:
-- rows of the session tenant and assigned platform rows only.
CREATE OR REPLACE VIEW content.question_delivery_v WITH (security_barrier = true) AS
  SELECT question_version_id, question_id, tenant_id, owner_scope, version_no AS version, type, stem, render_payload, math_assets,
         max_score, difficulty, grade_levels, subject_id, topic_id, media_refs, content_language, exam_only, efficacy_instrument,
         worked_example_ref, tags, curriculum_version_id
  FROM content.question_versions
  WHERE status = 'published' AND deleted_at IS NULL;
GRANT SELECT ON content.question_delivery_v TO learning_app, learning_platform, learning_readonly;

-- Immutability of published versions (ADR-0015, F022-AC-07) and of their objectives.
CREATE TRIGGER question_versions_immutable BEFORE UPDATE ON content.question_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.forbid_question_objective_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  s text;
BEGIN
  SELECT v.status INTO s FROM content.question_versions v WHERE v.question_version_id = NEW.question_version_id;
  IF s IS NOT NULL AND s <> 'draft' THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER question_version_objectives_draft_only BEFORE INSERT OR UPDATE ON content.question_version_objectives
  FOR EACH ROW EXECUTE FUNCTION content.forbid_question_objective_change();

-- Release pointer target (DATABASE_DESIGN §3.13): current_version_id is a published version of the same question.
CREATE FUNCTION content.questions_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.question_versions v
       WHERE v.question_version_id = NEW.current_version_id AND v.question_id = NEW.question_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER questions_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.questions
  FOR EACH ROW EXECUTE FUNCTION content.questions_release_target();
