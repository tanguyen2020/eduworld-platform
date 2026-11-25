-- F103 T009/T010 — Boss battles with several phases and co-op quests of module game: bosses as versioned platform game
-- content on the F071 release core, their assignment to tenants (ADR-0029), the boss encounters (members, contributions,
-- question plans of module learning, graded answers) and the shared progress of the co-op quests
-- (specs/103-boss-coop-quest: data-model §1; DATABASE_DESIGN §2.8a; CHALLENGE_BOSS_SYSTEM §3, §6; QUEST_SYSTEM §3.2, §6;
-- ADR-0016, ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0089). The encounter and session rows are the checkpoint of their
-- room (pkg/roomhost): Valkey only holds the registry and the lease (ADR-0004).
-- Applied by `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, the pointer trigger, immutability of
-- published versions, RLS of the tenant tables, append-only answers, platform-only writes of the content, _live views.

-- BEGIN entddl
CREATE TABLE game.boss_definitions (
  boss_id uuid NOT NULL,
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
  PRIMARY KEY (boss_id)
);
CREATE UNIQUE INDEX boss_definitions_code_uq ON game.boss_definitions (code) WHERE deleted_at IS NULL;

CREATE TABLE game.boss_versions (
  boss_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  boss_id uuid NOT NULL,
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
  mode varchar(64) NOT NULL,
  grade_levels jsonb NOT NULL,
  subject_id uuid NOT NULL,
  area_id varchar(64) NOT NULL,
  phase_count bigint NOT NULL,
  PRIMARY KEY (boss_version_id)
);
CREATE UNIQUE INDEX boss_versions_no_uq ON game.boss_versions (boss_id, version_no);

CREATE TABLE game.boss_encounters (
  encounter_id uuid NOT NULL,
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
  boss_id uuid NOT NULL,
  boss_version_id uuid NOT NULL,
  mode varchar(64) NOT NULL,
  party_id uuid,
  party_activity_id uuid,
  leader_id uuid,
  status varchar(64) NOT NULL,
  phase_index bigint NOT NULL DEFAULT 0,
  current_phase varchar(64),
  phase_retries bigint NOT NULL DEFAULT 0,
  plan_failures bigint NOT NULL DEFAULT 0,
  phases_cleared bigint NOT NULL DEFAULT 0,
  energy_remaining numeric(5,4) NOT NULL DEFAULT 1,
  state_version bigint NOT NULL DEFAULT 0,
  ready_deadline_at timestamptz NOT NULL,
  deadline_at timestamptz,
  phase_deadline_at timestamptz,
  outcome varchar(64),
  end_reason varchar(64),
  period_key varchar(16) NOT NULL,
  started_at timestamptz,
  ended_at timestamptz,
  PRIMARY KEY (encounter_id)
);
CREATE INDEX boss_encounters_live_idx ON game.boss_encounters (status) WHERE status IN ('lobby','phase_active') AND deleted_at IS NULL;
CREATE UNIQUE INDEX boss_encounters_party_activity_uq ON game.boss_encounters (party_activity_id) WHERE party_activity_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX boss_encounters_party_idx ON game.boss_encounters (tenant_id, party_id) WHERE party_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE game.boss_encounter_members (
  boss_encounter_member_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  encounter_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level bigint NOT NULL,
  join_order bigint NOT NULL,
  ready boolean NOT NULL DEFAULT false,
  joined_at timestamptz NOT NULL,
  disconnected_at timestamptz,
  left_at timestamptz,
  left_reason varchar(64),
  eligible boolean,
  first_defeat boolean NOT NULL DEFAULT false,
  PRIMARY KEY (boss_encounter_member_id)
);
CREATE INDEX boss_encounter_members_student_idx ON game.boss_encounter_members (tenant_id, student_id, joined_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX boss_encounter_members_uq ON game.boss_encounter_members (encounter_id, student_id) WHERE deleted_at IS NULL;

CREATE TABLE game.boss_encounter_contributions (
  contribution_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  encounter_id uuid NOT NULL,
  student_id uuid NOT NULL,
  phase_id varchar(64) NOT NULL,
  correct_count bigint NOT NULL DEFAULT 0,
  activities_completed bigint NOT NULL DEFAULT 0,
  contribution_pct numeric(5,2) NOT NULL DEFAULT 0,
  eligible boolean NOT NULL DEFAULT false,
  cleared boolean NOT NULL DEFAULT false,
  PRIMARY KEY (contribution_id)
);
CREATE UNIQUE INDEX boss_encounter_contributions_uq ON game.boss_encounter_contributions (encounter_id, student_id, phase_id) WHERE deleted_at IS NULL;

CREATE TABLE game.boss_phase_plans (
  boss_phase_plan_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  encounter_id uuid NOT NULL,
  phase_id varchar(64) NOT NULL,
  retry_no bigint NOT NULL,
  student_id uuid NOT NULL,
  requirement_id varchar(64) NOT NULL,
  plan_id uuid NOT NULL,
  item_count bigint NOT NULL,
  items jsonb NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (boss_phase_plan_id)
);
CREATE INDEX boss_phase_plans_plan_idx ON game.boss_phase_plans (plan_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX boss_phase_plans_uq ON game.boss_phase_plans (encounter_id, phase_id, retry_no, student_id, requirement_id) WHERE deleted_at IS NULL;

CREATE TABLE game.boss_answer_results (
  answer_result_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  encounter_id uuid NOT NULL,
  student_id uuid NOT NULL,
  plan_id uuid NOT NULL,
  item_index bigint NOT NULL,
  phase_id varchar(64) NOT NULL,
  retry_no bigint NOT NULL,
  requirement_id varchar(64) NOT NULL,
  is_correct boolean NOT NULL,
  counted boolean NOT NULL,
  rapid_guess boolean NOT NULL DEFAULT false,
  received_at timestamptz NOT NULL,
  PRIMARY KEY (answer_result_id)
);
CREATE INDEX boss_answer_results_member_idx ON game.boss_answer_results (encounter_id, student_id);
CREATE UNIQUE INDEX boss_answer_results_uq ON game.boss_answer_results (plan_id, item_index, student_id);

CREATE TABLE game.party_quest_sessions (
  party_quest_session_id uuid NOT NULL,
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
  party_id uuid NOT NULL,
  party_activity_id uuid NOT NULL,
  leader_id uuid NOT NULL,
  quest_id uuid NOT NULL,
  quest_version_id uuid NOT NULL,
  party_mode varchar(64) NOT NULL,
  shared_progress_policy varchar(64) NOT NULL,
  party_size_min bigint NOT NULL,
  status varchar(64) NOT NULL,
  member_ids jsonb NOT NULL,
  member_states jsonb NOT NULL,
  shared_step_states jsonb NOT NULL,
  role_assignments jsonb NOT NULL,
  shared_clues jsonb NOT NULL,
  state_version bigint NOT NULL DEFAULT 0,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  end_reason varchar(64),
  PRIMARY KEY (party_quest_session_id)
);
CREATE UNIQUE INDEX party_quest_sessions_activity_uq ON game.party_quest_sessions (party_activity_id) WHERE deleted_at IS NULL;
CREATE INDEX party_quest_sessions_party_idx ON game.party_quest_sessions (tenant_id, party_id, quest_id) WHERE status = 'active' AND deleted_at IS NULL;

CREATE TABLE game.party_quest_contributions (
  party_quest_contribution_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  party_quest_session_id uuid NOT NULL,
  student_id uuid NOT NULL,
  step_id varchar(64) NOT NULL,
  metric_value bigint NOT NULL DEFAULT 0,
  min_contribution_met boolean NOT NULL DEFAULT false,
  attempt_ids jsonb NOT NULL,
  PRIMARY KEY (party_quest_contribution_id)
);
CREATE UNIQUE INDEX party_quest_contributions_uq ON game.party_quest_contributions (party_quest_session_id, student_id, step_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- platform content (G): bosses and versions (ADR-0043) ----------
-- Written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations Console F078); the
-- student use cases (game_app) only read them. Published versions are immutable (R11, R31).
ALTER TABLE game.boss_versions
  ADD CONSTRAINT boss_versions_boss_fk FOREIGN KEY (boss_id) REFERENCES game.boss_definitions (boss_id),
  ADD CONSTRAINT boss_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT boss_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT boss_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT boss_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT boss_versions_mode_chk CHECK (mode IN ('solo','party_optional','party_required')),
  ADD CONSTRAINT boss_versions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) BETWEEN 1 AND 9),
  ADD CONSTRAINT boss_versions_phases_chk CHECK (phase_count BETWEEN 1 AND 8);
CREATE INDEX boss_versions_grades_gin ON game.boss_versions USING gin (grade_levels jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE game.boss_definitions
  ADD CONSTRAINT boss_definitions_scope_chk CHECK (owner_scope = 'platform'),
  ADD CONSTRAINT boss_definitions_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT boss_definitions_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.boss_versions (boss_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER boss_definitions_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.boss_definitions
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION game.check_current_version('boss_versions', 'boss_version_id', 'boss_id');
CREATE TRIGGER boss_versions_immutable BEFORE UPDATE ON game.boss_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();
REVOKE INSERT, UPDATE ON game.boss_definitions, game.boss_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.boss_definitions, game.boss_versions FROM game_app, game_platform;

-- ---------- tenant tables (T): encounters, members, contributions, plans ----------
ALTER TABLE game.boss_encounters
  ADD CONSTRAINT boss_encounters_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT boss_encounters_boss_fk FOREIGN KEY (boss_id) REFERENCES game.boss_definitions (boss_id),
  ADD CONSTRAINT boss_encounters_version_fk FOREIGN KEY (boss_version_id) REFERENCES game.boss_versions (boss_version_id),
  ADD CONSTRAINT boss_encounters_mode_chk CHECK (mode IN ('solo','party')),
  ADD CONSTRAINT boss_encounters_party_chk CHECK ((mode = 'party') = (party_id IS NOT NULL AND party_activity_id IS NOT NULL)),
  ADD CONSTRAINT boss_encounters_status_chk CHECK (status IN ('lobby','phase_active','phase_transition','phase_retry','defeated','ended')),
  ADD CONSTRAINT boss_encounters_outcome_chk CHECK (outcome IS NULL OR outcome IN ('defeated','time_up','abandoned')),
  ADD CONSTRAINT boss_encounters_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('defeated','total_time','retries_exhausted',
    'phase_failed','no_ready','below_min','plan_failure','recovery_lost','student_unavailable')),
  ADD CONSTRAINT boss_encounters_end_chk CHECK ((status IN ('defeated','ended')) = (ended_at IS NOT NULL)
    AND (status IN ('defeated','ended')) = (outcome IS NOT NULL) AND (status = 'defeated') = (outcome IS NOT DISTINCT FROM 'defeated')),
  ADD CONSTRAINT boss_encounters_counts_chk CHECK (phase_index BETWEEN 0 AND 7 AND phase_retries BETWEEN 0 AND 10
    AND plan_failures BETWEEN 0 AND 10 AND phases_cleared BETWEEN 0 AND 8 AND energy_remaining BETWEEN 0 AND 1 AND state_version >= 0),
  ADD CONSTRAINT boss_encounters_period_chk CHECK (period_key ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$');
ALTER TABLE game.boss_encounter_members
  ADD CONSTRAINT boss_encounter_members_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT boss_encounter_members_encounter_fk FOREIGN KEY (encounter_id) REFERENCES game.boss_encounters (encounter_id),
  ADD CONSTRAINT boss_encounter_members_grade_chk CHECK (grade_level BETWEEN 1 AND 9 AND join_order BETWEEN 1 AND 8),
  ADD CONSTRAINT boss_encounter_members_left_chk CHECK ((left_at IS NULL) = (left_reason IS NULL)
    AND (left_reason IS NULL OR left_reason IN ('left','disconnected','kicked','party_left','disbanded')));
ALTER TABLE game.boss_encounter_contributions
  ADD CONSTRAINT boss_encounter_contributions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT boss_encounter_contributions_encounter_fk FOREIGN KEY (encounter_id) REFERENCES game.boss_encounters (encounter_id),
  ADD CONSTRAINT boss_encounter_contributions_counts_chk CHECK (correct_count >= 0 AND activities_completed >= 0
    AND contribution_pct BETWEEN 0 AND 100 AND (cleared OR NOT eligible));
ALTER TABLE game.boss_phase_plans
  ADD CONSTRAINT boss_phase_plans_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT boss_phase_plans_encounter_fk FOREIGN KEY (encounter_id) REFERENCES game.boss_encounters (encounter_id),
  ADD CONSTRAINT boss_phase_plans_items_chk CHECK (jsonb_typeof(items) = 'array' AND jsonb_array_length(items) = item_count
    AND item_count BETWEEN 1 AND 30 AND octet_length(items::text) <= 65536 AND retry_no BETWEEN 0 AND 10);
ALTER TABLE game.boss_answer_results
  ADD CONSTRAINT boss_answer_results_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT boss_answer_results_encounter_fk FOREIGN KEY (encounter_id) REFERENCES game.boss_encounters (encounter_id),
  ADD CONSTRAINT boss_answer_results_index_chk CHECK (item_index BETWEEN 0 AND 29 AND retry_no BETWEEN 0 AND 10
    AND (NOT counted OR (is_correct AND NOT rapid_guess)));
ALTER TABLE game.party_quest_sessions
  ADD CONSTRAINT party_quest_sessions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT party_quest_sessions_quest_fk FOREIGN KEY (quest_id) REFERENCES game.quest_definitions (quest_id),
  ADD CONSTRAINT party_quest_sessions_version_fk FOREIGN KEY (quest_version_id) REFERENCES game.quest_versions (quest_version_id),
  ADD CONSTRAINT party_quest_sessions_policy_chk CHECK (shared_progress_policy IN ('individual','shared_steps','collective_count')),
  ADD CONSTRAINT party_quest_sessions_mode_chk CHECK (party_mode IN ('party_optional','party_required')),
  ADD CONSTRAINT party_quest_sessions_status_chk CHECK (status IN ('active','completed','failed','abandoned')),
  ADD CONSTRAINT party_quest_sessions_end_chk CHECK ((status = 'active') = (ended_at IS NULL) AND (status = 'active') = (end_reason IS NULL)
    AND (end_reason IS NULL OR end_reason IN ('completed','failed','disbanded','abandoned'))),
  ADD CONSTRAINT party_quest_sessions_docs_chk CHECK (jsonb_typeof(member_ids) = 'array' AND jsonb_array_length(member_ids) <= 8
    AND jsonb_typeof(member_states) = 'array' AND jsonb_array_length(member_states) <= 8 AND octet_length(member_states::text) <= 8192
    AND jsonb_typeof(shared_step_states) = 'object' AND octet_length(shared_step_states::text) <= 16384
    AND jsonb_typeof(role_assignments) = 'object' AND octet_length(role_assignments::text) <= 8192
    AND jsonb_typeof(shared_clues) = 'array' AND octet_length(shared_clues::text) <= 8192),
  ADD CONSTRAINT party_quest_sessions_size_chk CHECK (party_size_min BETWEEN 1 AND 8 AND state_version >= 0);
-- The consumers find the active session of a party quest a student is a present member of (member_ids ? student).
CREATE INDEX party_quest_sessions_members_gin ON game.party_quest_sessions USING gin (member_ids)
  WHERE status = 'active' AND deleted_at IS NULL;
ALTER TABLE game.party_quest_contributions
  ADD CONSTRAINT party_quest_contributions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT party_quest_contributions_session_fk FOREIGN KEY (party_quest_session_id)
    REFERENCES game.party_quest_sessions (party_quest_session_id),
  ADD CONSTRAINT party_quest_contributions_metric_chk CHECK (metric_value >= 0 AND jsonb_typeof(attempt_ids) = 'array'
    AND jsonb_array_length(attempt_ids) <= 200);

ALTER TABLE game.boss_encounters ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.boss_encounters FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.boss_encounters FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.boss_encounters FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.boss_encounters FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.boss_encounter_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.boss_encounter_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.boss_encounter_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.boss_encounter_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.boss_encounter_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.boss_encounter_contributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.boss_encounter_contributions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.boss_encounter_contributions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.boss_encounter_contributions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.boss_encounter_contributions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.boss_phase_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.boss_phase_plans FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.boss_phase_plans FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.boss_phase_plans FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.boss_phase_plans FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
-- Append-only (TA): the graded answers are never changed (R14: no raw response is stored).
ALTER TABLE game.boss_answer_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.boss_answer_results FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.boss_answer_results FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.boss_answer_results FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE ON game.boss_answer_results FROM game_app, game_platform;
ALTER TABLE game.party_quest_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.party_quest_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.party_quest_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.party_quest_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.party_quest_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.party_quest_contributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.party_quest_contributions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.party_quest_contributions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.party_quest_contributions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.party_quest_contributions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.boss_encounters, game.boss_encounter_members, game.boss_encounter_contributions, game.boss_phase_plans,
  game.boss_answer_results, game.party_quest_sessions, game.party_quest_contributions FROM game_app, game_platform;

-- The release history accepts the boss item type; resource assignments accept bosses (ADR-0029); the XP ledger accepts the
-- source type of the boss rewards (Reward Engine, source_id = encounter_id).
ALTER TABLE game.content_releases DROP CONSTRAINT content_releases_type_chk,
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config','progression_curve','reward_rule','shop_item','collection','achievement',
    'event','event_template','season_track','challenge','boss'));
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck,
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('quest','minigame_config','item','shop_item','event_template','challenge',
    'boss'));
ALTER TABLE gamification.xp_ledger DROP CONSTRAINT xp_ledger_source_type_chk,
  ADD CONSTRAINT xp_ledger_source_type_chk CHECK (source_type IN ('activity','quest','minigame','lesson','objective','world','merge',
    'adjustment','challenge','boss'));

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.boss_definitions_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_definitions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.boss_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.boss_encounters_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_encounters WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.boss_encounter_members_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_encounter_members WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.boss_encounter_contributions_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_encounter_contributions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.boss_phase_plans_live WITH (security_barrier = true) AS
  SELECT * FROM game.boss_phase_plans WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.party_quest_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM game.party_quest_sessions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.party_quest_contributions_live WITH (security_barrier = true) AS
  SELECT * FROM game.party_quest_contributions WHERE deleted_at IS NULL;
GRANT SELECT ON game.boss_definitions_live, game.boss_versions_live, game.boss_encounters_live, game.boss_encounter_members_live,
  game.boss_encounter_contributions_live, game.boss_phase_plans_live, game.party_quest_sessions_live, game.party_quest_contributions_live
  TO game_readonly;
