-- Test fixture of the sample module (F003): tables of each entx kind in database eduworld_learning,
-- schema sample. Applied by tests as learning_migrator; RLS policies come from tenancy.RLSPolicies.
CREATE SCHEMA sample;
GRANT USAGE ON SCHEMA sample TO learning_app, learning_platform;

CREATE TABLE sample.widgets (
  id uuid PRIMARY KEY, tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL, created_by uuid,
  updated_at timestamptz NOT NULL, updated_by uuid,
  deleted_at timestamptz, deleted_by uuid, delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(32) NOT NULL, name varchar(128) NOT NULL
);
CREATE UNIQUE INDEX widget_tenant_id_code ON sample.widgets (tenant_id, code) WHERE deleted_at IS NULL;

CREATE TABLE sample.widget_logs (
  id uuid PRIMARY KEY, tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL, created_by uuid,
  message varchar(255) NOT NULL
);

CREATE TABLE sample.templates (
  id uuid PRIMARY KEY, tenant_id uuid,
  created_at timestamptz NOT NULL, created_by uuid,
  updated_at timestamptz NOT NULL, updated_by uuid,
  deleted_at timestamptz, deleted_by uuid, delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  name varchar(128) NOT NULL
);

CREATE TABLE sample.units (
  id uuid PRIMARY KEY,
  created_at timestamptz NOT NULL, created_by uuid,
  updated_at timestamptz NOT NULL, updated_by uuid,
  deleted_at timestamptz, deleted_by uuid, delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  symbol varchar(16) NOT NULL
);

GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA sample TO learning_app, learning_platform;

-- Projection of widgets of another module (F003-AC-39): rebuilt by pkg/projection, upsert by source_version.
CREATE TABLE sample.ref_widgets (
  id uuid PRIMARY KEY, tenant_id uuid NOT NULL, name varchar(128) NOT NULL, source_version bigint NOT NULL
);
GRANT SELECT, INSERT, UPDATE ON sample.ref_widgets TO learning_app, learning_platform;

-- RLS (tenant condition only, ADR-0089 §1) — text of tenancy.RLSPolicies for each kind.
ALTER TABLE sample.widgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE sample.widgets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sample.widgets FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sample.widgets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sample.widgets FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sample.widget_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE sample.widget_logs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sample.widget_logs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sample.widget_logs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sample.templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE sample.templates FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sample.templates FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sample.templates FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sample.templates FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE sample.ref_widgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE sample.ref_widgets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON sample.ref_widgets FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON sample.ref_widgets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON sample.ref_widgets FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
