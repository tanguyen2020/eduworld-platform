-- F040 — LMS core: courses, school curriculum settings, learning paths and nodes, manual unlocks, schedule items, study
-- groups (specs/040-lms-course-path-schedule: data-model §1; DATABASE_DESIGN §2.14, §2.16; SCHOOL_LMS §2–§4; ADR-0084).
-- Applied by migrate --module=learning as learning_migrator. Hand-written tables (explicit-SQL repositories, like the
-- learning record of F031/F032); expand only (AGENTS R12); soft-delete only (R21).

-- ---------- lms.courses (T): class × subject, provisioned from class.synced ----------
CREATE TABLE lms.courses (
  course_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  academic_year_id uuid NOT NULL,
  academic_year varchar(9),
  curriculum_version_id uuid NOT NULL,
  textbook_series_id uuid,
  status varchar(64) NOT NULL DEFAULT 'active',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (course_id),
  CONSTRAINT courses_status_ck CHECK (status IN ('active','hidden','archived')),
  CONSTRAINT courses_grade_ck CHECK (grade_level BETWEEN 1 AND 12)
);
CREATE UNIQUE INDEX courses_class_subject_uq ON lms.courses (tenant_id, class_id, subject_id, academic_year_id) WHERE deleted_at IS NULL;
CREATE INDEX courses_school_class_idx ON lms.courses (tenant_id, school_id, class_id) WHERE deleted_at IS NULL;
CREATE INDEX courses_school_subject_idx ON lms.courses (tenant_id, school_id, subject_id) WHERE deleted_at IS NULL;

-- ---------- lms.school_curriculum_settings (T): curriculum version of a school per academic year (and grade) ----------
CREATE TABLE lms.school_curriculum_settings (
  setting_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  academic_year_id uuid NOT NULL,
  grade_level smallint,
  curriculum_version_id uuid NOT NULL,
  set_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (setting_id),
  CONSTRAINT school_curriculum_settings_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 12)
);
CREATE UNIQUE INDEX school_curriculum_settings_uq ON lms.school_curriculum_settings
  (tenant_id, school_id, academic_year_id, coalesce(grade_level, 0)) WHERE deleted_at IS NULL;

-- ---------- lms.learning_paths (H): own path of a course, or a platform template by textbook series ----------
CREATE TABLE lms.learning_paths (
  path_id uuid NOT NULL,
  tenant_id uuid,
  owner_scope varchar(64) NOT NULL,
  course_id uuid,
  school_id uuid,
  textbook_series_id uuid,
  subject_id uuid,
  grade_level smallint,
  title varchar(255) NOT NULL,
  unlock_mode varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  created_by uuid,
  updated_by uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (path_id),
  CONSTRAINT learning_paths_owner_ck CHECK (owner_scope IN ('platform','tenant') AND (owner_scope = 'platform') = (tenant_id IS NULL)),
  CONSTRAINT learning_paths_course_ck CHECK (owner_scope = 'platform' OR course_id IS NOT NULL),
  CONSTRAINT learning_paths_mode_ck CHECK (unlock_mode IN ('sequential','free','mastery_gate')),
  CONSTRAINT learning_paths_status_ck CHECK (status IN ('active','archived')),
  CONSTRAINT learning_paths_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 12)
);
CREATE UNIQUE INDEX learning_paths_course_active_uq ON lms.learning_paths (tenant_id, course_id)
  WHERE deleted_at IS NULL AND status = 'active' AND course_id IS NOT NULL;
CREATE INDEX learning_paths_template_idx ON lms.learning_paths (textbook_series_id, subject_id, grade_level)
  WHERE deleted_at IS NULL AND owner_scope = 'platform';

-- ---------- lms.path_nodes (H like its path): ordered nodes pinned to a published content version ----------
CREATE TABLE lms.path_nodes (
  node_id uuid NOT NULL,
  tenant_id uuid,
  path_id uuid NOT NULL REFERENCES lms.learning_paths (path_id),
  school_id uuid,
  position integer NOT NULL,
  content_type varchar(64) NOT NULL,
  content_id uuid NOT NULL,
  content_version_id uuid NOT NULL,
  title varchar(255),
  objective_ids uuid[] NOT NULL DEFAULT '{}',
  unlock_rule jsonb NOT NULL DEFAULT '{}',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (node_id),
  -- 1–200 in a path (domain); up to 1200 while an update moves the kept nodes out of the way (positions are unique on live rows).
  CONSTRAINT path_nodes_position_ck CHECK (position BETWEEN 1 AND 1200),
  CONSTRAINT path_nodes_type_ck CHECK (content_type IN ('lesson','activity','question_set','assessment_blueprint','quest')),
  CONSTRAINT path_nodes_objectives_ck CHECK (cardinality(objective_ids) <= 50),
  CONSTRAINT path_nodes_rule_ck CHECK (jsonb_typeof(unlock_rule) = 'object' AND octet_length(unlock_rule::text) <= 4096)
);
CREATE UNIQUE INDEX path_nodes_position_uq ON lms.path_nodes (path_id, position) WHERE deleted_at IS NULL;

-- ---------- lms.path_node_unlocks (T): manual unlock of a node for one student ----------
CREATE TABLE lms.path_node_unlocks (
  unlock_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  path_id uuid NOT NULL,
  node_id uuid NOT NULL,
  student_id uuid NOT NULL,
  reason varchar(1000) NOT NULL,
  unlocked_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (unlock_id)
);
CREATE UNIQUE INDEX path_node_unlocks_uq ON lms.path_node_unlocks (tenant_id, node_id, student_id) WHERE deleted_at IS NULL;

-- ---------- lms.schedule_items (T): lessons, due dates, exams, events of a course ----------
CREATE TABLE lms.schedule_items (
  schedule_item_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  course_id uuid NOT NULL,
  date date NOT NULL,
  period smallint,
  kind varchar(64) NOT NULL,
  ref_type varchar(64),
  ref_id uuid,
  note varchar(500),
  source_event_id uuid,
  created_by uuid,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (schedule_item_id),
  CONSTRAINT schedule_items_kind_ck CHECK (kind IN ('lesson','assignment_due','exam','event')),
  CONSTRAINT schedule_items_period_ck CHECK (period IS NULL OR period BETWEEN 1 AND 15)
);
CREATE UNIQUE INDEX schedule_items_source_uq ON lms.schedule_items (tenant_id, source_event_id)
  WHERE deleted_at IS NULL AND source_event_id IS NOT NULL;
CREATE INDEX schedule_items_course_date_idx ON lms.schedule_items (tenant_id, course_id, date) WHERE deleted_at IS NULL;

-- ---------- lms.study_groups, lms.study_group_members (T, ADR-0084) ----------
CREATE TABLE lms.study_groups (
  study_group_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid,
  grade_level smallint NOT NULL,
  name varchar(64) NOT NULL,
  member_count integer NOT NULL,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_group_id),
  CONSTRAINT study_groups_grade_ck CHECK (grade_level BETWEEN 1 AND 12),
  CONSTRAINT study_groups_members_ck CHECK (member_count BETWEEN 0 AND 100)
);
CREATE UNIQUE INDEX study_groups_name_uq ON lms.study_groups (tenant_id, school_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX study_groups_class_idx ON lms.study_groups (tenant_id, school_id, class_id) WHERE deleted_at IS NULL;

CREATE TABLE lms.study_group_members (
  study_group_member_id uuid NOT NULL,
  study_group_id uuid NOT NULL REFERENCES lms.study_groups (study_group_id),
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  added_by uuid NOT NULL,
  added_at timestamptz NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_group_member_id)
);
CREATE UNIQUE INDEX study_group_members_uq ON lms.study_group_members (study_group_id, student_id) WHERE deleted_at IS NULL;
CREATE INDEX study_group_members_student_idx ON lms.study_group_members (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- RLS: T tables by tenant; learning_paths/path_nodes also show platform templates (tenant_id NULL) ----------
ALTER TABLE lms.courses ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.courses FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.courses FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.courses FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.courses FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.courses_live WITH (security_barrier = true) AS SELECT * FROM lms.courses WHERE deleted_at IS NULL;
GRANT SELECT ON lms.courses_live TO learning_readonly;
REVOKE ALL ON lms.courses FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.courses FROM learning_app, learning_platform;

ALTER TABLE lms.school_curriculum_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.school_curriculum_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.school_curriculum_settings FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.school_curriculum_settings FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.school_curriculum_settings FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.school_curriculum_settings_live WITH (security_barrier = true) AS SELECT * FROM lms.school_curriculum_settings WHERE deleted_at IS NULL;
GRANT SELECT ON lms.school_curriculum_settings_live TO learning_readonly;
REVOKE ALL ON lms.school_curriculum_settings FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.school_curriculum_settings FROM learning_app, learning_platform;

ALTER TABLE lms.path_node_unlocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.path_node_unlocks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.path_node_unlocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.path_node_unlocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.path_node_unlocks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.path_node_unlocks_live WITH (security_barrier = true) AS SELECT * FROM lms.path_node_unlocks WHERE deleted_at IS NULL;
GRANT SELECT ON lms.path_node_unlocks_live TO learning_readonly;
REVOKE ALL ON lms.path_node_unlocks FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.path_node_unlocks FROM learning_app, learning_platform;

ALTER TABLE lms.schedule_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.schedule_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.schedule_items FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.schedule_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.schedule_items FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.schedule_items_live WITH (security_barrier = true) AS SELECT * FROM lms.schedule_items WHERE deleted_at IS NULL;
GRANT SELECT ON lms.schedule_items_live TO learning_readonly;
REVOKE ALL ON lms.schedule_items FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.schedule_items FROM learning_app, learning_platform;

ALTER TABLE lms.study_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.study_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.study_groups FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.study_groups FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.study_groups FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.study_groups_live WITH (security_barrier = true) AS SELECT * FROM lms.study_groups WHERE deleted_at IS NULL;
GRANT SELECT ON lms.study_groups_live TO learning_readonly;
REVOKE ALL ON lms.study_groups FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.study_groups FROM learning_app, learning_platform;

ALTER TABLE lms.study_group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.study_group_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.study_group_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.study_group_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.study_group_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.study_group_members_live WITH (security_barrier = true) AS SELECT * FROM lms.study_group_members WHERE deleted_at IS NULL;
GRANT SELECT ON lms.study_group_members_live TO learning_readonly;
REVOKE ALL ON lms.study_group_members FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.study_group_members FROM learning_app, learning_platform;

ALTER TABLE lms.learning_paths ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.learning_paths FORCE ROW LEVEL SECURITY;
CREATE POLICY hybrid_read ON lms.learning_paths FOR SELECT USING (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL);
CREATE POLICY tenant_insert ON lms.learning_paths FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.learning_paths FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.learning_paths_live WITH (security_barrier = true) AS SELECT * FROM lms.learning_paths WHERE deleted_at IS NULL;
GRANT SELECT ON lms.learning_paths_live TO learning_readonly;
REVOKE ALL ON lms.learning_paths FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.learning_paths FROM learning_app, learning_platform;

ALTER TABLE lms.path_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.path_nodes FORCE ROW LEVEL SECURITY;
CREATE POLICY hybrid_read ON lms.path_nodes FOR SELECT USING (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL);
CREATE POLICY tenant_insert ON lms.path_nodes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.path_nodes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.path_nodes_live WITH (security_barrier = true) AS SELECT * FROM lms.path_nodes WHERE deleted_at IS NULL;
GRANT SELECT ON lms.path_nodes_live TO learning_readonly;
REVOKE ALL ON lms.path_nodes FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.path_nodes FROM learning_app, learning_platform;
