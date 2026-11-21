-- ROLLBACK of 20261230000001_adr0095_drop_class_course.sql (module learning, ADR-0095 mục 8 bước 4).
-- Written in advance, NEVER applied automatically. Recreates the dropped structures EMPTY with the DDL of the applied
-- migrations that created them (copied verbatim from 20261122000001_f020_foundation.sql, 20261206000001_f040_lms.sql,
-- 20261223000001_f043_dashboard_rollups.sql, 20261227000001_f035_efficacy.sql; the expand migrations 20261229* had made
-- the dropped columns nullable, so they come back nullable), widens the CHECKs again and restores the projection guard.
-- Dropped columns come back at the end of their table (column order is not significant: views are recreated, data is
-- restored by column name). The rows come back from the dump taken before the migration:
--   scripts/db/adr-0095-backup.sh learning --restore-data <dump>   (tables: pg_restore --data-only; columns: by primary key
--                                                                   from eduworld_learning_restore_check)
-- Run as the table owner so the default privileges of the baseline apply, in one transaction:
--   psql -v ON_ERROR_STOP=1 --single-transaction -U learning_migrator -d eduworld_learning -f scripts/db/rollback/adr-0095-learning.sql
-- Code since WP-L1/WP-L2 never reads these objects: rolling back the structure restores the data for audit/analysis only.
-- Verified by tests/integration/adr0095_rollback_test.go (rollback → re-apply the contract migration).

-- ========== ref.classes, ref.enrollments (20261122000001_f020_foundation.sql) ==========
CREATE TABLE ref.classes (
  class_id         uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id        uuid NOT NULL,
  grade_level      smallint NOT NULL CHECK (grade_level BETWEEN 1 AND 9),
  name             varchar(128) NOT NULL,
  academic_year_id uuid,
  is_virtual       boolean NOT NULL DEFAULT false,
  source_version   bigint NOT NULL
);
CREATE INDEX ref_classes_tenant_school_idx ON ref.classes (tenant_id, school_id);

CREATE TABLE ref.enrollments (
  student_id     uuid NOT NULL,
  class_id       uuid NOT NULL,
  start_date     date NOT NULL,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status         varchar(64) NOT NULL CHECK (status IN ('active','transferred','ended')),
  end_date       date,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, class_id, start_date)
);
CREATE INDEX ref_enrollments_class_idx ON ref.enrollments (tenant_id, class_id) WHERE status = 'active';

ALTER TABLE ref.classes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.classes FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.enrollments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'schools' THEN (SELECT s.tenant_id FROM ref.schools s WHERE s.school_id = p_id)
    WHEN 'classes' THEN (SELECT c.tenant_id FROM ref.classes c WHERE c.class_id = p_id)
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
  END
$$;

-- ========== F040 tables (20261206000001_f040_lms.sql, without lms.school_curriculum_settings which stays) ==========
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
CREATE VIEW lms.courses_live WITH (security_barrier = true) AS SELECT * FROM lms.courses WHERE deleted_at IS NULL;
GRANT SELECT ON lms.courses_live TO learning_readonly;
REVOKE ALL ON lms.courses FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.courses FROM learning_app, learning_platform;


ALTER TABLE lms.path_node_unlocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.path_node_unlocks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.path_node_unlocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.path_node_unlocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.path_node_unlocks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.path_node_unlocks_live WITH (security_barrier = true) AS SELECT * FROM lms.path_node_unlocks WHERE deleted_at IS NULL;
GRANT SELECT ON lms.path_node_unlocks_live TO learning_readonly;
REVOKE ALL ON lms.path_node_unlocks FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.path_node_unlocks FROM learning_app, learning_platform;

ALTER TABLE lms.schedule_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.schedule_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.schedule_items FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.schedule_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.schedule_items FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.schedule_items_live WITH (security_barrier = true) AS SELECT * FROM lms.schedule_items WHERE deleted_at IS NULL;
GRANT SELECT ON lms.schedule_items_live TO learning_readonly;
REVOKE ALL ON lms.schedule_items FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.schedule_items FROM learning_app, learning_platform;

ALTER TABLE lms.study_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.study_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.study_groups FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.study_groups FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.study_groups FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.study_groups_live WITH (security_barrier = true) AS SELECT * FROM lms.study_groups WHERE deleted_at IS NULL;
GRANT SELECT ON lms.study_groups_live TO learning_readonly;
REVOKE ALL ON lms.study_groups FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.study_groups FROM learning_app, learning_platform;

ALTER TABLE lms.study_group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.study_group_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.study_group_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.study_group_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.study_group_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.study_group_members_live WITH (security_barrier = true) AS SELECT * FROM lms.study_group_members WHERE deleted_at IS NULL;
GRANT SELECT ON lms.study_group_members_live TO learning_readonly;
REVOKE ALL ON lms.study_group_members FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.study_group_members FROM learning_app, learning_platform;

ALTER TABLE lms.learning_paths ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.learning_paths FORCE ROW LEVEL SECURITY;
CREATE POLICY hybrid_read ON lms.learning_paths FOR SELECT USING (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL);
CREATE POLICY tenant_insert ON lms.learning_paths FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.learning_paths FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.learning_paths_live WITH (security_barrier = true) AS SELECT * FROM lms.learning_paths WHERE deleted_at IS NULL;
GRANT SELECT ON lms.learning_paths_live TO learning_readonly;
REVOKE ALL ON lms.learning_paths FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.learning_paths FROM learning_app, learning_platform;

ALTER TABLE lms.path_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.path_nodes FORCE ROW LEVEL SECURITY;
CREATE POLICY hybrid_read ON lms.path_nodes FOR SELECT USING (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL);
CREATE POLICY tenant_insert ON lms.path_nodes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.path_nodes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW lms.path_nodes_live WITH (security_barrier = true) AS SELECT * FROM lms.path_nodes WHERE deleted_at IS NULL;
GRANT SELECT ON lms.path_nodes_live TO learning_readonly;
REVOKE ALL ON lms.path_nodes FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.path_nodes FROM learning_app, learning_platform;

-- ========== class rollups and read models (20261223000001_f043_dashboard_rollups.sql) ==========
CREATE TABLE learning.rollup_class_daily (
  tenant_id uuid NOT NULL,
  class_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL DEFAULT 0,
  active_students integer,
  active_students_all integer,
  activities_completed bigint NOT NULL DEFAULT 0,
  lessons_completed bigint NOT NULL DEFAULT 0,
  accuracy_num bigint NOT NULL DEFAULT 0,
  accuracy_den bigint NOT NULL DEFAULT 0,
  rapid_guess_num bigint NOT NULL DEFAULT 0,
  rapid_guess_den bigint NOT NULL DEFAULT 0,
  active_seconds bigint NOT NULL DEFAULT 0,
  assignments_due bigint NOT NULL DEFAULT 0,
  assignments_completed bigint NOT NULL DEFAULT 0,
  assignments_on_time bigint NOT NULL DEFAULT 0,
  is_final boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, class_id, subject_id, bucket_date, metric_version),
  CONSTRAINT rollup_class_daily_counts_ck CHECK (grade_level BETWEEN 0 AND 12 AND activities_completed >= 0 AND lessons_completed >= 0
    AND accuracy_num BETWEEN 0 AND accuracy_den AND rapid_guess_num BETWEEN 0 AND rapid_guess_den AND active_seconds >= 0
    AND assignments_completed BETWEEN 0 AND assignments_due AND assignments_on_time BETWEEN 0 AND assignments_completed)
) PARTITION BY RANGE (bucket_date);
CREATE INDEX rollup_class_daily_school_idx ON learning.rollup_class_daily (tenant_id, school_id, bucket_date);
CREATE INDEX rollup_class_daily_bucket_brin ON learning.rollup_class_daily USING brin (bucket_date);
CREATE TABLE learning.rollup_class_weekly (LIKE learning.rollup_class_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, class_id, subject_id, bucket_date, metric_version)) PARTITION BY RANGE (bucket_date);
CREATE INDEX rollup_class_weekly_school_idx ON learning.rollup_class_weekly (tenant_id, school_id, bucket_date);
CREATE TABLE learning.rollup_class_monthly (LIKE learning.rollup_class_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, class_id, subject_id, bucket_date, metric_version));
CREATE INDEX rollup_class_monthly_school_idx ON learning.rollup_class_monthly (tenant_id, school_id, bucket_date);

CREATE TABLE learning.rollup_class_objective_weekly (
  tenant_id uuid NOT NULL,
  class_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  evidence_count bigint NOT NULL DEFAULT 0,
  correct_num bigint NOT NULL DEFAULT 0,
  correct_den bigint NOT NULL DEFAULT 0,
  level_dist integer[] NOT NULL DEFAULT '{0,0,0,0,0}',
  is_final boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, class_id, objective_id, bucket_date, metric_version),
  CONSTRAINT rollup_class_objective_weekly_ck CHECK (evidence_count >= 0 AND correct_num BETWEEN 0 AND correct_den
    AND cardinality(level_dist) = 5)
) PARTITION BY RANGE (bucket_date);

CREATE TABLE learning.rm_class_objective_stats (
  tenant_id uuid NOT NULL,
  class_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL DEFAULT 0,
  subject_id uuid NOT NULL,
  topic_id uuid,
  objective_code varchar(64) NOT NULL,
  yccd_ref varchar(64),
  level_dist integer[] NOT NULL DEFAULT '{0,0,0,0,0}',
  students_with_evidence integer NOT NULL DEFAULT 0,
  accuracy_num bigint NOT NULL DEFAULT 0,
  accuracy_den bigint NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, class_id, objective_id),
  CONSTRAINT rm_class_objective_stats_ck CHECK (cardinality(level_dist) = 5 AND students_with_evidence >= 0
    AND accuracy_num BETWEEN 0 AND accuracy_den)
);
CREATE INDEX rm_class_objective_stats_school_idx ON learning.rm_class_objective_stats (tenant_id, school_id, subject_id);

CREATE TABLE learning.rm_class_subject_summary (
  tenant_id uuid NOT NULL,
  class_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL DEFAULT 0,
  student_count integer NOT NULL DEFAULT 0,
  active_students_7d integer NOT NULL DEFAULT 0,
  weak_objective_ids uuid[] NOT NULL DEFAULT '{}',
  assignment_completion_num bigint NOT NULL DEFAULT 0,
  assignment_completion_den bigint NOT NULL DEFAULT 0,
  rapid_guess_students integer NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, class_id, subject_id),
  CONSTRAINT rm_class_subject_summary_ck CHECK (cardinality(weak_objective_ids) <= 20 AND student_count >= 0
    AND active_students_7d >= 0 AND rapid_guess_students >= 0 AND assignment_completion_num BETWEEN 0 AND assignment_completion_den)
);
CREATE INDEX rm_class_subject_summary_school_idx ON learning.rm_class_subject_summary (tenant_id, school_id);

CREATE TABLE learning.rollup_class_daily_default PARTITION OF learning.rollup_class_daily DEFAULT;
CREATE TABLE learning.rollup_class_weekly_default PARTITION OF learning.rollup_class_weekly DEFAULT;
CREATE TABLE learning.rollup_class_objective_weekly_default PARTITION OF learning.rollup_class_objective_weekly DEFAULT;
ALTER TABLE learning.rollup_class_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_class_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_class_daily FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_class_daily FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_class_daily FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_class_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_class_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_class_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_class_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_class_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_class_monthly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_class_monthly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_class_monthly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_class_monthly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_class_monthly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_class_objective_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_class_objective_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_class_objective_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_class_objective_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_class_objective_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rm_class_objective_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_class_objective_stats FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_class_objective_stats FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_class_objective_stats FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_class_objective_stats FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rm_class_subject_summary ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_class_subject_summary FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_class_subject_summary FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_class_subject_summary FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_class_subject_summary FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly,
  learning.rollup_class_objective_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly,
  learning.rollup_class_objective_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary TO learning_app, learning_platform;
GRANT SELECT ON learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly,
  learning.rollup_class_objective_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary TO learning_dashboard;
REVOKE ALL ON learning.rollup_class_daily_default, learning.rollup_class_weekly_default,
  learning.rollup_class_objective_weekly_default FROM learning_app, learning_platform;
GRANT SELECT ON ref.classes, ref.enrollments TO learning_dashboard;

-- ========== class cohorts of the efficacy study (20261227000001_f035_efficacy.sql) ==========
CREATE TABLE learning.efficacy_study_classes (
  study_id uuid NOT NULL,
  class_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  pair_no smallint,
  arm varchar(64),
  assignment_method varchar(64) NOT NULL DEFAULT 'manual',
  form_order varchar(64),
  status varchar(64) NOT NULL DEFAULT 'enrolled',
  withdrawn_at timestamptz,
  withdrawal_id uuid,
  added_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_id, class_id),
  CONSTRAINT efficacy_study_classes_pair_ck CHECK (pair_no IS NULL OR pair_no BETWEEN 1 AND 200),
  CONSTRAINT efficacy_study_classes_arm_ck CHECK (arm IS NULL OR arm IN ('intervention','comparison','validation')),
  CONSTRAINT efficacy_study_classes_validation_ck CHECK (arm IS DISTINCT FROM 'validation' OR pair_no IS NULL),
  CONSTRAINT efficacy_study_classes_method_ck CHECK (assignment_method IN ('manual','random')),
  CONSTRAINT efficacy_study_classes_order_ck CHECK (form_order IS NULL OR form_order IN ('a_then_b','b_then_a')),
  CONSTRAINT efficacy_study_classes_status_ck CHECK (status IN ('enrolled','withdrawn'))
);
-- A removed class (soft-deleted) added again revives its row (draft study only).
-- One enrolled study per class at a time (draft or active studies; the command checks the study status).
CREATE INDEX efficacy_study_classes_class_idx ON learning.efficacy_study_classes (tenant_id, class_id) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_study_classes_pair_idx ON learning.efficacy_study_classes (study_id, pair_no) WHERE deleted_at IS NULL;

CREATE TABLE learning.efficacy_wave_sessions (
  wave_id uuid NOT NULL,
  class_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  exam_session_id uuid NOT NULL,
  form varchar(16) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (wave_id, class_id),
  CONSTRAINT efficacy_wave_sessions_form_ck CHECK (form IN ('a','b')),
  CONSTRAINT efficacy_wave_sessions_status_ck CHECK (status IN ('active','cancelled'))
);
CREATE UNIQUE INDEX efficacy_wave_sessions_exam_uq ON learning.efficacy_wave_sessions (exam_session_id);
CREATE INDEX efficacy_wave_sessions_tenant_idx ON learning.efficacy_wave_sessions (tenant_id, class_id) WHERE deleted_at IS NULL;

ALTER TABLE learning.efficacy_study_classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_study_classes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_study_classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_study_classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_study_classes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.efficacy_wave_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_wave_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_wave_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_wave_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_wave_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE VIEW learning.efficacy_study_classes_live WITH (security_barrier = true) AS
  SELECT * FROM learning.efficacy_study_classes WHERE deleted_at IS NULL;
GRANT SELECT ON learning.efficacy_study_classes_live TO learning_readonly;
REVOKE ALL ON learning.efficacy_study_classes, learning.efficacy_wave_sessions FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON learning.efficacy_study_classes, learning.efficacy_wave_sessions FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.efficacy_study_classes, learning.efficacy_wave_sessions TO learning_app, learning_platform;

-- ========== dropped columns of live tables (nullable as the expand migrations left them) + their indexes ==========
DROP VIEW learning.lesson_sessions_live;
DROP VIEW lms.assignments_live;
DROP VIEW lms.assignment_targets_live;
DROP VIEW lms.assignment_target_students_live;
DROP VIEW lms.assignment_submissions_live;
DROP VIEW assessment.exam_sessions_live;
ALTER TABLE learning.learning_events ADD COLUMN class_id uuid;
ALTER TABLE learning.progress_daily ADD COLUMN class_id uuid;
CREATE INDEX progress_daily_class_idx ON learning.progress_daily (tenant_id, class_id, bucket_date);
ALTER TABLE learning.lesson_sessions ADD COLUMN learning_path_id uuid;
ALTER TABLE lms.assignments ADD COLUMN course_id uuid;
CREATE INDEX assignments_course_idx ON lms.assignments (tenant_id, course_id) WHERE deleted_at IS NULL;
ALTER TABLE lms.assignment_targets ADD COLUMN target_ref_id uuid;
CREATE INDEX assignment_targets_ref_idx ON lms.assignment_targets (tenant_id, target_type, target_ref_id) WHERE deleted_at IS NULL;
ALTER TABLE lms.assignment_target_students ADD COLUMN class_id_at_assign uuid;
ALTER TABLE lms.assignment_submissions ADD COLUMN class_id uuid;
ALTER TABLE assessment.exam_sessions ADD COLUMN course_id uuid;
ALTER TABLE learning.rm_student_attention ADD COLUMN class_id uuid;
DROP INDEX learning.rm_student_attention_school_idx;
CREATE INDEX rm_student_attention_school_idx ON learning.rm_student_attention (tenant_id, school_id, class_id) WHERE cardinality(reasons) > 0;
ALTER TABLE learning.rm_student_subject_reference_levels ADD COLUMN class_id uuid;
CREATE INDEX rm_reference_levels_school_idx ON learning.rm_student_subject_reference_levels
  (tenant_id, school_id, subject_id, academic_year, period, class_id);
ALTER TABLE learning.efficacy_participants ADD COLUMN class_id uuid;
CREATE INDEX efficacy_participants_class_idx ON learning.efficacy_participants (tenant_id, study_id, class_id) WHERE deleted_at IS NULL;
ALTER TABLE learning.efficacy_withdrawals ADD COLUMN class_id uuid;
ALTER TABLE learning.rollup_school_daily ADD COLUMN active_classes integer;
ALTER TABLE learning.rollup_school_weekly ADD COLUMN active_classes integer;
ALTER TABLE learning.rollup_school_monthly ADD COLUMN active_classes integer;
ALTER TABLE learning.rollup_tenant_daily ADD COLUMN active_classes integer;
ALTER TABLE learning.rollup_tenant_weekly ADD COLUMN active_classes integer;
ALTER TABLE learning.rollup_tenant_monthly ADD COLUMN active_classes integer;
CREATE VIEW learning.lesson_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM learning.lesson_sessions WHERE deleted_at IS NULL;
CREATE VIEW lms.assignments_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignments WHERE deleted_at IS NULL;
CREATE VIEW lms.assignment_targets_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_targets WHERE deleted_at IS NULL;
CREATE VIEW lms.assignment_target_students_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_target_students WHERE deleted_at IS NULL;
CREATE VIEW lms.assignment_submissions_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_submissions WHERE deleted_at IS NULL;
CREATE VIEW assessment.exam_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.exam_sessions WHERE deleted_at IS NULL;
GRANT SELECT ON learning.lesson_sessions_live, lms.assignments_live, lms.assignment_targets_live,
  lms.assignment_target_students_live, lms.assignment_submissions_live, assessment.exam_sessions_live TO learning_readonly;

-- ========== CHECKs accept the old values again (restored rows may carry them; harmless when the migration kept them) ==========
ALTER TABLE content.content_releases DROP CONSTRAINT content_releases_item_type_ck,
  ADD CONSTRAINT content_releases_item_type_ck CHECK (item_type IN ('curriculum','question','question_set',
    'assessment_blueprint','lesson','learning_material','learning_path','activity'));
ALTER TABLE content.content_transitions DROP CONSTRAINT content_transitions_type_ck,
  ADD CONSTRAINT content_transitions_type_ck CHECK (content_type IN ('curriculum','question','question_set',
    'assessment_blueprint','lesson','learning_material','learning_path','activity'));
ALTER TABLE content.media_usages DROP CONSTRAINT media_usages_item_type_ck,
  ADD CONSTRAINT media_usages_item_type_ck CHECK (item_type IN ('question','question_set','lesson','learning_material',
    'assessment_blueprint','activity','learning_path'));
ALTER TABLE content.review_tasks DROP CONSTRAINT review_tasks_type_ck,
  ADD CONSTRAINT review_tasks_type_ck CHECK (content_type IN ('question','question_set','assessment_blueprint','lesson',
    'learning_material','learning_path','activity'));
ALTER TABLE content.scheduled_publications DROP CONSTRAINT scheduled_publications_type_ck,
  ADD CONSTRAINT scheduled_publications_type_ck CHECK (item_type IN ('question','question_set','assessment_blueprint','lesson',
    'learning_material','learning_path','activity'));
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck,
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('curriculum','lesson','question','question_set',
    'assessment_blueprint','media','assignment','course','learning_path','activity'));
ALTER TABLE assessment.attempts DROP CONSTRAINT attempts_context_type_ck,
  ADD CONSTRAINT attempts_context_type_ck CHECK (context_type IN ('free_play','lesson','learning_path','recommendation',
    'question_set','selection','assignment','quest_step','minigame_session','exam_session','match','challenge_attempt','boss_encounter',
    'retry_wrong','diagnostic'));
ALTER TABLE learning.report_jobs DROP CONSTRAINT report_jobs_type_ck,
  ADD CONSTRAINT report_jobs_type_ck CHECK (report_type IN ('assignment_results','course_results',
  'regulation_reference','skill_map','school_monthly','learning_summary','class_progress','grade_progress','game_summary','efficacy_dataset'));
ALTER TABLE learning.efficacy_withdrawals DROP CONSTRAINT efficacy_withdrawals_scope_ck,
  ADD CONSTRAINT efficacy_withdrawals_scope_ck CHECK (scope IN ('tenant','school','class','cohort','students'));
ALTER TABLE learning.experiments DROP CONSTRAINT experiments_unit_ck,
  ADD CONSTRAINT experiments_unit_ck CHECK (unit IN ('class','school_grade','student'));
