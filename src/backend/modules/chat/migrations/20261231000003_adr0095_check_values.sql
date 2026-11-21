-- ADR-0095 mục 8 (D4, WP-R contract) for module chat: there is no table to drop (CODE_PLAN §4.2); only the CHECK values
-- of the removed concepts go — channel types class/study_group and the chat policy scope class — and only when no row,
-- soft-deleted included, carries them (ADR-0095 mục 8). Channels and policies of those kinds were archived/soft-deleted by
-- the expand migration 20261231000002 and their messages are kept (moderation evidence, R25): when such rows exist the
-- CHECK is kept, the domain, the AsyncAPI and the OpenAPI enums already refuse the values, and a NOTICE says so.
-- Nothing is deleted. Backup before applying (R12): scripts/db/adr-0095-backup.sh chat; rollback:
-- scripts/db/rollback/adr-0095-chat.sql. Applied as chat_migrator (table owner, NOBYPASSRLS): FORCE ROW LEVEL SECURITY is
-- lifted for the count only, inside this transaction, and restored.

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

SELECT pg_temp.adr0095_narrow_check('chat.channels', 'channels_type_chk', 'channel_type', ARRAY['class', 'study_group'],
  $$CHECK (channel_type IN ('world','area','match','party','team','direct'))$$);
SELECT pg_temp.adr0095_narrow_check('chat.chat_policies', 'chat_policies_scope_chk', 'scope_type', ARRAY['class'],
  $$CHECK (scope_type IN ('platform','tenant','school'))$$);

DROP FUNCTION pg_temp.adr0095_narrow_check(regclass, text, text, text[], text);
