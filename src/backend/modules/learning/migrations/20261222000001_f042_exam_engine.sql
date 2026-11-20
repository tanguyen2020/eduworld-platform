-- F042 — exam engine: exam sessions pinned to a blueprint version, immutable papers, participations, exam columns of the
-- attempts (specs/042-exam-engine: data-model §1; DATABASE_DESIGN §2.6, §2.16; EXAM_ENGINE §4–§7). Applied by
-- migrate --module=learning as learning_migrator. Hand-written tables (explicit-SQL repositories, like F040/F041);
-- expand only (AGENTS R12); soft-delete only (R21).

-- ---------- assessment.exam_sessions (T) ----------
CREATE TABLE assessment.exam_sessions (
  exam_session_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint,
  course_id uuid,
  assignment_id uuid,
  title varchar(255) NOT NULL,
  exam_type varchar(64) NOT NULL,
  is_official boolean NOT NULL,
  blueprint_id uuid NOT NULL,
  blueprint_version_id uuid NOT NULL,
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  duration_sec integer NOT NULL,
  target jsonb NOT NULL DEFAULT '[]',
  sections jsonb NOT NULL DEFAULT '[]',
  audience varchar(64) NOT NULL DEFAULT 'assigned',
  variant_strategy varchar(64) NOT NULL,
  variant_count smallint NOT NULL DEFAULT 1,
  feedback_timing varchar(64) NOT NULL,
  result_release varchar(64) NOT NULL,
  show_correct_answers boolean NOT NULL DEFAULT true,
  show_explanations boolean NOT NULL DEFAULT true,
  late_start_policy varchar(64) NOT NULL DEFAULT 'until_window_end',
  accommodations jsonb NOT NULL DEFAULT '[]',
  max_attempts smallint NOT NULL DEFAULT 1,
  score_policy varchar(64) NOT NULL DEFAULT 'last',
  question_count integer NOT NULL DEFAULT 0,
  total_points numeric(7,2) NOT NULL DEFAULT 0,
  status varchar(64) NOT NULL,
  created_by uuid NOT NULL,
  created_by_role varchar(64) NOT NULL,
  opened_at timestamptz,
  closed_at timestamptz,
  results_released_at timestamptz,
  results_released_by uuid,
  cancelled_at timestamptz,
  cancel_reason varchar(500),
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (exam_session_id),
  CONSTRAINT exam_sessions_type_ck CHECK (exam_type IN ('practice_test','mock','chapter','semester','subject','custom')),
  CONSTRAINT exam_sessions_window_ck CHECK (window_end > window_start),
  CONSTRAINT exam_sessions_duration_ck CHECK (duration_sec > 0),
  CONSTRAINT exam_sessions_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 12),
  CONSTRAINT exam_sessions_target_ck CHECK (octet_length(target::text) <= 65536),
  CONSTRAINT exam_sessions_sections_ck CHECK (octet_length(sections::text) <= 16384),
  CONSTRAINT exam_sessions_audience_ck CHECK (audience IN ('assigned','library')),
  CONSTRAINT exam_sessions_strategy_ck CHECK (variant_strategy IN ('fixed','independent_draw','per_student')),
  CONSTRAINT exam_sessions_variants_ck CHECK (variant_count BETWEEN 1 AND 20),
  CONSTRAINT exam_sessions_feedback_ck CHECK (feedback_timing IN ('after_submit','after_close','never')),
  CONSTRAINT exam_sessions_release_ck CHECK (result_release IN ('immediate','after_close','manual')),
  CONSTRAINT exam_sessions_feedback_release_ck CHECK (result_release <> 'after_close' OR feedback_timing IN ('after_close','never')),
  CONSTRAINT exam_sessions_late_ck CHECK (late_start_policy IN ('full_duration','until_window_end')),
  CONSTRAINT exam_sessions_accommodations_ck CHECK (jsonb_typeof(accommodations) = 'array' AND octet_length(accommodations::text) <= 16384),
  CONSTRAINT exam_sessions_attempts_ck CHECK (max_attempts BETWEEN 1 AND 10),
  CONSTRAINT exam_sessions_score_policy_ck CHECK (score_policy IN ('best','last','first')),
  CONSTRAINT exam_sessions_status_ck CHECK (status IN ('scheduled','open','closed','results_released','cancelled')),
  CONSTRAINT exam_sessions_role_ck CHECK (created_by_role IN ('school_admin','academic_manager','system_admin'))
);
CREATE INDEX exam_sessions_school_idx ON assessment.exam_sessions (tenant_id, school_id, status, window_start) WHERE deleted_at IS NULL;
CREATE INDEX exam_sessions_assignment_idx ON assessment.exam_sessions (tenant_id, assignment_id) WHERE deleted_at IS NULL;
CREATE INDEX exam_sessions_window_idx ON assessment.exam_sessions (status, window_start, window_end) WHERE deleted_at IS NULL;

-- ---------- assessment.exam_papers (T): immutable variants ----------
CREATE TABLE assessment.exam_papers (
  exam_paper_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  exam_session_id uuid NOT NULL,
  variant_no smallint NOT NULL,
  student_id uuid,
  attempt_no smallint,
  seed bigint NOT NULL,
  items jsonb NOT NULL,
  item_count integer NOT NULL,
  total_points numeric(7,2) NOT NULL,
  expected_p numeric(5,4),
  generator_version varchar(64) NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (exam_paper_id),
  CONSTRAINT exam_papers_variant_ck CHECK (variant_no >= 1),
  CONSTRAINT exam_papers_items_ck CHECK (jsonb_typeof(items) = 'array' AND octet_length(items::text) <= 262144),
  CONSTRAINT exam_papers_count_ck CHECK (item_count BETWEEN 1 AND 200),
  CONSTRAINT exam_papers_student_ck CHECK ((student_id IS NULL) = (attempt_no IS NULL))
);
CREATE UNIQUE INDEX exam_papers_variant_uq ON assessment.exam_papers (exam_session_id, variant_no)
  WHERE student_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX exam_papers_student_uq ON assessment.exam_papers (exam_session_id, student_id, attempt_no)
  WHERE student_id IS NOT NULL AND deleted_at IS NULL;

-- A paper is immutable once generated (EXAM_ENGINE §4): items, seed and variant never change.
CREATE OR REPLACE FUNCTION assessment.exam_papers_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.items IS DISTINCT FROM OLD.items OR NEW.seed IS DISTINCT FROM OLD.seed OR NEW.variant_no IS DISTINCT FROM OLD.variant_no
    OR NEW.exam_session_id IS DISTINCT FROM OLD.exam_session_id THEN
    RAISE EXCEPTION 'exam paper % is immutable', OLD.exam_paper_id USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER exam_papers_immutable BEFORE UPDATE ON assessment.exam_papers
  FOR EACH ROW EXECUTE FUNCTION assessment.exam_papers_immutable();

-- ---------- assessment.exam_participations (T): the audience snapshot and per-student status ----------
CREATE TABLE assessment.exam_participations (
  exam_session_id uuid NOT NULL,
  student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'not_started',
  attempts_used smallint NOT NULL DEFAULT 0,
  last_attempt_id uuid,
  effective_duration_sec integer NOT NULL,
  extra_time_percent smallint NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (exam_session_id, student_id),
  CONSTRAINT exam_participations_status_ck CHECK (status IN ('not_started','in_progress','submitted','graded','absent')),
  CONSTRAINT exam_participations_attempts_ck CHECK (attempts_used >= 0),
  CONSTRAINT exam_participations_duration_ck CHECK (effective_duration_sec > 0),
  CONSTRAINT exam_participations_extra_ck CHECK (extra_time_percent BETWEEN 0 AND 100)
);
CREATE INDEX exam_participations_student_idx ON assessment.exam_participations (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- assessment.attempts: exam columns (DATABASE_DESIGN §3.4) ----------
-- BEGIN entddl

ALTER TABLE assessment.attempts ADD COLUMN exam_paper_id uuid;
ALTER TABLE assessment.attempts ADD COLUMN active_device_token varchar(128);
ALTER TABLE assessment.attempts ADD COLUMN client_platform varchar(64);
ALTER TABLE assessment.attempts ADD COLUMN game_session_id uuid;
-- END entddl

ALTER TABLE assessment.attempts DROP CONSTRAINT attempts_source_kind_ck;
ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_source_kind_ck CHECK (source_kind IN ('activity','question_set','selection','exam_paper'));
ALTER TABLE assessment.attempts DROP CONSTRAINT attempts_submit_reason_ck;
ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_submit_reason_ck CHECK (submit_reason IS NULL OR submit_reason IN ('manual','timeout','auto_complete',
    'auto_timeout','window_closed'));
CREATE INDEX attempts_exam_paper_idx ON assessment.attempts (exam_paper_id) WHERE exam_paper_id IS NOT NULL AND deleted_at IS NULL;

-- ---------- RLS of the T tables by tenant ----------
ALTER TABLE assessment.exam_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.exam_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.exam_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.exam_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.exam_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.exam_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.exam_sessions WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.exam_sessions_live TO learning_readonly;
REVOKE ALL ON assessment.exam_sessions FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON assessment.exam_sessions FROM learning_app, learning_platform;

ALTER TABLE assessment.exam_papers ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.exam_papers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.exam_papers FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.exam_papers FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.exam_papers FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.exam_papers_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.exam_papers WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.exam_papers_live TO learning_readonly;
REVOKE ALL ON assessment.exam_papers FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON assessment.exam_papers FROM learning_app, learning_platform;

ALTER TABLE assessment.exam_participations ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.exam_participations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.exam_participations FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.exam_participations FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.exam_participations FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.exam_participations_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.exam_participations WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.exam_participations_live TO learning_readonly;
REVOKE ALL ON assessment.exam_participations FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON assessment.exam_participations FROM learning_app, learning_platform;
