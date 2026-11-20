-- F035 T010 — learning efficacy study and A/B experiments (specs/035-learning-efficacy-study: data-model §1;
-- LEARNING_EFFICACY_PLAN §3–§8, ADR-0080). Hand-written tables read and written by explicit SQL (no ent entity, like
-- F041–F044). Expand only (AGENTS R12); soft-delete only (R21). T = tenant table (tenant_id + RLS), G = platform table.

-- ---------- columns of existing tables ----------
-- An exam session of an efficacy wave (F042 sessions created by F035 only): no exam.* event, no mastery evidence.
ALTER TABLE assessment.exam_sessions ADD COLUMN efficacy_wave_id uuid;
CREATE INDEX exam_sessions_efficacy_wave_idx ON assessment.exam_sessions (tenant_id, efficacy_wave_id)
  WHERE efficacy_wave_id IS NOT NULL AND deleted_at IS NULL;
-- learning-event.schema.json $defs/context: efficacy_wave_id and experiment_arms ("<code>:<arm>", ≤ 64 + 1 + 32).
ALTER TABLE learning.learning_events ADD COLUMN efficacy_wave_id uuid;
ALTER TABLE learning.learning_events ADD COLUMN experiment_arms varchar(97)[];
ALTER TABLE learning.learning_events ADD CONSTRAINT learning_events_arms_ck
  CHECK (experiment_arms IS NULL OR cardinality(experiment_arms) <= 10);

-- The published platform efficacy instruments are read by the tenant sessions of an efficacy exam (delivery and grading
-- of the fixed paper, under the audited bypass efficacy_study): they are never assigned to a tenant (F035-AC-04), so the
-- assignment policies of F022 do not show them. No other read path selects them: the pools and the CMS filter on
-- efficacy_instrument and on explicit assignments (application layer).
CREATE POLICY efficacy_instrument_read ON content.question_versions FOR SELECT USING (
  tenant_id IS NULL AND efficacy_instrument AND status IN ('published','archived'));
CREATE POLICY efficacy_instrument_read ON content.questions FOR SELECT USING (
  tenant_id IS NULL AND EXISTS (SELECT 1 FROM content.question_versions v WHERE v.question_id = questions.question_id
    AND v.tenant_id IS NULL AND v.efficacy_instrument AND v.status IN ('published','archived')));
CREATE POLICY efficacy_instrument_read ON content.question_version_objectives FOR SELECT USING (
  tenant_id IS NULL AND EXISTS (SELECT 1 FROM content.question_versions v WHERE v.question_version_id = question_version_objectives.question_version_id
    AND v.tenant_id IS NULL AND v.efficacy_instrument AND v.status IN ('published','archived')));

-- ---------- learning.research_agreements (T) ----------
CREATE TABLE learning.research_agreements (
  agreement_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  agreement_ref varchar(128) NOT NULL,
  covers varchar(64)[] NOT NULL,
  agreed_on date NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  revoked_at timestamptz,
  revoked_by uuid,
  revoke_reason varchar(1000),
  recorded_by uuid NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (agreement_id),
  CONSTRAINT research_agreements_covers_ck CHECK (cardinality(covers) BETWEEN 1 AND 2
    AND covers <@ ARRAY['efficacy_study','experiment']::varchar(64)[]),
  CONSTRAINT research_agreements_status_ck CHECK (status IN ('active','revoked'))
);
CREATE UNIQUE INDEX research_agreements_ref_uq ON learning.research_agreements (tenant_id, agreement_ref) WHERE deleted_at IS NULL;
CREATE INDEX research_agreements_status_idx ON learning.research_agreements (tenant_id, status) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_studies (G) ----------
CREATE TABLE learning.efficacy_studies (
  study_id uuid NOT NULL,
  code varchar(64) NOT NULL,
  title varchar(255) NOT NULL,
  subject_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  objective_ids uuid[] NOT NULL,
  form_a_blueprint_version_id uuid NOT NULL,
  form_b_blueprint_version_id uuid NOT NULL,
  min_intervention_weeks smallint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  created_by uuid NOT NULL,
  activated_at timestamptz,
  completed_at timestamptz,
  idempotency_key varchar(64) NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_id),
  CONSTRAINT efficacy_studies_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  CONSTRAINT efficacy_studies_objectives_ck CHECK (cardinality(objective_ids) BETWEEN 1 AND 100),
  CONSTRAINT efficacy_studies_forms_ck CHECK (form_a_blueprint_version_id <> form_b_blueprint_version_id),
  CONSTRAINT efficacy_studies_weeks_ck CHECK (min_intervention_weeks BETWEEN 1 AND 52),
  CONSTRAINT efficacy_studies_status_ck CHECK (status IN ('draft','active','completed','cancelled'))
);
CREATE UNIQUE INDEX efficacy_studies_code_uq ON learning.efficacy_studies (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX efficacy_studies_idem_uq ON learning.efficacy_studies (created_by, idempotency_key) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_study_classes (T) ----------
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

-- ---------- learning.efficacy_participants (T) ----------
CREATE TABLE learning.efficacy_participants (
  study_id uuid NOT NULL,
  student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid NOT NULL,
  arm varchar(64) NOT NULL,
  form_order varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  exclusion_reason varchar(64),
  enrolled_at timestamptz NOT NULL,
  withdrawn_at timestamptz,
  withdrawal_id uuid,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_id, student_id),
  CONSTRAINT efficacy_participants_arm_ck CHECK (arm IN ('intervention','comparison','validation')),
  CONSTRAINT efficacy_participants_order_ck CHECK (form_order IN ('a_then_b','b_then_a')),
  CONSTRAINT efficacy_participants_status_ck CHECK (status IN ('active','withdrawn','excluded')),
  CONSTRAINT efficacy_participants_reason_ck CHECK (exclusion_reason IS NULL
    OR exclusion_reason IN ('withdrawn','anonymized','processing_restricted'))
);
CREATE INDEX efficacy_participants_class_idx ON learning.efficacy_participants (tenant_id, study_id, class_id) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_participants_student_idx ON learning.efficacy_participants (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_waves (G) ----------
CREATE TABLE learning.efficacy_waves (
  wave_id uuid NOT NULL,
  study_id uuid NOT NULL,
  kind varchar(64) NOT NULL,
  form varchar(16),
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  duration_sec integer NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'scheduled',
  opened_at timestamptz,
  closed_at timestamptz,
  created_by uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (wave_id),
  CONSTRAINT efficacy_waves_kind_ck CHECK (kind IN ('validation','pre','post')),
  CONSTRAINT efficacy_waves_form_ck CHECK ((kind = 'validation') = (form IS NOT NULL) AND (form IS NULL OR form IN ('a','b'))),
  CONSTRAINT efficacy_waves_window_ck CHECK (window_end > window_start),
  CONSTRAINT efficacy_waves_duration_ck CHECK (duration_sec > 0),
  CONSTRAINT efficacy_waves_status_ck CHECK (status IN ('scheduled','open','closed','cancelled'))
);
CREATE UNIQUE INDEX efficacy_waves_kind_uq ON learning.efficacy_waves (study_id, kind)
  WHERE kind IN ('pre','post') AND status <> 'cancelled' AND deleted_at IS NULL;
CREATE UNIQUE INDEX efficacy_waves_idem_uq ON learning.efficacy_waves (created_by, idempotency_key) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_waves_due_idx ON learning.efficacy_waves (status, window_start, window_end) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_wave_sessions (T) ----------
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

-- ---------- learning.efficacy_withdrawals (T, append-only) ----------
CREATE TABLE learning.efficacy_withdrawals (
  withdrawal_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  study_id uuid,
  scope varchar(64) NOT NULL,
  school_id uuid,
  class_id uuid,
  student_ids uuid[] NOT NULL DEFAULT '{}',
  reason varchar(1000) NOT NULL,
  school_request_ref varchar(128),
  affected_count integer NOT NULL DEFAULT 0,
  cancelled_session_count integer NOT NULL DEFAULT 0,
  requested_by uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (withdrawal_id),
  CONSTRAINT efficacy_withdrawals_scope_ck CHECK (scope IN ('tenant','school','class','students')),
  CONSTRAINT efficacy_withdrawals_students_ck CHECK (cardinality(student_ids) <= 500),
  CONSTRAINT efficacy_withdrawals_reason_ck CHECK (char_length(reason) >= 10),
  CONSTRAINT efficacy_withdrawals_counts_ck CHECK (affected_count >= 0 AND cancelled_session_count >= 0)
);
CREATE UNIQUE INDEX efficacy_withdrawals_idem_uq ON learning.efficacy_withdrawals (requested_by, idempotency_key);
CREATE INDEX efficacy_withdrawals_study_idx ON learning.efficacy_withdrawals (tenant_id, study_id);

-- ---------- learning.efficacy_mastery_snapshots (T) ----------
CREATE TABLE learning.efficacy_mastery_snapshots (
  study_id uuid NOT NULL,
  student_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  m_eff numeric(4,3) NOT NULL,
  level varchar(64) NOT NULL,
  model_version varchar(64) NOT NULL,
  evidence_count integer NOT NULL DEFAULT 0,
  taken_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (study_id, student_id, objective_id),
  CONSTRAINT efficacy_mastery_snapshots_value_ck CHECK (m_eff >= 0 AND m_eff <= 1),
  CONSTRAINT efficacy_mastery_snapshots_level_ck CHECK (level IN ('not_started','beginning','developing','proficient','mastered')),
  CONSTRAINT efficacy_mastery_snapshots_count_ck CHECK (evidence_count >= 0)
);
CREATE INDEX efficacy_mastery_snapshots_student_idx ON learning.efficacy_mastery_snapshots (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- learning.efficacy_reports (G) ----------
CREATE TABLE learning.efficacy_reports (
  report_id uuid NOT NULL,
  study_id uuid NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'queued',
  params jsonb NOT NULL DEFAULT '{}',
  results jsonb,
  inputs_hash bytea,
  descriptive_only boolean NOT NULL DEFAULT false,
  error varchar(1000),
  requested_by uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  run_count smallint NOT NULL DEFAULT 0,
  started_at timestamptz,
  finished_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (report_id),
  CONSTRAINT efficacy_reports_status_ck CHECK (status IN ('queued','running','succeeded','failed')),
  CONSTRAINT efficacy_reports_params_ck CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 8192),
  CONSTRAINT efficacy_reports_results_ck CHECK (results IS NULL OR (jsonb_typeof(results) = 'object' AND octet_length(results::text) <= 262144)),
  CONSTRAINT efficacy_reports_hash_ck CHECK (inputs_hash IS NULL OR octet_length(inputs_hash) = 32),
  CONSTRAINT efficacy_reports_runs_ck CHECK (run_count >= 0)
);
CREATE UNIQUE INDEX efficacy_reports_idem_uq ON learning.efficacy_reports (requested_by, idempotency_key) WHERE deleted_at IS NULL;
CREATE INDEX efficacy_reports_pending_idx ON learning.efficacy_reports (status, created_at)
  WHERE deleted_at IS NULL AND status IN ('queued','running');

-- ---------- learning.experiments (G) ----------
CREATE TABLE learning.experiments (
  experiment_id uuid NOT NULL,
  code varchar(64) NOT NULL,
  kind varchar(64) NOT NULL,
  unit varchar(64) NOT NULL DEFAULT 'class',
  arms jsonb NOT NULL,
  tenant_ids uuid[] NOT NULL,
  study_id uuid,
  status varchar(64) NOT NULL DEFAULT 'draft',
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  stop_reason varchar(64),
  stop_note varchar(1000),
  revision integer NOT NULL DEFAULT 1,
  owner_id uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  started_at timestamptz,
  stopped_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (experiment_id),
  CONSTRAINT experiments_code_ck CHECK (code ~ '^[a-z0-9_-]{1,64}$'),
  CONSTRAINT experiments_kind_ck CHECK (kind IN ('mastery_model','recommendation_rule')),
  CONSTRAINT experiments_unit_ck CHECK (unit IN ('class','student')),
  CONSTRAINT experiments_arms_ck CHECK (jsonb_typeof(arms) = 'array' AND octet_length(arms::text) <= 16384),
  CONSTRAINT experiments_tenants_ck CHECK (cardinality(tenant_ids) BETWEEN 1 AND 200),
  CONSTRAINT experiments_status_ck CHECK (status IN ('draft','shadow','live','stopped','completed')),
  CONSTRAINT experiments_window_ck CHECK (end_at > start_at),
  CONSTRAINT experiments_stop_ck CHECK (stop_reason IS NULL OR stop_reason IN ('manual','guardrail_breach','agreement_revoked','ended')),
  CONSTRAINT experiments_revision_ck CHECK (revision >= 1)
);
CREATE UNIQUE INDEX experiments_code_uq ON learning.experiments (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX experiments_idem_uq ON learning.experiments (owner_id, idempotency_key) WHERE deleted_at IS NULL;
CREATE INDEX experiments_running_idx ON learning.experiments (status) WHERE deleted_at IS NULL AND status IN ('shadow','live');

-- ---------- learning.experiment_assignments (T) ----------
CREATE TABLE learning.experiment_assignments (
  experiment_id uuid NOT NULL,
  student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  unit_id uuid NOT NULL,
  bucket smallint NOT NULL,
  arm_code varchar(32) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  assigned_at timestamptz NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (experiment_id, student_id),
  CONSTRAINT experiment_assignments_bucket_ck CHECK (bucket BETWEEN 0 AND 99),
  CONSTRAINT experiment_assignments_status_ck CHECK (status IN ('active','removed'))
);
CREATE INDEX experiment_assignments_student_idx ON learning.experiment_assignments (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- learning.experiment_mastery_states (T) ----------
CREATE TABLE learning.experiment_mastery_states (
  experiment_id uuid NOT NULL,
  arm_code varchar(32) NOT NULL,
  tenant_id uuid NOT NULL,
  student_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  value numeric(4,3) NOT NULL,
  level varchar(64) NOT NULL,
  evidence_count integer NOT NULL DEFAULT 0,
  distinct_days integer NOT NULL DEFAULT 0,
  last_evidence_at timestamptz,
  last_evidence_date date,
  model_version varchar(64) NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (experiment_id, arm_code, tenant_id, student_id, objective_id),
  CONSTRAINT experiment_mastery_states_value_ck CHECK (value >= 0 AND value <= 1),
  CONSTRAINT experiment_mastery_states_level_ck CHECK (level IN ('not_started','beginning','developing','proficient','mastered')),
  CONSTRAINT experiment_mastery_states_counts_ck CHECK (evidence_count >= 0 AND distinct_days >= 0)
);
CREATE INDEX experiment_mastery_states_student_idx ON learning.experiment_mastery_states (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- learning.experiment_results (G, append-only) ----------
CREATE TABLE learning.experiment_results (
  result_id uuid NOT NULL,
  experiment_id uuid NOT NULL,
  computed_at timestamptz NOT NULL,
  results jsonb NOT NULL,
  guardrail_breached boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (result_id),
  CONSTRAINT experiment_results_results_ck CHECK (jsonb_typeof(results) = 'object' AND octet_length(results::text) <= 65536)
);
CREATE INDEX experiment_results_experiment_idx ON learning.experiment_results (experiment_id, computed_at DESC);

-- ---------- RLS on the T tables ----------
ALTER TABLE learning.research_agreements ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.research_agreements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.research_agreements FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.research_agreements FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.research_agreements FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.efficacy_study_classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_study_classes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_study_classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_study_classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_study_classes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.efficacy_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_participants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_participants FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_participants FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_participants FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.efficacy_wave_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_wave_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_wave_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_wave_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_wave_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.efficacy_withdrawals ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_withdrawals FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_withdrawals FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_withdrawals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.efficacy_mastery_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.efficacy_mastery_snapshots FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.efficacy_mastery_snapshots FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.efficacy_mastery_snapshots FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.efficacy_mastery_snapshots FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.experiment_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.experiment_assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.experiment_assignments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.experiment_assignments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.experiment_assignments FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE learning.experiment_mastery_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.experiment_mastery_states FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.experiment_mastery_states FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.experiment_mastery_states FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.experiment_mastery_states FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- Second layer for deleted rows of the business tables (ADR-0089 §1).
CREATE OR REPLACE VIEW learning.research_agreements_live WITH (security_barrier = true) AS
  SELECT * FROM learning.research_agreements WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW learning.efficacy_studies_live WITH (security_barrier = true) AS
  SELECT * FROM learning.efficacy_studies WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW learning.efficacy_study_classes_live WITH (security_barrier = true) AS
  SELECT * FROM learning.efficacy_study_classes WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW learning.experiments_live WITH (security_barrier = true) AS
  SELECT * FROM learning.experiments WHERE deleted_at IS NULL;
GRANT SELECT ON learning.research_agreements_live, learning.efficacy_studies_live, learning.efficacy_study_classes_live,
  learning.experiments_live TO learning_readonly;
REVOKE ALL ON learning.research_agreements, learning.efficacy_studies, learning.efficacy_study_classes, learning.efficacy_participants,
  learning.efficacy_waves, learning.efficacy_wave_sessions, learning.efficacy_withdrawals, learning.efficacy_mastery_snapshots,
  learning.efficacy_reports, learning.experiments, learning.experiment_assignments, learning.experiment_mastery_states,
  learning.experiment_results FROM learning_readonly;

-- ---------- privileges: no DELETE/TRUNCATE (R21); the append-only tables get no UPDATE ----------
REVOKE DELETE, TRUNCATE ON learning.research_agreements, learning.efficacy_studies, learning.efficacy_study_classes,
  learning.efficacy_participants, learning.efficacy_waves, learning.efficacy_wave_sessions, learning.efficacy_withdrawals,
  learning.efficacy_mastery_snapshots, learning.efficacy_reports, learning.experiments, learning.experiment_assignments,
  learning.experiment_mastery_states, learning.experiment_results FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.research_agreements, learning.efficacy_studies, learning.efficacy_study_classes,
  learning.efficacy_participants, learning.efficacy_waves, learning.efficacy_wave_sessions, learning.efficacy_mastery_snapshots,
  learning.efficacy_reports, learning.experiments, learning.experiment_assignments, learning.experiment_mastery_states
  TO learning_app, learning_platform;
GRANT SELECT, INSERT ON learning.efficacy_withdrawals, learning.experiment_results TO learning_app, learning_platform;
