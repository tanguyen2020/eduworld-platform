-- atlas:nolint destructive

-- ADR-0095 mục 8 (D4, WP-R contract): one-time, controlled exception of ADR-0070 mục 2 and AGENTS R12/R21 for the closed
-- list of CODE_PLAN §4.2 — module learning drops the structures of the removed concepts (class, enrollment, course,
-- learning path, schedule, study group, class dashboards and class cohorts of the efficacy study). Valid only before the
-- first staging/production deployment (F130 not deployed); otherwise they would be moved to schema retired instead.
-- Backup before applying (R12): scripts/db/adr-0095-backup.sh learning; rollback (structures, empty):
-- scripts/db/rollback/adr-0095-learning.sql + scripts/db/adr-0095-backup.sh learning --restore-data <dump>.
-- No code reads or writes these objects since WP-L1/WP-L2 (columns: no longer written, expand migrations 20261229*).
-- Explicit DROP without CASCADE: an unexpected dependency stops the migration. The _live views of the tables that lose a
-- column are recreated (SELECT * views pin their columns) with their grants.
--
-- CHECK values of the removed concepts (content type learning_path, resource type course, attempt context learning_path,
-- report types course_results/class_progress, withdrawal scope class, experiment unit class) are removed only when no
-- row — soft-deleted or history included — carries them (ADR-0095 mục 8); otherwise the CHECK is kept, the domain and the
-- OpenAPI request enums already refuse them, and a NOTICE says so. Applied as learning_migrator (table owner,
-- NOBYPASSRLS): FORCE ROW LEVEL SECURITY is lifted for the count only, inside this transaction, and restored.

-- ---------- views of removed tables, or of tables losing a column ----------
DROP VIEW lms.courses_live;
DROP VIEW lms.learning_paths_live;
DROP VIEW lms.path_nodes_live;
DROP VIEW lms.path_node_unlocks_live;
DROP VIEW lms.schedule_items_live;
DROP VIEW lms.study_groups_live;
DROP VIEW lms.study_group_members_live;
DROP VIEW learning.efficacy_study_classes_live;
DROP VIEW learning.lesson_sessions_live;
DROP VIEW lms.assignments_live;
DROP VIEW lms.assignment_targets_live;
DROP VIEW lms.assignment_target_students_live;
DROP VIEW lms.assignment_submissions_live;
DROP VIEW assessment.exam_sessions_live;

-- ---------- columns of live tables (their indexes go with them) ----------
ALTER TABLE learning.learning_events DROP COLUMN class_id;
ALTER TABLE learning.progress_daily DROP COLUMN class_id;                         -- + progress_daily_class_idx
ALTER TABLE learning.lesson_sessions DROP COLUMN learning_path_id;
ALTER TABLE lms.assignments DROP COLUMN course_id;                                -- + assignments_course_idx
ALTER TABLE lms.assignment_targets DROP COLUMN target_ref_id;                     -- + assignment_targets_ref_idx
ALTER TABLE lms.assignment_target_students DROP COLUMN class_id_at_assign;
ALTER TABLE lms.assignment_submissions DROP COLUMN class_id;
ALTER TABLE assessment.exam_sessions DROP COLUMN course_id;
ALTER TABLE learning.rm_student_attention DROP COLUMN class_id;                   -- + rm_student_attention_school_idx
ALTER TABLE learning.rm_student_subject_reference_levels DROP COLUMN class_id;    -- + rm_reference_levels_school_idx
ALTER TABLE learning.efficacy_participants DROP COLUMN class_id;                  -- + efficacy_participants_class_idx
ALTER TABLE learning.efficacy_withdrawals DROP COLUMN class_id;
ALTER TABLE learning.rollup_school_daily DROP COLUMN active_classes;
ALTER TABLE learning.rollup_school_weekly DROP COLUMN active_classes;
ALTER TABLE learning.rollup_school_monthly DROP COLUMN active_classes;
ALTER TABLE learning.rollup_tenant_daily DROP COLUMN active_classes;
ALTER TABLE learning.rollup_tenant_weekly DROP COLUMN active_classes;
ALTER TABLE learning.rollup_tenant_monthly DROP COLUMN active_classes;
-- The school index of the attention list keeps its other columns (the grade comes from ref.students).
CREATE INDEX rm_student_attention_school_idx ON learning.rm_student_attention (tenant_id, school_id) WHERE cardinality(reasons) > 0;

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

-- ---------- the projection guard no longer knows classes (SQL function: its body is not tracked by DROP) ----------
CREATE OR REPLACE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'schools' THEN (SELECT s.tenant_id FROM ref.schools s WHERE s.school_id = p_id)
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
  END
$$;

-- ---------- tables (partitions, indexes, RLS policies, constraints and grants go with them) ----------
DROP TABLE lms.path_node_unlocks;
DROP TABLE lms.path_nodes;
DROP TABLE lms.learning_paths;
DROP TABLE lms.schedule_items;
DROP TABLE lms.study_group_members;
DROP TABLE lms.study_groups;
DROP TABLE lms.courses;
DROP TABLE ref.enrollments;
DROP TABLE ref.classes;
DROP TABLE learning.rollup_class_daily;
DROP TABLE learning.rollup_class_weekly;
DROP TABLE learning.rollup_class_monthly;
DROP TABLE learning.rollup_class_objective_weekly;
DROP TABLE learning.rm_class_objective_stats;
DROP TABLE learning.rm_class_subject_summary;
DROP TABLE learning.efficacy_wave_sessions;
DROP TABLE learning.efficacy_study_classes;

-- ---------- CHECK values of the removed concepts ----------
-- Narrows a CHECK constraint when no row carries an old value (helper of this migration, session-scoped).
CREATE OR REPLACE FUNCTION pg_temp.adr0095_narrow_check(tbl regclass, con text, col text, old text[], def text)
RETURNS void LANGUAGE plpgsql AS $fn$
DECLARE
  forced boolean;
  n bigint;
BEGIN
  SELECT relforcerowsecurity INTO forced FROM pg_class WHERE oid = tbl;
  IF forced THEN
    EXECUTE format('ALTER TABLE %s NO FORCE ROW LEVEL SECURITY', tbl);
  END IF;
  EXECUTE format('SELECT count(*) FROM %s WHERE %I::text = ANY ($1)', tbl, col) INTO n USING old;
  IF forced THEN
    EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', tbl);
  END IF;
  IF n = 0 THEN
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', tbl, con);
    EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s', tbl, con, def);
    RAISE NOTICE 'ADR-0095: % narrowed (no row carries %)', con, old;
  ELSE
    RAISE NOTICE 'ADR-0095: % keeps % (% rows, soft-deleted or history included, carry them)', con, old, n;
  END IF;
END
$fn$;

SELECT pg_temp.adr0095_narrow_check('content.content_releases', 'content_releases_item_type_ck', 'item_type', ARRAY['learning_path'],
  $$CHECK (item_type IN ('curriculum','question','question_set','assessment_blueprint','lesson','learning_material','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('content.content_transitions', 'content_transitions_type_ck', 'content_type', ARRAY['learning_path'],
  $$CHECK (content_type IN ('curriculum','question','question_set','assessment_blueprint','lesson','learning_material','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('content.media_usages', 'media_usages_item_type_ck', 'item_type', ARRAY['learning_path'],
  $$CHECK (item_type IN ('question','question_set','lesson','learning_material','assessment_blueprint','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('content.review_tasks', 'review_tasks_type_ck', 'content_type', ARRAY['learning_path'],
  $$CHECK (content_type IN ('question','question_set','assessment_blueprint','lesson','learning_material','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('content.scheduled_publications', 'scheduled_publications_type_ck', 'item_type', ARRAY['learning_path'],
  $$CHECK (item_type IN ('question','question_set','assessment_blueprint','lesson','learning_material','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('infra.tenant_resource_assignments', 'tra_resource_type_ck', 'resource_type',
  ARRAY['course', 'learning_path'],
  $$CHECK (resource_type IN ('curriculum','lesson','question','question_set','assessment_blueprint','media','assignment','activity'))$$);
SELECT pg_temp.adr0095_narrow_check('assessment.attempts', 'attempts_context_type_ck', 'context_type', ARRAY['learning_path'],
  $$CHECK (context_type IN ('free_play','lesson','recommendation','question_set','selection','assignment','quest_step',
    'minigame_session','exam_session','match','challenge_attempt','boss_encounter','retry_wrong','diagnostic'))$$);
SELECT pg_temp.adr0095_narrow_check('learning.report_jobs', 'report_jobs_type_ck', 'report_type', ARRAY['course_results', 'class_progress'],
  $$CHECK (report_type IN ('assignment_results','regulation_reference','skill_map','school_monthly','learning_summary',
    'grade_progress','game_summary','efficacy_dataset'))$$);
SELECT pg_temp.adr0095_narrow_check('learning.efficacy_withdrawals', 'efficacy_withdrawals_scope_ck', 'scope', ARRAY['class'],
  $$CHECK (scope IN ('tenant','school','cohort','students'))$$);
SELECT pg_temp.adr0095_narrow_check('learning.experiments', 'experiments_unit_ck', 'unit', ARRAY['class'],
  $$CHECK (unit IN ('school_grade','student'))$$);

DROP FUNCTION pg_temp.adr0095_narrow_check(regclass, text, text, text[], text);
