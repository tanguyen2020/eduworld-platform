-- F031 US5 — mastery recompute in shadow and model activation (specs/031-learning-events-mastery: data-model §1 Recompute;
-- LEARNING_ANALYTICS §3.4; ADR-0017). Applied by migrate --module=learning as learning_migrator. Hand-written tables
-- (explicit-SQL repository, F031 Clarifications [Proposed]); expand only (AGENTS R12).

-- ---------- learning.mastery_recompute_runs (G): one shadow recompute of a model version ----------
CREATE TABLE learning.mastery_recompute_runs (
  run_id uuid NOT NULL,
  model_version varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'running',
  report jsonb,
  started_by uuid,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  activated_at timestamptz,
  activated_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (run_id),
  CONSTRAINT mastery_recompute_runs_status_ck CHECK (status IN ('running','completed','failed','activated')),
  CONSTRAINT mastery_recompute_runs_report_ck CHECK (report IS NULL OR (jsonb_typeof(report) = 'object'
    AND octet_length(report::text) <= 65536))
);
CREATE INDEX mastery_recompute_runs_version_idx ON learning.mastery_recompute_runs (model_version, started_at DESC)
  WHERE deleted_at IS NULL;
-- Operations only (CLI on the platform role): the application roles read nothing here.
REVOKE ALL ON learning.mastery_recompute_runs FROM learning_app, learning_readonly;

-- ---------- learning.objective_mastery_shadows (T): the mastery of a run, never read by the product ----------
CREATE TABLE learning.objective_mastery_shadows (
  run_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  value numeric(4,3) NOT NULL,
  level varchar(64) NOT NULL,
  current_level varchar(64) NOT NULL,
  evidence_count integer NOT NULL DEFAULT 0,
  distinct_days integer NOT NULL DEFAULT 0,
  last_evidence_at timestamptz,
  first_mastered_at timestamptz,
  model_version varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (run_id, tenant_id, student_id, objective_id),
  CONSTRAINT objective_mastery_shadows_value_ck CHECK (value >= 0 AND value <= 1),
  CONSTRAINT objective_mastery_shadows_level_ck CHECK (level IN ('not_started','beginning','developing','proficient','mastered')
    AND current_level IN ('not_started','beginning','developing','proficient','mastered'))
);
ALTER TABLE learning.objective_mastery_shadows ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.objective_mastery_shadows FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.objective_mastery_shadows FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.objective_mastery_shadows FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.objective_mastery_shadows FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE ALL ON learning.objective_mastery_shadows FROM learning_readonly;
-- Activation retires the previous active version and activates the new one (platform role only).
GRANT SELECT, INSERT, UPDATE ON learning.mastery_model_configs TO learning_platform;
