-- F001 T015 — baseline of eduworld_identity (data-model §1–§2, DATABASE_DESIGN §1.2, §3.0, §4).
-- Applied by `migrate --module=identity` as identity_migrator (database owner). Binaries never auto-migrate.

CREATE SCHEMA IF NOT EXISTS platform;
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS org;
CREATE SCHEMA IF NOT EXISTS catalog;
CREATE SCHEMA IF NOT EXISTS sis;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS infra;

-- Tenant of the current transaction, set by UnitOfWork/ReadSession via set_config('app.tenant_id', …, true).
-- Unset → NULL → RLS returns no rows (MULTITENANCY §5.3).
CREATE OR REPLACE FUNCTION infra.current_tenant_id() RETURNS uuid
  LANGUAGE sql STABLE
  AS $$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;

GRANT USAGE ON SCHEMA platform, identity, org, catalog, sis, audit, infra TO identity_app, identity_platform;
GRANT EXECUTE ON FUNCTION infra.current_tenant_id() TO identity_app, identity_platform;

-- Soft-delete only (ADR-0028): application roles never get DELETE/TRUNCATE, now or for future tables.
REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA platform, identity, org, catalog, sis, audit, infra FROM identity_app, identity_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE identity_migrator IN SCHEMA platform, identity, org, catalog, sis, audit, infra
  GRANT SELECT, INSERT, UPDATE ON TABLES TO identity_app, identity_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE identity_migrator IN SCHEMA platform, identity, org, catalog, sis, audit, infra
  GRANT USAGE, SELECT ON SEQUENCES TO identity_app, identity_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE identity_migrator IN SCHEMA platform, identity, org, catalog, sis, audit, infra
  REVOKE DELETE, TRUNCATE ON TABLES FROM identity_app, identity_platform;

-- Binaries check the applied revision for readiness (atlas:<module>).
GRANT USAGE ON SCHEMA atlas_schema_revisions TO identity_app, identity_platform;
GRANT SELECT ON atlas_schema_revisions.atlas_schema_revisions TO identity_app, identity_platform;
