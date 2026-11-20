-- F032 — self-study practice: wrong-question review list, student recommendations, diagnostic priors
-- (specs/032-self-study-practice: data-model §1; DATABASE_DESIGN §2.7; LEARNING_ANALYTICS §3.5, §6; LEARNING_EXPERIENCE §6.1).
-- Applied by migrate --module=learning as learning_migrator. Hand-written tables (explicit-SQL repositories, like the
-- learning record of F031); expand only (AGENTS R12).

-- ---------- learning.objective_masteries: the prior M0 of a diagnostic (LEARNING_ANALYTICS §3.5) ----------
ALTER TABLE learning.objective_masteries
  ADD COLUMN prior_value numeric(4,3),
  ADD COLUMN prior_diagnostic_attempt_id uuid;
ALTER TABLE learning.objective_masteries DROP CONSTRAINT objective_masteries_prior_ck;
ALTER TABLE learning.objective_masteries
  ADD CONSTRAINT objective_masteries_prior_ck CHECK (prior_source IN ('default','diagnostic','propagated')),
  ADD CONSTRAINT objective_masteries_prior_value_ck CHECK (prior_value IS NULL OR (prior_value >= 0 AND prior_value <= 1)),
  ADD CONSTRAINT objective_masteries_prior_set_ck CHECK (prior_source = 'default' OR prior_value IS NOT NULL);
-- The live view lists the columns of the table at creation: recreate it with the new columns.
CREATE OR REPLACE VIEW learning.objective_masteries_live WITH (security_barrier = true) AS
  SELECT * FROM learning.objective_masteries WHERE deleted_at IS NULL;

-- ---------- learning.wrong_question_entries (T): the review list of first-try wrong answers ----------
CREATE TABLE learning.wrong_question_entries (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  question_id uuid NOT NULL,
  question_version_id uuid NOT NULL,
  subject_id uuid,
  objective_ids uuid[] NOT NULL DEFAULT '{}',
  question_type varchar(64) NOT NULL,
  difficulty smallint NOT NULL,
  first_wrong_attempt_id uuid NOT NULL,
  wrong_count smallint NOT NULL DEFAULT 1,
  last_wrong_at timestamptz NOT NULL,
  stage smallint NOT NULL DEFAULT 0,
  next_due_at timestamptz NOT NULL,
  correct_streak smallint NOT NULL DEFAULT 0,
  last_correct_on date,
  status varchar(64) NOT NULL DEFAULT 'active',
  retry_attempt_id uuid,
  retry_position smallint,
  cleared_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (entry_id),
  CONSTRAINT wrong_question_entries_status_ck CHECK (status IN ('active','cleared')),
  CONSTRAINT wrong_question_entries_objectives_ck CHECK (cardinality(objective_ids) <= 20),
  CONSTRAINT wrong_question_entries_difficulty_ck CHECK (difficulty BETWEEN 1 AND 5),
  CONSTRAINT wrong_question_entries_counts_ck CHECK (stage BETWEEN 0 AND 2 AND correct_streak BETWEEN 0 AND 2 AND wrong_count >= 1
    AND (retry_position IS NULL OR retry_position BETWEEN 1 AND 200)),
  CONSTRAINT wrong_question_entries_cleared_ck CHECK ((status = 'cleared') = (cleared_at IS NOT NULL))
);
-- One active entry per student and question (R21: partial unique on live rows).
CREATE UNIQUE INDEX wrong_question_entries_active_uq ON learning.wrong_question_entries (tenant_id, student_id, question_id)
  WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX wrong_question_entries_due_idx ON learning.wrong_question_entries (tenant_id, student_id, status, next_due_at)
  WHERE deleted_at IS NULL;
CREATE INDEX wrong_question_entries_retry_idx ON learning.wrong_question_entries (retry_attempt_id)
  WHERE deleted_at IS NULL AND retry_attempt_id IS NOT NULL;
ALTER TABLE learning.wrong_question_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.wrong_question_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.wrong_question_entries FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.wrong_question_entries FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.wrong_question_entries FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW learning.wrong_question_entries_live WITH (security_barrier = true) AS
  SELECT * FROM learning.wrong_question_entries WHERE deleted_at IS NULL;
GRANT SELECT ON learning.wrong_question_entries_live TO learning_readonly;
REVOKE ALL ON learning.wrong_question_entries FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON learning.wrong_question_entries FROM learning_app, learning_platform;

-- ---------- learning.rm_student_recommendations (T): read model of the recommendation engine (LEARNING_ANALYTICS §5, §6) ----------
CREATE TABLE learning.rm_student_recommendations (
  recommendation_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  rule_code varchar(64) NOT NULL,
  kind varchar(64) NOT NULL,
  target_type varchar(64) NOT NULL,
  target_id uuid NOT NULL,
  subject_id uuid,
  grade_level smallint,
  objective_ids uuid[] NOT NULL DEFAULT '{}',
  reason varchar(64) NOT NULL,
  reason_code varchar(64) NOT NULL,
  reason_params jsonb NOT NULL DEFAULT '{}',
  priority smallint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  skip_count smallint NOT NULL DEFAULT 0,
  started_attempt_id uuid,
  generated_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  closed_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (recommendation_id),
  CONSTRAINT rm_student_recommendations_rule_ck CHECK (rule_code IN ('R1','R2','R3','R4','R5','R6','R7','R8','R9')),
  CONSTRAINT rm_student_recommendations_kind_ck CHECK (kind IN ('assignment','activity','lesson','objective_review',
    'prerequisite_review','retry_wrong','diagnostic','interleave_practice','quest')),
  CONSTRAINT rm_student_recommendations_target_ck CHECK (target_type IN ('activity','lesson','question_set','assignment','selection','quest')),
  CONSTRAINT rm_student_recommendations_status_ck CHECK (status IN ('active','started','dismissed','completed','expired','superseded')),
  CONSTRAINT rm_student_recommendations_priority_ck CHECK (priority BETWEEN 1 AND 9 AND skip_count >= 0
    AND (grade_level IS NULL OR grade_level BETWEEN 1 AND 12)),
  CONSTRAINT rm_student_recommendations_objectives_ck CHECK (cardinality(objective_ids) <= 10),
  CONSTRAINT rm_student_recommendations_params_ck CHECK (jsonb_typeof(reason_params) = 'object' AND octet_length(reason_params::text) <= 2048),
  CONSTRAINT rm_student_recommendations_closed_ck CHECK ((status = 'active') = (closed_at IS NULL))
);
CREATE UNIQUE INDEX rm_student_recommendations_active_uq ON learning.rm_student_recommendations
  (tenant_id, student_id, rule_code, target_type, target_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX rm_student_recommendations_student_idx ON learning.rm_student_recommendations (tenant_id, student_id, status, priority)
  WHERE deleted_at IS NULL;
ALTER TABLE learning.rm_student_recommendations ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.rm_student_recommendations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.rm_student_recommendations FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.rm_student_recommendations FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.rm_student_recommendations FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW learning.rm_student_recommendations_live WITH (security_barrier = true) AS
  SELECT * FROM learning.rm_student_recommendations WHERE deleted_at IS NULL;
GRANT SELECT ON learning.rm_student_recommendations_live TO learning_readonly;
REVOKE ALL ON learning.rm_student_recommendations FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON learning.rm_student_recommendations FROM learning_app, learning_platform;


-- ---------- assessment.attempts: contexts of the ad-hoc attempts of F032 (retry_wrong, diagnostic) ----------
ALTER TABLE assessment.attempts DROP CONSTRAINT attempts_context_type_ck;
ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_context_type_ck CHECK (context_type IN ('free_play','lesson','learning_path','recommendation',
    'question_set','selection','assignment','quest_step','minigame_session','exam_session','match','challenge_attempt','boss_encounter',
    'retry_wrong','diagnostic'));
