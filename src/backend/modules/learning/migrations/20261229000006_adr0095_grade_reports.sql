-- ADR-0095 mục 4 (WP-L2, expand E3): the exports of F044 follow the grade instead of the class (DASHBOARD_DATA §10,
-- SCHOOL_LMS §10):
--   * learning.report_jobs accepts grade_progress (school × grade, replaces class_progress); course_results and
--     class_progress are no longer accepted by the API but stay in the CHECK for the jobs already recorded (expand/contract,
--     AGENTS R12);
--   * learning.rm_student_subject_reference_levels (derived read model of the TT27/TT22 reference report) records the grade
--     of the student when the level is computed; class_id is no longer written (nullable, dropped by WP-R). The nightly job
--     rewrites every row of the current period (no SQL backfill — ADR-0095 is applied before the first staging deployment).
-- Hand-written (explicit SQL tables of F044). Expand only.

ALTER TABLE learning.report_jobs DROP CONSTRAINT report_jobs_type_ck;
ALTER TABLE learning.report_jobs ADD CONSTRAINT report_jobs_type_ck CHECK (report_type IN ('assignment_results','course_results',
  'regulation_reference','skill_map','school_monthly','learning_summary','class_progress','grade_progress','game_summary','efficacy_dataset'));

ALTER TABLE learning.rm_student_subject_reference_levels ALTER COLUMN class_id DROP NOT NULL;
ALTER TABLE learning.rm_student_subject_reference_levels ADD COLUMN grade_level smallint,
  ADD CONSTRAINT rm_reference_levels_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
COMMENT ON COLUMN learning.rm_student_subject_reference_levels.class_id IS 'ADR-0095: no longer written; dropped by WP-R';
COMMENT ON COLUMN learning.rm_student_subject_reference_levels.grade_level IS 'ADR-0095: grade of the student when the level was computed';
CREATE INDEX rm_reference_levels_school_grade_idx ON learning.rm_student_subject_reference_levels
  (tenant_id, school_id, subject_id, academic_year, period, grade_level);
