-- ADR-0095 (WP-C1, expand step): party and matchmaking without classes. Code stops reading ref.enrollments and
-- stops writing the class concepts; tables and old CHECK values stay until WP-R (contract), which drops
-- ref.enrollments and removes the old CHECK values once no row (soft-deleted included) carries them.
-- The backfill runs as the table owner across every tenant: FORCE ROW LEVEL SECURITY is lifted for this transaction
-- only and restored below (the application roles never bypass RLS).
ALTER TABLE multiplayer.parties NO FORCE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_policies NO FORCE ROW LEVEL SECURITY;

-- ---------- D7: party type study_group → study_party (expand: both values allowed during the rollout) ----------
ALTER TABLE multiplayer.parties DROP CONSTRAINT parties_type_chk;
ALTER TABLE multiplayer.parties
  ADD CONSTRAINT parties_type_chk CHECK (party_type IN ('friend','study_party','study_group','quest','competition_team','custom'));
-- Backfill every row (open, disbanded and soft-deleted alike): the rename keeps the behaviour.
UPDATE multiplayer.parties SET party_type = 'study_party' WHERE party_type = 'study_group';

-- ---------- Party visibility class → private (no class circle any more) ----------
-- parties_visibility_chk keeps 'class' until WP-R; the code reads a legacy 'class' row as 'private'.
UPDATE multiplayer.parties SET visibility = 'private', updated_at = now() WHERE visibility = 'class';

-- ---------- party_policies: no class level, no cross_class flag (R21: soft-delete, never DELETE) ----------
UPDATE multiplayer.party_policies SET deleted_at = now(), delete_reason = 'adr0095_class_removed', updated_at = now(),
  version = version + 1
  WHERE scope_type = 'class' AND deleted_at IS NULL;
UPDATE multiplayer.party_policies SET rules = rules - 'cross_class', updated_at = now(), version = version + 1
  WHERE rules ? 'cross_class' AND deleted_at IS NULL;
ALTER TABLE multiplayer.parties FORCE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_policies FORCE ROW LEVEL SECURITY;

-- ---------- F104: matchmaking configuration version 2 without cost_weights.same_class ----------
-- Published versions never change (R11, trigger game_config_versions_immutable): a new version is published with the
-- same effective_from, the reader takes the highest version_no. The integration test TestSeedMatchmakingConfig checks
-- this literal equals modules/competition/seed/matchmaking_config.json.
-- BEGIN seed matchmaking_config.json
INSERT INTO competition.game_config_versions (config_version_id, created_at, updated_at, version, config_id, version_no, status, title, payload,
  effective_from, change_note, published_at)
VALUES ('01936a00-0000-7000-8000-000000001043', now(), now(), 1, '01936a00-0000-7000-8000-000000000104', 2, 'published',
  'Matchmaking mặc định (CONFIG_DEFAULTS, không lớp)', '{"schema_version":1,"rating_window":[{"after_s":0,"width":100},{"after_s":10,"width":200},{"after_s":20,"width":350},{"after_s":40,"width":null}],"ready_check_s":15,"bot_after_s":90,"ticket_ttl_s":300,"tickets_per_user":1,"ticket_rate":{"per_hour":30,"decline_limit":3,"decline_window_s":600,"cooldown_s":120},"grade_window_g6_9":{"after_s":30,"levels":1},"disconnect_cancel_s":30,"bucket_threshold":500,"batch":200,"cost_weights":{"rating":1,"rd":0.1,"recent_opponent":0.5},"recent_opponent_window_s":1800,"default_rating":{"mu":1500,"rd":350},"modes":[{"mode":"battle_1v1","min_grade":1,"matchmaking_types":["random","skill","grade","subject","topic"],"ranked_types":["skill","subject"]},{"mode":"team_battle","min_grade":4,"matchmaking_types":["team"],"ranked_types":["team"],"team_size":3,"min_team_size":2,"max_team_size":4}],"subjects":[]}',
  '2026-01-01T00:00:00Z', 'ADR-0095: bỏ trọng số same_class', '2026-11-21T00:00:00Z');
UPDATE competition.game_configs SET current_version_id = '01936a00-0000-7000-8000-000000001043', latest_version_no = 2,
  updated_at = now(), version = version + 1
  WHERE config_id = '01936a00-0000-7000-8000-000000000104';
INSERT INTO competition.content_releases (release_id, created_at, config_id, action, from_version_id, to_version_id, reason, released_at)
VALUES ('01936a00-0000-7000-8000-000000001044', now(), '01936a00-0000-7000-8000-000000000104', 'publish',
  '01936a00-0000-7000-8000-000000001041', '01936a00-0000-7000-8000-000000001043', 'ADR-0095: bỏ same_class', '2026-11-21T00:00:00Z');
-- END seed matchmaking_config.json
