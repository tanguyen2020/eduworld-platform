-- F074 T007/T008 — XP ledger, level, XP curve, titles and the Reward Engine of module game (specs/074-xp-level-reward-engine:
-- data-model §1; DATABASE_DESIGN §2.8, §2.8a, §2.13, §3.13; PROGRESSION_SYSTEM §3–§6; ADR-0028, ADR-0041, ADR-0043,
-- ADR-0049, ADR-0089). Applied by `migrate --module=game` as game_migrator. Order: ent tables and the column added to
-- game.game_profiles (entddl block, golden test modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs,
-- the release pointers, immutability of published versions, RLS of the tenant tables, append-only grants of the ledger,
-- the level history and the reward grants (F074-AC-12), platform-only writes of the content, _live views.

-- BEGIN entddl
CREATE TABLE gamification.xp_ledger (
  xp_entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  school_id uuid,
  amount bigint NOT NULL,
  reason_code varchar(64) NOT NULL DEFAULT 'reward',
  source_type varchar(64) NOT NULL,
  source_code varchar(64) NOT NULL,
  source_id uuid NOT NULL,
  content_version_id uuid,
  rule_id uuid,
  rule_version bigint,
  capped boolean NOT NULL DEFAULT false,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (xp_entry_id)
);
CREATE INDEX xp_ledger_content_idx ON gamification.xp_ledger (tenant_id, student_id, content_version_id, created_at);
CREATE UNIQUE INDEX xp_ledger_idem_uq ON gamification.xp_ledger (idempotency_key);
CREATE UNIQUE INDEX xp_ledger_source_uq ON gamification.xp_ledger (student_id, source_type, source_id, rule_id);
CREATE INDEX xp_ledger_student_idx ON gamification.xp_ledger (tenant_id, student_id, created_at);

CREATE TABLE gamification.xp_daily_caps (
  xp_daily_cap_id uuid NOT NULL,
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
  cap_date date NOT NULL,
  bucket varchar(64) NOT NULL,
  granted bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (xp_daily_cap_id)
);
CREATE UNIQUE INDEX xp_daily_caps_uq ON gamification.xp_daily_caps (tenant_id, student_id, cap_date, bucket) WHERE deleted_at IS NULL;

CREATE TABLE gamification.reward_rules (
  reward_rule_id uuid NOT NULL,
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
  PRIMARY KEY (reward_rule_id)
);
CREATE UNIQUE INDEX reward_rules_code_uq ON gamification.reward_rules (code) WHERE deleted_at IS NULL;

CREATE TABLE gamification.reward_rule_versions (
  reward_rule_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  reward_rule_id uuid NOT NULL,
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
  trigger_event_type varchar(64) NOT NULL,
  source_code varchar(64) NOT NULL,
  effective_from timestamptz NOT NULL,
  active_to timestamptz,
  PRIMARY KEY (reward_rule_version_id)
);
CREATE UNIQUE INDEX reward_rule_versions_no_uq ON gamification.reward_rule_versions (reward_rule_id, version_no);
CREATE INDEX reward_rule_versions_trigger_idx ON gamification.reward_rule_versions (trigger_event_type, effective_from) WHERE deleted_at IS NULL;

CREATE TABLE gamification.reward_grants (
  reward_grant_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  school_id uuid,
  rule_version_id uuid,
  trigger_event_id uuid NOT NULL,
  trigger_kind varchar(64) NOT NULL,
  level bigint,
  curve_id uuid,
  grants jsonb NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  granted_at timestamptz NOT NULL,
  PRIMARY KEY (reward_grant_id)
);
CREATE UNIQUE INDEX reward_grants_idem_uq ON gamification.reward_grants (idempotency_key);
CREATE INDEX reward_grants_student_idx ON gamification.reward_grants (tenant_id, student_id, granted_at);

CREATE TABLE game.progression_curves (
  curve_id uuid NOT NULL,
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
  PRIMARY KEY (curve_id)
);
CREATE UNIQUE INDEX progression_curves_code_uq ON game.progression_curves (code) WHERE deleted_at IS NULL;

CREATE TABLE game.progression_curve_versions (
  curve_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  curve_id uuid NOT NULL,
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
  max_level bigint NOT NULL,
  level_floor_protection boolean NOT NULL DEFAULT true,
  PRIMARY KEY (curve_version_id)
);
CREATE UNIQUE INDEX progression_curve_versions_no_uq ON game.progression_curve_versions (curve_id, version_no);

CREATE TABLE game.progression_levels (
  progression_level_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  curve_version_id uuid NOT NULL,
  level bigint NOT NULL,
  xp_required bigint,
  xp_cumulative bigint NOT NULL,
  title_code varchar(64) NOT NULL,
  rewards jsonb NOT NULL,
  PRIMARY KEY (progression_level_id)
);
CREATE UNIQUE INDEX progression_levels_uq ON game.progression_levels (curve_version_id, level);

CREATE TABLE game.level_history (
  level_history_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  student_id uuid NOT NULL,
  from_level bigint NOT NULL,
  to_level bigint NOT NULL,
  curve_id uuid NOT NULL,
  curve_version bigint NOT NULL,
  trigger_xp_entry_id uuid,
  reached_at timestamptz NOT NULL,
  PRIMARY KEY (level_history_id)
);
CREATE INDEX level_history_student_idx ON game.level_history (tenant_id, student_id, reached_at);
ALTER TABLE game.game_profiles ADD COLUMN grade_transition_year varchar(16);
-- END entddl

-- ---------- platform content (G): reward rules and progression curves (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- Reward Engine and the student queries (game_app) only read them. Published versions are immutable (R11, R31).
CREATE FUNCTION game.check_current_version_in() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  ok boolean;
  item text := to_jsonb(NEW) ->> TG_ARGV[3];
BEGIN
  IF NEW.current_version_id IS NULL THEN
    RETURN NEW;
  END IF;
  EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I.%I WHERE %I = $1 AND %I::text = $2 AND status = ''published'' AND deleted_at IS NULL)',
    TG_ARGV[0], TG_ARGV[1], TG_ARGV[2], TG_ARGV[3]) INTO ok USING NEW.current_version_id, item;
  IF NOT ok THEN
    RAISE EXCEPTION 'CONTENT_POINTER_NOT_PUBLISHED' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;

ALTER TABLE gamification.reward_rule_versions
  ADD CONSTRAINT reward_rule_versions_rule_fk FOREIGN KEY (reward_rule_id) REFERENCES gamification.reward_rules (reward_rule_id),
  ADD CONSTRAINT reward_rule_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT reward_rule_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT reward_rule_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT reward_rule_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT reward_rule_versions_window_chk CHECK (active_to IS NULL OR effective_from < active_to);
ALTER TABLE gamification.reward_rules
  ADD CONSTRAINT reward_rules_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT reward_rules_current_fk FOREIGN KEY (current_version_id)
    REFERENCES gamification.reward_rule_versions (reward_rule_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER reward_rules_current_published AFTER INSERT OR UPDATE OF current_version_id ON gamification.reward_rules
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('gamification', 'reward_rule_versions', 'reward_rule_version_id', 'reward_rule_id');
CREATE TRIGGER reward_rule_versions_immutable BEFORE UPDATE ON gamification.reward_rule_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

ALTER TABLE game.progression_curve_versions
  ADD CONSTRAINT progression_curve_versions_curve_fk FOREIGN KEY (curve_id) REFERENCES game.progression_curves (curve_id),
  ADD CONSTRAINT progression_curve_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT progression_curve_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT progression_curve_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT progression_curve_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT progression_curve_versions_max_chk CHECK (max_level BETWEEN 2 AND 200);
ALTER TABLE game.progression_curves
  ADD CONSTRAINT progression_curves_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)*$'),
  ADD CONSTRAINT progression_curves_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.progression_curve_versions (curve_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER progression_curves_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.progression_curves
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('game', 'progression_curve_versions', 'curve_version_id', 'curve_id');
CREATE TRIGGER progression_curve_versions_immutable BEFORE UPDATE ON game.progression_curve_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

-- The level table of a curve version (GA): generated in the publish transaction, never changed.
ALTER TABLE game.progression_levels
  ADD CONSTRAINT progression_levels_version_fk FOREIGN KEY (curve_version_id) REFERENCES game.progression_curve_versions (curve_version_id),
  ADD CONSTRAINT progression_levels_level_chk CHECK (level BETWEEN 1 AND 200),
  ADD CONSTRAINT progression_levels_xp_chk CHECK ((xp_required IS NULL OR xp_required >= 1) AND xp_cumulative >= 0),
  ADD CONSTRAINT progression_levels_rewards_chk CHECK (jsonb_typeof(rewards) = 'array' AND octet_length(rewards::text) <= 16384);

REVOKE INSERT, UPDATE ON gamification.reward_rules, gamification.reward_rule_versions, game.progression_curves,
  game.progression_curve_versions, game.progression_levels FROM game_app;
REVOKE UPDATE ON game.progression_levels FROM game_platform;
REVOKE DELETE, TRUNCATE ON gamification.reward_rules, gamification.reward_rule_versions, game.progression_curves,
  game.progression_curve_versions, game.progression_levels FROM game_app, game_platform;

-- The release history accepts the F074 item types.
ALTER TABLE game.content_releases DROP CONSTRAINT content_releases_type_chk,
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config','progression_curve','reward_rule'));

-- ---------- tenant tables: ledger and level history (TA), reward grants (TA), daily caps (T) ----------
ALTER TABLE gamification.xp_ledger
  ADD CONSTRAINT xp_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT xp_ledger_reason_chk CHECK (reason_code IN ('reward','adjustment')),
  ADD CONSTRAINT xp_ledger_amount_chk CHECK (amount >= 0 OR reason_code = 'adjustment'),
  ADD CONSTRAINT xp_ledger_rule_chk CHECK ((reason_code = 'reward') = (rule_id IS NOT NULL AND rule_version IS NOT NULL)),
  ADD CONSTRAINT xp_ledger_source_type_chk CHECK (source_type IN ('activity','quest','minigame','lesson','objective','world','merge',
    'adjustment')),
  ADD CONSTRAINT xp_ledger_source_code_chk CHECK (source_code ~ '^[a-z0-9_]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT xp_ledger_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
ALTER TABLE gamification.xp_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.xp_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.xp_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.xp_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.xp_ledger FROM game_app, game_platform;

ALTER TABLE game.level_history
  ADD CONSTRAINT level_history_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT level_history_curve_fk FOREIGN KEY (curve_id) REFERENCES game.progression_curves (curve_id),
  ADD CONSTRAINT level_history_levels_chk CHECK (from_level >= 1 AND to_level = from_level + 1 AND curve_version >= 1);
ALTER TABLE game.level_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.level_history FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.level_history FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.level_history FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.level_history FROM game_app, game_platform;

ALTER TABLE gamification.reward_grants
  ADD CONSTRAINT reward_grants_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT reward_grants_kind_chk CHECK (trigger_kind IN ('event_rule','level_up')),
  ADD CONSTRAINT reward_grants_level_chk CHECK ((trigger_kind = 'level_up') = (level IS NOT NULL AND curve_id IS NOT NULL)),
  ADD CONSTRAINT reward_grants_grants_chk CHECK (jsonb_typeof(grants) = 'array' AND octet_length(grants::text) <= 16384),
  ADD CONSTRAINT reward_grants_idem_chk CHECK (idempotency_key ~ '^[0-9a-f]{64}$');
CREATE UNIQUE INDEX reward_grants_trigger_uq ON gamification.reward_grants (trigger_event_id, rule_version_id, student_id) NULLS NOT DISTINCT;
ALTER TABLE gamification.reward_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.reward_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.reward_grants FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.reward_grants FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON gamification.reward_grants FROM game_app, game_platform;

ALTER TABLE gamification.xp_daily_caps
  ADD CONSTRAINT xp_daily_caps_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT xp_daily_caps_granted_chk CHECK (granted >= 0),
  ADD CONSTRAINT xp_daily_caps_bucket_chk CHECK (bucket ~ '^[a-z]+:[a-z0-9_.:]+$');
ALTER TABLE gamification.xp_daily_caps ENABLE ROW LEVEL SECURITY;
ALTER TABLE gamification.xp_daily_caps FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON gamification.xp_daily_caps FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON gamification.xp_daily_caps FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON gamification.xp_daily_caps FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON gamification.xp_daily_caps FROM game_app, game_platform;

-- game_profiles: progression cache written by the Reward Engine in the ledger transaction (xp_total = Σ ledger).
ALTER TABLE game.game_profiles
  ADD CONSTRAINT game_profiles_xp_chk CHECK (xp_total >= 0 AND level BETWEEN 1 AND 200),
  ADD CONSTRAINT game_profiles_grade_year_chk CHECK (grade_transition_year IS NULL OR grade_transition_year ~ '^[0-9]{4}-[0-9]{4}$');

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW gamification.reward_rules_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.reward_rules WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.reward_rule_versions_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.reward_rule_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW gamification.xp_daily_caps_live WITH (security_barrier = true) AS
  SELECT * FROM gamification.xp_daily_caps WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.progression_curves_live WITH (security_barrier = true) AS
  SELECT * FROM game.progression_curves WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.progression_curve_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.progression_curve_versions WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA gamification TO game_readonly;
GRANT SELECT ON gamification.reward_rules_live, gamification.reward_rule_versions_live, gamification.xp_daily_caps_live,
  game.progression_curves_live, game.progression_curve_versions_live TO game_readonly;
GRANT SELECT ON gamification.xp_ledger, gamification.reward_grants, game.level_history, game.progression_levels TO game_readonly;
