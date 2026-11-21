-- F104 T006/T007 — matchmaking of module competition (specs/104-matchmaking: data-model §1; DATABASE_DESIGN §2.10,
-- §2.16, §2.17; ADR-0004, ADR-0015, ADR-0028, ADR-0041, ADR-0043, ADR-0054, ADR-0089). Applied by
-- `migrate --module=competition` as competition_migrator. Tickets and proposals live in Valkey only (ADR-0004): no table.
-- Order: the new projection ref.schools (hand-written, rule 5), ent tables (entddl block, golden test
-- modules/competition/ent/ddl_test.go), then what ent cannot express: tenant_ids uuid[], CHECKs, FKs, RLS, soft-delete
-- only grants, the published-version immutability trigger, the append-only release log, _live views and the platform
-- seed of the matchmaking configuration.

-- ---------- Projection (DATABASE_DESIGN §2.17), written only by the consumer competition.projection_identity ----------
-- identity · school.updated (owner F010/F015): allowed_hours of the school for H5 (MATCHMAKING §2.1); settings_subset
-- only (no name, no PII).
CREATE TABLE ref.schools (
  school_id       uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status          varchar(64) NOT NULL DEFAULT 'active',
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);
CREATE INDEX ref_schools_tenant_idx ON ref.schools (tenant_id);
ALTER TABLE ref.schools ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- BEGIN entddl
CREATE TABLE competition.matches (
  match_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  mode varchar(64) NOT NULL,
  matchmaking_type varchar(64) NOT NULL,
  subject_id uuid NOT NULL,
  topic_id uuid,
  grade_level smallint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'lobby',
  ranked boolean NOT NULL,
  is_bot boolean NOT NULL DEFAULT false,
  scope_kind varchar(64) NOT NULL,
  tournament_id uuid,
  tournament_node_id uuid,
  duel_id uuid,
  ruleset_version_id uuid NOT NULL,
  question_plan jsonb,
  started_at timestamptz,
  ended_at timestamptz,
  PRIMARY KEY (match_id)
);
CREATE INDEX matches_status_created_idx ON competition.matches (status, created_at);
CREATE INDEX matches_tournament_idx ON competition.matches (tournament_id) WHERE tournament_id IS NOT NULL;

CREATE TABLE competition.match_participants (
  participant_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  match_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  team_no smallint NOT NULL,
  party_id uuid,
  outcome varchar(64),
  score numeric(9,3),
  connected_ms bigint,
  rating_mu_at_match numeric(8,3),
  rating_rd_at_match numeric(8,3),
  PRIMARY KEY (participant_id)
);
CREATE UNIQUE INDEX match_participants_match_student_uq ON competition.match_participants (match_id, student_id) WHERE deleted_at IS NULL;
CREATE INDEX match_participants_student_idx ON competition.match_participants (tenant_id, student_id, created_at);

CREATE TABLE competition.duels (
  duel_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  challenger_id uuid NOT NULL,
  opponent_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  topic_id uuid,
  status varchar(32) NOT NULL DEFAULT 'pending',
  match_id uuid,
  expires_at timestamptz NOT NULL,
  responded_at timestamptz,
  PRIMARY KEY (duel_id)
);
CREATE INDEX duels_expiry_idx ON competition.duels (expires_at) WHERE status = 'pending' AND deleted_at IS NULL;
CREATE INDEX duels_opponent_idx ON competition.duels (tenant_id, opponent_id, status);
CREATE UNIQUE INDEX duels_pending_uq ON competition.duels (challenger_id, opponent_id) WHERE status = 'pending' AND deleted_at IS NULL;

CREATE TABLE competition.competition_restrictions (
  restriction_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  kind varchar(64) NOT NULL,
  reason varchar(1000) NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  PRIMARY KEY (restriction_id)
);
CREATE INDEX competition_restrictions_student_idx ON competition.competition_restrictions (tenant_id, student_id, ends_at) WHERE deleted_at IS NULL;

CREATE TABLE competition.game_configs (
  config_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  config_type varchar(64) NOT NULL,
  code varchar(64) NOT NULL,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  owner_scope varchar(64) NOT NULL DEFAULT 'platform',
  PRIMARY KEY (config_id)
);
CREATE UNIQUE INDEX game_configs_type_code_uq ON competition.game_configs (config_type, code) WHERE deleted_at IS NULL;

CREATE TABLE competition.game_config_versions (
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
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  title varchar(255) NOT NULL,
  payload jsonb NOT NULL,
  effective_from timestamptz,
  author_id uuid,
  reviewer_id uuid,
  change_note varchar(500),
  based_on_version_id uuid,
  published_at timestamptz,
  PRIMARY KEY (config_version_id)
);
CREATE INDEX game_config_versions_active_idx ON competition.game_config_versions (config_id, status, effective_from);
CREATE UNIQUE INDEX game_config_versions_config_version_uq ON competition.game_config_versions (config_id, version_no);

CREATE TABLE competition.content_releases (
  release_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  config_id uuid NOT NULL,
  action varchar(64) NOT NULL,
  from_version_id uuid,
  to_version_id uuid,
  reason varchar(1000),
  released_by uuid,
  released_at timestamptz NOT NULL,
  PRIMARY KEY (release_id)
);
CREATE INDEX content_releases_config_idx ON competition.content_releases (config_id, released_at);
-- END entddl

-- ---------- competition.matches (G*: a match may span schools; reached through match_participants) ----------
ALTER TABLE competition.matches
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD COLUMN tenant_ids uuid[] NOT NULL,
  ADD CONSTRAINT matches_tenant_ids_chk CHECK (cardinality(tenant_ids) BETWEEN 1 AND 64),
  ADD CONSTRAINT matches_mode_chk CHECK (mode IN ('battle_1v1','team_battle','coop_challenge','timed_quiz','tournament')),
  ADD CONSTRAINT matches_type_chk CHECK (matchmaking_type IN ('random','skill','grade','subject','topic','friend','team','tournament','bot')),
  ADD CONSTRAINT matches_status_chk CHECK (status IN ('lobby','running','completed','cancelled','aborted')),
  ADD CONSTRAINT matches_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT matches_scope_chk CHECK (scope_kind IN ('school','tenant','platform')),
  ADD CONSTRAINT matches_scope_tenants_chk CHECK (scope_kind = 'platform' OR cardinality(tenant_ids) = 1),
  ADD CONSTRAINT matches_ranked_chk CHECK (NOT ranked OR (NOT is_bot AND matchmaking_type IN ('skill','grade','subject','team','tournament'))),
  ADD CONSTRAINT matches_bot_chk CHECK (is_bot = (matchmaking_type = 'bot')),
  ADD CONSTRAINT matches_question_plan_chk CHECK (question_plan IS NULL OR octet_length(question_plan::text) <= 65536);
CREATE INDEX matches_tenant_ids_idx ON competition.matches USING gin (tenant_ids);
REVOKE DELETE, TRUNCATE ON competition.matches FROM competition_app, competition_platform;

-- ---------- competition.match_participants (T) ----------
ALTER TABLE competition.match_participants
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT match_participants_team_chk CHECK (team_no BETWEEN 0 AND 1),
  ADD CONSTRAINT match_participants_outcome_chk CHECK (outcome IS NULL OR outcome IN ('win','loss','draw','abandoned')),
  ADD CONSTRAINT match_participants_match_fk FOREIGN KEY (match_id) REFERENCES competition.matches (match_id),
  ADD CONSTRAINT match_participants_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.match_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.match_participants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.match_participants FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.match_participants FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.match_participants FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.match_participants FROM competition_app, competition_platform;

-- ---------- competition.duels (T): pending 60 s, one pending per (challenger, opponent) ----------
ALTER TABLE competition.duels
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT duels_status_chk CHECK (status IN ('pending','accepted','declined','expired','cancelled')),
  ADD CONSTRAINT duels_self_chk CHECK (challenger_id <> opponent_id),
  ADD CONSTRAINT duels_responded_chk CHECK ((status = 'pending') = (responded_at IS NULL)),
  ADD CONSTRAINT duels_match_chk CHECK ((status = 'accepted') = (match_id IS NOT NULL)),
  ADD CONSTRAINT duels_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.duels ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.duels FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.duels FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.duels FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.duels FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.duels FROM competition_app, competition_platform;

-- ---------- competition.competition_restrictions (T, read by F104; written by F105 and the violation handling) ----------
ALTER TABLE competition.competition_restrictions
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT competition_restrictions_kind_chk CHECK (kind IN ('abandon_penalty','restrict_competition')),
  ADD CONSTRAINT competition_restrictions_period_chk CHECK (ends_at > starts_at),
  ADD CONSTRAINT competition_restrictions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.competition_restrictions ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.competition_restrictions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.competition_restrictions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.competition_restrictions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.competition_restrictions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.competition_restrictions FROM competition_app, competition_platform;

-- ---------- competition.game_configs / game_config_versions / content_releases (G/GA, platform only: R20) ----------
-- Written by the Game Operations Console (F108) with competition_platform; the tenant-serving role only reads.
ALTER TABLE competition.game_configs
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT game_configs_type_chk CHECK (config_type IN ('matchmaking-configs','ranking-rules','seasons','tournament-templates',
    'challenge-templates','leaderboard-definitions','competition-rewards')),
  ADD CONSTRAINT game_configs_owner_chk CHECK (owner_scope = 'platform'),
  ADD CONSTRAINT game_configs_latest_chk CHECK (latest_version_no >= 0);
ALTER TABLE competition.game_config_versions
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT game_config_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','unpublished','archived')),
  ADD CONSTRAINT game_config_versions_version_chk CHECK (version_no >= 1),
  ADD CONSTRAINT game_config_versions_payload_chk CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 65536),
  ADD CONSTRAINT game_config_versions_published_chk CHECK (status <> 'published' OR published_at IS NOT NULL),
  ADD CONSTRAINT game_config_versions_config_fk FOREIGN KEY (config_id) REFERENCES competition.game_configs (config_id);
ALTER TABLE competition.game_configs
  ADD CONSTRAINT game_configs_current_fk FOREIGN KEY (current_version_id) REFERENCES competition.game_config_versions (config_version_id);
ALTER TABLE competition.content_releases
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT content_releases_action_chk CHECK (action IN ('publish','rollback','unpublish')),
  ADD CONSTRAINT content_releases_config_fk FOREIGN KEY (config_id) REFERENCES competition.game_configs (config_id);
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON competition.game_configs, competition.game_config_versions, competition.content_releases
  FROM competition_app;
REVOKE DELETE, TRUNCATE ON competition.game_configs, competition.game_config_versions FROM competition_platform;
REVOKE UPDATE, DELETE, TRUNCATE ON competition.content_releases FROM competition_platform;

-- A published version never changes (R11, ADR-0015): only its status may move on (unpublished/archived).
CREATE FUNCTION competition.game_config_versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'published' AND (NEW.payload IS DISTINCT FROM OLD.payload OR NEW.version_no IS DISTINCT FROM OLD.version_no
    OR NEW.config_id IS DISTINCT FROM OLD.config_id OR NEW.effective_from IS DISTINCT FROM OLD.effective_from
    OR NEW.title IS DISTINCT FROM OLD.title OR NEW.published_at IS DISTINCT FROM OLD.published_at) THEN
    RAISE EXCEPTION 'GAME_CONFIG_VERSION_PUBLISHED' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER game_config_versions_immutable BEFORE UPDATE ON competition.game_config_versions FOR EACH ROW
  EXECUTE FUNCTION competition.game_config_versions_immutable();

-- ---------- Platform seed of the matchmaking configuration (CONFIG_DEFAULTS mm_*, grade_window_g6_9) ----------
-- Source: modules/competition/seed/matchmaking_config.json; the integration test TestSeedMatchmakingConfig checks this
-- literal equals the file. F108 publishes the next versions; F104 reads the latest published one in effect.
-- BEGIN seed matchmaking_config.json
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000104', now(), now(), 1, 'matchmaking-configs', 'default', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001041', now(), now(), 1, '01936a00-0000-7000-8000-000000000104', 1, 'published',
  'Matchmaking mặc định (CONFIG_DEFAULTS)', '{"schema_version":1,"rating_window":[{"after_s":0,"width":100},{"after_s":10,"width":200},{"after_s":20,"width":350},{"after_s":40,"width":null}],"ready_check_s":15,"bot_after_s":90,"ticket_ttl_s":300,"tickets_per_user":1,"ticket_rate":{"per_hour":30,"decline_limit":3,"decline_window_s":600,"cooldown_s":120},"grade_window_g6_9":{"after_s":30,"levels":1},"disconnect_cancel_s":30,"bucket_threshold":500,"batch":200,"cost_weights":{"rating":1,"rd":0.1,"recent_opponent":0.5,"same_class":0.2},"recent_opponent_window_s":1800,"default_rating":{"mu":1500,"rd":350},"modes":[{"mode":"battle_1v1","min_grade":1,"matchmaking_types":["random","skill","grade","subject","topic"],"ranked_types":["skill","subject"]},{"mode":"team_battle","min_grade":4,"matchmaking_types":["team"],"ranked_types":["team"],"team_size":3,"min_team_size":2,"max_team_size":4}],"subjects":[]}',
  '2026-01-01T00:00:00Z', 'Seed F104 theo CONFIG_DEFAULTS', '2026-01-01T00:00:00Z');
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001041'
  WHERE config_id = '01936a00-0000-7000-8000-000000000104';
INSERT INTO competition.content_releases (release_id, created_at, config_id, action, from_version_id, to_version_id, reason, released_at)
VALUES ('01936a00-0000-7000-8000-000000001042', now(), '01936a00-0000-7000-8000-000000000104', 'publish', NULL,
  '01936a00-0000-7000-8000-000000001041', 'Seed F104', '2026-01-01T00:00:00Z');
-- END seed matchmaking_config.json

-- ---------- _live views for reporting (competition_readonly) ----------
CREATE OR REPLACE VIEW competition.matches_live WITH (security_barrier = true) AS
  SELECT * FROM competition.matches WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.match_participants_live WITH (security_barrier = true) AS
  SELECT * FROM competition.match_participants WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.duels_live WITH (security_barrier = true) AS
  SELECT * FROM competition.duels WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.competition_restrictions_live WITH (security_barrier = true) AS
  SELECT * FROM competition.competition_restrictions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.game_configs_live WITH (security_barrier = true) AS
  SELECT * FROM competition.game_configs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW competition.game_config_versions_live WITH (security_barrier = true) AS
  SELECT * FROM competition.game_config_versions WHERE deleted_at IS NULL;
GRANT SELECT ON competition.matches_live, competition.match_participants_live, competition.duels_live,
  competition.competition_restrictions_live, competition.game_configs_live, competition.game_config_versions_live,
  competition.content_releases TO competition_readonly;
