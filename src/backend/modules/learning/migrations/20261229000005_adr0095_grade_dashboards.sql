-- ADR-0095 mục 4 (WP-L2, expand E3): the School Learning Dashboard is school → grade → student (DASHBOARD_DATA §2–§3,
-- LEARNING_ANALYTICS §5). The class level of F043 is replaced by the grade level:
--   * learning.rollup_grade_objective_weekly (T): school × grade × objective × ISO week (evidence, correct, level distribution),
--     replaces rollup_class_objective_weekly — written from learning_events.grade_level (grade at event time, WP-L1);
--   * learning.rm_grade_objective_stats (T): current level distribution of school × grade × objective (LO heatmap of a grade),
--     replaces rm_class_objective_stats — students of the grade from ref.students (current grade);
--   * learning.rm_school_grade_summary gains the assignment completion of the grade (submissions due in the last 14 days,
--     grade of the student when targeted), replacing rm_class_subject_summary.
-- Derived tables (no deleted_at, rebuilt by the jobs read-model-refresh / dashboard-day-close from their sources: the
-- current week and every closed day are rebuilt at the next run; no SQL backfill — ADR-0095 is applied before the first
-- staging deployment). The class tables and columns are no longer written nor read; they stay until WP-R (DROP, D4).
-- Hand-written like the F043 migration (explicit SQL only, no ent entity). Expand only (AGENTS R12).

-- ---------- learning.rollup_grade_objective_weekly (T, P year) ----------
-- level_dist = students of the school × grade per level (not_started, beginning, developing, proficient, mastered) at the
-- last refresh of the week; evidence/correct from the scored first answers of the week (rapid guesses excluded from correct_*).
CREATE TABLE learning.rollup_grade_objective_weekly (
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  objective_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  evidence_count bigint NOT NULL DEFAULT 0,
  correct_num bigint NOT NULL DEFAULT 0,
  correct_den bigint NOT NULL DEFAULT 0,
  level_dist integer[] NOT NULL DEFAULT '{0,0,0,0,0}',
  is_final boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, school_id, grade_level, objective_id, bucket_date, metric_version),
  CONSTRAINT rollup_grade_objective_weekly_ck CHECK (grade_level BETWEEN 0 AND 12 AND evidence_count >= 0
    AND correct_num BETWEEN 0 AND correct_den AND cardinality(level_dist) = 5)
) PARTITION BY RANGE (bucket_date);
CREATE TABLE learning.rollup_grade_objective_weekly_default PARTITION OF learning.rollup_grade_objective_weekly DEFAULT;

-- ---------- learning.rm_grade_objective_stats (T): refreshed every school_dashboard_refresh (job read-model-refresh) ----------
-- student_count = active, not deleted students of the school × grade (k-anonymity of the cell, FR-ANL-016); level_dist
-- not_started = student_count − students with a level.
CREATE TABLE learning.rm_grade_objective_stats (
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  objective_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  topic_id uuid,
  objective_code varchar(64) NOT NULL,
  yccd_ref varchar(64),
  student_count integer NOT NULL DEFAULT 0,
  level_dist integer[] NOT NULL DEFAULT '{0,0,0,0,0}',
  students_with_evidence integer NOT NULL DEFAULT 0,
  accuracy_num bigint NOT NULL DEFAULT 0,
  accuracy_den bigint NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, school_id, grade_level, objective_id),
  CONSTRAINT rm_grade_objective_stats_ck CHECK (grade_level BETWEEN 0 AND 12 AND cardinality(level_dist) = 5
    AND student_count >= 0 AND students_with_evidence >= 0 AND accuracy_num BETWEEN 0 AND accuracy_den)
);
CREATE INDEX rm_grade_objective_stats_subject_idx ON learning.rm_grade_objective_stats (tenant_id, school_id, subject_id);

-- ---------- learning.rm_school_grade_summary: assignment completion of the grade (replaces rm_class_subject_summary) ----------
ALTER TABLE learning.rm_school_grade_summary
  ADD COLUMN assignment_completion_num bigint NOT NULL DEFAULT 0,
  ADD COLUMN assignment_completion_den bigint NOT NULL DEFAULT 0,
  ADD CONSTRAINT rm_school_grade_summary_completion_ck CHECK (assignment_completion_num BETWEEN 0 AND assignment_completion_den);

-- ---------- columns and tables of the class level: no longer written nor read (DROP at WP-R, ADR-0095 D4) ----------
COMMENT ON COLUMN learning.learning_events.class_id IS 'ADR-0095: no longer written (grade_level instead); dropped by WP-R';
COMMENT ON COLUMN learning.progress_daily.class_id IS 'ADR-0095: no longer written (grade_level instead); dropped by WP-R';
COMMENT ON COLUMN learning.rm_student_attention.class_id IS 'ADR-0095: no longer written (grade from ref.students); dropped by WP-R';
COMMENT ON COLUMN learning.rollup_school_daily.active_classes IS 'ADR-0095: no longer written; dropped by WP-R';
COMMENT ON COLUMN learning.rollup_tenant_daily.active_classes IS 'ADR-0095: no longer written; dropped by WP-R';
COMMENT ON TABLE learning.rollup_class_daily IS 'ADR-0095: no longer written nor read; dropped by WP-R';
COMMENT ON TABLE learning.rollup_class_weekly IS 'ADR-0095: no longer written nor read; dropped by WP-R';
COMMENT ON TABLE learning.rollup_class_monthly IS 'ADR-0095: no longer written nor read; dropped by WP-R';
COMMENT ON TABLE learning.rollup_class_objective_weekly IS 'ADR-0095: replaced by rollup_grade_objective_weekly; dropped by WP-R';
COMMENT ON TABLE learning.rm_class_objective_stats IS 'ADR-0095: replaced by rm_grade_objective_stats; dropped by WP-R';
COMMENT ON TABLE learning.rm_class_subject_summary IS 'ADR-0095: replaced by rm_school_grade_summary; dropped by WP-R';

-- ---------- RLS (tenant condition only, ADR-0089 §1) and privileges ----------
ALTER TABLE learning.rollup_grade_objective_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_grade_objective_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_grade_objective_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_grade_objective_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_grade_objective_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rm_grade_objective_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_grade_objective_stats FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_grade_objective_stats FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_grade_objective_stats FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_grade_objective_stats FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON learning.rollup_grade_objective_weekly, learning.rm_grade_objective_stats FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.rollup_grade_objective_weekly, learning.rm_grade_objective_stats TO learning_app, learning_platform;
GRANT SELECT ON learning.rollup_grade_objective_weekly, learning.rm_grade_objective_stats TO learning_dashboard;
-- Default partitions are reached through their parent only (RLS of the parent, F031 rule).
REVOKE ALL ON learning.rollup_grade_objective_weekly_default FROM learning_app, learning_platform;
-- The grade of an assignment submission is the grade of the student when targeted (lms.assignment_target_students.grade_level,
-- WP-L1): the dashboard role reads it for the assignment progress by grade.
GRANT SELECT ON lms.assignment_target_students TO learning_dashboard;

-- ---------- partitions: the grade objective rollup replaces the class rollups (no longer written) ----------
CREATE OR REPLACE FUNCTION learning.ensure_rollup_partitions(p_from date, p_months int) RETURNS int
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, learning SET timezone = 'UTC' AS $$
DECLARE
  t text; m date; y date; part text; created int := 0; n bigint;
BEGIN
  IF p_months < 1 OR p_months > 24 THEN
    RAISE EXCEPTION 'p_months must be within 1..24';
  END IF;
  FOREACH t IN ARRAY ARRAY['rollup_school_daily', 'rollup_tenant_daily'] LOOP
    FOR i IN 0 .. p_months - 1 LOOP
      m := (date_trunc('month', p_from) + make_interval(months => i))::date;
      part := t || '_' || to_char(m, 'YYYY_MM');
      CONTINUE WHEN to_regclass('learning.' || part) IS NOT NULL;
      EXECUTE format('SELECT count(*) FROM learning.%I WHERE bucket_date >= %L AND bucket_date < %L', t || '_default', m,
        (m + interval '1 month')::date) INTO n;
      IF n > 0 THEN
        RAISE WARNING 'rows of % in %_default: partition % skipped', m, t, part;
        CONTINUE;
      END IF;
      EXECUTE format('CREATE TABLE learning.%I PARTITION OF learning.%I FOR VALUES FROM (%L) TO (%L)', part, t, m,
        (m + interval '1 month')::date);
      EXECUTE format('REVOKE ALL ON learning.%I FROM learning_app, learning_platform', part);
      created := created + 1;
    END LOOP;
  END LOOP;
  FOREACH t IN ARRAY ARRAY['rollup_school_weekly', 'rollup_tenant_weekly', 'rollup_grade_objective_weekly',
    'mastery_snapshots_weekly'] LOOP
    y := date_trunc('year', p_from)::date;
    WHILE y <= (date_trunc('month', p_from) + make_interval(months => p_months - 1))::date LOOP
      part := t || '_' || to_char(y, 'YYYY');
      IF to_regclass('learning.' || part) IS NULL THEN
        EXECUTE format('SELECT count(*) FROM learning.%I WHERE bucket_date >= %L AND bucket_date < %L', t || '_default', y,
          (y + interval '1 year')::date) INTO n;
        IF n > 0 THEN
          RAISE WARNING 'rows of % in %_default: partition % skipped', y, t, part;
        ELSE
          EXECUTE format('CREATE TABLE learning.%I PARTITION OF learning.%I FOR VALUES FROM (%L) TO (%L)', part, t, y,
            (y + interval '1 year')::date);
          EXECUTE format('REVOKE ALL ON learning.%I FROM learning_app, learning_platform', part);
          created := created + 1;
        END IF;
      END IF;
      y := (y + interval '1 year')::date;
    END LOOP;
  END LOOP;
  RETURN created;
END $$;
