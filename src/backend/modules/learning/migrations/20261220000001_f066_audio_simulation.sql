-- F066 — audio lessons and simulations (specs/066-audio-simulation-lessons: data-model §1; DATABASE_DESIGN §2.6,
-- §2.6a; ADR-0005, ADR-0028, ADR-0041, ADR-0052). Applied by migrate --module=learning as learning_migrator.
-- Hand-written (explicit-SQL repositories like F065); expand only (AGENTS R12).

-- ---------- assessment.video_progress: listening plays of an audio lesson (research R1; always 0 for a video) ----------
ALTER TABLE assessment.video_progress ADD COLUMN plays_used smallint NOT NULL DEFAULT 0;
ALTER TABLE assessment.video_progress ADD CONSTRAINT video_progress_plays_ck CHECK (plays_used BETWEEN 0 AND 20);
-- The _live view lists its columns at creation: replace it so it carries plays_used (column appended at the end).
CREATE OR REPLACE VIEW assessment.video_progress_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.video_progress WHERE deleted_at IS NULL;

-- ---------- assessment.simulation_runs (T): one verified, rejected or not verifiable run of a simulation attempt ----------
-- Immutable once written (a soft-delete is the only update); the action log is kept for the checker replay and audit,
-- never logged (R14).
CREATE TABLE assessment.simulation_runs (
  run_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  attempt_id uuid NOT NULL,
  student_id uuid NOT NULL,
  run_no smallint NOT NULL,
  template_code varchar(64) NOT NULL,
  template_version integer NOT NULL,
  seed bigint NOT NULL,
  action_log jsonb NOT NULL,
  client_outcome jsonb,
  verified_outcome jsonb,
  goals_met jsonb NOT NULL DEFAULT '[]',
  verification_status varchar(16) NOT NULL,
  reject_code varchar(64),
  idempotency_key varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (run_id),
  CONSTRAINT simulation_runs_status_ck CHECK (verification_status IN ('verified', 'rejected', 'not_verifiable')),
  CONSTRAINT simulation_runs_run_no_ck CHECK (run_no BETWEEN 1 AND 50),
  CONSTRAINT simulation_runs_template_ck CHECK (template_version >= 1 AND seed >= 0),
  CONSTRAINT simulation_runs_log_ck CHECK (jsonb_typeof(action_log) = 'array' AND octet_length(action_log::text) <= 65536),
  CONSTRAINT simulation_runs_outcome_ck CHECK (octet_length(coalesce(client_outcome, '{}')::text) <= 16384
    AND octet_length(coalesce(verified_outcome, '{}')::text) <= 16384 AND jsonb_typeof(goals_met) = 'array')
);
CREATE UNIQUE INDEX simulation_runs_key_uq ON assessment.simulation_runs (attempt_id, idempotency_key) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX simulation_runs_no_uq ON assessment.simulation_runs (attempt_id, run_no) WHERE deleted_at IS NULL;
CREATE INDEX simulation_runs_attempt_idx ON assessment.simulation_runs (tenant_id, attempt_id, run_no DESC) WHERE deleted_at IS NULL;
ALTER TABLE assessment.simulation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.simulation_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.simulation_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.simulation_runs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.simulation_runs FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.simulation_runs_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.simulation_runs WHERE deleted_at IS NULL;

GRANT SELECT ON assessment.simulation_runs_live TO learning_readonly;
REVOKE ALL ON assessment.simulation_runs FROM learning_readonly;
