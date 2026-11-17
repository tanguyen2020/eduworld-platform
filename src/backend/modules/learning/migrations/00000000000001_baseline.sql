-- F001 T015 — baseline of eduworld_learning (data-model §1–§2, DATABASE_DESIGN §1.2, §3.0, §4).
-- Applied by `migrate --module=learning` as learning_migrator (database owner). Binaries never auto-migrate.

CREATE SCHEMA IF NOT EXISTS curriculum;
CREATE SCHEMA IF NOT EXISTS content;
CREATE SCHEMA IF NOT EXISTS assessment;
CREATE SCHEMA IF NOT EXISTS learning;
CREATE SCHEMA IF NOT EXISTS lms;
CREATE SCHEMA IF NOT EXISTS ref;
CREATE SCHEMA IF NOT EXISTS infra;

-- Tenant of the current transaction, set by UnitOfWork/ReadSession via set_config('app.tenant_id', …, true).
-- Unset → NULL → RLS returns no rows (MULTITENANCY §5.3).
CREATE OR REPLACE FUNCTION infra.current_tenant_id() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

GRANT USAGE ON SCHEMA curriculum, content, assessment, learning, lms, ref, infra TO learning_app, learning_platform;
GRANT EXECUTE ON FUNCTION infra.current_tenant_id() TO learning_app, learning_platform;

-- Soft-delete only (ADR-0028): application roles never get DELETE/TRUNCATE, now or for future tables.
REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA curriculum, content, assessment, learning, lms, ref, infra FROM learning_app, learning_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE learning_migrator IN SCHEMA curriculum, content, assessment, learning, lms, ref, infra
  GRANT SELECT, INSERT, UPDATE ON TABLES TO learning_app, learning_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE learning_migrator IN SCHEMA curriculum, content, assessment, learning, lms, ref, infra
  GRANT USAGE, SELECT ON SEQUENCES TO learning_app, learning_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE learning_migrator IN SCHEMA curriculum, content, assessment, learning, lms, ref, infra
  REVOKE DELETE, TRUNCATE ON TABLES FROM learning_app, learning_platform;

-- Binaries check the applied revision for readiness (atlas:<module>).
GRANT USAGE ON SCHEMA atlas_schema_revisions TO learning_app, learning_platform;
GRANT SELECT ON atlas_schema_revisions.atlas_schema_revisions TO learning_app, learning_platform;
