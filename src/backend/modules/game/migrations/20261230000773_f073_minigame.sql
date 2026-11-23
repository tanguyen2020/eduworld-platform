-- F073 T006/T008 — mini-game engine of module game (specs/073-minigame-engine: data-model §1; DATABASE_DESIGN §2.8, §3.13;
-- MINIGAME_ENGINE §3; ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089). Applied by `migrate --module=game` as game_migrator.
-- Order: ent tables (entddl block, golden test modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs,
-- release pointer, immutability of published versions, platform-only writes of the configs, RLS of the plays, _live views.

-- BEGIN entddl
CREATE TABLE game.minigame_configs (
  minigame_config_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64),
  owner_scope varchar(64) NOT NULL DEFAULT 'platform',
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (minigame_config_id)
);
CREATE UNIQUE INDEX minigame_configs_code_uq ON game.minigame_configs (code) WHERE deleted_at IS NULL;

CREATE TABLE game.minigame_config_versions (
  minigame_config_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  minigame_config_id uuid NOT NULL,
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
  minigame_type varchar(64) NOT NULL,
  grade_levels jsonb NOT NULL,
  subject_id uuid NOT NULL,
  question_types jsonb NOT NULL,
  PRIMARY KEY (minigame_config_version_id)
);
CREATE UNIQUE INDEX minigame_config_versions_no_uq ON game.minigame_config_versions (minigame_config_id, version_no);

CREATE TABLE game.minigame_plays (
  play_id uuid NOT NULL,
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
  school_id uuid,
  minigame_config_id uuid NOT NULL,
  minigame_config_version_id uuid NOT NULL,
  minigame_type varchar(64) NOT NULL,
  attempt_id uuid NOT NULL,
  context_type varchar(64) NOT NULL,
  context_id uuid,
  play_date varchar(10) NOT NULL,
  status varchar(64) NOT NULL,
  learning_score numeric(7,2),
  max_score numeric(7,2),
  game_score bigint,
  correct_count bigint,
  item_count bigint,
  completion_met boolean,
  rewards_eligible boolean,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  PRIMARY KEY (play_id)
);
CREATE UNIQUE INDEX minigame_plays_attempt_uq ON game.minigame_plays (tenant_id, attempt_id) WHERE deleted_at IS NULL;
CREATE INDEX minigame_plays_day_idx ON game.minigame_plays (tenant_id, student_id, minigame_config_id, play_date) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- platform content (G): mini-game configs and versions (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: the release core; Game Operations Console F078); the student use cases
-- (game_app) only read them. Published versions are immutable (R11, R31). A config holds no question (MINIGAME_ENGINE M-1).
ALTER TABLE game.minigame_config_versions
  ADD CONSTRAINT minigame_config_versions_config_fk FOREIGN KEY (minigame_config_id) REFERENCES game.minigame_configs (minigame_config_id),
  ADD CONSTRAINT minigame_config_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT minigame_config_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT minigame_config_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT minigame_config_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT minigame_config_versions_type_chk CHECK (minigame_type IN ('multiple_choice','true_false','fill_in_blank','drag_and_drop',
    'matching','ordering','calculation','puzzle','word_game','science_simulation','timed_challenge')),
  ADD CONSTRAINT minigame_config_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels)
    BETWEEN 1 AND 9),
  ADD CONSTRAINT minigame_config_versions_qtypes_chk CHECK (jsonb_typeof(question_types) = 'array' AND jsonb_array_length(question_types) <= 11);
CREATE INDEX minigame_config_versions_grades_gin ON game.minigame_config_versions USING gin (grade_levels jsonb_path_ops)
  WHERE deleted_at IS NULL;
ALTER TABLE game.minigame_configs
  ADD CONSTRAINT minigame_configs_scope_chk CHECK (owner_scope = 'platform'),
  ADD CONSTRAINT minigame_configs_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT minigame_configs_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.minigame_config_versions (minigame_config_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER minigame_configs_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.minigame_configs
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version('minigame_config_versions', 'minigame_config_version_id', 'minigame_config_id');
CREATE TRIGGER minigame_config_versions_immutable BEFORE UPDATE ON game.minigame_config_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
REVOKE INSERT, UPDATE ON game.minigame_configs, game.minigame_config_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.minigame_configs, game.minigame_config_versions FROM game_app, game_platform;

-- ---------- tenant table (T): plays ----------
ALTER TABLE game.minigame_plays
  ADD CONSTRAINT minigame_plays_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT minigame_plays_config_fk FOREIGN KEY (minigame_config_id) REFERENCES game.minigame_configs (minigame_config_id),
  ADD CONSTRAINT minigame_plays_version_fk FOREIGN KEY (minigame_config_version_id)
    REFERENCES game.minigame_config_versions (minigame_config_version_id),
  ADD CONSTRAINT minigame_plays_type_chk CHECK (minigame_type IN ('multiple_choice','true_false','fill_in_blank','drag_and_drop',
    'matching','ordering','calculation','puzzle','word_game','science_simulation','timed_challenge')),
  ADD CONSTRAINT minigame_plays_context_chk CHECK (context_type IN ('free_play','quest_step','assignment')),
  ADD CONSTRAINT minigame_plays_status_chk CHECK (status IN ('in_progress','completed','expired','abandoned')),
  ADD CONSTRAINT minigame_plays_day_chk CHECK (play_date ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
  ADD CONSTRAINT minigame_plays_scores_chk CHECK (coalesce(learning_score, 0) >= 0 AND coalesce(max_score, 0) >= 0
    AND coalesce(game_score, 0) >= 0 AND coalesce(correct_count, 0) >= 0 AND coalesce(item_count, 0) >= 0),
  ADD CONSTRAINT minigame_plays_done_chk CHECK ((status = 'completed') = (completed_at IS NOT NULL));

ALTER TABLE game.minigame_plays ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.minigame_plays FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.minigame_plays FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.minigame_plays FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.minigame_plays FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.minigame_plays FROM game_app, game_platform;

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.minigame_configs_live WITH (security_barrier = true) AS
  SELECT * FROM game.minigame_configs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.minigame_config_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.minigame_config_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.minigame_plays_live WITH (security_barrier = true) AS
  SELECT * FROM game.minigame_plays WHERE deleted_at IS NULL;
GRANT SELECT ON game.minigame_configs_live, game.minigame_config_versions_live, game.minigame_plays_live TO game_readonly;
