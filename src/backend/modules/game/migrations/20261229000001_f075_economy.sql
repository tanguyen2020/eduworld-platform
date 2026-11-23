-- F075 T007/T008 — coin economy, cosmetic shop, inventory, collection, achievements and badges of module game
-- (specs/075-economy-shop-collection-achievement: data-model §1; DATABASE_DESIGN §2.8, §2.8a, §3.13; INVENTORY_ECONOMY
-- §2–§9; ADR-0028, ADR-0041, ADR-0043, ADR-0049, ADR-0089, ADR-0100). Applied by `migrate --module=game` as game_migrator.
-- Order: ent tables and the columns added to game.item_definition_versions (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, release pointers, immutability of published
-- versions, RLS of the tenant tables, append-only grants of the ledgers (F075-AC-05), platform-only writes of the
-- content, the currency xu, _live views. The ledgers are not partitioned yet (monthly partitions when large, like
-- xp_ledger — spec Clarifications), so the idempotency keys are unique on their own.

-- BEGIN entddl
CREATE TABLE gamification.currencies (
  currency_code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  name_vi varchar(128) NOT NULL,
  name_en varchar(128) NOT NULL,
  daily_cap bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (currency_code)
);

CREATE TABLE gamification.wallets (
  wallet_id uuid NOT NULL,
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
  currency_code varchar(64) NOT NULL,
  balance bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (wallet_id)
);
CREATE UNIQUE INDEX wallets_student_currency_uq ON gamification.wallets (tenant_id, student_id, currency_code) WHERE deleted_at IS NULL;

CREATE TABLE gamification.currency_ledger (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  currency_code varchar(64) NOT NULL,
  delta bigint NOT NULL,
  balance_after bigint NOT NULL,
  reason_code varchar(64) NOT NULL,
  source_type varchar(64) NOT NULL,
  source_id uuid NOT NULL,
  item_id varchar(64),
  capped boolean NOT NULL DEFAULT false,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (entry_id)
);
CREATE UNIQUE INDEX currency_ledger_idem_uq ON gamification.currency_ledger (idempotency_key);
CREATE INDEX currency_ledger_student_idx ON gamification.currency_ledger (tenant_id, student_id, created_at);

CREATE TABLE gamification.item_ledger (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  item_id varchar(64) NOT NULL,
  delta bigint NOT NULL,
  reason_code varchar(64) NOT NULL,
  source_type varchar(64) NOT NULL,
  source_id uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (entry_id)
);
CREATE UNIQUE INDEX item_ledger_idem_uq ON gamification.item_ledger (idempotency_key);
CREATE INDEX item_ledger_student_idx ON gamification.item_ledger (tenant_id, student_id, created_at);

CREATE TABLE gamification.shop_items (
  shop_item_id uuid NOT NULL,
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
  PRIMARY KEY (shop_item_id)
);

CREATE TABLE gamification.shop_item_versions (
  shop_item_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  shop_item_id uuid NOT NULL,
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
  item_id varchar(64) NOT NULL,
  item_kind varchar(64) NOT NULL,
  price_amount bigint NOT NULL,
  currency_code varchar(64) NOT NULL,
  purchase_limit bigint NOT NULL,
  available_from timestamptz,
  available_to timestamptz,
  grade_levels jsonb NOT NULL,
  featured_order bigint NOT NULL DEFAULT 0,
  event_id uuid,
  season_id uuid,
  PRIMARY KEY (shop_item_version_id)
);
CREATE UNIQUE INDEX shop_item_versions_no_uq ON gamification.shop_item_versions (shop_item_id, version_no);

CREATE TABLE gamification.shop_purchases (
  purchase_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  shop_item_id uuid NOT NULL,
  shop_item_version_id uuid NOT NULL,
  item_id varchar(64) NOT NULL,
  price_amount bigint NOT NULL,
  quantity bigint NOT NULL DEFAULT 1,
  currency_ledger_entry_id uuid NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (purchase_id)
);
CREATE UNIQUE INDEX shop_purchases_idem_uq ON gamification.shop_purchases (idempotency_key);
CREATE INDEX shop_purchases_student_item_idx ON gamification.shop_purchases (tenant_id, student_id, shop_item_id);

CREATE TABLE gamification.collections (
  collection_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64),
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (collection_id)
);
CREATE UNIQUE INDEX collections_code_uq ON gamification.collections (code) WHERE deleted_at IS NULL;

CREATE TABLE gamification.collection_versions (
  collection_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  collection_id uuid NOT NULL,
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
  code varchar(64) NOT NULL,
  collection_type varchar(64) NOT NULL,
  visibility varchar(64) NOT NULL,
  item_count bigint NOT NULL,
  PRIMARY KEY (collection_version_id)
);
CREATE UNIQUE INDEX collection_versions_no_uq ON gamification.collection_versions (collection_id, version_no);

CREATE TABLE gamification.collection_items (
  collection_item_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  collection_version_id uuid NOT NULL,
  collection_id uuid NOT NULL,
  item_id varchar(64) NOT NULL,
  position bigint NOT NULL,
  source_hint jsonb NOT NULL,
  objective_id uuid,
  PRIMARY KEY (collection_item_id)
);
CREATE INDEX collection_items_item_idx ON gamification.collection_items (item_id);
CREATE INDEX collection_items_objective_idx ON gamification.collection_items (objective_id);
CREATE UNIQUE INDEX collection_items_uq ON gamification.collection_items (collection_version_id, item_id);

CREATE TABLE gamification.collection_progress (
  collection_progress_id uuid NOT NULL,
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
  collection_id uuid NOT NULL,
  owned_item_ids jsonb NOT NULL,
  owned_count bigint NOT NULL DEFAULT 0,
  completed_at timestamptz,
  PRIMARY KEY (collection_progress_id)
);
CREATE UNIQUE INDEX collection_progress_uq ON gamification.collection_progress (tenant_id, student_id, collection_id) WHERE deleted_at IS NULL;

CREATE TABLE gamification.achievements (
  achievement_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64),
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (achievement_id)
);
CREATE UNIQUE INDEX achievements_code_uq ON gamification.achievements (code) WHERE deleted_at IS NULL;

CREATE TABLE gamification.achievement_versions (
  achievement_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  achievement_id uuid NOT NULL,
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
  code varchar(64) NOT NULL,
  category varchar(64) NOT NULL,
  visibility varchar(64) NOT NULL,
  trigger_event_types jsonb NOT NULL,
  PRIMARY KEY (achievement_version_id)
);
CREATE UNIQUE INDEX achievement_versions_no_uq ON gamification.achievement_versions (achievement_id, version_no);

CREATE TABLE gamification.student_achievements (
  student_achievement_id uuid NOT NULL,
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
  achievement_id uuid NOT NULL,
  tier varchar(64),
  progress bigint NOT NULL DEFAULT 0,
  progress_state jsonb NOT NULL,
  achievement_version bigint NOT NULL DEFAULT 1,
  unlocked_at timestamptz,
  PRIMARY KEY (student_achievement_id)
);
CREATE UNIQUE INDEX student_achievements_uq ON gamification.student_achievements (tenant_id, student_id, achievement_id) WHERE deleted_at IS NULL;

CREATE TABLE gamification.achievement_progress_events (
  progress_event_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  achievement_id uuid NOT NULL,
  event_id uuid NOT NULL,
  event_type varchar(64) NOT NULL,
  PRIMARY KEY (progress_event_id)
);
CREATE UNIQUE INDEX achievement_progress_events_uq ON gamification.achievement_progress_events (student_id, achievement_id, event_id);

CREATE TABLE gamification.economy_adjustments (
  adjustment_id uuid NOT NULL,
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
  kind varchar(64) NOT NULL,
  currency_code varchar(64),
  item_id varchar(64),
  delta bigint NOT NULL,
  reason varchar(1000) NOT NULL,
  ticket_ref varchar(128),
  status varchar(64) NOT NULL DEFAULT 'pending_approval',
  approved_by uuid,
  rejected_by uuid,
  reject_reason varchar(500),
  ledger_entry_id uuid,
  idempotency_key varchar(64) NOT NULL,
  applied_at timestamptz,
  rejected_at timestamptz,
  PRIMARY KEY (adjustment_id)
);
CREATE UNIQUE INDEX economy_adjustments_idem_uq ON gamification.economy_adjustments (idempotency_key) WHERE deleted_at IS NULL;
CREATE INDEX economy_adjustments_status_idx ON gamification.economy_adjustments (tenant_id, status, created_at) WHERE deleted_at IS NULL;
ALTER TABLE game.item_definition_versions ADD COLUMN stackable boolean NOT NULL DEFAULT false;
ALTER TABLE game.item_definition_versions ADD COLUMN max_stack bigint NOT NULL DEFAULT 1;
ALTER TABLE game.item_definition_versions ADD COLUMN payload jsonb;
ALTER TABLE game.item_definition_versions ADD COLUMN payload_sha256 varchar(64);
ALTER TABLE game.item_definition_versions ADD COLUMN change_note varchar(500);
ALTER TABLE game.item_definition_versions ADD COLUMN snapshot_key varchar(512);
ALTER TABLE game.item_definition_versions ADD COLUMN snapshot_sha256 varchar(64);
ALTER TABLE game.item_definition_versions ADD COLUMN published_at timestamptz;
ALTER TABLE game.item_definition_versions ADD COLUMN published_by uuid;
-- END entddl

-- ---------- currency (G): Xu, the only currency of Release 1.0 (ADR-0049, OQ-14) ----------
-- daily_cap is xu_daily_cap (INVENTORY_ECONOMY §4.3, CONFIG_DEFAULTS: 300 Xu/day); the Game Operations Console (F078,
-- config type currencies) changes it under the platform scope. The row is reference data the wallets point to.
ALTER TABLE gamification.currencies
  ADD CONSTRAINT currencies_code_chk CHECK (currency_code ~ '^[a-z][a-z0-9_]*$'),
  ADD CONSTRAINT currencies_cap_chk CHECK (daily_cap BETWEEN 0 AND 100000),
  ADD CONSTRAINT currencies_status_chk CHECK (status IN ('active','retired'));
INSERT INTO gamification.currencies (currency_code, created_at, updated_at, name_vi, name_en, daily_cap, status)
  VALUES ('xu', now(), now(), 'Xu', 'Coins', 300, 'active');
REVOKE INSERT, UPDATE ON gamification.currencies FROM game_app;
REVOKE DELETE, TRUNCATE ON gamification.currencies FROM game_app, game_platform;

-- ---------- platform content (G): items, shop items, collections, achievements (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- use cases of students (game_app) only read them. Published versions are immutable (R11, R31).
ALTER TABLE game.item_definition_versions DROP CONSTRAINT item_definition_versions_kind_chk,
  ADD CONSTRAINT item_definition_versions_kind_chk CHECK (kind IN ('hair','outfit','accessory','emote','pet','title','frame',
    'badge','consumable','collectible','decoration','knowledge_card','consumable_cosmetic')),
  ADD CONSTRAINT item_definition_versions_stack_chk CHECK (max_stack BETWEEN 1 AND 999 AND (stackable OR max_stack = 1)),
  ADD CONSTRAINT item_definition_versions_payload_chk CHECK (payload IS NULL OR octet_length(payload::text) <= 65536),
  ADD CONSTRAINT item_definition_versions_sha_chk CHECK (payload_sha256 IS NULL OR payload_sha256 ~ '^[0-9a-f]{64}$');

ALTER TABLE gamification.shop_item_versions
  ADD CONSTRAINT shop_item_versions_item_fk FOREIGN KEY (shop_item_id) REFERENCES gamification.shop_items (shop_item_id),
  ADD CONSTRAINT shop_item_versions_def_fk FOREIGN KEY (item_id) REFERENCES game.item_definitions (item_id),
  ADD CONSTRAINT shop_item_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT shop_item_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT shop_item_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT shop_item_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT shop_item_versions_price_chk CHECK (price_amount > 0 AND currency_code = 'xu'),
  ADD CONSTRAINT shop_item_versions_kind_chk CHECK (item_kind IN ('outfit','hair','accessory','emote','pet','title','frame','decoration',
    'consumable_cosmetic')),
  ADD CONSTRAINT shop_item_versions_limit_chk CHECK (purchase_limit BETWEEN 1 AND 999),
  ADD CONSTRAINT shop_item_versions_window_chk CHECK (available_from IS NULL OR available_to IS NULL OR available_from < available_to),
  ADD CONSTRAINT shop_item_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9);
ALTER TABLE gamification.shop_items
  ADD CONSTRAINT shop_items_current_fk FOREIGN KEY (current_version_id)
    REFERENCES gamification.shop_item_versions (shop_item_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER shop_items_current_published AFTER INSERT OR UPDATE OF current_version_id ON gamification.shop_items
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('gamification', 'shop_item_versions', 'shop_item_version_id', 'shop_item_id');
CREATE TRIGGER shop_item_versions_immutable BEFORE UPDATE ON gamification.shop_item_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

ALTER TABLE gamification.collection_versions
  ADD CONSTRAINT collection_versions_collection_fk FOREIGN KEY (collection_id) REFERENCES gamification.collections (collection_id),
  ADD CONSTRAINT collection_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT collection_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT collection_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT collection_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT collection_versions_type_chk CHECK (collection_type IN ('pet','knowledge_card','cosmetic','badge','exploration','event')),
  ADD CONSTRAINT collection_versions_visibility_chk CHECK (visibility IN ('always','hidden_until_first')),
  ADD CONSTRAINT collection_versions_count_chk CHECK (item_count BETWEEN 1 AND 200);
ALTER TABLE gamification.collections
  ADD CONSTRAINT collections_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT collections_current_fk FOREIGN KEY (current_version_id)
    REFERENCES gamification.collection_versions (collection_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER collections_current_published AFTER INSERT OR UPDATE OF current_version_id ON gamification.collections
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('gamification', 'collection_versions', 'collection_version_id', 'collection_id');
CREATE TRIGGER collection_versions_immutable BEFORE UPDATE ON gamification.collection_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
-- The items of a collection version (GA): written in the publish transaction, never changed.
ALTER TABLE gamification.collection_items
  ADD CONSTRAINT collection_items_version_fk FOREIGN KEY (collection_version_id) REFERENCES gamification.collection_versions (collection_version_id),
  ADD CONSTRAINT collection_items_def_fk FOREIGN KEY (item_id) REFERENCES game.item_definitions (item_id),
  ADD CONSTRAINT collection_items_position_chk CHECK (position BETWEEN 0 AND 199),
  ADD CONSTRAINT collection_items_hint_chk CHECK (jsonb_typeof(source_hint) = 'object' AND octet_length(source_hint::text) <= 2048);

ALTER TABLE gamification.achievement_versions
  ADD CONSTRAINT achievement_versions_achievement_fk FOREIGN KEY (achievement_id) REFERENCES gamification.achievements (achievement_id),
  ADD CONSTRAINT achievement_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT achievement_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT achievement_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT achievement_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT achievement_versions_category_chk CHECK (category IN ('first_time','progress','learning','exploration','social','team',
    'competition','seasonal','hidden')),
  ADD CONSTRAINT achievement_versions_visibility_chk CHECK (visibility IN ('visible','hidden_until_unlocked')),
  ADD CONSTRAINT achievement_versions_triggers_chk CHECK (jsonb_typeof(trigger_event_types) = 'array'
    AND jsonb_array_length(trigger_event_types) BETWEEN 1 AND 5);
CREATE INDEX achievement_versions_triggers_idx ON gamification.achievement_versions USING gin (trigger_event_types)
  WHERE deleted_at IS NULL AND status = 'published';
ALTER TABLE gamification.achievements
  ADD CONSTRAINT achievements_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT achievements_current_fk FOREIGN KEY (current_version_id)
    REFERENCES gamification.achievement_versions (achievement_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER achievements_current_published AFTER INSERT OR UPDATE OF current_version_id ON gamification.achievements
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('gamification', 'achievement_versions', 'achievement_version_id', 'achievement_id');
CREATE TRIGGER achievement_versions_immutable BEFORE UPDATE ON gamification.achievement_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

REVOKE INSERT, UPDATE ON gamification.shop_items, gamification.shop_item_versions, gamification.collections,
  gamification.collection_versions, gamification.collection_items, gamification.achievements, gamification.achievement_versions
  FROM game_app;
REVOKE UPDATE ON gamification.collection_items FROM game_platform;
REVOKE DELETE, TRUNCATE ON gamification.shop_items, gamification.shop_item_versions, gamification.collections,
  gamification.collection_versions, gamification.collection_items, gamification.achievements, gamification.achievement_versions
  FROM game_app, game_platform;

-- The release history accepts the F075 item types; resource assignments (ADR-0029) accept shop items.
ALTER TABLE game.content_releases DROP CONSTRAINT content_releases_type_chk,
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config','progression_curve','reward_rule','shop_item','collection','achievement'));
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck,
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('quest','minigame_config','item','shop_item'));
-- Reward grants of F075: achievement tiers, collection completions, knowledge cards.
ALTER TABLE gamification.reward_grants DROP CONSTRAINT reward_grants_kind_chk,
  ADD CONSTRAINT reward_grants_kind_chk CHECK (trigger_kind IN ('event_rule','level_up','achievement','collection','knowledge_card'));

-- ---------- tenant tables: wallets (T), ledgers (TA), purchases (TA) ----------
ALTER TABLE gamification.wallets
  ADD CONSTRAINT wallets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT wallets_currency_fk FOREIGN KEY (currency_code) REFERENCES gamification.currencies (currency_code),
  ADD CONSTRAINT wallets_balance_chk CHECK (balance >= 0);
ALTER TABLE gamification.wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.wallets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.wallets FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.wallets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON gamification.wallets FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON gamification.wallets FROM game_app, game_platform;

-- Xu ledger (TA, ADR-0049): never updated or deleted (F075-AC-05); wallets.balance = Σ delta.
ALTER TABLE gamification.currency_ledger
  ADD CONSTRAINT currency_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT currency_ledger_currency_fk FOREIGN KEY (currency_code) REFERENCES gamification.currencies (currency_code),
  ADD CONSTRAINT currency_ledger_delta_chk CHECK (delta <> 0 AND balance_after >= 0),
  ADD CONSTRAINT currency_ledger_reason_chk CHECK (reason_code IN ('reward','shop_purchase','adjustment','refund','duplicate_conversion')),
  ADD CONSTRAINT currency_ledger_sign_chk CHECK (CASE reason_code WHEN 'shop_purchase' THEN delta < 0 WHEN 'adjustment' THEN true
    ELSE delta > 0 END),
  ADD CONSTRAINT currency_ledger_source_chk CHECK (source_type IN ('reward_grant','shop_purchase','adjustment')),
  ADD CONSTRAINT currency_ledger_item_chk CHECK (item_id IS NULL OR item_id ~ '^[a-z0-9_]+$'),
  ADD CONSTRAINT currency_ledger_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
ALTER TABLE gamification.currency_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.currency_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.currency_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.currency_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.currency_ledger FROM game_app, game_platform;

-- Item ledger (TA): every change of ownership (F075-AC-05).
ALTER TABLE gamification.item_ledger
  ADD CONSTRAINT item_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT item_ledger_item_fk FOREIGN KEY (item_id) REFERENCES game.item_definitions (item_id),
  ADD CONSTRAINT item_ledger_delta_chk CHECK (delta <> 0 AND (delta > 0 OR reason_code = 'adjustment')),
  ADD CONSTRAINT item_ledger_reason_chk CHECK (reason_code IN ('reward','shop_purchase','collection','adjustment','duplicate_conversion')),
  ADD CONSTRAINT item_ledger_source_chk CHECK (source_type IN ('reward_grant','shop_purchase','adjustment')),
  ADD CONSTRAINT item_ledger_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
ALTER TABLE gamification.item_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.item_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.item_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.item_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.item_ledger FROM game_app, game_platform;

ALTER TABLE gamification.shop_purchases
  ADD CONSTRAINT shop_purchases_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT shop_purchases_item_fk FOREIGN KEY (shop_item_id) REFERENCES gamification.shop_items (shop_item_id),
  ADD CONSTRAINT shop_purchases_version_fk FOREIGN KEY (shop_item_version_id) REFERENCES gamification.shop_item_versions (shop_item_version_id),
  ADD CONSTRAINT shop_purchases_price_chk CHECK (price_amount > 0 AND quantity BETWEEN 1 AND 99),
  ADD CONSTRAINT shop_purchases_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
ALTER TABLE gamification.shop_purchases ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.shop_purchases FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.shop_purchases FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.shop_purchases FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.shop_purchases FROM game_app, game_platform;

-- ---------- tenant tables: collection progress, achievements (T), counted events (TA), adjustments (T) ----------
ALTER TABLE gamification.collection_progress
  ADD CONSTRAINT collection_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT collection_progress_collection_fk FOREIGN KEY (collection_id) REFERENCES gamification.collections (collection_id),
  ADD CONSTRAINT collection_progress_owned_chk CHECK (jsonb_typeof(owned_item_ids) = 'array'
    AND jsonb_array_length(owned_item_ids) <= 500 AND owned_count = jsonb_array_length(owned_item_ids));
ALTER TABLE gamification.collection_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.collection_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.collection_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.collection_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON gamification.collection_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON gamification.collection_progress FROM game_app, game_platform;

ALTER TABLE gamification.student_achievements
  ADD CONSTRAINT student_achievements_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT student_achievements_achievement_fk FOREIGN KEY (achievement_id) REFERENCES gamification.achievements (achievement_id),
  ADD CONSTRAINT student_achievements_progress_chk CHECK (progress >= 0 AND achievement_version >= 1),
  ADD CONSTRAINT student_achievements_tier_chk CHECK (tier IS NULL OR tier IN ('single','bronze','silver','gold','platinum')),
  ADD CONSTRAINT student_achievements_unlocked_chk CHECK ((tier IS NULL) = (unlocked_at IS NULL)),
  ADD CONSTRAINT student_achievements_state_chk CHECK (jsonb_typeof(progress_state) = 'object' AND octet_length(progress_state::text) <= 4096);
ALTER TABLE gamification.student_achievements ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.student_achievements FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.student_achievements FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.student_achievements FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON gamification.student_achievements FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON gamification.student_achievements FROM game_app, game_platform;

ALTER TABLE gamification.achievement_progress_events
  ADD CONSTRAINT achievement_progress_events_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT achievement_progress_events_achievement_fk FOREIGN KEY (achievement_id) REFERENCES gamification.achievements (achievement_id);
ALTER TABLE gamification.achievement_progress_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.achievement_progress_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.achievement_progress_events FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.achievement_progress_events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.achievement_progress_events FROM game_app, game_platform;

-- Economy adjustments (T): four-eyes — the approver or rejecter is never the creator (ADR-0049, ANTI_CHEAT §7).
ALTER TABLE gamification.economy_adjustments
  ADD CONSTRAINT economy_adjustments_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT economy_adjustments_kind_chk CHECK (kind IN ('currency','item','xp')),
  ADD CONSTRAINT economy_adjustments_target_chk CHECK (CASE kind WHEN 'currency' THEN currency_code = 'xu' AND item_id IS NULL
    WHEN 'item' THEN item_id IS NOT NULL AND currency_code IS NULL ELSE item_id IS NULL AND currency_code IS NULL END),
  ADD CONSTRAINT economy_adjustments_delta_chk CHECK (delta <> 0),
  ADD CONSTRAINT economy_adjustments_reason_chk CHECK (char_length(btrim(reason)) >= 10),
  ADD CONSTRAINT economy_adjustments_status_chk CHECK (status IN ('pending_approval','applied','rejected')),
  ADD CONSTRAINT economy_adjustments_creator_chk CHECK (created_by IS NOT NULL),
  ADD CONSTRAINT economy_adjustments_four_eyes_chk CHECK ((approved_by IS NULL OR approved_by <> created_by)
    AND (rejected_by IS NULL OR rejected_by <> created_by)),
  ADD CONSTRAINT economy_adjustments_state_chk CHECK (CASE status
    WHEN 'applied' THEN approved_by IS NOT NULL AND applied_at IS NOT NULL AND ledger_entry_id IS NOT NULL
    WHEN 'rejected' THEN rejected_by IS NOT NULL AND rejected_at IS NOT NULL
    ELSE approved_by IS NULL AND rejected_by IS NULL END),
  ADD CONSTRAINT economy_adjustments_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
ALTER TABLE gamification.economy_adjustments ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.economy_adjustments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.economy_adjustments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.economy_adjustments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON gamification.economy_adjustments FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON gamification.economy_adjustments FROM game_app, game_platform;

-- inventory_items (F070) is written by F075 now: the quantity of a live row is ≥ 1 (a full removal by adjustment keeps 0).
ALTER TABLE game.inventory_items DROP CONSTRAINT inventory_items_quantity_chk,
  ADD CONSTRAINT inventory_items_quantity_chk CHECK (quantity BETWEEN 0 AND 999);
CREATE INDEX inventory_items_student_idx ON game.inventory_items (tenant_id, student_id, item_id) WHERE deleted_at IS NULL;

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW gamification.currencies_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.currencies WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.wallets_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.wallets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.shop_items_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.shop_items WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.shop_item_versions_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.shop_item_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.collections_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.collections WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.collection_versions_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.collection_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.collection_progress_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.collection_progress WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.achievements_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.achievements WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.achievement_versions_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.achievement_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.student_achievements_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.student_achievements WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.economy_adjustments_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.economy_adjustments WHERE deleted_at IS NULL;
GRANT SELECT ON gamification.currencies_live, gamification.wallets_live, gamification.shop_items_live, gamification.shop_item_versions_live,
  gamification.collections_live, gamification.collection_versions_live, gamification.collection_progress_live, gamification.achievements_live,
  gamification.achievement_versions_live, gamification.student_achievements_live, gamification.economy_adjustments_live TO game_readonly;
GRANT SELECT ON gamification.currency_ledger, gamification.item_ledger, gamification.shop_purchases, gamification.collection_items,
  gamification.achievement_progress_events TO game_readonly;
