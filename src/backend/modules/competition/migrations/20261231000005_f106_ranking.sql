-- F106 T007 — ranking Glicko-2, tiers, Competition Points, 12 leaderboards and seasons of module competition
-- (specs/106-ranking-leaderboard/data-model.md; DATABASE_DESIGN §2.10, §2.16; ADR-0004, ADR-0018, ADR-0028, ADR-0041,
-- ADR-0042, ADR-0095). Applied by `migrate --module=competition` as competition_migrator. Expand only (AGENTS R12): new
-- tables. PostgreSQL is the source of truth (ledgers, ratings, snapshots); the open periods of the leaderboards are a
-- rebuildable cache in Valkey (ew:lb:*, VALKEY_DESIGN §3.1). No class scope, no class_goal (ADR-0095).

-- BEGIN entddl
CREATE TABLE competition.ratings (
  rating_id uuid NOT NULL,
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
  mode varchar(64) NOT NULL,
  subject_id uuid NOT NULL,
  season_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  rating numeric(9,4) NOT NULL,
  rd numeric(9,4) NOT NULL,
  volatility numeric(9,6) NOT NULL,
  games_played bigint NOT NULL DEFAULT 0,
  tier_code varchar(64),
  peak_tier_code varchar(64),
  last_played_at timestamptz,
  rule_version_id uuid NOT NULL,
  PRIMARY KEY (rating_id)
);
CREATE UNIQUE INDEX ratings_key_uq ON competition.ratings (student_id, mode, subject_id, season_id) WHERE deleted_at IS NULL;
CREATE INDEX ratings_season_played_idx ON competition.ratings (tenant_id, season_id, last_played_at);

CREATE TABLE competition.rating_changes (
  change_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  match_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  mode varchar(64) NOT NULL,
  subject_id uuid NOT NULL,
  season_id uuid NOT NULL,
  before jsonb NOT NULL,
  after jsonb NOT NULL,
  rule_version_id uuid NOT NULL,
  PRIMARY KEY (change_id)
);
CREATE UNIQUE INDEX rating_changes_match_student_uq ON competition.rating_changes (match_id, student_id);

CREATE TABLE competition.competition_point_ledger (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  season_id uuid NOT NULL,
  subject_id uuid,
  topic_id uuid,
  mode varchar(64),
  amount bigint NOT NULL,
  source_type varchar(64) NOT NULL,
  source_id uuid NOT NULL,
  reason varchar(64) NOT NULL,
  breakdown jsonb NOT NULL,
  rule_version_id uuid NOT NULL,
  day_key date NOT NULL,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (entry_id)
);
CREATE INDEX competition_point_ledger_day_idx ON competition.competition_point_ledger (tenant_id, student_id, day_key);
CREATE INDEX competition_point_ledger_scope_idx ON competition.competition_point_ledger (tenant_id, season_id, school_id, grade_level);
CREATE UNIQUE INDEX competition_point_ledger_source_uq ON competition.competition_point_ledger (source_type, source_id, student_id);

CREATE TABLE competition.leaderboard_contributions (
  contribution_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  metric varchar(64) NOT NULL,
  amount bigint NOT NULL,
  source_type varchar(64) NOT NULL,
  source_id varchar(64) NOT NULL,
  day_key date NOT NULL,
  rule_version_id uuid,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (contribution_id)
);
CREATE INDEX leaderboard_contributions_day_idx ON competition.leaderboard_contributions (tenant_id, metric, day_key, student_id);
CREATE INDEX leaderboard_contributions_grade_idx ON competition.leaderboard_contributions (tenant_id, school_id, grade_level, metric, day_key);
CREATE UNIQUE INDEX leaderboard_contributions_source_uq ON competition.leaderboard_contributions (metric, source_type, source_id, student_id);

CREATE TABLE competition.leaderboard_definitions (
  leaderboard_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  title varchar(128) NOT NULL,
  metric varchar(64) NOT NULL,
  scope_type varchar(64) NOT NULL,
  period varchar(64) NOT NULL,
  filters jsonb NOT NULL,
  grade_levels jsonb NOT NULL,
  display jsonb NOT NULL,
  sort_order bigint NOT NULL DEFAULT 0,
  status varchar(64) NOT NULL DEFAULT 'active',
  config_version_id uuid,
  PRIMARY KEY (leaderboard_id)
);
CREATE UNIQUE INDEX leaderboard_definitions_code_uq ON competition.leaderboard_definitions (code) WHERE deleted_at IS NULL;

CREATE TABLE competition.leaderboard_snapshots (
  snapshot_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  leaderboard_id uuid NOT NULL,
  scope_id varchar(80) NOT NULL,
  school_id uuid,
  grade_level smallint,
  period_key varchar(64) NOT NULL,
  student_id uuid,
  rank bigint,
  value numeric(12,2) NOT NULL,
  target numeric(12,2),
  reached boolean,
  members bigint,
  captured_at timestamptz NOT NULL,
  PRIMARY KEY (snapshot_id)
);
CREATE UNIQUE INDEX leaderboard_snapshots_collective_uq ON competition.leaderboard_snapshots (leaderboard_id, scope_id, period_key) WHERE student_id IS NULL;
CREATE INDEX leaderboard_snapshots_history_idx ON competition.leaderboard_snapshots (tenant_id, leaderboard_id, scope_id, captured_at);
CREATE UNIQUE INDEX leaderboard_snapshots_member_uq ON competition.leaderboard_snapshots (leaderboard_id, scope_id, period_key, student_id) WHERE student_id IS NOT NULL;

CREATE TABLE competition.seasons (
  season_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name varchar(128) NOT NULL,
  theme_code varchar(64),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'scheduled',
  rules jsonb NOT NULL,
  config_version_id uuid,
  started_at timestamptz,
  ended_at timestamptz,
  PRIMARY KEY (season_id)
);
CREATE UNIQUE INDEX seasons_code_uq ON competition.seasons (code) WHERE deleted_at IS NULL;
CREATE INDEX seasons_status_idx ON competition.seasons (status, starts_at);

CREATE TABLE competition.season_results (
  result_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  season_id uuid NOT NULL,
  student_id uuid NOT NULL,
  mode varchar(64) NOT NULL,
  subject_id uuid NOT NULL,
  school_id uuid NOT NULL,
  final_rating numeric(9,4) NOT NULL,
  final_rd numeric(9,4) NOT NULL,
  peak_tier_code varchar(64),
  games_played bigint NOT NULL,
  competition_points bigint NOT NULL,
  PRIMARY KEY (result_id)
);
CREATE UNIQUE INDEX season_results_key_uq ON competition.season_results (season_id, student_id, mode, subject_id);
-- END entddl

-- ---------- competition.ratings (T): never returned to a student as a number, never to module learning (ADR-0018) ----------
ALTER TABLE competition.ratings
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT ratings_mode_chk CHECK (mode IN ('battle_1v1','team_battle','timed_quiz')),
  ADD CONSTRAINT ratings_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT ratings_rd_chk CHECK (rd > 0 AND rd <= 1000),
  ADD CONSTRAINT ratings_volatility_chk CHECK (volatility > 0 AND volatility < 1),
  ADD CONSTRAINT ratings_games_chk CHECK (games_played >= 0),
  ADD CONSTRAINT ratings_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.ratings ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.ratings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.ratings FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.ratings FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON competition.ratings FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON competition.ratings FROM competition_app, competition_platform;

-- ---------- append-only tenant tables (TA): no UPDATE, no DELETE for any application role ----------
ALTER TABLE competition.rating_changes
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT rating_changes_mode_chk CHECK (mode IN ('battle_1v1','team_battle','timed_quiz')),
  ADD CONSTRAINT rating_changes_before_chk CHECK (jsonb_typeof(before) = 'object' AND octet_length(before::text) <= 2048),
  ADD CONSTRAINT rating_changes_after_chk CHECK (jsonb_typeof(after) = 'object' AND octet_length(after::text) <= 2048),
  ADD CONSTRAINT rating_changes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.competition_point_ledger
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT competition_point_ledger_source_chk CHECK (source_type IN ('match','tournament')),
  ADD CONSTRAINT competition_point_ledger_reason_chk CHECK (reason IN ('completed','win','draw','correct','upset','abandon','capped','tournament')),
  ADD CONSTRAINT competition_point_ledger_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT competition_point_ledger_amount_chk CHECK (amount BETWEEN -100 AND 1000),
  ADD CONSTRAINT competition_point_ledger_breakdown_chk CHECK (jsonb_typeof(breakdown) = 'object' AND octet_length(breakdown::text) <= 2048),
  ADD CONSTRAINT competition_point_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.leaderboard_contributions
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT leaderboard_contributions_metric_chk CHECK (metric IN ('effort_points','xp','quest_completed','team_points')),
  ADD CONSTRAINT leaderboard_contributions_source_chk CHECK (source_type IN ('activity_completed','review_activity','active_day',
    'xp_awarded','quest_completed','team_match')),
  ADD CONSTRAINT leaderboard_contributions_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT leaderboard_contributions_amount_chk CHECK (amount >= 0),
  ADD CONSTRAINT leaderboard_contributions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.leaderboard_snapshots
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT leaderboard_snapshots_rank_chk CHECK ((student_id IS NULL) = (rank IS NULL) AND (rank IS NULL OR rank >= 1)),
  ADD CONSTRAINT leaderboard_snapshots_grade_chk CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT leaderboard_snapshots_definition_fk FOREIGN KEY (leaderboard_id) REFERENCES competition.leaderboard_definitions (leaderboard_id),
  ADD CONSTRAINT leaderboard_snapshots_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.season_results
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT season_results_mode_chk CHECK (mode IN ('battle_1v1','team_battle','timed_quiz')),
  ADD CONSTRAINT season_results_games_chk CHECK (games_played >= 0),
  ADD CONSTRAINT season_results_season_fk FOREIGN KEY (season_id) REFERENCES competition.seasons (season_id),
  ADD CONSTRAINT season_results_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE competition.rating_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.rating_changes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.rating_changes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.rating_changes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.rating_changes FROM competition_app, competition_platform;
ALTER TABLE competition.competition_point_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.competition_point_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.competition_point_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.competition_point_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.competition_point_ledger FROM competition_app, competition_platform;
ALTER TABLE competition.leaderboard_contributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.leaderboard_contributions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.leaderboard_contributions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.leaderboard_contributions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.leaderboard_contributions FROM competition_app, competition_platform;
ALTER TABLE competition.leaderboard_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.leaderboard_snapshots FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.leaderboard_snapshots FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.leaderboard_snapshots FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.leaderboard_snapshots FROM competition_app, competition_platform;
ALTER TABLE competition.season_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.season_results FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.season_results FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.season_results FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.season_results FROM competition_app, competition_platform;

-- ---------- competition.leaderboard_definitions / seasons (G, platform only: R20, ADR-0027) ----------
-- Published by the Game Operations Console (F108) and materialized by module competition with competition_platform;
-- the tenant-serving role only reads. Never a class scope or a class_goal metric (ADR-0095).
ALTER TABLE competition.leaderboard_definitions
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT leaderboard_definitions_metric_chk CHECK (metric IN ('competition_points','effort_points','grade_goal','xp',
    'quest_completed','rating','team_points')),
  ADD CONSTRAINT leaderboard_definitions_scope_chk CHECK (scope_type IN ('global','tenant','school','grade','friends','team','tournament')),
  ADD CONSTRAINT leaderboard_definitions_period_chk CHECK (period IN ('daily','weekly','monthly','season','all_time')),
  ADD CONSTRAINT leaderboard_definitions_xp_chk CHECK (metric <> 'xp' OR scope_type = 'friends'),
  ADD CONSTRAINT leaderboard_definitions_grade_goal_chk CHECK (metric <> 'grade_goal' OR (scope_type = 'grade' AND period = 'weekly')),
  ADD CONSTRAINT leaderboard_definitions_all_time_chk CHECK (period <> 'all_time' OR metric IN ('xp','quest_completed')),
  ADD CONSTRAINT leaderboard_definitions_status_chk CHECK (status IN ('active','retired')),
  ADD CONSTRAINT leaderboard_definitions_filters_chk CHECK (jsonb_typeof(filters) = 'object' AND octet_length(filters::text) <= 2048),
  ADD CONSTRAINT leaderboard_definitions_grades_chk CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) BETWEEN 1 AND 9),
  ADD CONSTRAINT leaderboard_definitions_display_chk CHECK (jsonb_typeof(display) = 'object' AND octet_length(display::text) <= 2048);
ALTER TABLE competition.seasons
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT seasons_status_chk CHECK (status IN ('scheduled','active','ended')),
  ADD CONSTRAINT seasons_period_chk CHECK (ends_at > starts_at),
  ADD CONSTRAINT seasons_rules_chk CHECK (jsonb_typeof(rules) = 'object' AND octet_length(rules::text) <= 8192);
-- At most one active season (the scheduler ends the previous one before starting the next, F106-AC-22).
CREATE UNIQUE INDEX seasons_one_active_uq ON competition.seasons ((true)) WHERE status = 'active' AND deleted_at IS NULL;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON competition.leaderboard_definitions, competition.seasons FROM competition_app;
REVOKE DELETE, TRUNCATE ON competition.leaderboard_definitions, competition.seasons FROM competition_platform;

-- ---------- _live views: reporting (competition_readonly) reads definitions and seasons only, ratings stay internal ----------
CREATE VIEW competition.ratings_live WITH (security_barrier = true) AS
  SELECT * FROM competition.ratings WHERE deleted_at IS NULL;
CREATE VIEW competition.leaderboard_definitions_live WITH (security_barrier = true) AS
  SELECT * FROM competition.leaderboard_definitions WHERE deleted_at IS NULL;
CREATE VIEW competition.seasons_live WITH (security_barrier = true) AS
  SELECT * FROM competition.seasons WHERE deleted_at IS NULL;
GRANT SELECT ON competition.leaderboard_definitions_live, competition.seasons_live TO competition_readonly;

-- ---------- Platform seed of the Ranking Rules, the default leaderboard catalog and the pre-season ----------
-- Sources: modules/competition/seed/{ranking_rules,leaderboard_definitions,seasons}.json; the unit test
-- TestF106SeedMatchesMigration checks each literal equals its file. F108 publishes the next versions.
-- BEGIN seed ranking_rules.json
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000106', now(), now(), 1, 'ranking-rules', 'default', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001061', now(), now(), 1, '01936a00-0000-7000-8000-000000000106', 1, 'published', 'Ranking Rules mặc định (CONFIG_DEFAULTS)', '{"schema_version":1,"glicko2_params":{"r0":1500,"rd0":350,"sigma0":0.06,"tau":0.5,"rd_min":50,"rd_max":350,"epsilon":0.000001},"rating_provisional":{"min_games":10,"max_rd":110},"tiers":{"hysteresis":50,"levels":[{"code":"T1","min_rating":null,"names":{"vi-VN":"Mầm Non Tri Thức","en-US":"Knowledge Sprout"}},{"code":"T2","min_rating":1300,"names":{"vi-VN":"Hạt Giống","en-US":"Seed"}},{"code":"T3","min_rating":1450,"names":{"vi-VN":"Chồi Non","en-US":"Young Bud"}},{"code":"T4","min_rating":1600,"names":{"vi-VN":"Cây Xanh","en-US":"Green Tree"}},{"code":"T5","min_rating":1750,"names":{"vi-VN":"Đại Thụ","en-US":"Great Tree"}},{"code":"T6","min_rating":1900,"names":{"vi-VN":"Rừng Trí Tuệ","en-US":"Forest of Wisdom"}}]},"season_soft_reset":{"factor":0.5,"rd_floor":150},"competition_points":{"completed":10,"win":15,"draw":7,"correct":1,"correct_max":10,"upset":5,"abandon":-5,"daily_cap":300,"daily_matches":20},"effort_points":{"activity":1,"activity_daily_max":10,"review":2,"active_day":3,"daily_cap":25},"leaderboard_visibility":{"g1_3":{"personal_rank":false,"top":0,"percentile":false,"hide_last":true,"around":0},"g4_5":{"personal_rank":true,"top":10,"percentile":true,"hide_last":true,"around":0},"g6_9":{"personal_rank":true,"top":50,"percentile":false,"hide_last":false,"around":3}},"analytics_min_cell_size":5,"friends_max":200}',
  NULL, 'F106 seed (CONFIG_DEFAULTS)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001061' WHERE config_id = '01936a00-0000-7000-8000-000000000106';
-- END seed ranking_rules.json
-- BEGIN seed leaderboard_definitions.json
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000107', now(), now(), 1, 'leaderboard-definitions', 'default', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001071', now(), now(), 1, '01936a00-0000-7000-8000-000000000107', 1, 'published', 'Danh mục bảng xếp hạng mặc định (COMPETITION_SYSTEM §9.2)', '{"schema_version":1,"definitions":[{"leaderboard_id":"01936a00-0000-7000-8000-000000106101","code":"grade_goal_weekly","title":"Mục tiêu của khối","metric":"grade_goal","scope_type":"grade","period":"weekly","filters":{},"grade_levels":[1,2,3,4,5,6,7,8,9],"display":{"grade_goal_target":300},"sort_order":10,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106102","code":"school_arena_weekly","title":"Đấu trường trường (tuần)","metric":"competition_points","scope_type":"grade","period":"weekly","filters":{},"grade_levels":[4,5,6,7,8,9],"display":{},"sort_order":20,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106103","code":"school_arena_season","title":"Đấu trường trường (mùa)","metric":"competition_points","scope_type":"grade","period":"season","filters":{},"grade_levels":[4,5,6,7,8,9],"display":{},"sort_order":30,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106104","code":"friends_points_weekly","title":"Bạn bè — điểm thi đấu","metric":"competition_points","scope_type":"friends","period":"weekly","filters":{},"grade_levels":[4,5,6,7,8,9],"display":{},"sort_order":40,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106105","code":"friends_effort_weekly","title":"Bạn bè — nỗ lực","metric":"effort_points","scope_type":"friends","period":"weekly","filters":{},"grade_levels":[4,5,6,7,8,9],"display":{},"sort_order":50,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106106","code":"friends_xp_weekly","title":"Bạn bè — XP","metric":"xp","scope_type":"friends","period":"weekly","filters":{},"grade_levels":[4,5,6,7,8,9],"display":{},"sort_order":60,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106107","code":"team_season","title":"Đội","metric":"team_points","scope_type":"school","period":"season","filters":{},"grade_levels":[6,7,8,9],"display":{},"sort_order":70,"status":"active"},{"leaderboard_id":"01936a00-0000-7000-8000-000000106108","code":"global_season","title":"Toàn hệ thống","metric":"competition_points","scope_type":"global","period":"season","filters":{},"grade_levels":[6,7,8,9],"display":{},"sort_order":80,"status":"retired"}]}',
  NULL, 'F106 seed (CONFIG_DEFAULTS)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001071' WHERE config_id = '01936a00-0000-7000-8000-000000000107';
-- END seed leaderboard_definitions.json
-- BEGIN seed seasons.json
INSERT INTO competition.game_configs (config_id, created_at, updated_at, version, config_type, code, current_version_id, latest_version_no, owner_scope)
VALUES ('01936a00-0000-7000-8000-000000000108', now(), now(), 1, 'seasons', 'default', NULL, 1, 'platform');
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001081', now(), now(), 1, '01936a00-0000-7000-8000-000000000108', 1, 'published', 'Mùa khởi động (pre-season)', '{"schema_version":1,"seasons":[{"season_id":"01936a00-0000-7000-8000-000000106001","code":"pre-season","name":"Mùa khởi động","theme_code":null,"starts_at":"2026-01-01T00:00:00+07:00","ends_at":"2099-12-31T00:00:00+07:00","status":"active"}]}',
  NULL, 'F106 seed (CONFIG_DEFAULTS)', now());
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001081' WHERE config_id = '01936a00-0000-7000-8000-000000000108';
-- END seed seasons.json

-- The materialized definitions and seasons of the published versions above (MaterializeDefinitions keeps them in sync).
INSERT INTO competition.leaderboard_definitions (leaderboard_id, created_at, updated_at, version, code, title, metric, scope_type, period, filters,
  grade_levels, display, sort_order, status, config_version_id)
SELECT (d->>'leaderboard_id')::uuid, now(), now(), 1, d->>'code', d->>'title', d->>'metric', d->>'scope_type', d->>'period', d->'filters',
  d->'grade_levels', d->'display', (d->>'sort_order')::int, d->>'status', v.config_version_id
FROM competition.game_config_versions v, jsonb_array_elements(v.payload->'definitions') d
WHERE v.config_version_id = '01936a00-0000-7000-8000-000000001071';
INSERT INTO competition.seasons (season_id, created_at, updated_at, version, code, name, theme_code, starts_at, ends_at, status, rules,
  config_version_id, started_at)
SELECT (s->>'season_id')::uuid, now(), now(), 1, s->>'code', s->>'name', s->>'theme_code', (s->>'starts_at')::timestamptz,
  (s->>'ends_at')::timestamptz, s->>'status', '{}'::jsonb, v.config_version_id,
  CASE WHEN s->>'status' = 'active' THEN (s->>'starts_at')::timestamptz END
FROM competition.game_config_versions v, jsonb_array_elements(v.payload->'seasons') s
WHERE v.config_version_id = '01936a00-0000-7000-8000-000000001081';
