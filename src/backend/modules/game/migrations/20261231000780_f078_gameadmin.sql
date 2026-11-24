-- F078 T007/T008 — Game Operations Console backend of module game (specs/078-game-ops-console-backend: data-model §1;
-- DATABASE_DESIGN §2.16, §3.13; CONTENT_MODEL §5–7; ADR-0027, ADR-0028, ADR-0041, ADR-0043, ADR-0044, ADR-0047). Applied by
-- `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test modules/game/ent/ddl_test.go), then
-- what ent cannot express: CHECKs, FKs, the freeze of a version under review and of a published version, platform-only
-- writes (game_platform), the backfill of the console tables from the release core, _live views.

-- BEGIN entddl
CREATE TABLE game.config_items (
  config_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  item_type varchar(64) NOT NULL,
  item_key varchar(64) NOT NULL,
  latest_version_no bigint NOT NULL DEFAULT 0,
  current_version_id uuid,
  PRIMARY KEY (config_id)
);
CREATE UNIQUE INDEX config_items_key_uq ON game.config_items (item_type, item_key) WHERE deleted_at IS NULL;

CREATE TABLE game.config_versions (
  config_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  config_id uuid NOT NULL,
  item_type varchar(64) NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  title varchar(255),
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  author_id uuid,
  reviewer_id uuid,
  review_note varchar(2000),
  change_note varchar(500),
  based_on_version_id uuid,
  submitted_at timestamptz,
  reviewed_at timestamptz,
  published_at timestamptz,
  published_by uuid,
  PRIMARY KEY (config_version_id)
);
CREATE INDEX config_versions_list_idx ON game.config_versions (item_type, status, config_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX config_versions_no_uq ON game.config_versions (config_id, version_no);

CREATE TABLE game.scheduled_publications (
  schedule_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  config_id uuid NOT NULL,
  item_type varchar(64) NOT NULL,
  config_version_id uuid NOT NULL,
  version_no bigint NOT NULL,
  publish_at timestamptz NOT NULL,
  status varchar(16) NOT NULL,
  failure_code varchar(64),
  change_note varchar(500),
  reason varchar(500),
  requested_by uuid NOT NULL,
  cancelled_by uuid,
  finished_at timestamptz,
  PRIMARY KEY (schedule_id)
);
CREATE INDEX scheduled_publications_due_idx ON game.scheduled_publications (publish_at) WHERE status = 'scheduled' AND deleted_at IS NULL;
CREATE UNIQUE INDEX scheduled_publications_one_uq ON game.scheduled_publications (config_id) WHERE status = 'scheduled' AND deleted_at IS NULL;

CREATE TABLE game.preview_sessions (
  preview_session_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  token_hash varchar(128) NOT NULL,
  config_type varchar(64) NOT NULL,
  config_id uuid NOT NULL,
  config_version_id uuid NOT NULL,
  version_no bigint NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (preview_session_id)
);
CREATE UNIQUE INDEX preview_sessions_token_uq ON game.preview_sessions (token_hash);

CREATE TABLE game.render_jobs (
  render_job_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  config_type varchar(64) NOT NULL,
  item_type varchar(64) NOT NULL,
  config_version_id uuid NOT NULL,
  source_asset_id uuid NOT NULL,
  kind varchar(64) NOT NULL,
  status varchar(16) NOT NULL,
  attempts bigint NOT NULL DEFAULT 0,
  last_error varchar(1000),
  result_asset_id uuid,
  next_attempt_at timestamptz,
  PRIMARY KEY (render_job_id)
);
CREATE INDEX render_jobs_retry_idx ON game.render_jobs (next_attempt_at) WHERE status = 'failed' AND deleted_at IS NULL;
CREATE UNIQUE INDEX render_jobs_uq ON game.render_jobs (config_version_id, source_asset_id, kind);

CREATE TABLE game.media_assets (
  asset_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  kind varchar(64) NOT NULL,
  mime_type varchar(127) NOT NULL,
  original_filename varchar(255) NOT NULL,
  size_bytes bigint NOT NULL,
  sha256 varchar(64) NOT NULL,
  bucket varchar(64) NOT NULL,
  quarantine_key varchar(512),
  status varchar(16) NOT NULL,
  reject_reason varchar(255),
  variants jsonb,
  width bigint,
  height bigint,
  license varchar(255),
  alt_text varchar(500),
  uploaded_by uuid,
  upload_expires_at timestamptz,
  processed_at timestamptz,
  PRIMARY KEY (asset_id)
);
CREATE INDEX media_assets_pipeline_idx ON game.media_assets (status, created_at) WHERE status IN ('quarantined', 'processing') AND deleted_at IS NULL;
CREATE UNIQUE INDEX media_assets_sha_uq ON game.media_assets (sha256) WHERE deleted_at IS NULL;

CREATE TABLE game.media_usages (
  usage_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  asset_id uuid NOT NULL,
  item_type varchar(64) NOT NULL,
  config_id uuid NOT NULL,
  config_version_id uuid NOT NULL,
  usage varchar(64) NOT NULL,
  PRIMARY KEY (usage_id)
);
CREATE UNIQUE INDEX media_usages_uq ON game.media_usages (asset_id, config_version_id, usage) WHERE deleted_at IS NULL;
CREATE INDEX media_usages_version_idx ON game.media_usages (config_version_id, usage) WHERE deleted_at IS NULL;

CREATE TABLE game.client_manifests (
  client_manifest_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  client_version varchar(32) NOT NULL,
  platform varchar(32) NOT NULL,
  manifest jsonb NOT NULL,
  published_at timestamptz NOT NULL,
  PRIMARY KEY (client_manifest_id)
);
CREATE UNIQUE INDEX client_manifests_uq ON game.client_manifests (client_version, platform) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- workflow of the game config versions (G, platform only) ----------
ALTER TABLE game.config_items
  ADD CONSTRAINT config_items_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','item','quest','minigame_config',
    'progression_curve','reward_rule','shop_item','collection','achievement','event','event_template','season_track','challenge','boss')),
  ADD CONSTRAINT config_items_latest_chk CHECK (latest_version_no >= 0);
ALTER TABLE game.config_versions
  ADD CONSTRAINT config_versions_item_fk FOREIGN KEY (config_id) REFERENCES game.config_items (config_id),
  ADD CONSTRAINT config_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT config_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT config_versions_payload_chk CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 65536),
  ADD CONSTRAINT config_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT config_versions_reviewer_chk CHECK (reviewer_id IS NULL OR author_id IS NULL OR reviewer_id <> author_id),
  ADD CONSTRAINT config_versions_published_chk CHECK (status <> 'published' OR published_at IS NOT NULL);

-- A version under review, approved, published or archived never changes its content (R11, R31): only the workflow columns
-- move. A published/archived version only moves between published and archived (explicit archive; rollback target).
CREATE FUNCTION game.config_version_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  mutable text[] := ARRAY['status', 'reviewer_id', 'review_note', 'submitted_at', 'reviewed_at', 'published_at', 'published_by',
    'updated_at', 'updated_by', 'version', 'deleted_at', 'deleted_by', 'delete_reason'];
BEGIN
  IF OLD.status IN ('draft', 'rejected') THEN
    RETURN NEW;
  END IF;
  IF (to_jsonb(NEW) - mutable) <> (to_jsonb(OLD) - mutable) THEN
    RAISE EXCEPTION 'CONTENT_VERSION_IMMUTABLE' USING ERRCODE = 'P0001';
  END IF;
  IF OLD.status IN ('published', 'archived') AND NEW.status NOT IN ('published', 'archived') THEN
    RAISE EXCEPTION 'CONTENT_VERSION_IMMUTABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER config_versions_guard BEFORE UPDATE ON game.config_versions
  FOR EACH ROW EXECUTE FUNCTION game.config_version_guard();

ALTER TABLE game.scheduled_publications
  ADD CONSTRAINT scheduled_publications_item_fk FOREIGN KEY (config_id) REFERENCES game.config_items (config_id),
  ADD CONSTRAINT scheduled_publications_version_fk FOREIGN KEY (config_version_id) REFERENCES game.config_versions (config_version_id),
  ADD CONSTRAINT scheduled_publications_status_chk CHECK (status IN ('scheduled','applied','cancelled','failed')),
  ADD CONSTRAINT scheduled_publications_done_chk CHECK ((status = 'scheduled') = (finished_at IS NULL)),
  ADD CONSTRAINT scheduled_publications_failure_chk CHECK ((status = 'failed') = (failure_code IS NOT NULL));

ALTER TABLE game.preview_sessions
  ADD CONSTRAINT preview_sessions_version_fk FOREIGN KEY (config_version_id) REFERENCES game.config_versions (config_version_id),
  ADD CONSTRAINT preview_sessions_token_chk CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT preview_sessions_ttl_chk CHECK (expires_at > created_at);

-- ---------- game media and renders (ADR-0044, FR-CMS-017) ----------
ALTER TABLE game.media_assets
  ADD CONSTRAINT media_assets_kind_chk CHECK (kind IN ('image','audio','model3d','render_thumbnail','render_turntable')),
  ADD CONSTRAINT media_assets_status_chk CHECK (status IN ('quarantined','processing','ready','rejected')),
  ADD CONSTRAINT media_assets_bucket_chk CHECK (bucket IN ('ew-media-quarantine','ew-media-public')),
  ADD CONSTRAINT media_assets_sha_chk CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT media_assets_size_chk CHECK (size_bytes > 0 AND size_bytes <= 52428800),
  ADD CONSTRAINT media_assets_variants_chk CHECK (variants IS NULL OR (jsonb_typeof(variants) = 'array'
    AND octet_length(variants::text) <= 16384)),
  ADD CONSTRAINT media_assets_ready_chk CHECK (status <> 'ready' OR bucket = 'ew-media-public'),
  ADD CONSTRAINT media_assets_reject_chk CHECK ((status = 'rejected') = (reject_reason IS NOT NULL));
ALTER TABLE game.media_usages
  ADD CONSTRAINT media_usages_asset_fk FOREIGN KEY (asset_id) REFERENCES game.media_assets (asset_id),
  ADD CONSTRAINT media_usages_version_fk FOREIGN KEY (config_version_id) REFERENCES game.config_versions (config_version_id),
  ADD CONSTRAINT media_usages_usage_chk CHECK (usage IN ('content','render_thumbnail','render_turntable'));
ALTER TABLE game.render_jobs
  ADD CONSTRAINT render_jobs_version_fk FOREIGN KEY (config_version_id) REFERENCES game.config_versions (config_version_id),
  ADD CONSTRAINT render_jobs_source_fk FOREIGN KEY (source_asset_id) REFERENCES game.media_assets (asset_id),
  ADD CONSTRAINT render_jobs_result_fk FOREIGN KEY (result_asset_id) REFERENCES game.media_assets (asset_id),
  ADD CONSTRAINT render_jobs_kind_chk CHECK (kind IN ('render_thumbnail','render_turntable')),
  ADD CONSTRAINT render_jobs_status_chk CHECK (status IN ('queued','done','failed','dead')),
  ADD CONSTRAINT render_jobs_attempts_chk CHECK (attempts >= 0),
  ADD CONSTRAINT render_jobs_done_chk CHECK ((status = 'done') = (result_asset_id IS NOT NULL));
ALTER TABLE game.client_manifests
  ADD CONSTRAINT client_manifests_platform_chk CHECK (platform IN ('windows','macos','linux','android','ios','web')),
  ADD CONSTRAINT client_manifests_doc_chk CHECK (jsonb_typeof(manifest) = 'object' AND octet_length(manifest::text) <= 1048576);

-- Platform-only writes: the console, the jobs and the consumers run under PlatformScope (game_platform); the student use
-- cases (game_app) never write these tables. Soft delete only (R21).
REVOKE INSERT, UPDATE ON game.config_items, game.config_versions, game.scheduled_publications, game.preview_sessions,
  game.render_jobs, game.media_assets, game.media_usages, game.client_manifests FROM game_app;
REVOKE DELETE, TRUNCATE ON game.config_items, game.config_versions, game.scheduled_publications, game.preview_sessions,
  game.render_jobs, game.media_assets, game.media_usages, game.client_manifests FROM game_app, game_platform;

-- ---------- release core: superseded versions stay published (ADR-0043 rollback target) ----------
-- Before F078 the release core archived the previous published version on each publish (no explicit archive existed):
-- those versions are published versions that are not the pointer, so they become rollback targets again. Status only
-- (allowed by game.forbid_published_version_change); no content changes, nothing is deleted.
UPDATE game.world_area_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.story_chapter_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.npc_dialogue_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.quest_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.minigame_config_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.progression_curve_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE gamification.reward_rule_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.item_definition_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE gamification.shop_item_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE gamification.collection_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE gamification.achievement_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.event_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL AND tenant_id IS NULL;
UPDATE game.event_template_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.season_track_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;
UPDATE game.challenge_versions SET status = 'published' WHERE status = 'archived' AND published_at IS NOT NULL AND deleted_at IS NULL;

-- ---------- backfill: the items and published versions of the release core enter the console ----------
-- A UUID-keyed item keeps its key as config_id; a code-keyed item (area_id, chapter_code, item_id) gets a new config_id. The
-- default items of the F070 seed (no payload) and the tenant events of the schools (F077) stay out of the console.
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT gen_random_uuid(), i.created_at, i.created_by, i.updated_at, 'world_area', i.area_id::text, i.latest_version_no, i.current_version_id
  FROM game.world_areas i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.world_area_versions v WHERE v.area_id = i.area_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'world_area' AND c.item_key = i.area_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.area_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'world_area', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.world_area_versions v JOIN game.config_items c ON c.item_type = 'world_area' AND c.item_key = v.area_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.area_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT gen_random_uuid(), i.created_at, i.created_by, i.updated_at, 'story_chapter', i.chapter_code::text, i.latest_version_no, i.current_version_id
  FROM game.story_chapters i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.story_chapter_versions v WHERE v.chapter_code = i.chapter_code AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'story_chapter' AND c.item_key = i.chapter_code::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.chapter_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'story_chapter', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.story_chapter_versions v JOIN game.config_items c ON c.item_type = 'story_chapter' AND c.item_key = v.chapter_code::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.chapter_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.dialogue_id, i.created_at, i.created_by, i.updated_at, 'npc_dialogue', i.dialogue_id::text, i.latest_version_no, i.current_version_id
  FROM game.npc_dialogues i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.npc_dialogue_versions v WHERE v.dialogue_id = i.dialogue_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'npc_dialogue' AND c.item_key = i.dialogue_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.dialogue_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'npc_dialogue', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.npc_dialogue_versions v JOIN game.config_items c ON c.item_type = 'npc_dialogue' AND c.item_key = v.dialogue_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.dialogue_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.quest_id, i.created_at, i.created_by, i.updated_at, 'quest', i.quest_id::text, i.latest_version_no, i.current_version_id
  FROM game.quest_definitions i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.quest_versions v WHERE v.quest_id = i.quest_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'quest' AND c.item_key = i.quest_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.quest_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'quest', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.quest_versions v JOIN game.config_items c ON c.item_type = 'quest' AND c.item_key = v.quest_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.quest_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.minigame_config_id, i.created_at, i.created_by, i.updated_at, 'minigame_config', i.minigame_config_id::text, i.latest_version_no, i.current_version_id
  FROM game.minigame_configs i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.minigame_config_versions v WHERE v.minigame_config_id = i.minigame_config_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'minigame_config' AND c.item_key = i.minigame_config_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.minigame_config_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'minigame_config', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.minigame_config_versions v JOIN game.config_items c ON c.item_type = 'minigame_config' AND c.item_key = v.minigame_config_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.minigame_config_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.curve_id, i.created_at, i.created_by, i.updated_at, 'progression_curve', i.curve_id::text, i.latest_version_no, i.current_version_id
  FROM game.progression_curves i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.progression_curve_versions v WHERE v.curve_id = i.curve_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'progression_curve' AND c.item_key = i.curve_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.curve_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'progression_curve', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.progression_curve_versions v JOIN game.config_items c ON c.item_type = 'progression_curve' AND c.item_key = v.curve_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.curve_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.reward_rule_id, i.created_at, i.created_by, i.updated_at, 'reward_rule', i.reward_rule_id::text, i.latest_version_no, i.current_version_id
  FROM gamification.reward_rules i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM gamification.reward_rule_versions v WHERE v.reward_rule_id = i.reward_rule_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'reward_rule' AND c.item_key = i.reward_rule_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.reward_rule_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'reward_rule', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM gamification.reward_rule_versions v JOIN game.config_items c ON c.item_type = 'reward_rule' AND c.item_key = v.reward_rule_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.reward_rule_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT gen_random_uuid(), i.created_at, i.created_by, i.updated_at, 'item', i.item_id::text, i.latest_version_no, i.current_version_id
  FROM game.item_definitions i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.item_definition_versions v WHERE v.item_id = i.item_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'item' AND c.item_key = i.item_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.item_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'item', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.item_definition_versions v JOIN game.config_items c ON c.item_type = 'item' AND c.item_key = v.item_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.item_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.shop_item_id, i.created_at, i.created_by, i.updated_at, 'shop_item', i.shop_item_id::text, i.latest_version_no, i.current_version_id
  FROM gamification.shop_items i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM gamification.shop_item_versions v WHERE v.shop_item_id = i.shop_item_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'shop_item' AND c.item_key = i.shop_item_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.shop_item_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'shop_item', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM gamification.shop_item_versions v JOIN game.config_items c ON c.item_type = 'shop_item' AND c.item_key = v.shop_item_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.shop_item_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.collection_id, i.created_at, i.created_by, i.updated_at, 'collection', i.collection_id::text, i.latest_version_no, i.current_version_id
  FROM gamification.collections i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM gamification.collection_versions v WHERE v.collection_id = i.collection_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'collection' AND c.item_key = i.collection_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.collection_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'collection', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM gamification.collection_versions v JOIN game.config_items c ON c.item_type = 'collection' AND c.item_key = v.collection_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.collection_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.achievement_id, i.created_at, i.created_by, i.updated_at, 'achievement', i.achievement_id::text, i.latest_version_no, i.current_version_id
  FROM gamification.achievements i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM gamification.achievement_versions v WHERE v.achievement_id = i.achievement_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'achievement' AND c.item_key = i.achievement_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.achievement_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'achievement', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM gamification.achievement_versions v JOIN game.config_items c ON c.item_type = 'achievement' AND c.item_key = v.achievement_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.achievement_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.event_id, i.created_at, i.created_by, i.updated_at, 'event', i.event_id::text, i.latest_version_no, i.current_version_id
  FROM game.events i
  WHERE i.deleted_at IS NULL AND i.tenant_id IS NULL AND EXISTS (SELECT 1 FROM game.event_versions v WHERE v.event_id = i.event_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'event' AND c.item_key = i.event_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.event_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'event', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.event_versions v JOIN game.config_items c ON c.item_type = 'event' AND c.item_key = v.event_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.tenant_id IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.event_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.template_id, i.created_at, i.created_by, i.updated_at, 'event_template', i.template_id::text, i.latest_version_no, i.current_version_id
  FROM game.event_templates i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.event_template_versions v WHERE v.template_id = i.template_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'event_template' AND c.item_key = i.template_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.template_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'event_template', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.event_template_versions v JOIN game.config_items c ON c.item_type = 'event_template' AND c.item_key = v.template_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.template_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.season_track_id, i.created_at, i.created_by, i.updated_at, 'season_track', i.season_track_id::text, i.latest_version_no, i.current_version_id
  FROM game.season_tracks i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.season_track_versions v WHERE v.season_track_id = i.season_track_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'season_track' AND c.item_key = i.season_track_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.season_track_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'season_track', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.season_track_versions v JOIN game.config_items c ON c.item_type = 'season_track' AND c.item_key = v.season_track_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.season_track_version_id);
INSERT INTO game.config_items (config_id, created_at, created_by, updated_at, item_type, item_key, latest_version_no,
    current_version_id)
  SELECT i.challenge_id, i.created_at, i.created_by, i.updated_at, 'challenge', i.challenge_id::text, i.latest_version_no, i.current_version_id
  FROM game.challenges i
  WHERE i.deleted_at IS NULL AND EXISTS (SELECT 1 FROM game.challenge_versions v WHERE v.challenge_id = i.challenge_id AND v.payload IS NOT NULL)
    AND NOT EXISTS (SELECT 1 FROM game.config_items c WHERE c.item_type = 'challenge' AND c.item_key = i.challenge_id::text AND c.deleted_at IS NULL);
INSERT INTO game.config_versions (config_version_id, created_at, created_by, updated_at, config_id, item_type, version_no, status,
    payload, payload_sha256, change_note, published_at, published_by)
  SELECT v.challenge_version_id, v.created_at, v.created_by, v.updated_at, c.config_id, 'challenge', v.version_no, v.status, v.payload, v.payload_sha256,
    v.change_note, v.published_at, v.published_by
  FROM game.challenge_versions v JOIN game.config_items c ON c.item_type = 'challenge' AND c.item_key = v.challenge_id::text AND c.deleted_at IS NULL
  WHERE v.deleted_at IS NULL AND v.payload IS NOT NULL AND v.published_at IS NOT NULL AND v.status IN ('published', 'archived')
    AND NOT EXISTS (SELECT 1 FROM game.config_versions x WHERE x.config_version_id = v.challenge_version_id);

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.config_items_live WITH (security_barrier = true) AS
  SELECT * FROM game.config_items WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.config_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.config_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.scheduled_publications_live WITH (security_barrier = true) AS
  SELECT * FROM game.scheduled_publications WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.render_jobs_live WITH (security_barrier = true) AS
  SELECT * FROM game.render_jobs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.media_assets_live WITH (security_barrier = true) AS
  SELECT * FROM game.media_assets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.media_usages_live WITH (security_barrier = true) AS
  SELECT * FROM game.media_usages WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.client_manifests_live WITH (security_barrier = true) AS
  SELECT * FROM game.client_manifests WHERE deleted_at IS NULL;
GRANT SELECT ON game.config_items_live, game.config_versions_live, game.scheduled_publications_live, game.render_jobs_live,
  game.media_assets_live, game.media_usages_live, game.client_manifests_live TO game_readonly;
