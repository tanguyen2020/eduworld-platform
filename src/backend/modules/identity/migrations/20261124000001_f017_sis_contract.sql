-- F017 T008 — SIS contract_v1 columns of module identity (DATABASE_DESIGN §2.3, data-model F017). Expand only
-- (AGENTS R12): new nullable columns or NOT NULL with a default, no data removed. The entddl block is rendered
-- from the ent schema (golden test modules/identity/ent/ddl_test.go, ALTER form). Hand-written: CHECKs, the
-- copy of F011's config.capabilities into the new column, and the re-created <table>_live views (SELECT * is
-- expanded when a view is created, so new columns need CREATE OR REPLACE). Applied as identity_migrator.

-- BEGIN entddl
ALTER TABLE sis.sis_connections ADD COLUMN webhook_secret_ref varchar(512);
ALTER TABLE sis.sis_connections ADD COLUMN capabilities jsonb;
ALTER TABLE sis.sis_connections ADD COLUMN last_health_at timestamptz;
ALTER TABLE sis.sis_connections ADD COLUMN last_success_at timestamptz;
ALTER TABLE sis.sis_connections ADD COLUMN consecutive_failed_runs integer NOT NULL DEFAULT 0;
ALTER TABLE sis.sis_connections ADD COLUMN status_reason varchar(64);
CREATE INDEX sis_connections_schedule_idx ON sis.sis_connections (status, provider_kind) WHERE deleted_at IS NULL;
ALTER TABLE sis.sync_runs ADD COLUMN requested_by uuid;
ALTER TABLE sis.sync_runs ADD COLUMN delivery_id uuid;
-- END entddl

-- Capabilities stored by F011 inside config (login sync read config.capabilities) move to their own column.
UPDATE sis.sis_connections SET capabilities = config -> 'capabilities'
  WHERE config ? 'capabilities' AND jsonb_typeof(config -> 'capabilities') = 'array';

-- ---------- CHECK constraints ----------
ALTER TABLE sis.sis_connections
  ADD CONSTRAINT sis_connections_capabilities_chk CHECK (capabilities IS NULL OR (jsonb_typeof(capabilities) = 'array'
    AND jsonb_array_length(capabilities) <= 4 AND capabilities <@ '["full_sync","delta","webhook","student_lookup"]'::jsonb)),
  ADD CONSTRAINT sis_connections_failed_runs_chk CHECK (consecutive_failed_runs >= 0),
  ADD CONSTRAINT sis_connections_status_reason_chk CHECK (status_reason IS NULL OR status_reason IN
    ('version_mismatch','violation_threshold','credential_rejected','secret_unavailable','circuit_open','manual')),
  ADD CONSTRAINT sis_connections_contract_chk CHECK (provider_kind <> 'contract_v1' OR (config IS NOT NULL AND secret_ref IS NOT NULL)),
  ADD CONSTRAINT sis_connections_webhook_chk CHECK (provider_kind = 'contract_v1' OR webhook_secret_ref IS NULL);
ALTER TABLE sis.sync_runs
  ADD CONSTRAINT sync_runs_delivery_chk CHECK (delivery_id IS NULL OR mode = 'webhook');

-- ---------- <table>_live views for identity_readonly (ADR-0089 §1): pick up the new columns ----------
CREATE OR REPLACE VIEW sis.sis_connections_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sis_connections WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sis_connections_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.sync_runs_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sync_runs WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sync_runs_live TO identity_readonly;
