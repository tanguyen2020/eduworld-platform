-- F011 T006/T007 — SIS sync tables of module identity (DATABASE_DESIGN §2.3, DDL §3.14, ADR-0021, ADR-0046).
-- The entddl block is rendered from the ent schema (golden test modules/identity/ent/ddl_test.go). Hand-written:
-- partitioned sis.staging_records / sis.sync_record_errors (monthly range, DEFAULT partition), CHECKs, foreign
-- keys, the expression unique index of login keys, RLS of the T tables, <table>_live views. Composite keys of
-- the DDL are a surrogate id plus a unique index (ent needs a single id). Applied as identity_migrator.

-- BEGIN entddl
CREATE TABLE sis.reconciliation_issues (
  issue_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid,
  sis_connection_id uuid,
  sync_run_id uuid,
  issue_type varchar(64) NOT NULL,
  entity_refs jsonb NOT NULL,
  suggested_action varchar(64),
  status varchar(64) NOT NULL DEFAULT 'open',
  resolution varchar(64),
  resolution_note varchar(1000),
  resolved_by uuid,
  resolved_at timestamptz,
  PRIMARY KEY (issue_id)
);
CREATE INDEX reconciliation_issues_open_idx ON sis.reconciliation_issues (tenant_id, status, issue_type) WHERE deleted_at IS NULL;

CREATE TABLE sis.sis_connection_schools (
  connection_school_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  sis_connection_id uuid NOT NULL,
  school_id uuid NOT NULL,
  active boolean NOT NULL DEFAULT true,
  assigned_by uuid NOT NULL,
  assigned_at timestamptz NOT NULL,
  PRIMARY KEY (connection_school_id)
);
CREATE UNIQUE INDEX sis_connection_schools_one_active ON sis.sis_connection_schools (school_id) WHERE active AND deleted_at IS NULL;
CREATE UNIQUE INDEX sis_connection_schools_pair_uq ON sis.sis_connection_schools (sis_connection_id, school_id) WHERE deleted_at IS NULL;

CREATE TABLE sis.sis_connections (
  sis_connection_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  provider_kind varchar(64) NOT NULL,
  contract_version varchar(64),
  config jsonb,
  secret_ref varchar(512),
  status varchar(64) NOT NULL DEFAULT 'active',
  delta_watermark varchar(512),
  schedule varchar(64),
  PRIMARY KEY (sis_connection_id)
);

CREATE TABLE identity.sis_identity_links (
  link_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_id uuid NOT NULL,
  school_id uuid NOT NULL,
  sis_connection_id uuid NOT NULL,
  student_external_id varchar(64) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'active',
  linked_at timestamptz NOT NULL,
  superseded_at timestamptz,
  superseded_by uuid,
  PRIMARY KEY (link_id)
);
CREATE UNIQUE INDEX sis_identity_links_one_active ON identity.sis_identity_links (user_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE UNIQUE INDEX sis_identity_links_source_active ON identity.sis_identity_links (sis_connection_id, student_external_id) WHERE status = 'active' AND deleted_at IS NULL;

CREATE TABLE sis.sis_imports (
  import_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  requested_by uuid NOT NULL,
  storage_key varchar(512),
  file_sha256 varchar(128),
  size_bytes bigint NOT NULL DEFAULT 0,
  entity_kinds jsonb NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'awaiting_upload',
  dry_run boolean NOT NULL DEFAULT true,
  sync_run_id uuid,
  stats jsonb NOT NULL,
  idempotency_key varchar(64),
  PRIMARY KEY (import_id)
);

CREATE TABLE sis.sis_links (
  sis_link_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  entity_type varchar(64) NOT NULL,
  entity_id uuid NOT NULL,
  sis_connection_id uuid NOT NULL,
  external_id varchar(64) NOT NULL,
  school_id uuid,
  last_hash varchar(128) NOT NULL,
  last_synced_at timestamptz NOT NULL,
  PRIMARY KEY (sis_link_id)
);
CREATE UNIQUE INDEX sis_links_source_uq ON sis.sis_links (sis_connection_id, entity_type, external_id) WHERE deleted_at IS NULL;

CREATE TABLE sis.student_login_keys (
  login_key_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  sis_connection_id uuid NOT NULL,
  login_type varchar(16) NOT NULL,
  idp_alias varchar(64),
  login_key varchar(255) NOT NULL,
  student_external_id varchar(64) NOT NULL,
  PRIMARY KEY (login_key_id)
);

CREATE TABLE sis.sync_runs (
  sync_run_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  sis_connection_id uuid NOT NULL,
  school_id uuid,
  mode varchar(64) NOT NULL,
  trigger varchar(64) NOT NULL DEFAULT 'manual',
  dry_run boolean NOT NULL DEFAULT false,
  status varchar(64) NOT NULL DEFAULT 'running',
  failure_code varchar(64),
  stats jsonb NOT NULL,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  PRIMARY KEY (sync_run_id)
);
CREATE INDEX sync_runs_connection_idx ON sis.sync_runs (sis_connection_id, started_at) WHERE deleted_at IS NULL;

CREATE TABLE sis.synced_students (
  synced_student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  sis_connection_id uuid NOT NULL,
  student_external_id varchar(64) NOT NULL,
  school_id uuid NOT NULL,
  student_code varchar(32) NOT NULL,
  record_status varchar(32) NOT NULL,
  national_student_id_hash varchar(64),
  last_hash varchar(64) NOT NULL,
  last_synced_at timestamptz NOT NULL,
  missing_runs smallint NOT NULL DEFAULT 0,
  PRIMARY KEY (synced_student_id)
);
CREATE INDEX synced_students_school_idx ON sis.synced_students (tenant_id, school_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX synced_students_source_uq ON sis.synced_students (sis_connection_id, student_external_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- Partitioned append-only staging (G, P): payloads are anonymized after sis_staging_retention ----------
CREATE TABLE sis.staging_records (
  staging_record_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  sync_run_id uuid NOT NULL,
  entity_kind varchar(64) NOT NULL,
  external_id varchar(64) NOT NULL,
  row_no bigint,
  payload jsonb,
  raw_hash varchar(128) NOT NULL,
  action varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'pending',
  PRIMARY KEY (staging_record_id, created_at),
  CHECK (entity_kind IN ('school','class','student','enrollment')),
  CHECK (action IN ('create','update','noop','deactivate')),
  CHECK (status IN ('pending','applied','skipped','pending_identifier')),
  CHECK (payload IS NULL OR octet_length(payload::text) <= 65536)
) PARTITION BY RANGE (created_at);
CREATE TABLE sis.staging_records_default PARTITION OF sis.staging_records DEFAULT;
CREATE INDEX staging_records_run_idx ON sis.staging_records (sync_run_id, entity_kind);

CREATE TABLE sis.sync_record_errors (
  sync_record_error_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  sync_run_id uuid NOT NULL,
  entity_kind varchar(64) NOT NULL,
  external_id varchar(64),
  row_no bigint,
  column_name varchar(64),
  error_code varchar(64) NOT NULL,
  message varchar(1000) NOT NULL,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (sync_record_error_id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE sis.sync_record_errors_default PARTITION OF sis.sync_record_errors DEFAULT;
CREATE INDEX sync_record_errors_run_idx ON sis.sync_record_errors (sync_run_id, row_no);

-- ---------- CHECK constraints ----------
ALTER TABLE sis.sis_connections
  ADD CONSTRAINT sis_connections_kind_chk CHECK (provider_kind IN ('contract_v1','csv_import')),
  ADD CONSTRAINT sis_connections_status_chk CHECK (status IN ('active','auth_error','contract_error','paused')),
  ADD CONSTRAINT sis_connections_config_size_chk CHECK (config IS NULL OR octet_length(config::text) <= 65536),
  ADD CONSTRAINT sis_connections_csv_chk CHECK (provider_kind <> 'csv_import' OR (config IS NULL AND secret_ref IS NULL));
ALTER TABLE sis.sync_runs
  ADD CONSTRAINT sync_runs_mode_chk CHECK (mode IN ('full','delta','webhook','csv_import')),
  ADD CONSTRAINT sync_runs_trigger_chk CHECK ("trigger" IN ('schedule','manual','webhook','fallback_full')),
  ADD CONSTRAINT sync_runs_status_chk CHECK (status IN ('queued','running','succeeded','partially_failed','failed'));
ALTER TABLE sis.sis_imports
  ADD CONSTRAINT sis_imports_status_chk CHECK (status IN ('awaiting_upload','queued','validating','applying','completed',
    'completed_with_issues','failed','expired')),
  ADD CONSTRAINT sis_imports_size_chk CHECK (size_bytes >= 0);
ALTER TABLE sis.sis_links
  ADD CONSTRAINT sis_links_type_chk CHECK (entity_type IN ('school','class','enrollment'));
ALTER TABLE sis.synced_students
  ADD CONSTRAINT synced_students_status_chk CHECK (record_status IN ('active','inactive','transferred_out','graduated','suspended'));
ALTER TABLE sis.student_login_keys
  ADD CONSTRAINT student_login_keys_type_chk CHECK (login_type IN ('username','idp_subject')),
  ADD CONSTRAINT student_login_keys_alias_chk CHECK ((login_type = 'idp_subject') = (idp_alias IS NOT NULL));
ALTER TABLE sis.reconciliation_issues
  ADD CONSTRAINT reconciliation_issues_status_chk CHECK (status IN ('open','resolved','ignored')),
  ADD CONSTRAINT reconciliation_issues_resolution_chk CHECK (resolution IS NULL OR resolution IN ('link','same_person','ignore','fixed_in_source')),
  ADD CONSTRAINT reconciliation_issues_resolved_chk CHECK ((status = 'open') = (resolved_at IS NULL));
ALTER TABLE identity.sis_identity_links
  ADD CONSTRAINT sis_identity_links_status_chk CHECK (status IN ('active','superseded')),
  ADD CONSTRAINT sis_identity_links_superseded_chk CHECK ((status = 'superseded') = (superseded_at IS NOT NULL));

-- ---------- Foreign keys ----------
ALTER TABLE sis.sis_connections ADD CONSTRAINT sis_connections_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);
ALTER TABLE sis.sis_connection_schools
  ADD CONSTRAINT sis_connection_schools_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id),
  ADD CONSTRAINT sis_connection_schools_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id);
ALTER TABLE sis.sync_runs ADD CONSTRAINT sync_runs_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id);
ALTER TABLE sis.sis_imports
  ADD CONSTRAINT sis_imports_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT sis_imports_run_fk FOREIGN KEY (sync_run_id) REFERENCES sis.sync_runs (sync_run_id);
ALTER TABLE sis.sis_links ADD CONSTRAINT sis_links_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id);
ALTER TABLE sis.synced_students
  ADD CONSTRAINT synced_students_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id),
  ADD CONSTRAINT synced_students_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id);
ALTER TABLE sis.student_login_keys ADD CONSTRAINT student_login_keys_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id);
ALTER TABLE sis.reconciliation_issues ADD CONSTRAINT reconciliation_issues_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id);
ALTER TABLE identity.sis_identity_links
  ADD CONSTRAINT sis_identity_links_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id),
  ADD CONSTRAINT sis_identity_links_connection_fk FOREIGN KEY (sis_connection_id) REFERENCES sis.sis_connections (sis_connection_id),
  ADD CONSTRAINT sis_identity_links_superseded_by_fk FOREIGN KEY (superseded_by) REFERENCES identity.sis_identity_links (link_id);

-- ---------- Expression unique index (ADR-0046): one source record per login key and connection ----------
CREATE UNIQUE INDEX student_login_keys_uq ON sis.student_login_keys (sis_connection_id, login_type, coalesce(idp_alias, ''), login_key)
  WHERE deleted_at IS NULL;
CREATE INDEX student_login_keys_student_idx ON sis.student_login_keys (sis_connection_id, student_external_id) WHERE deleted_at IS NULL;

-- ---------- Row-Level Security: T tables, tenant condition only (DATABASE_DESIGN §4, ADR-0089 §1) ----------
ALTER TABLE sis.sis_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.sis_connections FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.sis_connections FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.sis_connections FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.sis_connections FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.sis_connection_schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.sis_connection_schools FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.sis_connection_schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.sis_connection_schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.sis_connection_schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.sync_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.sync_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.sync_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.sync_runs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.sync_runs FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.sis_imports ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.sis_imports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.sis_imports FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.sis_imports FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.sis_imports FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.sis_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.sis_links FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.sis_links FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.sis_links FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.sis_links FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.synced_students ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.synced_students FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.synced_students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.synced_students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.synced_students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.student_login_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.student_login_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.student_login_keys FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.student_login_keys FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.student_login_keys FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sis.reconciliation_issues ENABLE ROW LEVEL SECURITY;
ALTER TABLE sis.reconciliation_issues FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sis.reconciliation_issues FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sis.reconciliation_issues FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sis.reconciliation_issues FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE identity.sis_identity_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.sis_identity_links FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.sis_identity_links FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.sis_identity_links FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.sis_identity_links FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (ADR-0028); staging tables are append-only ----------
REVOKE DELETE, TRUNCATE ON sis.sis_connections, sis.sis_connection_schools, sis.sync_runs, sis.sis_imports, sis.sis_links, sis.synced_students, sis.student_login_keys, sis.reconciliation_issues, identity.sis_identity_links, sis.staging_records, sis.sync_record_errors FROM identity_app, identity_platform;

-- ---------- <table>_live views for identity_readonly (ADR-0089 §1) ----------
CREATE OR REPLACE VIEW sis.sis_connections_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sis_connections WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sis_connections_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.sis_connection_schools_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sis_connection_schools WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sis_connection_schools_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.sync_runs_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sync_runs WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sync_runs_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.sis_imports_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sis_imports WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sis_imports_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.sis_links_live WITH (security_barrier = true) AS
  SELECT * FROM sis.sis_links WHERE deleted_at IS NULL;
GRANT SELECT ON sis.sis_links_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.synced_students_live WITH (security_barrier = true) AS
  SELECT * FROM sis.synced_students WHERE deleted_at IS NULL;
GRANT SELECT ON sis.synced_students_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.student_login_keys_live WITH (security_barrier = true) AS
  SELECT * FROM sis.student_login_keys WHERE deleted_at IS NULL;
GRANT SELECT ON sis.student_login_keys_live TO identity_readonly;
CREATE OR REPLACE VIEW sis.reconciliation_issues_live WITH (security_barrier = true) AS
  SELECT * FROM sis.reconciliation_issues WHERE deleted_at IS NULL;
GRANT SELECT ON sis.reconciliation_issues_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.sis_identity_links_live WITH (security_barrier = true) AS
  SELECT * FROM identity.sis_identity_links WHERE deleted_at IS NULL;
GRANT SELECT ON identity.sis_identity_links_live TO identity_readonly;
