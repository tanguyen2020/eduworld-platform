-- F108 T003/T013 — write side of the competition configurations (specs/108-competition-config-portal: data-model §1;
-- DATABASE_DESIGN §2.9; API_DESIGN §9.13b; ADR-0043, ADR-0041, ADR-0089). Applied by `migrate --module=competition` as
-- competition_migrator. Expand only (AGENTS R12): nullable columns, a CHECK widened with one more status, one backfill of
-- release rows. Order: added columns (entddl block, golden test modules/competition/ent/ddl_test.go), then what ent cannot
-- express.

-- BEGIN entddl
ALTER TABLE competition.game_config_versions ADD COLUMN review_note varchar(2000);
ALTER TABLE competition.game_config_versions ADD COLUMN published_by uuid;
ALTER TABLE competition.content_releases ADD COLUMN effective_from timestamptz;
-- END entddl

-- A version returned by a reviewer is `rejected` (ContentStatus of the console, F108-AC-01): reworked in place, then
-- submitted again.
ALTER TABLE competition.game_config_versions DROP CONSTRAINT game_config_versions_status_chk;
ALTER TABLE competition.game_config_versions
  ADD CONSTRAINT game_config_versions_status_chk CHECK (status IN ('draft','in_review','approved','rejected','published','unpublished','archived'));

-- A published version keeps its publisher (R11: the trigger already freezes the content of a published version).
CREATE OR REPLACE FUNCTION competition.game_config_versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'published' AND (NEW.payload IS DISTINCT FROM OLD.payload OR NEW.version_no IS DISTINCT FROM OLD.version_no
    OR NEW.config_id IS DISTINCT FROM OLD.config_id OR NEW.effective_from IS DISTINCT FROM OLD.effective_from
    OR NEW.title IS DISTINCT FROM OLD.title OR NEW.published_at IS DISTINCT FROM OLD.published_at
    OR NEW.published_by IS DISTINCT FROM OLD.published_by) THEN
    RAISE EXCEPTION 'GAME_CONFIG_VERSION_PUBLISHED' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;

-- The runtime reads the configuration in effect from the release timeline (ADR-0043: rollback/unpublish only move the
-- pointer): the latest release (by insertion) whose effective time — content_releases.effective_from, else the
-- effective_from of its version — is not after the read time. Every configuration published by a seed without a release
-- row gets its publish release here (expand: rows added, none changed).
INSERT INTO competition.content_releases (release_id, created_at, config_id, action, from_version_id, to_version_id, reason, released_at)
SELECT gen_random_uuid(), COALESCE(v.published_at, now()), c.config_id, 'publish', NULL, c.current_version_id, 'Seed (F108 backfill)',
  COALESCE(v.published_at, now())
FROM competition.game_configs c
JOIN competition.game_config_versions v ON v.config_version_id = c.current_version_id
WHERE c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM competition.content_releases r WHERE r.config_id = c.config_id);

CREATE INDEX content_releases_config_created_idx ON competition.content_releases (config_id, created_at DESC, release_id DESC);
