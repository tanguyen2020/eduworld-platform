-- F043 T006/T007 — rollup analytics and School Learning Dashboard (DASHBOARD_DATA §3–§9, LEARNING_ANALYTICS §5, ADR-0062).
-- Derived tables (no deleted_at, rebuilt from their sources, DASHBOARD_DATA §3.1): time-series rollups of class, school and
-- tenant per day/week/month, the class × objective weekly rollup, the weekly mastery snapshots, the state read models rm_*
-- and the closing state of the days. Written by the worker (learning_app, RLS of the session tenant); read by the dashboard
-- role learning_dashboard (SELECT on rollups/read models only, never learning_events, attempts nor attempt_answers —
-- F043-AC-18). Daily tables are partitioned by month, weekly ones by year, with a DEFAULT partition (DASHBOARD_DATA §6);
-- partitions are kept ahead by learning.ensure_rollup_partitions (job learning-events-partitions, learning_partition_admin).
-- Hand-written like progress_daily of F031 (explicit SQL only, no ent entity). Expand only (AGENTS R12).

-- ---------- learning.progress_daily: correct answers that were rapid guesses (accuracy excludes them, F043-AC-05) ----------
ALTER TABLE learning.progress_daily ADD COLUMN items_correct_rapid_guess integer NOT NULL DEFAULT 0;
CREATE INDEX progress_daily_class_idx ON learning.progress_daily (tenant_id, class_id, bucket_date);
CREATE INDEX progress_daily_student_idx ON learning.progress_daily (tenant_id, student_id, bucket_date);
CREATE INDEX progress_daily_school_idx ON learning.progress_daily (tenant_id, school_id, bucket_date);
CREATE INDEX progress_daily_bucket_brin ON learning.progress_daily USING brin (bucket_date);
-- Days still open for the job dashboard-day-close.
CREATE INDEX progress_daily_open_idx ON learning.progress_daily (tenant_id, bucket_date) WHERE NOT is_final;

-- ---------- learning.rollup_days (T): closing state of a tenant day (job dashboard-day-close) ----------
CREATE TABLE learning.rollup_days (
  tenant_id uuid NOT NULL,
  bucket_date date NOT NULL,
  is_final boolean NOT NULL DEFAULT false,
  needs_reclose boolean NOT NULL DEFAULT false,
  closed_at timestamptz,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, bucket_date)
);
CREATE INDEX rollup_days_reclose_idx ON learning.rollup_days (tenant_id, bucket_date) WHERE needs_reclose;

-- ---------- class rollups (T): class × subject × bucket ----------
-- grade_level 0 = unknown grade [Proposed]; active_students = distinct students of the class × subject, active_students_all
-- = distinct students of the class in any subject (same value on every subject row of the bucket); both exact only when
-- is_final (today: refresh job). assignments_due/completed/on_time: submissions due in the bucket (not excused) / submitted
-- (submitted or late) / submitted on time — recomputed from lms.assignment_submissions by the jobs.
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

-- ---------- school rollups (T): school × grade × subject × bucket (+ classes with activity) ----------
CREATE TABLE learning.rollup_school_daily (
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  subject_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  active_students integer,
  active_students_all integer,
  active_classes integer,
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
  PRIMARY KEY (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version),
  CONSTRAINT rollup_school_daily_counts_ck CHECK (grade_level BETWEEN 0 AND 12 AND activities_completed >= 0 AND lessons_completed >= 0
    AND accuracy_num BETWEEN 0 AND accuracy_den AND rapid_guess_num BETWEEN 0 AND rapid_guess_den AND active_seconds >= 0
    AND assignments_completed BETWEEN 0 AND assignments_due AND assignments_on_time BETWEEN 0 AND assignments_completed)
) PARTITION BY RANGE (bucket_date);
CREATE INDEX rollup_school_daily_bucket_brin ON learning.rollup_school_daily USING brin (bucket_date);
CREATE TABLE learning.rollup_school_weekly (LIKE learning.rollup_school_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version)) PARTITION BY RANGE (bucket_date);
CREATE TABLE learning.rollup_school_monthly (LIKE learning.rollup_school_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version));

-- ---------- tenant rollups (T): grade × subject × bucket (+ schools with activity) ----------
CREATE TABLE learning.rollup_tenant_daily (
  tenant_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  subject_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  active_students integer,
  active_students_all integer,
  active_classes integer,
  active_schools integer,
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
  PRIMARY KEY (tenant_id, grade_level, subject_id, bucket_date, metric_version),
  CONSTRAINT rollup_tenant_daily_counts_ck CHECK (grade_level BETWEEN 0 AND 12 AND activities_completed >= 0 AND lessons_completed >= 0
    AND accuracy_num BETWEEN 0 AND accuracy_den AND rapid_guess_num BETWEEN 0 AND rapid_guess_den AND active_seconds >= 0
    AND assignments_completed BETWEEN 0 AND assignments_due AND assignments_on_time BETWEEN 0 AND assignments_completed)
) PARTITION BY RANGE (bucket_date);
CREATE INDEX rollup_tenant_daily_bucket_brin ON learning.rollup_tenant_daily USING brin (bucket_date);
CREATE TABLE learning.rollup_tenant_weekly (LIKE learning.rollup_tenant_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, grade_level, subject_id, bucket_date, metric_version)) PARTITION BY RANGE (bucket_date);
CREATE TABLE learning.rollup_tenant_monthly (LIKE learning.rollup_tenant_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (tenant_id, grade_level, subject_id, bucket_date, metric_version));

-- ---------- learning.rollup_class_objective_weekly (T): class × objective × ISO week ----------
-- level_dist = students of the class per level (not_started, beginning, developing, proficient, mastered) at the last refresh
-- of the week; evidence/correct from the scored first answers of the week (rapid guesses excluded from correct_*).
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

-- ---------- learning.mastery_snapshots_weekly (T, P year): mastery of student × subject at the end of an ISO week ----------
CREATE TABLE learning.mastery_snapshots_weekly (
  tenant_id uuid NOT NULL,
  student_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  bucket_date date NOT NULL,
  school_id uuid NOT NULL,
  mean_mastery numeric(4,3) NOT NULL,
  mastered_count integer NOT NULL DEFAULT 0,
  level_dist integer[] NOT NULL DEFAULT '{0,0,0,0,0}',
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, student_id, subject_id, bucket_date),
  CONSTRAINT mastery_snapshots_weekly_ck CHECK (mean_mastery BETWEEN 0 AND 1 AND mastered_count >= 0 AND cardinality(level_dist) = 5)
) PARTITION BY RANGE (bucket_date);

-- ---------- state read models (T): refreshed every school_dashboard_refresh (job read-model-refresh) ----------
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

-- student_count = enrolled students of the school × grade (k-anonymity of the tenant/platform summaries, F043-AC-25);
-- recommendation_started_* = recommendations generated in the last 7 days / started; objectives_improved = objectives
-- recommended in the last 28 days whose level rose within 14 days (recommendation_follow_rate, LEARNING_ANALYTICS §4).
CREATE TABLE learning.rm_school_grade_summary (
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  subject_id uuid NOT NULL,
  student_count integer NOT NULL DEFAULT 0,
  curriculum_coverage_num bigint NOT NULL DEFAULT 0,
  curriculum_coverage_den bigint NOT NULL DEFAULT 0,
  participation_num bigint NOT NULL DEFAULT 0,
  participation_den bigint NOT NULL DEFAULT 0,
  recommendation_started_num bigint NOT NULL DEFAULT 0,
  recommendation_started_den bigint NOT NULL DEFAULT 0,
  objectives_improved integer NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, school_id, grade_level, subject_id),
  CONSTRAINT rm_school_grade_summary_ck CHECK (student_count >= 0 AND objectives_improved >= 0
    AND curriculum_coverage_num BETWEEN 0 AND curriculum_coverage_den AND participation_num BETWEEN 0 AND participation_den
    AND recommendation_started_num BETWEEN 0 AND recommendation_started_den)
);

-- reasons ⊆ {missed_assignments, beginning_objectives, inactive_game, rapid_guess, scheduled_cleanup}; a student without
-- reason keeps an empty array (no delete, ADR-0028).
CREATE TABLE learning.rm_student_attention (
  tenant_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid,
  reasons varchar(64)[] NOT NULL DEFAULT '{}',
  missed_assignments integer NOT NULL DEFAULT 0,
  beginning_objectives integer NOT NULL DEFAULT 0,
  rapid_guess_num bigint NOT NULL DEFAULT 0,
  rapid_guess_den bigint NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, student_id),
  CONSTRAINT rm_student_attention_ck CHECK (cardinality(reasons) <= 8 AND reasons <@ ARRAY['missed_assignments','beginning_objectives',
    'inactive_game','rapid_guess','scheduled_cleanup']::varchar(64)[] AND missed_assignments >= 0 AND beginning_objectives >= 0
    AND rapid_guess_num BETWEEN 0 AND rapid_guess_den)
);
CREATE INDEX rm_student_attention_school_idx ON learning.rm_student_attention (tenant_id, school_id, class_id) WHERE cardinality(reasons) > 0;

-- ---------- DEFAULT partitions (DASHBOARD_DATA §6: writes keep working; alert when rows land there) ----------
CREATE TABLE learning.rollup_class_daily_default PARTITION OF learning.rollup_class_daily DEFAULT;
CREATE TABLE learning.rollup_class_weekly_default PARTITION OF learning.rollup_class_weekly DEFAULT;
CREATE TABLE learning.rollup_school_daily_default PARTITION OF learning.rollup_school_daily DEFAULT;
CREATE TABLE learning.rollup_school_weekly_default PARTITION OF learning.rollup_school_weekly DEFAULT;
CREATE TABLE learning.rollup_tenant_daily_default PARTITION OF learning.rollup_tenant_daily DEFAULT;
CREATE TABLE learning.rollup_tenant_weekly_default PARTITION OF learning.rollup_tenant_weekly DEFAULT;
CREATE TABLE learning.rollup_class_objective_weekly_default PARTITION OF learning.rollup_class_objective_weekly DEFAULT;
CREATE TABLE learning.mastery_snapshots_weekly_default PARTITION OF learning.mastery_snapshots_weekly DEFAULT;

-- ---------- RLS (tenant condition only, ADR-0089 §1) and privileges ----------
ALTER TABLE learning.rollup_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_days FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_days FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_days FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_days FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
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
ALTER TABLE learning.rollup_school_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_school_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_school_daily FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_school_daily FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_school_daily FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_school_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_school_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_school_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_school_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_school_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_school_monthly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_school_monthly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_school_monthly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_school_monthly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_school_monthly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_tenant_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_tenant_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_tenant_daily FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_tenant_daily FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_tenant_daily FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_tenant_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_tenant_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_tenant_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_tenant_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_tenant_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_tenant_monthly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_tenant_monthly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_tenant_monthly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_tenant_monthly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_tenant_monthly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rollup_class_objective_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rollup_class_objective_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rollup_class_objective_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rollup_class_objective_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rollup_class_objective_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.mastery_snapshots_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.mastery_snapshots_weekly FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.mastery_snapshots_weekly FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.mastery_snapshots_weekly FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.mastery_snapshots_weekly FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
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
ALTER TABLE learning.rm_school_grade_summary ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_school_grade_summary FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_school_grade_summary FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_school_grade_summary FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_school_grade_summary FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.rm_student_attention ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_student_attention FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_student_attention FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_student_attention FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_student_attention FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON learning.rollup_days, learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly, learning.rollup_school_daily, learning.rollup_school_weekly, learning.rollup_school_monthly, learning.rollup_tenant_daily, learning.rollup_tenant_weekly, learning.rollup_tenant_monthly, learning.rollup_class_objective_weekly, learning.mastery_snapshots_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary, learning.rm_school_grade_summary, learning.rm_student_attention FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.rollup_days, learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly, learning.rollup_school_daily, learning.rollup_school_weekly, learning.rollup_school_monthly, learning.rollup_tenant_daily, learning.rollup_tenant_weekly, learning.rollup_tenant_monthly, learning.rollup_class_objective_weekly, learning.mastery_snapshots_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary, learning.rm_school_grade_summary, learning.rm_student_attention TO learning_app, learning_platform;
GRANT SELECT ON learning.rollup_days, learning.rollup_class_daily, learning.rollup_class_weekly, learning.rollup_class_monthly, learning.rollup_school_daily, learning.rollup_school_weekly, learning.rollup_school_monthly, learning.rollup_tenant_daily, learning.rollup_tenant_weekly, learning.rollup_tenant_monthly, learning.rollup_class_objective_weekly, learning.mastery_snapshots_weekly, learning.rm_class_objective_stats, learning.rm_class_subject_summary, learning.rm_school_grade_summary, learning.rm_student_attention TO learning_dashboard;
-- Default partitions are reached through their parent only (RLS of the parent, F031 rule).
REVOKE ALL ON learning.rollup_class_daily_default, learning.rollup_class_weekly_default, learning.rollup_school_daily_default,
  learning.rollup_school_weekly_default, learning.rollup_tenant_daily_default, learning.rollup_tenant_weekly_default,
  learning.rollup_class_objective_weekly_default, learning.mastery_snapshots_weekly_default FROM learning_app, learning_platform;

-- learning_dashboard (DASHBOARD_DATA §7, F043-AC-18): SELECT only on the rollups/read models above and the state sources of
-- DASHBOARD_DATA §2 (progress_daily, objective_masteries, rm_student_recommendations, lms.assignments,
-- lms.assignment_submissions), the ref projections (names, enrolments, scope) and the curriculum metadata (codes, topics;
-- their content_read policy reads infra.tenant_resource_assignments). No privilege on learning_events, mastery_history,
-- assessment.* nor any other table; statement_timeout / idle_in_transaction_session_timeout / work_mem are role settings of
-- the cluster bootstrap (deploy/postgres/init/02-roles.sh).
GRANT USAGE ON SCHEMA learning, lms, ref, curriculum, infra TO learning_dashboard;
GRANT EXECUTE ON FUNCTION infra.current_tenant_id() TO learning_dashboard;
GRANT SELECT ON learning.progress_daily, learning.objective_masteries, learning.rm_student_recommendations,
  lms.assignments, lms.assignment_submissions, ref.tenants, ref.schools, ref.classes, ref.students, ref.enrollments,
  curriculum.topics, curriculum.learning_objectives, infra.tenant_resource_assignments TO learning_dashboard;

-- ---------- partitions of the rollups (DASHBOARD_DATA §6: three months ahead) ----------
-- Monthly partitions of the daily rollups and yearly partitions of the weekly ones covering [month of p_from, + p_months),
-- named <table>_YYYY_MM / <table>_YYYY. A range whose rows already sit in the DEFAULT partition is skipped with a warning
-- (alert on rows in *_default). Runs as the owner; new partitions get no direct grant (access through the parent).
CREATE FUNCTION learning.ensure_rollup_partitions(p_from date, p_months int) RETURNS int
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, learning SET timezone = 'UTC' AS $$
DECLARE
  t text; m date; y date; part text; created int := 0; n bigint;
BEGIN
  IF p_months < 1 OR p_months > 24 THEN
    RAISE EXCEPTION 'p_months must be within 1..24';
  END IF;
  FOREACH t IN ARRAY ARRAY['rollup_class_daily', 'rollup_school_daily', 'rollup_tenant_daily'] LOOP
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
  FOREACH t IN ARRAY ARRAY['rollup_class_weekly', 'rollup_school_weekly', 'rollup_tenant_weekly', 'rollup_class_objective_weekly',
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
REVOKE ALL ON FUNCTION learning.ensure_rollup_partitions(date, int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION learning.ensure_rollup_partitions(date, int) TO learning_partition_admin;
