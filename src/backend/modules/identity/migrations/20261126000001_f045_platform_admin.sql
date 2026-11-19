-- F045 T007 — Platform Administration of module identity (DATABASE_DESIGN §2.0, §2.16, ADR-0028, ADR-0029,
-- ADR-0067): clone jobs between tenants, default assignments for new tenants and the merges of two accounts of
-- one person. The saga runs live in platform.saga_runs/saga_steps (F016). Hand-written tables accessed by
-- explicit SQL; soft-delete only, no row is ever removed. Applied as identity_migrator.

-- ---------- Clone jobs (G): one source tenant, one target tenant, one resource type (ADR-0029) ----------
CREATE TABLE platform.tenant_clone_jobs (
  job_id            uuid PRIMARY KEY,
  source_tenant_id  uuid NOT NULL REFERENCES platform.tenants (tenant_id),
  target_tenant_id  uuid NOT NULL REFERENCES platform.tenants (tenant_id),
  resource_type     varchar(64) NOT NULL CHECK (resource_type ~ '^[a-z][a-z_]{1,63}$'),
  resource_ids      uuid[] NOT NULL CHECK (cardinality(resource_ids) BETWEEN 1 AND 500),
  status            varchar(32) NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued', 'running', 'succeeded', 'partially_failed', 'failed')),
  id_map            jsonb NOT NULL DEFAULT '[]'::jsonb
                    CHECK (jsonb_typeof(id_map) = 'array' AND octet_length(id_map::text) <= 1048576),
  stats             jsonb NOT NULL DEFAULT '{}'::jsonb
                    CHECK (jsonb_typeof(stats) = 'object' AND octet_length(stats::text) <= 16384),
  errors            jsonb NOT NULL DEFAULT '[]'::jsonb
                    CHECK (jsonb_typeof(errors) = 'array' AND octet_length(errors::text) <= 65536),
  saga_id           uuid NOT NULL REFERENCES platform.saga_runs (saga_id),
  requested_by      uuid NOT NULL,
  idempotency_key   varchar(128) NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  finished_at       timestamptz,
  version           bigint NOT NULL DEFAULT 1,
  CHECK (source_tenant_id <> target_tenant_id),
  CHECK ((status IN ('succeeded', 'partially_failed', 'failed')) = (finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX tenant_clone_jobs_idem_uq ON platform.tenant_clone_jobs (requested_by, idempotency_key);
CREATE UNIQUE INDEX tenant_clone_jobs_saga_uq ON platform.tenant_clone_jobs (saga_id);
CREATE INDEX tenant_clone_jobs_open_idx ON platform.tenant_clone_jobs (created_at) WHERE status IN ('queued', 'running');
REVOKE ALL ON platform.tenant_clone_jobs FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.tenant_clone_jobs TO identity_platform; -- System Admin / jobs only

-- ---------- Default assignments (G): platform resources assigned to every new tenant (tenant.created) ----------
CREATE TABLE platform.tenant_default_assignments (
  default_assignment_id  uuid PRIMARY KEY,
  resource_type          varchar(64) NOT NULL CHECK (resource_type ~ '^[a-z][a-z_]{1,63}$'),
  resource_id            uuid NOT NULL,
  owner_module           varchar(64) NOT NULL CHECK (owner_module IN ('learning', 'game', 'competition', 'chat', 'identity')),
  created_by             uuid NOT NULL,
  created_at             timestamptz NOT NULL DEFAULT now(),
  deleted_at             timestamptz,
  deleted_by             uuid,
  delete_reason          varchar(64)
);
CREATE UNIQUE INDEX tenant_default_assignments_live_uq ON platform.tenant_default_assignments (resource_type, resource_id)
  WHERE deleted_at IS NULL;
REVOKE ALL ON platform.tenant_default_assignments FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.tenant_default_assignments TO identity_platform;
CREATE OR REPLACE VIEW platform.tenant_default_assignments_live WITH (security_barrier = true) AS
  SELECT * FROM platform.tenant_default_assignments WHERE deleted_at IS NULL;
GRANT SELECT ON platform.tenant_default_assignments_live TO identity_readonly;

-- ---------- User merges (G, §2.16, append-only): one source account merged into a target account ----------
CREATE TABLE identity.user_merges (
  merge_id                 uuid PRIMARY KEY,
  tenant_id                uuid NOT NULL REFERENCES platform.tenants (tenant_id),
  from_user_id             uuid NOT NULL REFERENCES identity.users (user_id),
  into_user_id             uuid NOT NULL REFERENCES identity.users (user_id),
  school_id                uuid,
  reason                   varchar(1000) NOT NULL CHECK (length(btrim(reason)) >= 3),
  merged_by                uuid NOT NULL,
  merged_at                timestamptz NOT NULL DEFAULT now(),
  saga_id                  uuid NOT NULL REFERENCES platform.saga_runs (saga_id),
  reconciliation_issue_id  uuid REFERENCES sis.reconciliation_issues (issue_id),
  CHECK (from_user_id <> into_user_id)
);
CREATE UNIQUE INDEX user_merges_from_uq ON identity.user_merges (from_user_id);
CREATE UNIQUE INDEX user_merges_saga_uq ON identity.user_merges (saga_id);
CREATE INDEX user_merges_into_idx ON identity.user_merges (into_user_id);
ALTER TABLE identity.user_merges ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.user_merges FORCE ROW LEVEL SECURITY;
-- identity_app has no privilege; identity_platform (BYPASSRLS) runs the System Admin commands and the saga.
CREATE POLICY tenant_read ON identity.user_merges FOR SELECT USING (tenant_id = infra.current_tenant_id());
REVOKE ALL ON identity.user_merges FROM identity_app, identity_platform;
GRANT SELECT, INSERT ON identity.user_merges TO identity_platform; -- append-only

-- ---------- Reconciliation issues: School Admin proposes a merge, F045 closes the issue as merged ----------
ALTER TABLE sis.reconciliation_issues DROP CONSTRAINT reconciliation_issues_resolution_chk;
ALTER TABLE sis.reconciliation_issues ADD CONSTRAINT reconciliation_issues_resolution_chk CHECK (resolution IS NULL
  OR resolution IN ('link', 'same_person', 'ignore', 'fixed_in_source', 'merge_requested', 'merged'));
