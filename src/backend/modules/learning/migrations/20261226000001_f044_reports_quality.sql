-- F044 T006/T007 — reports, asynchronous exports, item analysis, difficulty calibration and exam quality
-- (specs/044-reports-item-analysis: data-model §1; DASHBOARD_DATA §10, QUESTION_BANK §9, EXAM_ENGINE §4.1/§8.1,
-- ASSESSMENT_REGULATION_PROFILE §4). Hand-written tables read and written by explicit SQL (no ent entity, like F041–F043).
-- Expand only (AGENTS R12); soft-delete only on the business tables (R21); derived tables have no deleted_at.

-- ---------- learning.report_jobs (T; tenant_id NULL = cross-tenant export of System Admin) ----------
CREATE TABLE learning.report_jobs (
  report_id uuid NOT NULL,
  tenant_id uuid,
  scope_level varchar(64) NOT NULL,
  report_type varchar(64) NOT NULL,
  format varchar(16) NOT NULL,
  params jsonb NOT NULL DEFAULT '{}',
  requested_by uuid NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'queued',
  row_count integer,
  storage_key varchar(512),
  error varchar(1000),
  idempotency_key varchar(64) NOT NULL,
  run_count smallint NOT NULL DEFAULT 0,
  started_at timestamptz,
  finished_at timestamptz,
  file_expires_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (report_id),
  CONSTRAINT report_jobs_scope_ck CHECK (scope_level IN ('lms','tenant','platform')),
  CONSTRAINT report_jobs_tenant_ck CHECK ((scope_level = 'platform') = (tenant_id IS NULL)),
  CONSTRAINT report_jobs_type_ck CHECK (report_type IN ('assignment_results','course_results','regulation_reference','skill_map',
    'school_monthly','learning_summary','class_progress','game_summary','efficacy_dataset')),
  CONSTRAINT report_jobs_format_ck CHECK (format IN ('csv','xlsx','pdf')),
  CONSTRAINT report_jobs_params_ck CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 4096),
  CONSTRAINT report_jobs_status_ck CHECK (status IN ('queued','running','succeeded','failed','expired')),
  CONSTRAINT report_jobs_rows_ck CHECK (row_count IS NULL OR row_count >= 0),
  CONSTRAINT report_jobs_runs_ck CHECK (run_count >= 0)
);
CREATE UNIQUE INDEX report_jobs_idem_uq ON learning.report_jobs (requested_by, idempotency_key) WHERE deleted_at IS NULL;
CREATE INDEX report_jobs_requester_idx ON learning.report_jobs (tenant_id, requested_by, status) WHERE deleted_at IS NULL;
-- Worker: queued/running jobs of every tenant; expiry of the succeeded ones.
CREATE INDEX report_jobs_pending_idx ON learning.report_jobs (status, created_at)
  WHERE deleted_at IS NULL AND status IN ('queued','running');
CREATE INDEX report_jobs_expiry_idx ON learning.report_jobs (file_expires_at) WHERE deleted_at IS NULL AND status = 'succeeded';

-- ---------- learning.assessment_profiles (H): reference profiles TT27/TT22 with versions ----------
CREATE TABLE learning.assessment_profiles (
  profile_id uuid NOT NULL,
  tenant_id uuid,
  owner_scope varchar(64) NOT NULL DEFAULT 'platform',
  profile_code varchar(64) NOT NULL,
  profile_version integer NOT NULL,
  grade_levels smallint[] NOT NULL,
  subject_ids uuid[] NOT NULL DEFAULT '{}',
  rules jsonb NOT NULL,
  disclaimer jsonb NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  created_by uuid,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (profile_id),
  CONSTRAINT assessment_profiles_owner_ck CHECK (owner_scope IN ('platform','tenant') AND (owner_scope = 'platform') = (tenant_id IS NULL)),
  CONSTRAINT assessment_profiles_code_ck CHECK (profile_code IN ('primary_tt27','lower_secondary_tt22')),
  CONSTRAINT assessment_profiles_version_ck CHECK (profile_version >= 1),
  CONSTRAINT assessment_profiles_grades_ck CHECK (cardinality(grade_levels) BETWEEN 1 AND 9),
  CONSTRAINT assessment_profiles_subjects_ck CHECK (cardinality(subject_ids) <= 50),
  CONSTRAINT assessment_profiles_rules_ck CHECK (jsonb_typeof(rules) = 'object' AND octet_length(rules::text) <= 16384),
  CONSTRAINT assessment_profiles_disclaimer_ck CHECK (jsonb_typeof(disclaimer) = 'object' AND octet_length(disclaimer::text) <= 2048),
  CONSTRAINT assessment_profiles_status_ck CHECK (status IN ('active','superseded'))
);
CREATE UNIQUE INDEX assessment_profiles_version_uq ON learning.assessment_profiles
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), profile_code, profile_version) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX assessment_profiles_active_uq ON learning.assessment_profiles
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), profile_code) WHERE status = 'active' AND deleted_at IS NULL;

-- Version 1 of the platform profiles: assessment_reference_thresholds (CONFIG_DEFAULTS, ASSESSMENT_REGULATION_PROFILE §4).
INSERT INTO learning.assessment_profiles (profile_id, tenant_id, owner_scope, profile_code, profile_version, grade_levels, rules, disclaimer)
VALUES
  ('01936b2a-0000-7000-8000-000000000271', NULL, 'platform', 'primary_tt27', 1, '{1,2,3,4,5}',
   '{"min_coverage":0.7,"min_evidence":3,"excellent_proficient_ratio":0.8,"excellent_mastered_ratio":0.4,"completed_developing_ratio":0.6,"pass_developing_ratio":0.6}',
   '{"vi":"Mức tham khảo do EduWorld ước lượng từ dữ liệu học tập, chỉ để nhà trường tham khảo, không thay thế đánh giá chính thức theo Thông tư 27/2020 và Thông tư 22/2021.","en":"Reference level estimated by EduWorld from learning data, for the school''s reference only; it does not replace the official assessment under Circulars 27/2020 and 22/2021."}'),
  ('01936b2a-0000-7000-8000-000000000221', NULL, 'platform', 'lower_secondary_tt22', 1, '{6,7,8,9}',
   '{"min_coverage":0.7,"min_evidence":3,"excellent_proficient_ratio":0.8,"excellent_mastered_ratio":0.4,"completed_developing_ratio":0.6,"pass_developing_ratio":0.6}',
   '{"vi":"Mức tham khảo do EduWorld ước lượng từ dữ liệu học tập, chỉ để nhà trường tham khảo, không thay thế đánh giá chính thức theo Thông tư 27/2020 và Thông tư 22/2021.","en":"Reference level estimated by EduWorld from learning data, for the school''s reference only; it does not replace the official assessment under Circulars 27/2020 and 22/2021."}');

-- ---------- learning.rm_student_subject_reference_levels (T, derived read model) ----------
CREATE TABLE learning.rm_student_subject_reference_levels (
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid NOT NULL,
  student_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  academic_year smallint NOT NULL,
  period varchar(64) NOT NULL,
  profile_id uuid NOT NULL,
  profile_code varchar(64) NOT NULL,
  profile_version integer NOT NULL,
  level_code varchar(64) NOT NULL,
  objectives_in_scope integer NOT NULL,
  coverage_ratio numeric(5,4) NOT NULL,
  proficient_ratio numeric(5,4) NOT NULL,
  mastered_ratio numeric(5,4) NOT NULL,
  official_score numeric(4,1),
  low_reliability_exam_ids uuid[] NOT NULL DEFAULT '{}',
  rapid_guess_ratio numeric(5,4) NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, student_id, subject_id, academic_year, period, profile_version),
  CONSTRAINT rm_reference_levels_period_ck CHECK (period IN ('mid_term_1','end_term_1','mid_term_2','end_term_2')),
  CONSTRAINT rm_reference_levels_level_ck CHECK (level_code IN ('excellent','completed','not_completed','pass','not_pass','insufficient_data')),
  CONSTRAINT rm_reference_levels_ratios_ck CHECK (coverage_ratio BETWEEN 0 AND 1 AND proficient_ratio BETWEEN 0 AND 1
    AND mastered_ratio BETWEEN 0 AND 1 AND rapid_guess_ratio BETWEEN 0 AND 1),
  CONSTRAINT rm_reference_levels_score_ck CHECK (official_score IS NULL OR official_score BETWEEN 0 AND 10),
  CONSTRAINT rm_reference_levels_exams_ck CHECK (cardinality(low_reliability_exam_ids) <= 20)
);
CREATE INDEX rm_reference_levels_school_idx ON learning.rm_student_subject_reference_levels
  (tenant_id, school_id, subject_id, academic_year, period, class_id);

-- ---------- content.question_stats (H, derived): item analysis per question version × grade ----------
CREATE TABLE content.question_stats (
  tenant_id uuid,
  question_id uuid NOT NULL,
  question_version_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  first_attempt_count integer NOT NULL DEFAULT 0,
  excluded_rapid_guess_count integer NOT NULL DEFAULT 0,
  with_hint_count integer NOT NULL DEFAULT 0,
  p_value numeric(5,4),
  p_value_with_hint numeric(5,4),
  discrimination numeric(5,4),
  distractor_rates jsonb NOT NULL DEFAULT '{}',
  avg_time_ms integer NOT NULL DEFAULT 0,
  rapid_guess_rate numeric(5,4) NOT NULL DEFAULT 0,
  hint_usage_rate numeric(5,4) NOT NULL DEFAULT 0,
  flags varchar(64)[] NOT NULL DEFAULT '{}',
  computed_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT question_stats_counts_ck CHECK (first_attempt_count >= 0 AND excluded_rapid_guess_count >= 0 AND with_hint_count >= 0
    AND avg_time_ms >= 0),
  CONSTRAINT question_stats_grade_ck CHECK (grade_level BETWEEN 0 AND 12),
  CONSTRAINT question_stats_rates_ck CHECK ((p_value IS NULL OR p_value BETWEEN 0 AND 1)
    AND (p_value_with_hint IS NULL OR p_value_with_hint BETWEEN 0 AND 1) AND (discrimination IS NULL OR discrimination BETWEEN -1 AND 1)
    AND rapid_guess_rate BETWEEN 0 AND 1 AND hint_usage_rate BETWEEN 0 AND 1),
  CONSTRAINT question_stats_distractors_ck CHECK (jsonb_typeof(distractor_rates) = 'object' AND octet_length(distractor_rates::text) <= 4096),
  CONSTRAINT question_stats_flags_ck CHECK (cardinality(flags) <= 8)
);
CREATE UNIQUE INDEX question_stats_pk ON content.question_stats
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), question_version_id, grade_level);
CREATE INDEX question_stats_question_idx ON content.question_stats (question_id);

-- ---------- content.difficulty_calibration_proposals (H) ----------
CREATE TABLE content.difficulty_calibration_proposals (
  proposal_id uuid NOT NULL,
  tenant_id uuid,
  owner_scope varchar(64) NOT NULL,
  question_id uuid NOT NULL,
  based_on_version_id uuid NOT NULL,
  current_difficulty smallint NOT NULL,
  proposed_difficulty smallint NOT NULL,
  p_value numeric(5,4) NOT NULL,
  sample_size integer NOT NULL,
  grade_level smallint NOT NULL,
  grade_distribution jsonb NOT NULL DEFAULT '{}',
  status varchar(64) NOT NULL DEFAULT 'pending',
  decided_by uuid,
  decided_at timestamptz,
  created_version_id uuid,
  consecutive_dismissals smallint NOT NULL DEFAULT 0,
  suppressed_until date,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (proposal_id),
  CONSTRAINT calibration_owner_ck CHECK (owner_scope IN ('platform','tenant') AND (owner_scope = 'platform') = (tenant_id IS NULL)),
  CONSTRAINT calibration_difficulty_ck CHECK (current_difficulty BETWEEN 1 AND 5 AND proposed_difficulty BETWEEN 1 AND 5
    AND current_difficulty <> proposed_difficulty),
  CONSTRAINT calibration_p_ck CHECK (p_value BETWEEN 0 AND 1 AND sample_size >= 0),
  CONSTRAINT calibration_grades_ck CHECK (jsonb_typeof(grade_distribution) = 'object' AND octet_length(grade_distribution::text) <= 2048),
  CONSTRAINT calibration_status_ck CHECK (status IN ('pending','accepted','dismissed')),
  CONSTRAINT calibration_decision_ck CHECK ((status = 'pending') = (decided_at IS NULL)),
  CONSTRAINT calibration_dismissals_ck CHECK (consecutive_dismissals >= 0)
);
CREATE UNIQUE INDEX calibration_pending_uq ON content.difficulty_calibration_proposals (question_id)
  WHERE status = 'pending' AND deleted_at IS NULL;
CREATE INDEX calibration_queue_idx ON content.difficulty_calibration_proposals (tenant_id, status, created_at) WHERE deleted_at IS NULL;
CREATE INDEX calibration_question_idx ON content.difficulty_calibration_proposals (question_id, created_at DESC) WHERE deleted_at IS NULL;

-- ---------- assessment.exam_quality_reports (T, derived) ----------
CREATE TABLE assessment.exam_quality_reports (
  exam_session_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_count integer NOT NULL DEFAULT 0,
  variants jsonb NOT NULL DEFAULT '[]',
  variant_gap_flag boolean NOT NULL DEFAULT false,
  problem_items jsonb NOT NULL DEFAULT '[]',
  items jsonb NOT NULL DEFAULT '[]',
  score_distribution integer[] NOT NULL DEFAULT '{}',
  mean_score numeric(4,2),
  median_score numeric(4,2),
  pass_rate numeric(5,4),
  reliability_warning boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (exam_session_id),
  CONSTRAINT exam_quality_variants_ck CHECK (jsonb_typeof(variants) = 'array' AND octet_length(variants::text) <= 32768),
  CONSTRAINT exam_quality_problem_ck CHECK (jsonb_typeof(problem_items) = 'array' AND octet_length(problem_items::text) <= 32768),
  CONSTRAINT exam_quality_items_ck CHECK (jsonb_typeof(items) = 'array' AND octet_length(items::text) <= 131072),
  CONSTRAINT exam_quality_distribution_ck CHECK (cardinality(score_distribution) <= 21),
  CONSTRAINT exam_quality_scores_ck CHECK ((mean_score IS NULL OR mean_score BETWEEN 0 AND 10) AND (median_score IS NULL OR median_score BETWEEN 0 AND 10)
    AND (pass_rate IS NULL OR pass_rate BETWEEN 0 AND 1) AND student_count >= 0)
);
CREATE INDEX exam_quality_school_idx ON assessment.exam_quality_reports (tenant_id, school_id, computed_at);

-- ---------- assessment.exam_papers: result of the variant balancer (EXAM_ENGINE §4.1) ----------
ALTER TABLE assessment.exam_papers ADD COLUMN balance_warning boolean NOT NULL DEFAULT false;

-- ---------- RLS (tenant condition only, ADR-0089 §1) ----------
ALTER TABLE learning.report_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.report_jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.report_jobs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.report_jobs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.report_jobs FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.assessment_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.assessment_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON learning.assessment_profiles FOR SELECT USING (tenant_id = infra.current_tenant_id() OR tenant_id IS NULL);
CREATE POLICY tenant_insert ON learning.assessment_profiles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.assessment_profiles FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.rm_student_subject_reference_levels ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_student_subject_reference_levels FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_student_subject_reference_levels FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_student_subject_reference_levels FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_student_subject_reference_levels FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- Statistics of a platform question (tenant_id NULL) are read by the tenants the question is assigned to (read-only).
ALTER TABLE content.question_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.question_stats FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.question_stats FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question' AND a.resource_id = question_stats.question_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.question_stats FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.question_stats FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- Proposals of a platform question are decided by platform reviewers only (learning_platform); a tenant sees its own and,
-- read-only, those of the platform questions assigned to it (accepting them is refused with 403, F044-AC-21).
ALTER TABLE content.difficulty_calibration_proposals ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.difficulty_calibration_proposals FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.difficulty_calibration_proposals FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'question' AND a.resource_id = difficulty_calibration_proposals.question_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.difficulty_calibration_proposals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.difficulty_calibration_proposals FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE assessment.exam_quality_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.exam_quality_reports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.exam_quality_reports FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.exam_quality_reports FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.exam_quality_reports FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows of the business tables (ADR-0089 §1).
CREATE OR REPLACE VIEW learning.report_jobs_live WITH (security_barrier = true) AS
  SELECT * FROM learning.report_jobs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW learning.assessment_profiles_live WITH (security_barrier = true) AS
  SELECT * FROM learning.assessment_profiles WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.difficulty_calibration_proposals_live WITH (security_barrier = true) AS
  SELECT * FROM content.difficulty_calibration_proposals WHERE deleted_at IS NULL;
GRANT SELECT ON learning.report_jobs_live, learning.assessment_profiles_live, content.difficulty_calibration_proposals_live TO learning_readonly;
REVOKE ALL ON learning.report_jobs, learning.assessment_profiles, content.difficulty_calibration_proposals FROM learning_readonly;

-- ---------- privileges: no DELETE/TRUNCATE (R21) ----------
REVOKE DELETE, TRUNCATE ON learning.report_jobs, learning.assessment_profiles, learning.rm_student_subject_reference_levels,
  content.question_stats, content.difficulty_calibration_proposals, assessment.exam_quality_reports FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.report_jobs, learning.assessment_profiles, learning.rm_student_subject_reference_levels,
  content.question_stats, content.difficulty_calibration_proposals, assessment.exam_quality_reports TO learning_app, learning_platform;

-- learning_dashboard reads the exports on the replica (DASHBOARD_DATA §10): the reference read model and the profiles,
-- besides the rollups and LMS state sources granted by F043 (assignments, submissions; course rows are resolved on the
-- primary at request time). Still no privilege on lms.courses, attempts, attempt_answers, learning_events nor chat.
GRANT SELECT ON learning.rm_student_subject_reference_levels, learning.assessment_profiles TO learning_dashboard;
