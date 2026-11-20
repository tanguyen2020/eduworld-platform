-- F070 T006/T007 — player profile, nickname, avatar, character catalog of module game (specs/070-player-profile-avatar:
-- data-model §1; DATABASE_DESIGN §2.8, §2.8a, §3.13; CHARACTER_SYSTEM §5; ADR-0028, ADR-0041, ADR-0043, ADR-0089).
-- Applied by `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: defaults, CHECKs, FKs, the case-insensitive nickname
-- index, RLS, append-only grants, platform-only writes of the catalogs, immutability of published versions, _live views.

-- BEGIN entddl
CREATE TABLE game.game_profiles (
  game_profile_id uuid NOT NULL,
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
  school_id uuid NOT NULL,
  nickname varchar(32),
  level bigint NOT NULL DEFAULT 1,
  xp_total bigint NOT NULL DEFAULT 0,
  curve_version bigint,
  title_code varchar(64),
  avatar_config jsonb,
  avatar_config_hash varchar(128),
  avatar_created_at timestamptz,
  last_area_id varchar(64),
  settings jsonb NOT NULL,
  preferred_language varchar(64),
  last_game_login_at timestamptz,
  PRIMARY KEY (game_profile_id)
);
CREATE UNIQUE INDEX game_profiles_student_uq ON game.game_profiles (tenant_id, student_id) WHERE deleted_at IS NULL;
CREATE INDEX game_profiles_tenant_school_idx ON game.game_profiles (tenant_id, school_id);

CREATE TABLE game.nickname_changes (
  nickname_change_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  old_nickname varchar(32),
  new_nickname varchar(32) NOT NULL,
  reason varchar(64) NOT NULL,
  changed_by uuid NOT NULL,
  changed_at timestamptz NOT NULL,
  PRIMARY KEY (nickname_change_id)
);
CREATE INDEX nickname_changes_student_idx ON game.nickname_changes (tenant_id, student_id, changed_at);

CREATE TABLE game.avatar_presets (
  avatar_preset_id uuid NOT NULL,
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
  slot smallint NOT NULL,
  name varchar(64),
  avatar_config jsonb NOT NULL,
  PRIMARY KEY (avatar_preset_id)
);
CREATE UNIQUE INDEX avatar_presets_slot_uq ON game.avatar_presets (student_id, slot) WHERE deleted_at IS NULL;

CREATE TABLE game.character_catalogs (
  catalog_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (catalog_id)
);
CREATE UNIQUE INDEX character_catalogs_code_uq ON game.character_catalogs (code) WHERE deleted_at IS NULL;

CREATE TABLE game.character_catalog_versions (
  catalog_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  catalog_id uuid NOT NULL,
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL,
  payload jsonb NOT NULL,
  payload_sha256 varchar(64) NOT NULL,
  change_note varchar(500),
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  published_at timestamptz,
  PRIMARY KEY (catalog_version_id)
);
CREATE UNIQUE INDEX character_catalog_versions_no_uq ON game.character_catalog_versions (catalog_id, version_no);

CREATE TABLE game.item_definitions (
  item_id varchar(64) NOT NULL,
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
  PRIMARY KEY (item_id)
);

CREATE TABLE game.item_definition_versions (
  item_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  item_id varchar(64) NOT NULL,
  version_no bigint NOT NULL,
  kind varchar(64) NOT NULL,
  equip_slot varchar(64) NOT NULL,
  rarity varchar(64) NOT NULL,
  name jsonb NOT NULL,
  sources jsonb NOT NULL,
  asset_refs jsonb,
  status varchar(64) NOT NULL,
  PRIMARY KEY (item_version_id)
);
CREATE UNIQUE INDEX item_definition_versions_no_uq ON game.item_definition_versions (item_id, version_no);

CREATE TABLE game.inventory_items (
  inventory_item_id uuid NOT NULL,
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
  item_id varchar(64) NOT NULL,
  school_id uuid,
  quantity bigint NOT NULL DEFAULT 1,
  acquired_at timestamptz NOT NULL,
  reward_grant_id uuid,
  PRIMARY KEY (inventory_item_id)
);
CREATE UNIQUE INDEX inventory_items_student_item_uq ON game.inventory_items (student_id, item_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- game.game_profiles (T): one profile per student, nickname unique in the tenant ----------
-- Nickname unique per tenant, case-insensitive, among live profiles (CHARACTER_SYSTEM §5.3, F070 Clarifications): a
-- soft-deleted (cleaned up) profile frees its nickname; a restore that meets a taken nickname clears it (setup again).
ALTER TABLE game.game_profiles ALTER COLUMN settings SET DEFAULT '{}'::jsonb;
ALTER TABLE game.game_profiles
  ADD CONSTRAINT game_profiles_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT game_profiles_level_chk CHECK (level >= 1 AND xp_total >= 0),
  ADD CONSTRAINT game_profiles_nickname_chk CHECK (nickname IS NULL OR char_length(btrim(nickname)) BETWEEN 1 AND 32),
  ADD CONSTRAINT game_profiles_avatar_size_chk CHECK (avatar_config IS NULL OR octet_length(avatar_config::text) <= 16384),
  ADD CONSTRAINT game_profiles_avatar_hash_chk CHECK ((avatar_config IS NULL) = (avatar_config_hash IS NULL)),
  ADD CONSTRAINT game_profiles_settings_size_chk CHECK (octet_length(settings::text) <= 8192),
  ADD CONSTRAINT game_profiles_language_chk CHECK (preferred_language IS NULL OR preferred_language ~ '^[a-z]{2}-[A-Z]{2}$');
CREATE UNIQUE INDEX game_profiles_nickname_uq ON game.game_profiles (tenant_id, lower(nickname))
  WHERE deleted_at IS NULL AND nickname IS NOT NULL;

ALTER TABLE game.game_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.game_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.game_profiles FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.game_profiles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.game_profiles FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.game_profiles FROM game_app, game_platform;

-- ---------- game.nickname_changes (TA): append-only nickname history ----------
ALTER TABLE game.nickname_changes
  ADD CONSTRAINT nickname_changes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT nickname_changes_reason_chk CHECK (reason IN ('initial','player_change','reset'));
ALTER TABLE game.nickname_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.nickname_changes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.nickname_changes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.nickname_changes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.nickname_changes FROM game_app, game_platform;

-- ---------- game.avatar_presets (T): saved outfits, slot 1..saved_outfits_max (3) ----------
ALTER TABLE game.avatar_presets
  ADD CONSTRAINT avatar_presets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT avatar_presets_slot_chk CHECK (slot BETWEEN 1 AND 3),
  ADD CONSTRAINT avatar_presets_avatar_size_chk CHECK (octet_length(avatar_config::text) <= 16384);
ALTER TABLE game.avatar_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.avatar_presets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.avatar_presets FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.avatar_presets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.avatar_presets FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.avatar_presets FROM game_app, game_platform;

-- ---------- game.inventory_items (T): owned items (written by F075; F070 only reads them) ----------
ALTER TABLE game.inventory_items
  ADD CONSTRAINT inventory_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT inventory_items_item_fk FOREIGN KEY (item_id) REFERENCES game.item_definitions (item_id),
  ADD CONSTRAINT inventory_items_quantity_chk CHECK (quantity >= 0);
ALTER TABLE game.inventory_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.inventory_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.inventory_items FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.inventory_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.inventory_items FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.inventory_items FROM game_app, game_platform;

-- ---------- platform content (G): character catalog and item definitions (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed, Game Operations Console F078); the use cases of students
-- (game_app) only read them. Published versions are immutable (R11, R31): only status, published_at and the audit
-- columns may change.
ALTER TABLE game.character_catalog_versions
  ADD CONSTRAINT character_catalog_versions_catalog_fk FOREIGN KEY (catalog_id) REFERENCES game.character_catalogs (catalog_id),
  ADD CONSTRAINT character_catalog_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT character_catalog_versions_status_chk CHECK (status IN ('draft','published','archived')),
  ADD CONSTRAINT character_catalog_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT character_catalog_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$');
ALTER TABLE game.character_catalogs
  ADD CONSTRAINT character_catalogs_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.character_catalog_versions (catalog_version_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE game.item_definition_versions
  ADD CONSTRAINT item_definition_versions_item_fk FOREIGN KEY (item_id) REFERENCES game.item_definitions (item_id),
  ADD CONSTRAINT item_definition_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT item_definition_versions_kind_chk CHECK (kind IN ('hair','outfit','accessory','emote','pet','title','frame',
    'badge','consumable','collectible','decoration')),
  ADD CONSTRAINT item_definition_versions_slot_chk CHECK (equip_slot IN ('hair','outfit','head','face','back','hand','emote','pet',
    'title','frame','badge','none')),
  ADD CONSTRAINT item_definition_versions_status_chk CHECK (status IN ('draft','published','archived')),
  ADD CONSTRAINT item_definition_versions_name_chk CHECK (octet_length(name::text) <= 1024),
  ADD CONSTRAINT item_definition_versions_sources_chk CHECK (jsonb_typeof(sources) = 'array' AND jsonb_array_length(sources) <= 20),
  ADD CONSTRAINT item_definition_versions_assets_chk CHECK (asset_refs IS NULL OR octet_length(asset_refs::text) <= 8192);
ALTER TABLE game.item_definitions
  ADD CONSTRAINT item_definitions_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.item_definition_versions (item_version_id) DEFERRABLE INITIALLY DEFERRED;

REVOKE INSERT, UPDATE ON game.character_catalogs, game.character_catalog_versions, game.item_definitions,
  game.item_definition_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.character_catalogs, game.character_catalog_versions, game.item_definitions,
  game.item_definition_versions FROM game_app, game_platform;

CREATE FUNCTION game.forbid_published_version_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  mutable text[] := ARRAY['status', 'published_at', 'updated_at', 'updated_by', 'version', 'deleted_at', 'deleted_by',
    'delete_reason'];
BEGIN
  IF (to_jsonb(NEW) - mutable) <> (to_jsonb(OLD) - mutable) THEN
    RAISE EXCEPTION 'CONTENT_VERSION_IMMUTABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER character_catalog_versions_immutable BEFORE UPDATE ON game.character_catalog_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
CREATE TRIGGER item_definition_versions_immutable BEFORE UPDATE ON game.item_definition_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.game_profiles_live WITH (security_barrier = true) AS
  SELECT * FROM game.game_profiles WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.avatar_presets_live WITH (security_barrier = true) AS
  SELECT * FROM game.avatar_presets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.inventory_items_live WITH (security_barrier = true) AS
  SELECT * FROM game.inventory_items WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.character_catalogs_live WITH (security_barrier = true) AS
  SELECT * FROM game.character_catalogs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.character_catalog_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.character_catalog_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.item_definitions_live WITH (security_barrier = true) AS
  SELECT * FROM game.item_definitions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.item_definition_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.item_definition_versions WHERE deleted_at IS NULL;
GRANT SELECT ON game.game_profiles_live, game.avatar_presets_live, game.inventory_items_live, game.character_catalogs_live,
  game.character_catalog_versions_live, game.item_definitions_live, game.item_definition_versions_live TO game_readonly;
