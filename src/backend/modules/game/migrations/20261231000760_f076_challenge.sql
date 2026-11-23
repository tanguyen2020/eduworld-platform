-- F076 T009/T010 — Challenge Engine solo of module game: challenges as versioned platform game content on the F071
-- release core, their assignment to tenants (ADR-0029), and the ChallengeAttempt of a student on a pinned version
-- (specs/076-challenge-engine-solo: data-model §1; DATABASE_DESIGN §2.8a, §3.13; CHALLENGE_BOSS_SYSTEM §2, §6;
-- ADR-0016, ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089). The attempt row is the checkpoint of its room
-- (pkg/roomhost): Valkey only holds the registry and the lease (ADR-0004).
-- Applied by `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, the pointer trigger, immutability of
-- published versions, RLS of the tenant table, the GIN index of the learning attempts, platform-only writes of the
-- content, _live views.

-- BEGIN entddl
CREATE TABLE game.challenges (
  challenge_id uuid NOT NULL,
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
  PRIMARY KEY (challenge_id)
);
CREATE UNIQUE INDEX challenges_code_uq ON game.challenges (code) WHERE deleted_at IS NULL;

CREATE TABLE game.challenge_versions (
  challenge_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  challenge_id uuid NOT NULL,
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
  challenge_type varchar(64) NOT NULL,
  mode varchar(64) NOT NULL,
  grade_levels jsonb NOT NULL,
  subject_id uuid NOT NULL,
  objective_ids jsonb NOT NULL,
  area_id varchar(64),
  available_from timestamptz,
  available_to timestamptz,
  PRIMARY KEY (challenge_version_id)
);
CREATE UNIQUE INDEX challenge_versions_no_uq ON game.challenge_versions (challenge_id, version_no);

CREATE TABLE game.challenge_attempts (
  challenge_attempt_id uuid NOT NULL,
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
  challenge_id uuid NOT NULL,
  challenge_version_id uuid NOT NULL,
  challenge_type varchar(64) NOT NULL,
  difficulty varchar(64) NOT NULL,
  party_id uuid,
  status varchar(64) NOT NULL,
  end_reason varchar(64),
  score bigint NOT NULL DEFAULT 0,
  items_answered bigint NOT NULL DEFAULT 0,
  correct_streak bigint NOT NULL DEFAULT 0,
  lives_left bigint,
  goal_progress numeric(10,2) NOT NULL DEFAULT 0,
  activities_completed bigint NOT NULL DEFAULT 0,
  period_key varchar(16) NOT NULL,
  first_clear boolean NOT NULL DEFAULT false,
  started_at timestamptz NOT NULL,
  deadline_at timestamptz,
  ended_at timestamptz,
  learning_attempt_ids jsonb NOT NULL,
  processed_attempt_ids jsonb NOT NULL,
  PRIMARY KEY (challenge_attempt_id)
);
CREATE UNIQUE INDEX challenge_attempts_one_active_uq ON game.challenge_attempts (student_id) WHERE status = 'in_progress' AND deleted_at IS NULL;
CREATE INDEX challenge_attempts_period_idx ON game.challenge_attempts (tenant_id, student_id, challenge_id, period_key) WHERE deleted_at IS NULL;
CREATE INDEX challenge_attempts_student_idx ON game.challenge_attempts (tenant_id, student_id, started_at) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- platform content (G): challenges and versions (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- student use cases (game_app) only read them. Published versions are immutable (R11, R31).
ALTER TABLE game.challenge_versions
  ADD CONSTRAINT challenge_versions_challenge_fk FOREIGN KEY (challenge_id) REFERENCES game.challenges (challenge_id),
  ADD CONSTRAINT challenge_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT challenge_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT challenge_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT challenge_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT challenge_versions_type_chk CHECK (challenge_type IN ('time','perfect','survival','puzzle','knowledge','boss','coop',
    'party','weekly','seasonal','tournament')),
  ADD CONSTRAINT challenge_versions_mode_chk CHECK (mode IN ('solo','party_optional','party_required')),
  ADD CONSTRAINT challenge_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels)
    BETWEEN 1 AND 9),
  ADD CONSTRAINT challenge_versions_objectives_chk CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids)
    BETWEEN 1 AND 30),
  ADD CONSTRAINT challenge_versions_window_chk CHECK (available_from IS NULL OR available_to IS NULL OR available_from < available_to);
CREATE INDEX challenge_versions_grades_gin ON game.challenge_versions USING gin (grade_levels jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE game.challenges
  ADD CONSTRAINT challenges_scope_chk CHECK (owner_scope = 'platform'),
  ADD CONSTRAINT challenges_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT challenges_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.challenge_versions (challenge_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER challenges_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.challenges
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('challenge_versions', 'challenge_version_id',
  'challenge_id');
CREATE TRIGGER challenge_versions_immutable BEFORE UPDATE ON game.challenge_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
REVOKE INSERT, UPDATE ON game.challenges, game.challenge_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.challenges, game.challenge_versions FROM game_app, game_platform;

-- ---------- tenant table (T): challenge attempts ----------
ALTER TABLE game.challenge_attempts
  ADD CONSTRAINT challenge_attempts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT challenge_attempts_challenge_fk FOREIGN KEY (challenge_id) REFERENCES game.challenges (challenge_id),
  ADD CONSTRAINT challenge_attempts_version_fk FOREIGN KEY (challenge_version_id) REFERENCES game.challenge_versions (challenge_version_id),
  ADD CONSTRAINT challenge_attempts_type_chk CHECK (challenge_type IN ('time','perfect','survival','puzzle','knowledge','weekly','seasonal')),
  ADD CONSTRAINT challenge_attempts_difficulty_chk CHECK (difficulty IN ('easy','normal','hard','expert','legendary')),
  ADD CONSTRAINT challenge_attempts_status_chk CHECK (status IN ('in_progress','succeeded','failed','abandoned')),
  ADD CONSTRAINT challenge_attempts_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('goal_reached','lives_depleted',
    'accuracy_missed','deadline_passed','activities_exhausted','player','resume_expired','student_unavailable')),
  ADD CONSTRAINT challenge_attempts_party_chk CHECK (party_id IS NULL),
  ADD CONSTRAINT challenge_attempts_counts_chk CHECK (score >= 0 AND items_answered >= score AND correct_streak >= 0
    AND activities_completed BETWEEN 0 AND 200 AND (lives_left IS NULL OR lives_left BETWEEN 0 AND 10) AND goal_progress >= 0),
  ADD CONSTRAINT challenge_attempts_period_chk CHECK (period_key ~ '^([0-9]{4}-(W[0-9]{2}|[0-9]{2}-[0-9]{2})|S[0-9]{4}-[0-9]{2}-[0-9]{2})$'),
  ADD CONSTRAINT challenge_attempts_end_chk CHECK ((status = 'in_progress') = (ended_at IS NULL)
    AND (status = 'in_progress') = (end_reason IS NULL)),
  ADD CONSTRAINT challenge_attempts_first_clear_chk CHECK (NOT first_clear OR status = 'succeeded'),
  ADD CONSTRAINT challenge_attempts_lists_chk CHECK (jsonb_typeof(learning_attempt_ids) = 'array'
    AND jsonb_array_length(learning_attempt_ids) <= 200 AND jsonb_typeof(processed_attempt_ids) = 'array'
    AND jsonb_array_length(processed_attempt_ids) <= 200);
-- activity.graded finds its challenge attempt by learning attempt id; the deadline job scans the attempts in progress.
CREATE INDEX challenge_attempts_learning_gin ON game.challenge_attempts USING gin (learning_attempt_ids jsonb_path_ops)
  WHERE status = 'in_progress' AND deleted_at IS NULL;
CREATE INDEX challenge_attempts_deadline_idx ON game.challenge_attempts (deadline_at) WHERE status = 'in_progress'
  AND deadline_at IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX challenge_attempts_resume_idx ON game.challenge_attempts (started_at) WHERE status = 'in_progress'
  AND deadline_at IS NULL AND deleted_at IS NULL;
CREATE INDEX challenge_attempts_cleared_idx ON game.challenge_attempts (tenant_id, student_id, challenge_id)
  WHERE status = 'succeeded' AND deleted_at IS NULL;
ALTER TABLE game.challenge_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.challenge_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.challenge_attempts FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.challenge_attempts FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.challenge_attempts FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.challenge_attempts FROM game_app, game_platform;

-- The release history accepts the challenge item type; resource assignments accept challenges (ADR-0029).
ALTER TABLE game.content_releases DROP CONSTRAINT content_releases_type_chk,
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config','progression_curve','reward_rule','shop_item','collection','achievement',
    'event','event_template','season_track','challenge'));
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck,
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('quest','minigame_config','item','shop_item','event_template','challenge'));
-- The XP ledger accepts the source type of challenge.completed (Reward Engine, source_id = challenge_attempt_id).
ALTER TABLE gamification.xp_ledger DROP CONSTRAINT xp_ledger_source_type_chk,
  ADD CONSTRAINT xp_ledger_source_type_chk CHECK (source_type IN ('activity','quest','minigame','lesson','objective','world','merge',
    'adjustment','challenge'));

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.challenges_live WITH (security_barrier = true) AS
  SELECT * FROM game.challenges WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.challenge_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.challenge_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.challenge_attempts_live WITH (security_barrier = true) AS
  SELECT * FROM game.challenge_attempts WHERE deleted_at IS NULL;
GRANT SELECT ON game.challenges_live, game.challenge_versions_live, game.challenge_attempts_live TO game_readonly;
