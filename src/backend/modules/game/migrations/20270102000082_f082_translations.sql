-- F082 T006 — translation of game content (specs/082-game-studio-chat-assets-translations: data-model §1;
-- DATABASE_DESIGN §2.16, §3.13; CONTENT_MODEL §7, §8; ADR-0041, ADR-0043, ADR-0044, ADR-0056). Applied by
-- `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test modules/game/ent/ddl_test.go),
-- then what ent cannot express: CHECKs, the platform-only writes, the document kind of game media (translation files to
-- import, never public), _live views. Nothing is deleted (R12, R21).

-- BEGIN entddl
CREATE TABLE game.config_translation_states (
  config_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  config_type varchar(64) NOT NULL,
  config_id uuid NOT NULL,
  translation_status varchar(64) NOT NULL,
  missing_pointers jsonb NOT NULL,
  outdated_pointers jsonb NOT NULL,
  translation_meta jsonb NOT NULL,
  computed_at timestamptz NOT NULL,
  PRIMARY KEY (config_version_id)
);
CREATE INDEX config_translation_states_item_idx ON game.config_translation_states (config_id) WHERE deleted_at IS NULL;
CREATE INDEX config_translation_states_status_idx ON game.config_translation_states (config_type, translation_status) WHERE deleted_at IS NULL;

CREATE TABLE game.translation_jobs (
  job_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  kind varchar(64) NOT NULL,
  status varchar(64) NOT NULL,
  format varchar(64) NOT NULL,
  source_language varchar(8) NOT NULL,
  target_language varchar(8) NOT NULL,
  only_missing boolean NOT NULL DEFAULT true,
  dry_run boolean NOT NULL DEFAULT false,
  request jsonb NOT NULL,
  input_asset_id uuid,
  result_storage_key varchar(512),
  report jsonb,
  error_message varchar(1000),
  requested_by uuid NOT NULL,
  idempotency_key varchar(64),
  attempts bigint NOT NULL DEFAULT 0,
  started_at timestamptz,
  finished_at timestamptz,
  PRIMARY KEY (job_id)
);
CREATE UNIQUE INDEX translation_jobs_idem_uq ON game.translation_jobs (requested_by, idempotency_key) WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX translation_jobs_queue_idx ON game.translation_jobs (status, created_at) WHERE status IN ('queued', 'running') AND deleted_at IS NULL;
-- END entddl

ALTER TABLE game.config_translation_states
  ADD CONSTRAINT config_translation_states_version_fk FOREIGN KEY (config_version_id) REFERENCES game.config_versions (config_version_id),
  ADD CONSTRAINT config_translation_states_item_fk FOREIGN KEY (config_id) REFERENCES game.config_items (config_id),
  ADD CONSTRAINT config_translation_states_status_chk CHECK (translation_status IN ('complete','partial','outdated')),
  ADD CONSTRAINT config_translation_states_missing_chk CHECK (jsonb_typeof(missing_pointers) = 'array'
    AND jsonb_array_length(missing_pointers) <= 500),
  ADD CONSTRAINT config_translation_states_outdated_chk CHECK (jsonb_typeof(outdated_pointers) = 'array'
    AND jsonb_array_length(outdated_pointers) <= 500),
  ADD CONSTRAINT config_translation_states_meta_chk CHECK (jsonb_typeof(translation_meta) = 'object'
    AND octet_length(translation_meta::text) <= 262144);

ALTER TABLE game.translation_jobs
  ADD CONSTRAINT translation_jobs_kind_chk CHECK (kind IN ('export','import')),
  ADD CONSTRAINT translation_jobs_status_chk CHECK (status IN ('queued','running','succeeded','failed')),
  ADD CONSTRAINT translation_jobs_format_chk CHECK (format IN ('xliff','csv')),
  ADD CONSTRAINT translation_jobs_lang_chk CHECK (source_language IN ('vi','en') AND target_language IN ('vi','en')
    AND source_language <> target_language),
  ADD CONSTRAINT translation_jobs_request_chk CHECK (jsonb_typeof(request) = 'object' AND octet_length(request::text) <= 65536),
  ADD CONSTRAINT translation_jobs_report_chk CHECK (report IS NULL OR (jsonb_typeof(report) = 'object'
    AND octet_length(report::text) <= 524288)),
  ADD CONSTRAINT translation_jobs_input_chk CHECK ((kind = 'import') = (input_asset_id IS NOT NULL)),
  ADD CONSTRAINT translation_jobs_input_fk FOREIGN KEY (input_asset_id) REFERENCES game.media_assets (asset_id),
  ADD CONSTRAINT translation_jobs_attempts_chk CHECK (attempts >= 0),
  ADD CONSTRAINT translation_jobs_done_chk CHECK ((status IN ('succeeded','failed')) = (finished_at IS NOT NULL)),
  ADD CONSTRAINT translation_jobs_failed_chk CHECK (status <> 'failed' OR error_message IS NOT NULL);

-- Platform-only writes (game_platform: console, worker jobs); no physical delete (R21).
REVOKE INSERT, UPDATE ON game.config_translation_states, game.translation_jobs FROM game_app;
REVOKE DELETE, TRUNCATE ON game.config_translation_states, game.translation_jobs FROM game_app, game_platform;

-- ---------- game media kind document: a translation file to import (XLIFF/CSV) ----------
-- Checked by the media pipeline (size, sha256, virus scan) and kept in the private quarantine bucket once ready: never
-- copied to ew-media-public, never linked in media_usages, never on the CDN (R31, Clarifications 2026-11-26).
ALTER TABLE game.media_assets DROP CONSTRAINT media_assets_kind_chk,
  ADD CONSTRAINT media_assets_kind_chk CHECK (kind IN ('image','audio','model3d','render_thumbnail','render_turntable','document'));
ALTER TABLE game.media_assets DROP CONSTRAINT media_assets_ready_chk,
  ADD CONSTRAINT media_assets_ready_chk CHECK (status <> 'ready' OR (kind <> 'document' AND bucket = 'ew-media-public')
    OR (kind = 'document' AND bucket = 'ew-media-quarantine'));

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.config_translation_states_live WITH (security_barrier = true) AS
  SELECT * FROM game.config_translation_states WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.translation_jobs_live WITH (security_barrier = true) AS
  SELECT * FROM game.translation_jobs WHERE deleted_at IS NULL;
GRANT SELECT ON game.config_translation_states_live, game.translation_jobs_live TO game_readonly;
