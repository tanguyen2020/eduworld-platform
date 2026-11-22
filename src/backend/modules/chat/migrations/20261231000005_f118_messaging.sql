-- F118 T006/T007 — full messaging (edit/recall, reply/quote/forward, reactions, pins, mentions, receipts) and the platform
-- sticker/GIF library (specs/118-chat-messaging-features: data-model §1; DATABASE_DESIGN §2.12, §3.8, §3.8a; CHAT_SYSTEM §7–§8;
-- ADR-0028, ADR-0029, ADR-0033, ADR-0034, ADR-0035, ADR-0041, ADR-0043, ADR-0089). Applied by `migrate --module=chat` as
-- chat_migrator. Every table is hand-written (no ent entity, precedent F115/F117): the message side tables are written by
-- chat-persister only with explicit SQL (R25) and purged physically with the gameplay channel (ADR-0033); the asset tables
-- are written with explicit SQL by the Game Studio use cases under PlatformScope. Nothing is deleted by this migration (R12).

-- ---------- chat.message_revisions (T): every earlier version of an edited message + held edit proposals (evidence) ----------
-- state previous: the version replaced by an applied edit (revision = the revision that body had); state held: an edit
-- whose new content the pipeline held for review (revision = the proposed revision, never shown to other members).
-- Rows are never updated; only chat_purger deletes them with their closed gameplay channel (ADR-0033).
CREATE TABLE chat.message_revisions (
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  channel_id     uuid NOT NULL,
  message_id     uuid NOT NULL,
  revision       int NOT NULL CHECK (revision >= 1),
  state          varchar(64) NOT NULL DEFAULT 'previous' CHECK (state IN ('previous','held')),
  body           varchar(1000),
  body_display   varchar(1000),
  mentions       uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(mentions) <= 5),
  decision       varchar(64) NOT NULL CHECK (decision IN ('allow','unchecked','mask','hold_for_review','block_and_alert')),
  filter_verdict jsonb CHECK (filter_verdict IS NULL OR octet_length(filter_verdict::text) <= 4096),
  edited_at      timestamptz NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (message_id, revision, state)
);
CREATE INDEX message_revisions_channel_idx ON chat.message_revisions (channel_id);
CREATE TRIGGER message_revisions_append_only BEFORE UPDATE ON chat.message_revisions
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();

-- ---------- chat.message_reactions (T): removing a reaction is a soft delete (deleted_at), re-adding clears it ----------
CREATE TABLE chat.message_reactions (
  tenant_id  uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  channel_id uuid NOT NULL,
  message_id uuid NOT NULL,
  user_id    uuid NOT NULL,
  emoji_code varchar(64) NOT NULL CHECK (emoji_code ~ '^[a-z0-9_]{1,64}$'),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  PRIMARY KEY (message_id, user_id, emoji_code)
);
CREATE INDEX message_reactions_channel_idx ON chat.message_reactions (channel_id);
CREATE INDEX message_reactions_live_idx ON chat.message_reactions (message_id) WHERE deleted_at IS NULL;

-- ---------- chat.pinned_messages (T): unpinning is a soft delete; at most 3 live pins per channel (chat_pins_max ceiling) ----------
CREATE TABLE chat.pinned_messages (
  tenant_id  uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  channel_id uuid NOT NULL,
  message_id uuid NOT NULL,
  pinned_by  uuid NOT NULL,
  pinned_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (channel_id, message_id)
);
CREATE INDEX pinned_messages_live_idx ON chat.pinned_messages (channel_id) WHERE deleted_at IS NULL;
-- Database ceiling of chat_pins_max (CONFIG_DEFAULTS 3 [Confirmed]); the use case enforces the effective value first.
CREATE FUNCTION chat.f_pins_max() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.deleted_at IS NULL AND (TG_OP = 'INSERT' OR OLD.deleted_at IS NOT NULL) AND (SELECT count(*) FROM chat.pinned_messages p
      WHERE p.channel_id = NEW.channel_id AND p.deleted_at IS NULL AND p.message_id <> NEW.message_id) >= 3 THEN
    RAISE EXCEPTION 'chat.pinned_messages: at most 3 live pins per channel' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER pinned_messages_max BEFORE INSERT OR UPDATE ON chat.pinned_messages
  FOR EACH ROW EXECUTE FUNCTION chat.f_pins_max();

-- ---------- ref.sticker_unlocks (projection, T): locked sticker packs a student owns (event of module game, F075) ----------
CREATE TABLE ref.sticker_unlocks (
  student_id     uuid NOT NULL,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  pack_id        uuid NOT NULL,
  unlocked_at    timestamptz NOT NULL,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, pack_id)
);
CREATE INDEX ref_sticker_unlocks_tenant_idx ON ref.sticker_unlocks (tenant_id, student_id);

-- ---------- Platform sticker/GIF library (G, DATABASE_DESIGN §3.8a): Game Studio only (ADR-0027, AGENTS R20) ----------
-- G tables: no tenant_id column at all (the §3.8a sketch had one always NULL); a tenant sees a pack through
-- infra.tenant_resource_assignments.
CREATE TABLE chat.sticker_packs (
  pack_id            uuid PRIMARY KEY,
  owner_scope        varchar(64) NOT NULL DEFAULT 'platform' CHECK (owner_scope = 'platform'),
  code               varchar(64) NOT NULL CHECK (code ~ '^[a-z0-9][a-z0-9_-]{0,63}$'),
  name_vi            varchar(128) NOT NULL,
  name_en            varchar(128),
  unlock_rule        jsonb CHECK (unlock_rule IS NULL OR (jsonb_typeof(unlock_rule) = 'object' AND octet_length(unlock_rule::text) <= 1024)),
  unlock_hint_vi     varchar(1000),
  unlock_hint_en     varchar(1000),
  status             varchar(64) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','in_review','published','archived')),
  current_version_id uuid,
  created_by         uuid NOT NULL,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  deleted_by         uuid
);
CREATE UNIQUE INDEX sticker_packs_code_uq ON chat.sticker_packs (code) WHERE deleted_at IS NULL;

-- Stickers of a published version are never changed (R31): a new version adds new rows; asset_key media/<sha256>/… is immutable.
CREATE TABLE chat.stickers (
  sticker_id   uuid PRIMARY KEY,
  pack_id      uuid NOT NULL REFERENCES chat.sticker_packs (pack_id),
  asset_key    varchar(512) NOT NULL CHECK (asset_key ~ '^media/[0-9a-f]{64}/'),
  content_type varchar(64) NOT NULL CHECK (content_type IN ('image/webp','image/png')),
  size_bytes   int NOT NULL CHECK (size_bytes BETWEEN 1 AND 524288),
  alt_text_vi  varchar(1000) NOT NULL,
  alt_text_en  varchar(1000),
  sort_order   int NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  deleted_by   uuid
);
CREATE INDEX stickers_pack_idx ON chat.stickers (pack_id) WHERE deleted_at IS NULL;

CREATE TABLE chat.gif_assets (
  gif_id             uuid PRIMARY KEY,
  owner_scope        varchar(64) NOT NULL DEFAULT 'platform' CHECK (owner_scope = 'platform'),
  -- The file columns hold the published version; they stay NULL until the first publish.
  asset_key          varchar(512) CHECK (asset_key ~ '^media/[0-9a-f]{64}/'),
  preview_key        varchar(512) CHECK (preview_key ~ '^media/[0-9a-f]{64}/'),
  size_bytes         int CHECK (size_bytes BETWEEN 1 AND 2097152),
  width              int CHECK (width BETWEEN 1 AND 480),
  height             int CHECK (height BETWEEN 1 AND 480),
  status             varchar(64) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','in_review','published','archived')),
  current_version_id uuid,
  created_by         uuid NOT NULL,
  reviewed_by        uuid,
  reviewed_at        timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  deleted_by         uuid,
  CHECK (reviewed_by IS NULL OR reviewed_by <> created_by),
  CHECK (status <> 'published' OR (asset_key IS NOT NULL AND preview_key IS NOT NULL AND size_bytes IS NOT NULL AND width IS NOT NULL
    AND height IS NOT NULL))
);
CREATE INDEX gif_assets_status_idx ON chat.gif_assets (status) WHERE deleted_at IS NULL;
CREATE TABLE chat.gif_tags (
  gif_id uuid NOT NULL REFERENCES chat.gif_assets (gif_id),
  tag    varchar(64) NOT NULL CHECK (tag ~ '^[a-z0-9 _-]{1,64}$'),
  locale varchar(64) NOT NULL DEFAULT 'vi' CHECK (locale IN ('vi','en')),
  deleted_at timestamptz, -- a tag dropped by a newer published version (soft delete, R21)
  PRIMARY KEY (gif_id, tag, locale)
);
CREATE INDEX gif_tags_tag_idx ON chat.gif_tags (tag) WHERE deleted_at IS NULL;

-- ---------- chat.chat_asset_versions (G): the versions of a pack/GIF; immutable once published (R31, ADR-0043) ----------
CREATE TABLE chat.chat_asset_versions (
  version_id   uuid PRIMARY KEY,
  config_type  varchar(64) NOT NULL CHECK (config_type IN ('sticker-packs','gifs')),
  config_id    uuid NOT NULL,
  version      int NOT NULL CHECK (version >= 1),
  status       varchar(64) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','in_review','approved','rejected','published','archived')),
  payload      jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 65536),
  change_note  varchar(1000),
  created_by   uuid NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  reviewed_by  uuid,
  reviewed_at  timestamptz,
  published_by uuid,
  published_at timestamptz,
  UNIQUE (config_type, config_id, version),
  CHECK (config_type <> 'gifs' OR reviewed_by IS NULL OR reviewed_by <> created_by)
);
CREATE INDEX chat_asset_versions_item_idx ON chat.chat_asset_versions (config_type, config_id, version DESC);
-- A published or archived version is never changed again, except published → archived (status and updated_at only).
CREATE FUNCTION chat.f_asset_version_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('published','archived') AND NOT (OLD.status = 'published' AND NEW.status = 'archived'
      AND NEW.payload = OLD.payload AND NEW.version = OLD.version AND NEW.config_id = OLD.config_id
      AND NEW.published_at IS NOT DISTINCT FROM OLD.published_at) THEN
    RAISE EXCEPTION 'chat.chat_asset_versions: version % is %, immutable', OLD.version, OLD.status USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER chat_asset_versions_immutable BEFORE UPDATE ON chat.chat_asset_versions
  FOR EACH ROW EXECUTE FUNCTION chat.f_asset_version_immutable();
CREATE TRIGGER chat_asset_versions_no_delete BEFORE DELETE ON chat.chat_asset_versions
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();

-- Releases of a pack/GIF (publish, rollback, unpublish): the history of current_version_id (append-only).
CREATE TABLE chat.chat_asset_releases (
  release_id      uuid PRIMARY KEY,
  config_type     varchar(64) NOT NULL CHECK (config_type IN ('sticker-packs','gifs')),
  config_id       uuid NOT NULL,
  action          varchar(64) NOT NULL CHECK (action IN ('publish','rollback','unpublish')),
  from_version_id uuid,
  to_version_id   uuid,
  reason          varchar(1000),
  actor_id        uuid NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  CHECK ((action = 'unpublish') = (to_version_id IS NULL)),
  CHECK (action = 'publish' OR reason IS NOT NULL)
);
CREATE INDEX chat_asset_releases_item_idx ON chat.chat_asset_releases (config_type, config_id, created_at DESC);
CREATE TRIGGER chat_asset_releases_append_only BEFORE UPDATE OR DELETE ON chat.chat_asset_releases
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();

-- ---------- infra.tenant_resource_assignments of eduworld_chat (ADR-0029): sticker packs a tenant sees ----------
-- Same DDL as the other modules (learning F020). Written by the System Admin assignment step (F045 saga, chat_platform);
-- chat_app only reads it. No RLS (schema infra, ADR-0089 §4).
CREATE TABLE infra.tenant_resource_assignments (
  assignment_id uuid NOT NULL,
  tenant_id     uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  created_by    uuid,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  updated_by    uuid,
  deleted_at    timestamptz,
  deleted_by    uuid,
  delete_reason varchar(64),
  version       bigint NOT NULL DEFAULT 1,
  resource_type varchar(64) NOT NULL CHECK (resource_type IN ('sticker_pack')),
  resource_id   uuid NOT NULL,
  assigned_by   uuid NOT NULL,
  assigned_at   timestamptz NOT NULL,
  PRIMARY KEY (assignment_id)
);
CREATE UNIQUE INDEX tra_active_uq ON infra.tenant_resource_assignments (resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX tra_tenant_idx ON infra.tenant_resource_assignments (tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL;

-- ---------- RLS (FORCE) on the tenant tables: tenant condition only (ADR-0089 §1) ----------
ALTER TABLE chat.message_revisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.message_revisions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.message_revisions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.message_revisions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.message_reactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.message_reactions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.message_reactions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.message_reactions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.message_reactions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.pinned_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.pinned_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.pinned_messages FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.pinned_messages FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.pinned_messages FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.sticker_unlocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE ref.sticker_unlocks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.sticker_unlocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.sticker_unlocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.sticker_unlocks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Grants: soft-delete only for the application roles (R21); the library is written by chat_platform ----------
GRANT SELECT, INSERT ON chat.message_revisions TO chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON chat.message_reactions, chat.pinned_messages, ref.sticker_unlocks TO chat_app, chat_platform;
REVOKE UPDATE, DELETE, TRUNCATE ON chat.message_revisions FROM chat_app, chat_platform;
REVOKE DELETE, TRUNCATE ON chat.message_reactions, chat.pinned_messages, ref.sticker_unlocks FROM chat_app, chat_platform;
REVOKE ALL ON chat.sticker_packs, chat.stickers, chat.gif_assets, chat.gif_tags, chat.chat_asset_versions,
  chat.chat_asset_releases, infra.tenant_resource_assignments FROM chat_app, chat_platform;
GRANT SELECT ON chat.sticker_packs, chat.stickers, chat.gif_assets, chat.gif_tags, chat.chat_asset_versions,
  chat.chat_asset_releases, infra.tenant_resource_assignments TO chat_app;
GRANT SELECT, INSERT, UPDATE ON chat.sticker_packs, chat.stickers, chat.gif_assets, chat.gif_tags, chat.chat_asset_versions,
  infra.tenant_resource_assignments TO chat_platform;
GRANT SELECT, INSERT ON chat.chat_asset_releases TO chat_platform;

-- ---------- _live views for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW chat.message_reactions_live WITH (security_barrier = true) AS
  SELECT * FROM chat.message_reactions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.pinned_messages_live WITH (security_barrier = true) AS
  SELECT * FROM chat.pinned_messages WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.sticker_packs_live WITH (security_barrier = true) AS
  SELECT * FROM chat.sticker_packs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.gif_assets_live WITH (security_barrier = true) AS
  SELECT * FROM chat.gif_assets WHERE deleted_at IS NULL;
GRANT SELECT ON chat.message_reactions_live, chat.pinned_messages_live, chat.sticker_packs_live, chat.gif_assets_live TO chat_readonly;

-- ---------- chat_purger (ADR-0033, F117 left these tables to F118): revisions, reactions, pins of a CLOSED gameplay channel ----------
-- The job copies the evidence (messages + revisions) first, then deletes these rows before the messages of the channel.
GRANT SELECT, DELETE ON chat.message_revisions, chat.message_reactions, chat.pinned_messages TO chat_purger;
REVOKE INSERT, UPDATE, TRUNCATE ON chat.message_revisions, chat.message_reactions, chat.pinned_messages FROM chat_purger;
CREATE POLICY purger_gameplay_only ON chat.message_revisions FOR ALL TO chat_purger
  USING (EXISTS (SELECT 1 FROM chat.channels c WHERE c.channel_id = message_revisions.channel_id AND c.is_gameplay
    AND c.status = 'closed' AND c.closed_at IS NOT NULL));
CREATE POLICY purger_gameplay_only ON chat.message_reactions FOR ALL TO chat_purger
  USING (EXISTS (SELECT 1 FROM chat.channels c WHERE c.channel_id = message_reactions.channel_id AND c.is_gameplay
    AND c.status = 'closed' AND c.closed_at IS NOT NULL));
CREATE POLICY purger_gameplay_only ON chat.pinned_messages FOR ALL TO chat_purger
  USING (EXISTS (SELECT 1 FROM chat.channels c WHERE c.channel_id = pinned_messages.channel_id AND c.is_gameplay
    AND c.status = 'closed' AND c.closed_at IS NOT NULL));
