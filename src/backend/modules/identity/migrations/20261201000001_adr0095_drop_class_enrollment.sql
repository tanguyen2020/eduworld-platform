-- atlas:nolint destructive

-- ADR-0095 mục 8 (D4, WP-R contract): one-time, controlled exception of ADR-0070 mục 2 and AGENTS R12/R21 for the closed
-- list of CODE_PLAN §4.2 — module identity drops org.classes and org.enrollments (their _live views, RLS policies,
-- indexes, foreign keys and grants go with the tables; the 9 virtual classes common-g1…g9 of the F010 seed go with
-- org.classes). Valid only before the first staging/production deployment (F130 not deployed); otherwise the tables
-- would be moved to schema retired instead. Backup before applying (R12): scripts/db/adr-0095-backup.sh identity;
-- rollback (structures, empty): scripts/db/rollback/adr-0095-identity.sql + pg_restore --data-only from that dump.
-- No code reads or writes these tables since WP-I1 (identity.tenant_memberships_v is built from org.students,
-- 20261130000002). Explicit DROP without CASCADE: an unexpected dependency stops the migration.
--
-- CHECK values of the removed concepts (sis.sis_links.entity_type, sis.staging_records.entity_kind) are removed only
-- when no row — soft-deleted or history included — carries them (ADR-0095 mục 8); otherwise the CHECK is kept, the
-- domain and the OpenAPI enums already refuse them, and a NOTICE says so. Applied as identity_migrator (table owner,
-- NOBYPASSRLS): FORCE ROW LEVEL SECURITY is lifted for the count only, inside this transaction, and restored.

DROP VIEW org.enrollments_live;
DROP VIEW org.classes_live;
DROP TABLE org.enrollments;
DROP TABLE org.classes;

-- Narrows a CHECK constraint when no row carries an old value (helper of this migration, session-scoped).
CREATE OR REPLACE FUNCTION pg_temp.adr0095_narrow_check(tbl regclass, con text, col text, old text[], def text)
RETURNS void LANGUAGE plpgsql AS $fn$
DECLARE
  forced boolean;
  n bigint;
BEGIN
  SELECT relforcerowsecurity INTO forced FROM pg_class WHERE oid = tbl;
  IF forced THEN
    EXECUTE format('ALTER TABLE %s NO FORCE ROW LEVEL SECURITY', tbl);
  END IF;
  EXECUTE format('SELECT count(*) FROM %s WHERE %I::text = ANY ($1)', tbl, col) INTO n USING old;
  IF forced THEN
    EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', tbl);
  END IF;
  IF n = 0 THEN
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', tbl, con);
    EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s', tbl, con, def);
    RAISE NOTICE 'ADR-0095: % narrowed (no row carries %)', con, old;
  ELSE
    RAISE NOTICE 'ADR-0095: % keeps % (% rows, soft-deleted or history included, carry them)', con, old, n;
  END IF;
END
$fn$;

SELECT pg_temp.adr0095_narrow_check('sis.sis_links', 'sis_links_type_chk', 'entity_type', ARRAY['class', 'enrollment'],
  $$CHECK (entity_type IN ('school'))$$);
SELECT pg_temp.adr0095_narrow_check('sis.staging_records', 'staging_records_entity_kind_check', 'entity_kind',
  ARRAY['class', 'enrollment'], $$CHECK (entity_kind IN ('school','student'))$$);

DROP FUNCTION pg_temp.adr0095_narrow_check(regclass, text, text, text[], text);
