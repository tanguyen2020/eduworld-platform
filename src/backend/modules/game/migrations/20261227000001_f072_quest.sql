-- F072 T008/T009 — quest engine of module game: quests as versioned platform game content on the F071 release core,
-- their assignment to tenants (ADR-0029), the progress of a student on a pinned quest version, the link between a
-- learning attempt and a quest step, and the daily/weekly grants (specs/072-quest-engine: data-model §1; DATABASE_DESIGN
-- §2.8, §3.0, §3.13; QUEST_SYSTEM §2–§5; ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089).
-- Applied by `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, the pointer trigger, immutability of
-- published versions, RLS of the tenant tables, platform-only writes of the content and the assignments, _live views.

-- BEGIN entddl
CREATE TABLE game.quest_definitions (
  quest_id uuid NOT NULL,
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
  PRIMARY KEY (quest_id)
);
CREATE UNIQUE INDEX quest_definitions_code_uq ON game.quest_definitions (code) WHERE deleted_at IS NULL;

CREATE TABLE game.quest_versions (
  quest_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  quest_id uuid NOT NULL,
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
  quest_type varchar(64) NOT NULL,
  grade_levels jsonb NOT NULL,
  subject_id uuid NOT NULL,
  subject_code varchar(64),
  area_id varchar(64),
  giver_npc_id varchar(64),
  story_chapter_code varchar(64),
  start_trigger varchar(64) NOT NULL,
  objective_ids jsonb NOT NULL,
  available_from timestamptz,
  available_to timestamptz,
  PRIMARY KEY (quest_version_id)
);
CREATE UNIQUE INDEX quest_versions_no_uq ON game.quest_versions (quest_id, version_no);

CREATE TABLE game.quest_progress (
  quest_progress_id uuid NOT NULL,
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
  quest_id uuid NOT NULL,
  quest_version_id uuid NOT NULL,
  quest_type varchar(64) NOT NULL,
  status varchar(64) NOT NULL,
  step_states jsonb NOT NULL,
  start_trigger varchar(64) NOT NULL,
  period_key varchar(16),
  completion_no bigint NOT NULL DEFAULT 1,
  party_id uuid,
  started_at timestamptz NOT NULL,
  completed_at timestamptz,
  expires_at timestamptz,
  deadline_at timestamptz,
  failed_at timestamptz,
  ended_at timestamptz,
  PRIMARY KEY (quest_progress_id)
);
CREATE UNIQUE INDEX quest_progress_one_active_uq ON game.quest_progress (tenant_id, student_id, quest_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE UNIQUE INDEX quest_progress_period_uq ON game.quest_progress (tenant_id, student_id, quest_id, period_key) WHERE period_key IS NOT NULL AND status IN ('active', 'completed') AND deleted_at IS NULL;
CREATE INDEX quest_progress_student_idx ON game.quest_progress (tenant_id, student_id, status) WHERE deleted_at IS NULL;

CREATE TABLE game.quest_step_attempts (
  quest_step_attempt_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  quest_progress_id uuid NOT NULL,
  step_id varchar(64) NOT NULL,
  student_id uuid NOT NULL,
  attempt_id uuid NOT NULL,
  content_version_id uuid,
  started_on varchar(10) NOT NULL,
  status varchar(64) NOT NULL,
  correct_count bigint,
  item_count bigint,
  graded_at timestamptz,
  PRIMARY KEY (quest_step_attempt_id)
);
CREATE UNIQUE INDEX quest_step_attempts_attempt_uq ON game.quest_step_attempts (tenant_id, attempt_id) WHERE deleted_at IS NULL;
CREATE INDEX quest_step_attempts_day_idx ON game.quest_step_attempts (tenant_id, quest_progress_id, step_id, started_on) WHERE deleted_at IS NULL;

CREATE TABLE game.periodic_quest_grants (
  grant_id uuid NOT NULL,
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
  period_key varchar(16) NOT NULL,
  quest_ids jsonb NOT NULL,
  granted_at timestamptz NOT NULL,
  PRIMARY KEY (grant_id)
);
CREATE UNIQUE INDEX periodic_quest_grants_uq ON game.periodic_quest_grants (tenant_id, student_id, kind, period_key) WHERE deleted_at IS NULL;

CREATE TABLE infra.tenant_resource_assignments (
  assignment_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  resource_type varchar(64) NOT NULL,
  resource_id uuid NOT NULL,
  assigned_by uuid NOT NULL,
  assigned_at timestamptz NOT NULL,
  PRIMARY KEY (assignment_id)
);
CREATE UNIQUE INDEX tra_active_uq ON infra.tenant_resource_assignments (resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX tra_tenant_idx ON infra.tenant_resource_assignments (tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- platform content (G): quest definitions and versions (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- student use cases (game_app) only read them. Published versions are immutable (R11, R31).
ALTER TABLE game.quest_versions
  ADD CONSTRAINT quest_versions_quest_fk FOREIGN KEY (quest_id) REFERENCES game.quest_definitions (quest_id),
  ADD CONSTRAINT quest_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT quest_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT quest_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT quest_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT quest_versions_type_chk CHECK (quest_type IN ('main','side','daily','weekly','learning','exploration','social',
    'party','challenge','seasonal','special_event','boss')),
  ADD CONSTRAINT quest_versions_trigger_chk CHECK (start_trigger IN ('npc','quest_board','auto_daily','auto_weekly','assignment',
    'event','chain','discovery','level_reached')),
  ADD CONSTRAINT quest_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels)
    BETWEEN 1 AND 9),
  ADD CONSTRAINT quest_versions_objectives_chk CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT quest_versions_window_chk CHECK (available_from IS NULL OR available_to IS NULL OR available_from < available_to);
CREATE INDEX quest_versions_grades_gin ON game.quest_versions USING gin (grade_levels jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE game.quest_definitions
  ADD CONSTRAINT quest_definitions_scope_chk CHECK (owner_scope = 'platform'),
  ADD CONSTRAINT quest_definitions_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT quest_definitions_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.quest_versions (quest_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER quest_definitions_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.quest_definitions
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('quest_versions', 'quest_version_id', 'quest_id');
CREATE TRIGGER quest_versions_immutable BEFORE UPDATE ON game.quest_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
REVOKE INSERT, UPDATE ON game.quest_definitions, game.quest_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.quest_definitions, game.quest_versions FROM game_app, game_platform;

-- ---------- infra.tenant_resource_assignments of eduworld_game (DATABASE_DESIGN §3.0, ADR-0029) ----------
-- Resource types owned by module game, tenant from the projection. No RLS (schema infra, ADR-0089 §4): game_app only
-- reads it with an explicit tenant_id condition; System Admin (saga F045) and the seed write it through game_platform.
ALTER TABLE infra.tenant_resource_assignments
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('quest','minigame_config','item')),
  ADD CONSTRAINT tra_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON infra.tenant_resource_assignments FROM game_app;
GRANT SELECT ON infra.tenant_resource_assignments TO game_app;
GRANT SELECT, INSERT, UPDATE ON infra.tenant_resource_assignments TO game_platform;
REVOKE DELETE, TRUNCATE ON infra.tenant_resource_assignments FROM game_platform;
CREATE OR REPLACE VIEW infra.tenant_resource_assignments_live WITH (security_barrier = true) AS
  SELECT * FROM infra.tenant_resource_assignments WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA infra TO game_readonly;
GRANT SELECT ON infra.tenant_resource_assignments_live TO game_readonly;
REVOKE ALL ON infra.tenant_resource_assignments FROM game_readonly;

-- ---------- tenant tables (T): progress, step attempts, periodic grants ----------
ALTER TABLE game.quest_progress
  ADD CONSTRAINT quest_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT quest_progress_quest_fk FOREIGN KEY (quest_id) REFERENCES game.quest_definitions (quest_id),
  ADD CONSTRAINT quest_progress_version_fk FOREIGN KEY (quest_version_id) REFERENCES game.quest_versions (quest_version_id),
  ADD CONSTRAINT quest_progress_status_chk CHECK (status IN ('active','completed','expired','abandoned','failed')),
  ADD CONSTRAINT quest_progress_type_chk CHECK (quest_type IN ('main','side','daily','weekly','learning','exploration','social',
    'party','challenge','seasonal','special_event','boss')),
  ADD CONSTRAINT quest_progress_trigger_chk CHECK (start_trigger IN ('npc','quest_board','auto_daily','auto_weekly','assignment',
    'event','chain','discovery','level_reached')),
  ADD CONSTRAINT quest_progress_steps_chk CHECK (jsonb_typeof(step_states) = 'object' AND octet_length(step_states::text) <= 65536),
  ADD CONSTRAINT quest_progress_period_chk CHECK (period_key IS NULL OR period_key ~ '^[0-9]{4}-(W[0-9]{2}|[0-9]{2}-[0-9]{2})$'),
  ADD CONSTRAINT quest_progress_no_chk CHECK (completion_no >= 1),
  ADD CONSTRAINT quest_progress_done_chk CHECK ((status = 'completed') = (completed_at IS NOT NULL)),
  ADD CONSTRAINT quest_progress_end_chk CHECK ((status = 'active') = (ended_at IS NULL));
CREATE INDEX quest_progress_expiry_idx ON game.quest_progress (expires_at) WHERE status = 'active' AND expires_at IS NOT NULL
  AND deleted_at IS NULL;
CREATE INDEX quest_progress_deadline_idx ON game.quest_progress (deadline_at) WHERE status = 'active' AND deadline_at IS NOT NULL
  AND deleted_at IS NULL;
ALTER TABLE game.quest_step_attempts
  ADD CONSTRAINT quest_step_attempts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT quest_step_attempts_progress_fk FOREIGN KEY (quest_progress_id) REFERENCES game.quest_progress (quest_progress_id),
  ADD CONSTRAINT quest_step_attempts_status_chk CHECK (status IN ('in_progress','passed','not_passed','expired','abandoned')),
  ADD CONSTRAINT quest_step_attempts_day_chk CHECK (started_on ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
  ADD CONSTRAINT quest_step_attempts_counts_chk CHECK (coalesce(correct_count, 0) >= 0 AND coalesce(item_count, 0) >= 0);
ALTER TABLE game.periodic_quest_grants
  ADD CONSTRAINT periodic_quest_grants_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT periodic_quest_grants_kind_chk CHECK (kind IN ('daily','weekly')),
  ADD CONSTRAINT periodic_quest_grants_period_chk CHECK (period_key ~ '^[0-9]{4}-(W[0-9]{2}|[0-9]{2}-[0-9]{2})$'),
  ADD CONSTRAINT periodic_quest_grants_ids_chk CHECK (jsonb_typeof(quest_ids) = 'array' AND jsonb_array_length(quest_ids) <= 10);

ALTER TABLE game.quest_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.quest_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.quest_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.quest_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.quest_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.quest_progress FROM game_app, game_platform;
ALTER TABLE game.quest_step_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.quest_step_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.quest_step_attempts FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.quest_step_attempts FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.quest_step_attempts FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.quest_step_attempts FROM game_app, game_platform;
ALTER TABLE game.periodic_quest_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.periodic_quest_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.periodic_quest_grants FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.periodic_quest_grants FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.periodic_quest_grants FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.periodic_quest_grants FROM game_app, game_platform;

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.quest_definitions_live WITH (security_barrier = true) AS
  SELECT * FROM game.quest_definitions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.quest_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.quest_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.quest_progress_live WITH (security_barrier = true) AS
  SELECT * FROM game.quest_progress WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.quest_step_attempts_live WITH (security_barrier = true) AS
  SELECT * FROM game.quest_step_attempts WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.periodic_quest_grants_live WITH (security_barrier = true) AS
  SELECT * FROM game.periodic_quest_grants WHERE deleted_at IS NULL;
GRANT SELECT ON game.quest_definitions_live, game.quest_versions_live, game.quest_progress_live, game.quest_step_attempts_live,
  game.periodic_quest_grants_live TO game_readonly;
