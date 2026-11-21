-- ADR-0095 D3 (WP-L2, expand E4): the unit of the efficacy study of F035 is the cluster (school, grade) — a study targets
-- one grade (efficacy_studies.grade_level), so a cohort is a school of a tenant in the study: pairs of cohorts of the same
-- grade in two schools, arms, form orders, waves, withdrawals and the design effect by cohort (LEARNING_EFFICACY_PLAN §3).
--   * learning.efficacy_study_cohorts (T): one row per study × school (grade = grade of the study), backfilled from the
--     classes of the study; a school whose classes sat in different arms, pairs or form orders is flagged conflict = true
--     (never merged automatically: the researcher removes and adds the cohort again before activation);
--   * learning.efficacy_wave_cohort_sessions (T): the exam session of a cohort in a wave (replaces efficacy_wave_sessions);
--   * learning.efficacy_participants.class_id and learning.efficacy_withdrawals.class_id are no longer written (nullable);
--     the withdrawal scope 'cohort' (study × school) replaces 'class' (kept in the CHECK for the rows already recorded);
--   * learning.experiments.unit accepts 'school_grade' (default) beside 'student'; 'class' stays for the rows recorded.
-- The class tables (efficacy_study_classes, efficacy_wave_sessions) are no longer written nor read; DROP at WP-R (D4).
-- Hand-written like the F035 migration (explicit SQL, no ent entity). Expand only (AGENTS R12).

-- ---------- learning.efficacy_study_cohorts (T) ----------
CREATE TABLE learning.efficacy_study_cohorts (
  study_id uuid NOT NULL,
  school_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  pair_no smallint,
  arm varchar(64),
  assignment_method varchar(64) NOT NULL DEFAULT 'manual',
  form_order varchar(64),
  status varchar(64) NOT NULL DEFAULT 'enrolled',
  conflict boolean NOT NULL DEFAULT false,
  withdrawn_at timestamptz,
  withdrawal_id uuid,
  added_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_id, school_id),
  CONSTRAINT efficacy_study_cohorts_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  CONSTRAINT efficacy_study_cohorts_pair_ck CHECK (pair_no IS NULL OR pair_no BETWEEN 1 AND 200),
  CONSTRAINT efficacy_study_cohorts_arm_ck CHECK (arm IS NULL OR arm IN ('intervention','comparison','validation')),
  CONSTRAINT efficacy_study_cohorts_validation_ck CHECK (arm IS DISTINCT FROM 'validation' OR pair_no IS NULL),
  CONSTRAINT efficacy_study_cohorts_method_ck CHECK (assignment_method IN ('manual','random')),
  CONSTRAINT efficacy_study_cohorts_order_ck CHECK (form_order IS NULL OR form_order IN ('a_then_b','b_then_a')),
  CONSTRAINT efficacy_study_cohorts_status_ck CHECK (status IN ('enrolled','withdrawn'))
);
-- One enrolled study per cohort at a time (draft or active studies; the command checks the study status).
CREATE INDEX efficacy_study_cohorts_school_idx ON learning.efficacy_study_cohorts (tenant_id, school_id, grade_level) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_study_cohorts_pair_idx ON learning.efficacy_study_cohorts (study_id, pair_no) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_wave_cohort_sessions (T) ----------
CREATE TABLE learning.efficacy_wave_cohort_sessions (
  wave_id uuid NOT NULL,
  school_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  exam_session_id uuid NOT NULL,
  form varchar(16) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (wave_id, school_id),
  CONSTRAINT efficacy_wave_cohort_sessions_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  CONSTRAINT efficacy_wave_cohort_sessions_form_ck CHECK (form IN ('a','b')),
  CONSTRAINT efficacy_wave_cohort_sessions_status_ck CHECK (status IN ('active','cancelled'))
);
CREATE UNIQUE INDEX efficacy_wave_cohort_sessions_exam_uq ON learning.efficacy_wave_cohort_sessions (exam_session_id);
CREATE INDEX efficacy_wave_cohort_sessions_tenant_idx ON learning.efficacy_wave_cohort_sessions (tenant_id, school_id) WHERE deleted_at IS NULL;

-- ---------- participants, withdrawals, experiments ----------
ALTER TABLE learning.efficacy_participants ALTER COLUMN class_id DROP NOT NULL;
CREATE INDEX efficacy_participants_school_idx ON learning.efficacy_participants (tenant_id, study_id, school_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN learning.efficacy_participants.class_id IS 'ADR-0095: no longer written (cohort = school_id of the study); dropped by WP-R';
ALTER TABLE learning.efficacy_withdrawals ADD COLUMN grade_level smallint,
  ADD CONSTRAINT efficacy_withdrawals_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
ALTER TABLE learning.efficacy_withdrawals DROP CONSTRAINT efficacy_withdrawals_scope_ck;
ALTER TABLE learning.efficacy_withdrawals ADD CONSTRAINT efficacy_withdrawals_scope_ck
  CHECK (scope IN ('tenant','school','class','cohort','students'));
COMMENT ON COLUMN learning.efficacy_withdrawals.class_id IS 'ADR-0095: no longer written (scope cohort = school_id + grade_level); dropped by WP-R';
ALTER TABLE learning.experiments DROP CONSTRAINT experiments_unit_ck;
ALTER TABLE learning.experiments ADD CONSTRAINT experiments_unit_ck CHECK (unit IN ('class','school_grade','student'));
ALTER TABLE learning.experiments ALTER COLUMN unit SET DEFAULT 'school_grade';
COMMENT ON TABLE learning.efficacy_study_classes IS 'ADR-0095: replaced by efficacy_study_cohorts; dropped by WP-R';
COMMENT ON TABLE learning.efficacy_wave_sessions IS 'ADR-0095: replaced by efficacy_wave_cohort_sessions; dropped by WP-R';

-- ---------- RLS (tenant condition only, ADR-0089 §1), live view and privileges ----------
ALTER TABLE learning.efficacy_study_cohorts ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_study_cohorts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_study_cohorts FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_study_cohorts FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_study_cohorts FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE learning.efficacy_wave_cohort_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_wave_cohort_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_wave_cohort_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_wave_cohort_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_wave_cohort_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW learning.efficacy_study_cohorts_live WITH (security_barrier = true) AS
  SELECT * FROM learning.efficacy_study_cohorts WHERE deleted_at IS NULL;
GRANT SELECT ON learning.efficacy_study_cohorts_live TO learning_readonly;
REVOKE ALL ON learning.efficacy_study_cohorts, learning.efficacy_wave_cohort_sessions FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON learning.efficacy_study_cohorts, learning.efficacy_wave_cohort_sessions FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.efficacy_study_cohorts, learning.efficacy_wave_cohort_sessions TO learning_app, learning_platform;

-- ---------- backfill of the cohorts from the classes (one-off, learning_migrator over every tenant) ----------
-- BEGIN adr0095-cohort-backfill
CREATE POLICY adr0095_backfill_read ON learning.efficacy_study_classes FOR SELECT TO learning_migrator USING (true);
CREATE POLICY adr0095_backfill_insert ON learning.efficacy_study_cohorts FOR INSERT TO learning_migrator WITH CHECK (true);
CREATE POLICY adr0095_backfill_read ON learning.efficacy_study_cohorts FOR SELECT TO learning_migrator USING (true);

-- tenancyguard:ok one-off backfill as learning_migrator over every tenant; the cohort keeps the tenant of its classes
INSERT INTO learning.efficacy_study_cohorts (study_id, school_id, tenant_id, grade_level, pair_no, arm, assignment_method, form_order,
    status, conflict, withdrawn_at, withdrawal_id, added_by, created_at, updated_at)
SELECT c.study_id, c.school_id, min(c.tenant_id::text)::uuid, min(c.grade_level),
       CASE WHEN count(DISTINCT c.pair_no) <= 1 THEN min(c.pair_no) END,
       CASE WHEN count(DISTINCT COALESCE(c.arm, '')) = 1 THEN min(c.arm) END,
       CASE WHEN count(DISTINCT c.assignment_method) = 1 THEN min(c.assignment_method) ELSE 'manual' END,
       CASE WHEN count(DISTINCT COALESCE(c.form_order, '')) = 1 THEN min(c.form_order) END,
       CASE WHEN bool_and(c.status = 'withdrawn') THEN 'withdrawn' ELSE 'enrolled' END,
       count(DISTINCT COALESCE(c.arm, '')) > 1 OR count(DISTINCT COALESCE(c.pair_no, 0)) > 1
         OR count(DISTINCT COALESCE(c.form_order, '')) > 1 OR count(DISTINCT c.tenant_id) > 1,
       max(c.withdrawn_at) FILTER (WHERE c.status = 'withdrawn'), NULL, min(c.added_by::text)::uuid, min(c.created_at), now()
FROM learning.efficacy_study_classes c
WHERE c.deleted_at IS NULL
GROUP BY c.study_id, c.school_id
ON CONFLICT (study_id, school_id) DO NOTHING;

DROP POLICY adr0095_backfill_read ON learning.efficacy_study_classes;
DROP POLICY adr0095_backfill_insert ON learning.efficacy_study_cohorts;
DROP POLICY adr0095_backfill_read ON learning.efficacy_study_cohorts;
-- END adr0095-cohort-backfill
