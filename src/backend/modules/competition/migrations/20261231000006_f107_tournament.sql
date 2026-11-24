-- F107 T006 — tournaments (6 formats, brackets, schedule), grade challenges and school tournaments from platform templates,
-- template assignments of module competition (specs/107-tournament-challenges/data-model.md; DATABASE_DESIGN §2.10, §3.0;
-- COMPETITION_SYSTEM §10; ADR-0004, ADR-0027, ADR-0028, ADR-0029, ADR-0041, ADR-0095). Applied by `migrate --module=competition`
-- as competition_migrator. Expand only (AGENTS R12): new tables and seed rows. Brackets, results and rewards are durable in
-- PostgreSQL (FR-TRN-003); audiences are grades ± students of one school, never a class (ADR-0095).

-- BEGIN entddl
CREATE TABLE competition.challenges (
  challenge_id uuid NOT NULL,
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
  template_config_id uuid NOT NULL,
  template_version_id uuid NOT NULL,
  kind varchar(64) NOT NULL,
  format varchar(64) NOT NULL,
  name varchar(128) NOT NULL,
  grade_level smallint,
  targets jsonb NOT NULL,
  exclude_student_ids jsonb NOT NULL,
  participation varchar(64) NOT NULL DEFAULT 'individual',
  subject_id uuid NOT NULL,
  question_set_version_id uuid,
  params jsonb NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  join_opens_at timestamptz NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'scheduled',
  tournament_id uuid,
  match_ids jsonb,
  cancel_reason varchar(1000),
  PRIMARY KEY (challenge_id)
);
CREATE INDEX challenges_due_idx ON competition.challenges (status, starts_at) WHERE deleted_at IS NULL;
CREATE INDEX challenges_keyset_idx ON competition.challenges (tenant_id, starts_at, challenge_id) WHERE deleted_at IS NULL;
CREATE INDEX challenges_school_status_idx ON competition.challenges (tenant_id, school_id, status, starts_at) WHERE deleted_at IS NULL;

CREATE TABLE competition.tournaments (
  tournament_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid,
  owner_scope varchar(64) NOT NULL,
  owner_id uuid NOT NULL,
  challenge_id uuid,
  template_config_id uuid NOT NULL,
  rule_version_id uuid NOT NULL,
  name varchar(128) NOT NULL,
  format varchar(64) NOT NULL,
  participant_type varchar(64) NOT NULL DEFAULT 'individual',
  team_size smallint,
  mode varchar(64) NOT NULL,
  subject_id uuid NOT NULL,
  topic_ids jsonb,
  grade_levels jsonb NOT NULL,
  question_policy jsonb,
  registration_open_at timestamptz NOT NULL,
  registration_close_at timestamptz NOT NULL,
  check_in_opens_at timestamptz NOT NULL,
  starts_at timestamptz NOT NULL,
  max_entries smallint NOT NULL,
  min_entries smallint NOT NULL,
  seeding_policy varchar(64) NOT NULL,
  seed_value bigint,
  auto_confirm boolean NOT NULL DEFAULT false,
  reward_rule_refs jsonb NOT NULL,
  reward_summary varchar(1000),
  round_gap_s bigint NOT NULL DEFAULT 300,
  status varchar(64) NOT NULL DEFAULT 'draft',
  cancel_reason varchar(1000),
  completed_at timestamptz,
  rewarded_at timestamptz,
  PRIMARY KEY (tournament_id)
);
CREATE UNIQUE INDEX tournaments_challenge_uq ON competition.tournaments (challenge_id) WHERE challenge_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX tournaments_due_idx ON competition.tournaments (status, starts_at) WHERE deleted_at IS NULL;
CREATE INDEX tournaments_keyset_idx ON competition.tournaments (tenant_id, starts_at, tournament_id) WHERE deleted_at IS NULL;
CREATE INDEX tournaments_school_status_idx ON competition.tournaments (tenant_id, school_id, status) WHERE deleted_at IS NULL;

CREATE TABLE competition.tournament_entries (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  tournament_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid,
  party_id uuid,
  member_ids jsonb NOT NULL,
  seed smallint,
  group_id uuid,
  status varchar(64) NOT NULL DEFAULT 'registered',
  registered_by uuid NOT NULL,
  checked_in_at timestamptz,
  PRIMARY KEY (entry_id)
);
CREATE UNIQUE INDEX tournament_entries_party_uq ON competition.tournament_entries (tournament_id, party_id) WHERE deleted_at IS NULL AND party_id IS NOT NULL;
CREATE INDEX tournament_entries_status_idx ON competition.tournament_entries (tenant_id, tournament_id, status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tournament_entries_student_uq ON competition.tournament_entries (tournament_id, student_id) WHERE deleted_at IS NULL AND student_id IS NOT NULL;

CREATE TABLE competition.tournament_groups (
  group_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  tournament_id uuid NOT NULL,
  code varchar(16) NOT NULL,
  standings jsonb,
  PRIMARY KEY (group_id)
);
CREATE UNIQUE INDEX tournament_groups_code_uq ON competition.tournament_groups (tournament_id, code) WHERE deleted_at IS NULL;

CREATE TABLE competition.bracket_nodes (
  node_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  tournament_id uuid NOT NULL,
  bracket varchar(64) NOT NULL,
  round smallint NOT NULL,
  position smallint NOT NULL,
  group_id uuid,
  entry_a_id uuid,
  entry_b_id uuid,
  winner_entry_id uuid,
  is_draw boolean NOT NULL DEFAULT false,
  result jsonb,
  match_id uuid,
  scheduled_at timestamptz,
  started_at timestamptz,
  next_win_node_id uuid,
  next_win_slot smallint,
  next_lose_node_id uuid,
  next_lose_slot smallint,
  wave smallint NOT NULL DEFAULT 0,
  attempts smallint NOT NULL DEFAULT 0,
  reschedule_count smallint NOT NULL DEFAULT 0,
  notices smallint NOT NULL DEFAULT 0,
  status varchar(64) NOT NULL DEFAULT 'pending',
  confirm_after timestamptz,
  decision_required boolean NOT NULL DEFAULT false,
  override_reason varchar(1000),
  overridden_by uuid,
  PRIMARY KEY (node_id)
);
CREATE INDEX bracket_nodes_due_idx ON competition.bracket_nodes (status, scheduled_at) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX bracket_nodes_match_uq ON competition.bracket_nodes (match_id) WHERE match_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX bracket_nodes_slot_uq ON competition.bracket_nodes (tournament_id, bracket, round, position) WHERE deleted_at IS NULL;

CREATE TABLE competition.tournament_results (
  result_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  tournament_id uuid NOT NULL,
  entry_id uuid NOT NULL,
  school_id uuid NOT NULL,
  final_rank smallint NOT NULL,
  wins smallint NOT NULL,
  losses smallint NOT NULL,
  draws smallint NOT NULL,
  points bigint NOT NULL,
  matches_played smallint NOT NULL,
  PRIMARY KEY (result_id)
);
CREATE UNIQUE INDEX tournament_results_entry_uq ON competition.tournament_results (tournament_id, entry_id);

CREATE TABLE competition.tournament_rewards (
  reward_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  tournament_id uuid NOT NULL,
  entry_id uuid NOT NULL,
  rule_id varchar(64) NOT NULL,
  student_ids jsonb NOT NULL,
  reward_ref jsonb NOT NULL,
  points bigint NOT NULL DEFAULT 0,
  granted_at timestamptz NOT NULL,
  PRIMARY KEY (reward_id)
);
CREATE UNIQUE INDEX tournament_rewards_rule_uq ON competition.tournament_rewards (tournament_id, entry_id, rule_id);

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

-- ---------- competition.challenges (T): grade challenges / school tournaments of one school (ADR-0095) ----------
ALTER TABLE competition.challenges
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT challenges_kind_chk CHECK (kind IN ('grade_challenge','school_tournament')),
  ADD CONSTRAINT challenges_format_chk CHECK (format IN ('timed_quiz','tournament')),
  ADD CONSTRAINT challenges_participation_chk CHECK (participation IN ('individual')),
  ADD CONSTRAINT challenges_status_chk CHECK (status IN ('scheduled','running','completed','cancelled')),
  ADD CONSTRAINT challenges_grade_chk CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT challenges_window_chk CHECK (ends_at > starts_at AND join_opens_at <= starts_at),
  ADD CONSTRAINT challenges_targets_chk CHECK (jsonb_typeof(targets) = 'array' AND jsonb_array_length(targets) BETWEEN 1 AND 2
    AND octet_length(targets::text) <= 20480),
  ADD CONSTRAINT challenges_exclude_chk CHECK (jsonb_typeof(exclude_student_ids) = 'array' AND jsonb_array_length(exclude_student_ids) <= 500),
  ADD CONSTRAINT challenges_params_chk CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 8192),
  ADD CONSTRAINT challenges_matches_chk CHECK (match_ids IS NULL OR (jsonb_typeof(match_ids) = 'array' AND jsonb_array_length(match_ids) <= 50)),
  ADD CONSTRAINT challenges_template_fk FOREIGN KEY (template_config_id) REFERENCES competition.game_configs (config_id),
  ADD CONSTRAINT challenges_version_fk FOREIGN KEY (template_version_id) REFERENCES competition.game_config_versions (config_version_id),
  ADD CONSTRAINT challenges_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.challenges FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.challenges FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.challenges FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.challenges FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.challenges FROM competition_app, competition_platform;

-- ---------- competition.tournaments (T; platform tournaments prepared by tenant_ids, no API in F107) ----------
ALTER TABLE competition.tournaments
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD COLUMN tenant_ids uuid[],
  ADD CONSTRAINT tournaments_scope_chk CHECK (owner_scope IN ('school','tenant','platform')),
  ADD CONSTRAINT tournaments_school_chk CHECK (owner_scope <> 'school' OR school_id IS NOT NULL),
  ADD CONSTRAINT tournaments_format_chk CHECK (format IN ('single_elim','double_elim','round_robin','groups_then_single_elim')),
  ADD CONSTRAINT tournaments_participant_chk CHECK (participant_type IN ('individual','team')),
  ADD CONSTRAINT tournaments_team_chk CHECK ((participant_type = 'team') = (team_size IS NOT NULL) AND (team_size IS NULL OR team_size BETWEEN 2 AND 8)),
  ADD CONSTRAINT tournaments_mode_chk CHECK (mode IN ('battle_1v1','team_battle')),
  ADD CONSTRAINT tournaments_entries_chk CHECK (max_entries BETWEEN 2 AND 256 AND min_entries BETWEEN 2 AND max_entries),
  ADD CONSTRAINT tournaments_seeding_chk CHECK (seeding_policy IN ('rating','random','manual')),
  ADD CONSTRAINT tournaments_status_chk CHECK (status IN ('draft','published','registration_open','registration_closed','seeding',
    'scheduled','in_progress','completed','rewarded','archived','cancelled')),
  ADD CONSTRAINT tournaments_window_chk CHECK (registration_open_at < registration_close_at AND registration_close_at <= check_in_opens_at
    AND check_in_opens_at <= starts_at),
  ADD CONSTRAINT tournaments_gap_chk CHECK (round_gap_s BETWEEN 60 AND 604800),
  ADD CONSTRAINT tournaments_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) BETWEEN 1 AND 9),
  ADD CONSTRAINT tournaments_topics_chk CHECK (topic_ids IS NULL OR (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50)),
  ADD CONSTRAINT tournaments_policy_chk CHECK (question_policy IS NULL OR (jsonb_typeof(question_policy) = 'object'
    AND octet_length(question_policy::text) <= 4096)),
  ADD CONSTRAINT tournaments_rewards_chk CHECK (jsonb_typeof(reward_rule_refs) = 'array' AND octet_length(reward_rule_refs::text) <= 8192),
  ADD CONSTRAINT tournaments_tenant_ids_chk CHECK (tenant_ids IS NULL OR cardinality(tenant_ids) <= 100),
  ADD CONSTRAINT tournaments_template_fk FOREIGN KEY (template_config_id) REFERENCES competition.game_configs (config_id),
  ADD CONSTRAINT tournaments_version_fk FOREIGN KEY (rule_version_id) REFERENCES competition.game_config_versions (config_version_id),
  ADD CONSTRAINT tournaments_challenge_fk FOREIGN KEY (challenge_id) REFERENCES competition.challenges (challenge_id),
  ADD CONSTRAINT tournaments_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.tournaments ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.tournaments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.tournaments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.tournaments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.tournaments FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.tournaments FROM competition_app, competition_platform;

-- ---------- entries, groups, bracket nodes (T): the bracket lives in PostgreSQL (ADR-0004, F107-AC-14, AC-21) ----------
ALTER TABLE competition.tournament_entries
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT tournament_entries_status_chk CHECK (status IN ('registered','confirmed','checked_in','withdrawn','disqualified',
    'eliminated','champion')),
  ADD CONSTRAINT tournament_entries_members_chk CHECK (jsonb_typeof(member_ids) = 'array' AND jsonb_array_length(member_ids) BETWEEN 1 AND 8),
  ADD CONSTRAINT tournament_entries_kind_chk CHECK ((student_id IS NULL) <> (party_id IS NULL)),
  ADD CONSTRAINT tournament_entries_seed_chk CHECK (seed IS NULL OR seed BETWEEN 1 AND 256),
  ADD CONSTRAINT tournament_entries_tournament_fk FOREIGN KEY (tournament_id) REFERENCES competition.tournaments (tournament_id),
  ADD CONSTRAINT tournament_entries_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE INDEX tournament_entries_members_gin ON competition.tournament_entries USING gin (member_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE competition.tournament_groups
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT tournament_groups_standings_chk CHECK (standings IS NULL OR (jsonb_typeof(standings) = 'array'
    AND octet_length(standings::text) <= 16384)),
  ADD CONSTRAINT tournament_groups_tournament_fk FOREIGN KEY (tournament_id) REFERENCES competition.tournaments (tournament_id),
  ADD CONSTRAINT tournament_groups_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.tournament_entries
  ADD CONSTRAINT tournament_entries_group_fk FOREIGN KEY (group_id) REFERENCES competition.tournament_groups (group_id);
ALTER TABLE competition.bracket_nodes
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT bracket_nodes_bracket_chk CHECK (bracket IN ('winners','losers','grand_final','group')),
  ADD CONSTRAINT bracket_nodes_status_chk CHECK (status IN ('pending','scheduled','in_match','awaiting_confirm','confirmed','bye',
    'overridden','void')),
  ADD CONSTRAINT bracket_nodes_slot_chk CHECK (round >= 1 AND position >= 0 AND wave >= 0),
  ADD CONSTRAINT bracket_nodes_next_slot_chk CHECK ((next_win_slot IS NULL OR next_win_slot IN (0, 1))
    AND (next_lose_slot IS NULL OR next_lose_slot IN (0, 1))),
  ADD CONSTRAINT bracket_nodes_counts_chk CHECK (attempts >= 0 AND reschedule_count BETWEEN 0 AND 2 AND notices BETWEEN 0 AND 3),
  ADD CONSTRAINT bracket_nodes_result_chk CHECK (result IS NULL OR (jsonb_typeof(result) = 'object' AND octet_length(result::text) <= 2048)),
  ADD CONSTRAINT bracket_nodes_override_chk CHECK (status <> 'overridden' OR (override_reason IS NOT NULL AND overridden_by IS NOT NULL)),
  ADD CONSTRAINT bracket_nodes_tournament_fk FOREIGN KEY (tournament_id) REFERENCES competition.tournaments (tournament_id),
  ADD CONSTRAINT bracket_nodes_group_fk FOREIGN KEY (group_id) REFERENCES competition.tournament_groups (group_id),
  ADD CONSTRAINT bracket_nodes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE INDEX bracket_nodes_tournament_idx ON competition.bracket_nodes (tenant_id, tournament_id, wave, bracket, round, position)
  WHERE deleted_at IS NULL;
ALTER TABLE competition.tournament_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.tournament_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.tournament_entries FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.tournament_entries FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.tournament_entries FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE competition.tournament_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.tournament_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.tournament_groups FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.tournament_groups FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.tournament_groups FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE competition.bracket_nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.bracket_nodes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.bracket_nodes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.bracket_nodes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.bracket_nodes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.tournament_entries, competition.tournament_groups, competition.bracket_nodes
  FROM competition_app, competition_platform;

-- ---------- results and rewards (TA): written once, never updated (F107-AC-20, AC-21) ----------
ALTER TABLE competition.tournament_results
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT tournament_results_rank_chk CHECK (final_rank >= 1 AND wins >= 0 AND losses >= 0 AND draws >= 0 AND matches_played >= 0),
  ADD CONSTRAINT tournament_results_tournament_fk FOREIGN KEY (tournament_id) REFERENCES competition.tournaments (tournament_id),
  ADD CONSTRAINT tournament_results_entry_fk FOREIGN KEY (entry_id) REFERENCES competition.tournament_entries (entry_id),
  ADD CONSTRAINT tournament_results_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.tournament_rewards
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT tournament_rewards_students_chk CHECK (jsonb_typeof(student_ids) = 'array' AND jsonb_array_length(student_ids) BETWEEN 1 AND 8),
  ADD CONSTRAINT tournament_rewards_ref_chk CHECK (jsonb_typeof(reward_ref) = 'object' AND octet_length(reward_ref::text) <= 2048),
  ADD CONSTRAINT tournament_rewards_points_chk CHECK (points BETWEEN 0 AND 1000),
  ADD CONSTRAINT tournament_rewards_tournament_fk FOREIGN KEY (tournament_id) REFERENCES competition.tournaments (tournament_id),
  ADD CONSTRAINT tournament_rewards_entry_fk FOREIGN KEY (entry_id) REFERENCES competition.tournament_entries (entry_id),
  ADD CONSTRAINT tournament_rewards_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.tournament_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.tournament_results FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.tournament_results FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.tournament_results FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE competition.tournament_rewards ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.tournament_rewards FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.tournament_rewards FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.tournament_rewards FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.tournament_results, competition.tournament_rewards FROM competition_app, competition_platform;

-- ---------- infra.tenant_resource_assignments of eduworld_competition (DATABASE_DESIGN §3.0, ADR-0029, F107-AC-29) ----------
-- Tournament and challenge templates (competition.game_configs) assigned to tenants by the saga of module identity (F045)
-- through the facade applyTenantAssignments. No RLS (schema infra, ADR-0089 §4): competition_app only reads it with an
-- explicit tenant_id condition; the facade writes it through competition_platform (CrossTenantScope).
ALTER TABLE infra.tenant_resource_assignments
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('tournament_template','challenge_template')),
  ADD CONSTRAINT tra_resource_fk FOREIGN KEY (resource_id) REFERENCES competition.game_configs (config_id),
  ADD CONSTRAINT tra_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON infra.tenant_resource_assignments FROM competition_app;
GRANT SELECT ON infra.tenant_resource_assignments TO competition_app;
GRANT SELECT, INSERT, UPDATE ON infra.tenant_resource_assignments TO competition_platform;
REVOKE DELETE, TRUNCATE ON infra.tenant_resource_assignments FROM competition_platform;
CREATE OR REPLACE VIEW infra.tenant_resource_assignments_live WITH (security_barrier = true) AS
  SELECT * FROM infra.tenant_resource_assignments WHERE deleted_at IS NULL;

-- ---------- _live views: reporting (competition_readonly) reads challenges and tournaments, never entries (student ids) ----------
CREATE VIEW competition.challenges_live WITH (security_barrier = true) AS
  SELECT * FROM competition.challenges WHERE deleted_at IS NULL;
CREATE VIEW competition.tournaments_live WITH (security_barrier = true) AS
  SELECT * FROM competition.tournaments WHERE deleted_at IS NULL;
CREATE VIEW competition.tournament_entries_live WITH (security_barrier = true) AS
  SELECT * FROM competition.tournament_entries WHERE deleted_at IS NULL;
CREATE VIEW competition.tournament_groups_live WITH (security_barrier = true) AS
  SELECT * FROM competition.tournament_groups WHERE deleted_at IS NULL;
CREATE VIEW competition.bracket_nodes_live WITH (security_barrier = true) AS
  SELECT * FROM competition.bracket_nodes WHERE deleted_at IS NULL;
GRANT SELECT ON competition.challenges_live, competition.tournaments_live TO competition_readonly;

-- ---------- Platform seed of the tournament and challenge templates (COMPETITION_SYSTEM §10.2) ----------
-- Source: modules/competition/seed/tournament_templates.json; the unit test TestF107SeedMatchesMigration checks each
-- literal equals its file. F108 publishes the next versions; assignment to tenants goes through F045 (no tenant seed here).
-- BEGIN seed tournament_templates.json
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000171', now(), now(), 1, 'challenge-templates', 'grade_math_single_elim', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001711', now(), now(), 1, '01936a00-0000-7000-8000-000000000171', 1, 'published', 'Giải Toán khối (loại trực tiếp)', '{"schema_version":1,"kind":"grade_challenge","format":"tournament","name":"Giải Toán khối","description":"Giải loại trực tiếp một lượt của một khối trong trường; học sinh đấu 1v1 theo bộ câu hỏi của môn.","grade_levels":[1,2,3,4,5,6,7,8,9],"mode":"battle_1v1","tournament":{"format":"single_elim","participant_type":"individual","team_size":null,"max_entries":64,"min_entries":4,"seeding_policy":"random","auto_confirm":false,"round_gap_minutes":5,"bracket_reset":false},"schedule":{"min_lead_minutes":30,"max_lead_days":60,"max_duration_minutes":480},"timers":{"check_in_minutes":15,"no_show_minutes":3,"complaint_minutes":10,"reschedule_minutes":2,"reschedule_max":2},"allowed_params":["subject_id","max_entries","seeding_policy","registration_open_at","registration_close_at","auto_confirm"],"reward_rule_refs":[{"rule_id":"champion","rank_min":1,"rank_max":1,"points":30,"reward_ref":{"badge_code":"badge.tournament_champion"}},{"rule_id":"runner_up","rank_min":2,"rank_max":2,"points":20,"reward_ref":{"badge_code":"badge.tournament_runner_up"}},{"rule_id":"top4","rank_min":3,"rank_max":4,"points":10,"reward_ref":{"badge_code":"badge.tournament_top4"}},{"rule_id":"participation","min_matches":1,"points":5,"reward_ref":{"badge_code":"badge.tournament_participant"}}],"reward_summary":"Huy hiệu vô địch, á quân, top 4; huy hiệu tham gia cho mọi bạn thi đấu ít nhất 1 trận. Không ảnh hưởng điểm học tập."}',
  NULL, 'F107 seed (COMPETITION_SYSTEM §10)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001711' WHERE config_id = '01936a00-0000-7000-8000-000000000171';
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000172', now(), now(), 1, 'challenge-templates', 'grade_round_robin', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001721', now(), now(), 1, '01936a00-0000-7000-8000-000000000172', 1, 'published', 'Giải vòng tròn khối', '{"schema_version":1,"kind":"grade_challenge","format":"tournament","name":"Giải vòng tròn","description":"Mỗi bạn đấu với mọi bạn khác trong giải; xếp hạng thắng 3, hòa 1, thua 0.","grade_levels":[4,5,6,7,8,9],"mode":"battle_1v1","tournament":{"format":"round_robin","participant_type":"individual","team_size":null,"max_entries":8,"min_entries":3,"seeding_policy":"rating","auto_confirm":true,"round_gap_minutes":5,"bracket_reset":false},"schedule":{"min_lead_minutes":30,"max_lead_days":60,"max_duration_minutes":480},"timers":{"check_in_minutes":15,"no_show_minutes":3,"complaint_minutes":10,"reschedule_minutes":2,"reschedule_max":2},"allowed_params":["subject_id","max_entries","registration_open_at","registration_close_at"],"reward_rule_refs":[{"rule_id":"champion","rank_min":1,"rank_max":1,"points":30,"reward_ref":{"badge_code":"badge.tournament_champion"}},{"rule_id":"participation","min_matches":1,"points":5,"reward_ref":{"badge_code":"badge.tournament_participant"}}],"reward_summary":"Huy hiệu vô địch; huy hiệu tham gia cho mọi bạn thi đấu ít nhất 1 trận."}',
  NULL, 'F107 seed (COMPETITION_SYSTEM §10)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001721' WHERE config_id = '01936a00-0000-7000-8000-000000000172';
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000173', now(), now(), 1, 'challenge-templates', 'grade_quick_quiz', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001731', now(), now(), 1, '01936a00-0000-7000-8000-000000000173', 1, 'published', 'Thử thách nhân nhanh khối', '{"schema_version":1,"kind":"grade_challenge","format":"timed_quiz","name":"Thử thách nhân nhanh","description":"Cả khối cùng trả lời một bộ câu trong thời gian giới hạn; lớp 1–3 chỉ thấy kết quả của chính em và tiến độ của khối.","grade_levels":[1,2,3,4,5,6,7,8,9],"mode":"timed_quiz","timed_quiz":{"join_before_minutes":5,"duration_minutes":10},"schedule":{"min_lead_minutes":10,"max_lead_days":60,"max_duration_minutes":60},"allowed_params":["subject_id"],"reward_rule_refs":[],"reward_summary":"Không có phần thưởng xếp hạng; thử thách để luyện tập cùng khối."}',
  NULL, 'F107 seed (COMPETITION_SYSTEM §10)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001731' WHERE config_id = '01936a00-0000-7000-8000-000000000173';
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000174', now(), now(), 1, 'tournament-templates', 'school_team_groups', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001741', now(), now(), 1, '01936a00-0000-7000-8000-000000000174', 1, 'published', 'Giải trường theo đội (vòng bảng + loại trực tiếp)', '{"schema_version":1,"kind":"school_tournament","format":"tournament","name":"Giải trường theo đội","description":"Đội 3 bạn cùng trường; chia bảng 4–6 đội theo hạt giống kiểu snake, top 2 mỗi bảng vào loại trực tiếp.","grade_levels":[6,7,8,9],"mode":"team_battle","tournament":{"format":"groups_then_single_elim","participant_type":"team","team_size":3,"max_entries":32,"min_entries":8,"seeding_policy":"random","auto_confirm":false,"round_gap_minutes":1440,"bracket_reset":false},"schedule":{"min_lead_minutes":1440,"max_lead_days":90,"max_duration_minutes":20160},"timers":{"check_in_minutes":15,"no_show_minutes":3,"complaint_minutes":10,"reschedule_minutes":2,"reschedule_max":2},"allowed_params":["subject_id","max_entries","registration_open_at","registration_close_at"],"reward_rule_refs":[{"rule_id":"champion","rank_min":1,"rank_max":1,"points":30,"reward_ref":{"title_code":"title.school_team_champion"}},{"rule_id":"participation","min_matches":1,"points":5,"reward_ref":{"badge_code":"badge.tournament_participant"}}],"reward_summary":"Danh hiệu đội vô địch trường; huy hiệu tham gia."}',
  NULL, 'F107 seed (COMPETITION_SYSTEM §10)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001741' WHERE config_id = '01936a00-0000-7000-8000-000000000174';
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000175', now(), now(), 1, 'tournament-templates', 'school_double_elim', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001751', now(), now(), 1, '01936a00-0000-7000-8000-000000000175', 1, 'published', 'Giải trường nhánh thắng/thua', '{"schema_version":1,"kind":"school_tournament","format":"tournament","name":"Giải nhánh thắng/thua","description":"Thua một trận vẫn còn cơ hội ở nhánh thua; chung kết một trận (không bracket reset).","grade_levels":[6,7,8,9],"mode":"battle_1v1","tournament":{"format":"double_elim","participant_type":"individual","team_size":null,"max_entries":32,"min_entries":4,"seeding_policy":"rating","auto_confirm":false,"round_gap_minutes":5,"bracket_reset":false},"schedule":{"min_lead_minutes":30,"max_lead_days":60,"max_duration_minutes":480},"timers":{"check_in_minutes":15,"no_show_minutes":3,"complaint_minutes":10,"reschedule_minutes":2,"reschedule_max":2},"allowed_params":["subject_id","max_entries","seeding_policy","registration_open_at","registration_close_at"],"reward_rule_refs":[{"rule_id":"champion","rank_min":1,"rank_max":1,"points":30,"reward_ref":{"badge_code":"badge.tournament_champion"}},{"rule_id":"runner_up","rank_min":2,"rank_max":2,"points":20,"reward_ref":{"badge_code":"badge.tournament_runner_up"}},{"rule_id":"participation","min_matches":1,"points":5,"reward_ref":{"badge_code":"badge.tournament_participant"}}],"reward_summary":"Huy hiệu vô địch, á quân; huy hiệu tham gia."}',
  NULL, 'F107 seed (COMPETITION_SYSTEM §10)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001751' WHERE config_id = '01936a00-0000-7000-8000-000000000175';
-- END seed tournament_templates.json
