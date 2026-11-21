-- F071 T007/T008 — world of module game: the versioned world content (areas, story chapters, NPC dialogues, level
-- packages), the release history, level sessions and the world progress of a student (specs/071-world-areas-npc-story:
-- data-model §1; DATABASE_DESIGN §2.8, §2.8a, §2.17, §3.13; WORLD_DESIGN §2–§8; ADR-0028, ADR-0041, ADR-0043, ADR-0089).
-- Applied by `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, the pointer trigger (a current version is a
-- published version of the same item), immutability of published versions, RLS of the tenant tables, platform-only
-- writes of the content, append-only release history, the projection ref.mastery_summary and the _live views.

-- BEGIN entddl
CREATE TABLE game.world_areas (
  area_id varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (area_id)
);

CREATE TABLE game.world_area_versions (
  area_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  area_id varchar(64) NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  change_note varchar(500),
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  published_at timestamptz,
  published_by uuid,
  region_code varchar(64) NOT NULL,
  area_class varchar(64) NOT NULL,
  subject_code varchar(64),
  PRIMARY KEY (area_version_id)
);
CREATE UNIQUE INDEX world_area_versions_no_uq ON game.world_area_versions (area_id, version_no);

CREATE TABLE game.story_chapters (
  chapter_code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (chapter_code)
);

CREATE TABLE game.story_chapter_versions (
  chapter_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  chapter_code varchar(64) NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  change_note varchar(500),
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  published_at timestamptz,
  published_by uuid,
  region_code varchar(64) NOT NULL,
  order_no bigint NOT NULL,
  subject_code varchar(64),
  PRIMARY KEY (chapter_version_id)
);
CREATE UNIQUE INDEX story_chapter_versions_no_uq ON game.story_chapter_versions (chapter_code, version_no);

CREATE TABLE game.npc_dialogues (
  dialogue_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  npc_id varchar(64) NOT NULL,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (dialogue_id)
);
CREATE INDEX npc_dialogues_npc_idx ON game.npc_dialogues (npc_id);

CREATE TABLE game.npc_dialogue_versions (
  dialogue_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  dialogue_id uuid NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  change_note varchar(500),
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  published_at timestamptz,
  published_by uuid,
  npc_id varchar(64) NOT NULL,
  grade_levels jsonb NOT NULL,
  PRIMARY KEY (dialogue_version_id)
);
CREATE UNIQUE INDEX npc_dialogue_versions_no_uq ON game.npc_dialogue_versions (dialogue_id, version_no);

CREATE TABLE game.level_definitions (
  level_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  area_id varchar(64) NOT NULL,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (level_id)
);
CREATE UNIQUE INDEX level_definitions_area_uq ON game.level_definitions (area_id) WHERE deleted_at IS NULL;

CREATE TABLE game.level_versions (
  level_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  level_id uuid NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  change_note varchar(500),
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  published_at timestamptz,
  published_by uuid,
  bundle_id varchar(64) NOT NULL,
  bundle_url varchar(2048) NOT NULL,
  bundle_sha256 varchar(64) NOT NULL,
  size_bytes bigint NOT NULL,
  PRIMARY KEY (level_version_id)
);
CREATE UNIQUE INDEX level_versions_no_uq ON game.level_versions (level_id, version_no);

CREATE TABLE game.content_releases (
  release_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  item_type varchar(64) NOT NULL,
  item_id varchar(64) NOT NULL,
  action varchar(16) NOT NULL,
  from_version_id uuid,
  to_version_id uuid,
  reason varchar(500),
  released_by uuid NOT NULL,
  released_at timestamptz NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (release_id)
);
CREATE UNIQUE INDEX content_releases_idem_uq ON game.content_releases (item_type, item_id, idempotency_key);

CREATE TABLE game.level_sessions (
  level_session_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  student_id uuid NOT NULL,
  game_session_id uuid NOT NULL,
  level_id uuid NOT NULL,
  level_version_id uuid NOT NULL,
  level_version_no bigint NOT NULL,
  area_id varchar(64) NOT NULL,
  manifest_version bigint NOT NULL,
  content_versions jsonb NOT NULL,
  status varchar(32) NOT NULL,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  PRIMARY KEY (level_session_id)
);
CREATE UNIQUE INDEX level_sessions_one_active_uq ON game.level_sessions (tenant_id, student_id) WHERE status = 'active' AND deleted_at IS NULL;

CREATE TABLE game.world_area_unlocks (
  area_unlock_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  student_id uuid NOT NULL,
  area_id varchar(64) NOT NULL,
  rule_version bigint NOT NULL,
  unlocked_by varchar(64) NOT NULL,
  unlocked_at timestamptz NOT NULL,
  PRIMARY KEY (area_unlock_id)
);
CREATE UNIQUE INDEX world_area_unlocks_uq ON game.world_area_unlocks (tenant_id, student_id, area_id) WHERE deleted_at IS NULL;

CREATE TABLE game.exploration_progress (
  exploration_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  student_id uuid NOT NULL,
  area_id varchar(64) NOT NULL,
  discoveries jsonb NOT NULL,
  collectibles jsonb NOT NULL,
  completion_pct numeric(5,2) NOT NULL DEFAULT 0,
  completed_at timestamptz,
  PRIMARY KEY (exploration_id)
);
CREATE UNIQUE INDEX exploration_progress_uq ON game.exploration_progress (tenant_id, student_id, area_id) WHERE deleted_at IS NULL;

CREATE TABLE game.story_progress (
  story_progress_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  student_id uuid NOT NULL,
  chapter_code varchar(64) NOT NULL,
  chapter_version_id uuid NOT NULL,
  status varchar(32) NOT NULL,
  main_quests_completed bigint NOT NULL DEFAULT 0,
  completed_quest_ids jsonb NOT NULL,
  completed_at timestamptz,
  PRIMARY KEY (story_progress_id)
);
CREATE UNIQUE INDEX story_progress_uq ON game.story_progress (tenant_id, student_id, chapter_code) WHERE deleted_at IS NULL;

CREATE TABLE game.world_unlock_facts (
  fact_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  student_id uuid NOT NULL,
  fact_kind varchar(64) NOT NULL,
  fact_ref varchar(64) NOT NULL,
  fact_value bigint NOT NULL DEFAULT 0,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (fact_id)
);
CREATE UNIQUE INDEX world_unlock_facts_uq ON game.world_unlock_facts (tenant_id, student_id, fact_kind, fact_ref) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- platform content (G): world areas, story chapters, NPC dialogues, level packages (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- student use cases (game_app) only read them. Published versions are immutable (R11, R31).
ALTER TABLE game.world_area_versions
  ADD CONSTRAINT world_area_versions_area_fk FOREIGN KEY (area_id) REFERENCES game.world_areas (area_id),
  ADD CONSTRAINT world_area_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT world_area_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT world_area_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT world_area_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT world_area_versions_class_chk CHECK (area_class IN ('hub','learning','practice','social','quest','exam',
    'challenge','arena')),
  ADD CONSTRAINT world_area_versions_subject_chk CHECK (area_class <> 'learning' OR subject_code IS NOT NULL),
  ADD CONSTRAINT world_area_versions_unlock_size_chk CHECK (octet_length(coalesce(payload->'unlock_rule', 'null'::jsonb)::text) <= 8192);
ALTER TABLE game.world_areas
  ADD CONSTRAINT world_areas_id_chk CHECK (area_id ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)?$'),
  ADD CONSTRAINT world_areas_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.world_area_versions (area_version_id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE game.story_chapter_versions
  ADD CONSTRAINT story_chapter_versions_chapter_fk FOREIGN KEY (chapter_code) REFERENCES game.story_chapters (chapter_code),
  ADD CONSTRAINT story_chapter_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT story_chapter_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT story_chapter_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT story_chapter_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT story_chapter_versions_order_chk CHECK (order_no BETWEEN 0 AND 50),
  ADD CONSTRAINT story_chapter_versions_quests_chk CHECK (jsonb_typeof(payload->'main_quest_ids') = 'array'
    AND jsonb_array_length(payload->'main_quest_ids') <= 50);
ALTER TABLE game.story_chapters
  ADD CONSTRAINT story_chapters_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.story_chapter_versions (chapter_version_id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE game.npc_dialogue_versions
  ADD CONSTRAINT npc_dialogue_versions_dialogue_fk FOREIGN KEY (dialogue_id) REFERENCES game.npc_dialogues (dialogue_id),
  ADD CONSTRAINT npc_dialogue_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT npc_dialogue_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT npc_dialogue_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT npc_dialogue_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT npc_dialogue_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9),
  ADD CONSTRAINT npc_dialogue_versions_npc_chk CHECK (npc_id ~ '^npc\.[a-z0-9_]+$');
ALTER TABLE game.npc_dialogues
  ADD CONSTRAINT npc_dialogues_npc_chk CHECK (npc_id ~ '^npc\.[a-z0-9_]+$'),
  ADD CONSTRAINT npc_dialogues_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.npc_dialogue_versions (dialogue_version_id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE game.level_versions
  ADD CONSTRAINT level_versions_level_fk FOREIGN KEY (level_id) REFERENCES game.level_definitions (level_id),
  ADD CONSTRAINT level_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT level_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT level_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT level_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$' AND bundle_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT level_versions_size_chk CHECK (size_bytes >= 0),
  ADD CONSTRAINT level_versions_platforms_chk CHECK (jsonb_typeof(payload->'platforms') = 'array'
    AND jsonb_array_length(payload->'platforms') BETWEEN 1 AND 8);
ALTER TABLE game.level_definitions
  ADD CONSTRAINT level_definitions_area_fk FOREIGN KEY (area_id) REFERENCES game.world_areas (area_id),
  ADD CONSTRAINT level_definitions_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.level_versions (level_version_id) DEFERRABLE INITIALLY DEFERRED;

-- A release pointer only points at a published version of its own item (data-model §3, DDL §3.13). Arguments: version
-- table, version key column, item key column of both tables.
CREATE FUNCTION game.check_current_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  ok boolean;
  item text := to_jsonb(NEW) ->> TG_ARGV[2];
BEGIN
  IF NEW.current_version_id IS NULL THEN
    RETURN NEW;
  END IF;
  EXECUTE format('SELECT EXISTS (SELECT 1 FROM game.%I WHERE %I = $1 AND %I::text = $2 AND status = ''published'' AND deleted_at IS NULL)',
    TG_ARGV[0], TG_ARGV[1], TG_ARGV[2]) INTO ok USING NEW.current_version_id, item;
  IF NOT ok THEN
    RAISE EXCEPTION 'CONTENT_POINTER_NOT_PUBLISHED' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE CONSTRAINT TRIGGER world_areas_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.world_areas
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('world_area_versions', 'area_version_id', 'area_id');
CREATE CONSTRAINT TRIGGER story_chapters_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.story_chapters
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('story_chapter_versions', 'chapter_version_id', 'chapter_code');
CREATE CONSTRAINT TRIGGER npc_dialogues_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.npc_dialogues
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('npc_dialogue_versions', 'dialogue_version_id', 'dialogue_id');
CREATE CONSTRAINT TRIGGER level_definitions_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.level_definitions
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('level_versions', 'level_version_id', 'level_id');

CREATE TRIGGER world_area_versions_immutable BEFORE UPDATE ON game.world_area_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
CREATE TRIGGER story_chapter_versions_immutable BEFORE UPDATE ON game.story_chapter_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
CREATE TRIGGER npc_dialogue_versions_immutable BEFORE UPDATE ON game.npc_dialogue_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
CREATE TRIGGER level_versions_immutable BEFORE UPDATE ON game.level_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

REVOKE INSERT, UPDATE ON game.world_areas, game.world_area_versions, game.story_chapters, game.story_chapter_versions,
  game.npc_dialogues, game.npc_dialogue_versions, game.level_definitions, game.level_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.world_areas, game.world_area_versions, game.story_chapters, game.story_chapter_versions,
  game.npc_dialogues, game.npc_dialogue_versions, game.level_definitions, game.level_versions FROM game_app, game_platform;

-- ---------- game.content_releases (GA): append-only history of the release pointers ----------
ALTER TABLE game.content_releases
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config')),
  ADD CONSTRAINT content_releases_action_chk CHECK (action IN ('publish','rollback','unpublish')),
  ADD CONSTRAINT content_releases_to_chk CHECK (action = 'unpublish' OR to_version_id IS NOT NULL);
CREATE INDEX content_releases_item_idx ON game.content_releases (item_type, item_id, released_at);
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON game.content_releases FROM game_app;
REVOKE UPDATE, DELETE, TRUNCATE ON game.content_releases FROM game_platform;

-- ---------- tenant tables (T): level sessions and world progress of a student ----------
ALTER TABLE game.level_sessions
  ADD CONSTRAINT level_sessions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT level_sessions_status_chk CHECK (status IN ('active','completed','abandoned','expired')),
  ADD CONSTRAINT level_sessions_end_chk CHECK ((status = 'active') = (ended_at IS NULL)),
  ADD CONSTRAINT level_sessions_pins_chk CHECK (jsonb_typeof(content_versions) = 'array'
    AND octet_length(content_versions::text) <= 65536);
ALTER TABLE game.world_area_unlocks
  ADD CONSTRAINT world_area_unlocks_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT world_area_unlocks_area_fk FOREIGN KEY (area_id) REFERENCES game.world_areas (area_id),
  ADD CONSTRAINT world_area_unlocks_rule_chk CHECK (rule_version >= 1),
  ADD CONSTRAINT world_area_unlocks_by_chk CHECK (unlocked_by IN ('level','quest','learning_objective','achievement','event',
    'chapters_completed'));
ALTER TABLE game.exploration_progress
  ADD CONSTRAINT exploration_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT exploration_progress_area_fk FOREIGN KEY (area_id) REFERENCES game.world_areas (area_id),
  ADD CONSTRAINT exploration_progress_pct_chk CHECK (completion_pct BETWEEN 0 AND 100),
  ADD CONSTRAINT exploration_progress_found_chk CHECK (jsonb_typeof(discoveries) = 'array' AND jsonb_array_length(discoveries) <= 500
    AND jsonb_typeof(collectibles) = 'array' AND jsonb_array_length(collectibles) <= 500);
ALTER TABLE game.story_progress
  ADD CONSTRAINT story_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT story_progress_chapter_fk FOREIGN KEY (chapter_code) REFERENCES game.story_chapters (chapter_code),
  ADD CONSTRAINT story_progress_status_chk CHECK (status IN ('available','in_progress','completed')),
  ADD CONSTRAINT story_progress_done_chk CHECK ((status = 'completed') = (completed_at IS NOT NULL)),
  ADD CONSTRAINT story_progress_quests_chk CHECK (main_quests_completed >= 0 AND jsonb_typeof(completed_quest_ids) = 'array'
    AND jsonb_array_length(completed_quest_ids) <= 50);
ALTER TABLE game.world_unlock_facts
  ADD CONSTRAINT world_unlock_facts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT world_unlock_facts_kind_chk CHECK (fact_kind IN ('quest_completed','achievement_unlocked','event_participation',
    'level'));

ALTER TABLE game.level_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.level_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.level_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.level_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.level_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.level_sessions FROM game_app, game_platform;
ALTER TABLE game.world_area_unlocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.world_area_unlocks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.world_area_unlocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.world_area_unlocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.world_area_unlocks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.world_area_unlocks FROM game_app, game_platform;
ALTER TABLE game.exploration_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.exploration_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.exploration_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.exploration_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.exploration_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.exploration_progress FROM game_app, game_platform;
ALTER TABLE game.story_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.story_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.story_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.story_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.story_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.story_progress FROM game_app, game_platform;
ALTER TABLE game.world_unlock_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.world_unlock_facts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.world_unlock_facts FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.world_unlock_facts FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.world_unlock_facts FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.world_unlock_facts FROM game_app, game_platform;

-- ---------- ref.mastery_summary: projection of learning_objective.progressed (module learning, owner F031) ----------
-- Minimal columns (DATABASE_DESIGN §2.17), no entx mixin; written only by the consumer game.mastery_projection in the
-- tenant of the event, upserted by source_version (envelope occurred_at, rule 1). Read by the unlock rule
-- objective_progress (F071) and the quests (F072). RLS tenant condition only, like the other tenant projections.
CREATE TABLE ref.mastery_summary (
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  student_id     uuid NOT NULL,
  objective_id   uuid NOT NULL,
  level          varchar(64) NOT NULL CHECK (level IN ('not_started','beginning','developing','proficient','mastered')),
  value          numeric(5,4) NOT NULL CHECK (value BETWEEN 0 AND 1),
  source_version bigint NOT NULL,
  PRIMARY KEY (tenant_id, student_id, objective_id)
);
ALTER TABLE ref.mastery_summary ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.mastery_summary FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.mastery_summary FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.mastery_summary FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON ref.mastery_summary FROM game_app, game_platform;

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.world_areas_live WITH (security_barrier = true) AS
  SELECT * FROM game.world_areas WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.world_area_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.world_area_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.story_chapters_live WITH (security_barrier = true) AS
  SELECT * FROM game.story_chapters WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.story_chapter_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.story_chapter_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.npc_dialogues_live WITH (security_barrier = true) AS
  SELECT * FROM game.npc_dialogues WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.npc_dialogue_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.npc_dialogue_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.level_definitions_live WITH (security_barrier = true) AS
  SELECT * FROM game.level_definitions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.level_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.level_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.level_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM game.level_sessions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.world_area_unlocks_live WITH (security_barrier = true) AS
  SELECT * FROM game.world_area_unlocks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.exploration_progress_live WITH (security_barrier = true) AS
  SELECT * FROM game.exploration_progress WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.story_progress_live WITH (security_barrier = true) AS
  SELECT * FROM game.story_progress WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.world_unlock_facts_live WITH (security_barrier = true) AS
  SELECT * FROM game.world_unlock_facts WHERE deleted_at IS NULL;
GRANT SELECT ON game.world_areas_live, game.world_area_versions_live, game.story_chapters_live, game.story_chapter_versions_live,
  game.npc_dialogues_live, game.npc_dialogue_versions_live, game.level_definitions_live, game.level_versions_live,
  game.level_sessions_live, game.world_area_unlocks_live, game.exploration_progress_live, game.story_progress_live,
  game.world_unlock_facts_live TO game_readonly;
