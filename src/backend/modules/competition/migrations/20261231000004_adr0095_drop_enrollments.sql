-- atlas:nolint destructive

-- ADR-0095 mục 8 (D4, WP-R contract): one-time, controlled exception of ADR-0070 mục 2 and AGENTS R12/R21 for the closed
-- list of CODE_PLAN §4.2 — module competition drops the projection ref.enrollments (its index, RLS policies and grants go
-- with the table). Valid only before the first staging/production deployment (F130 not deployed); otherwise the table
-- would be moved to schema retired instead. Backup before applying (R12): scripts/db/adr-0095-backup.sh competition;
-- rollback (structure, empty): scripts/db/rollback/adr-0095-competition.sql + scripts/db/adr-0095-backup.sh competition
-- --restore-data <dump>. No code reads or writes it since WP-C1 (20261231000002).
--
-- CHECK values of the removed concepts (party type study_group → study_party, party visibility class, party policy scope
-- class) are removed only when no row — soft-deleted included — carries them (ADR-0095 mục 8); otherwise the CHECK is
-- kept, the domain and the OpenAPI enums already refuse them, and a NOTICE says so. Applied as competition_migrator
-- (table owner, NOBYPASSRLS): FORCE ROW LEVEL SECURITY is lifted for the count only, inside this transaction, and restored.

DROP TABLE ref.enrollments;

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
    RAISE NOTICE 'ADR-0095: % keeps % (% rows, soft-deleted included, carry them)', con, old, n;
  END IF;
END
$fn$;

SELECT pg_temp.adr0095_narrow_check('multiplayer.parties', 'parties_type_chk', 'party_type', ARRAY['study_group'],
  $$CHECK (party_type IN ('friend','study_party','quest','competition_team','custom'))$$);
SELECT pg_temp.adr0095_narrow_check('multiplayer.parties', 'parties_visibility_chk', 'visibility', ARRAY['class'],
  $$CHECK (visibility IN ('private','friends'))$$);
SELECT pg_temp.adr0095_narrow_check('multiplayer.party_policies', 'party_policies_scope_chk', 'scope_type', ARRAY['class'],
  $$CHECK (scope_type IN ('school'))$$);

DROP FUNCTION pg_temp.adr0095_narrow_check(regclass, text, text, text[], text);
