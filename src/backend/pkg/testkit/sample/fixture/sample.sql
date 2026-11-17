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
