-- F077 T008/T009 — event & season live-ops of module game (specs/077-event-season-liveops: data-model §1; DATABASE_DESIGN
-- §2.8c, §2.17, §3.13; EVENT_SEASON_SYSTEM §4–§13; ADR-0028, ADR-0041, ADR-0043, ADR-0049, ADR-0089, ADR-0095). Applied by
-- `migrate --module=game` as game_migrator. Order: ent tables (entddl block, golden test modules/game/ent/ddl_test.go), then
-- what ent cannot express: the projection ref.seasons, CHECKs, FKs, release pointers, immutability of published versions,
-- RLS (G/T tables: platform rows readable by every tenant, tenant rows by their tenant), append-only grants of the ledgers,
-- the SECURITY DEFINER total of a platform goal, _live views. The ledgers are not partitioned yet (monthly/daily partitions
-- when large, like xp_ledger — spec Clarifications), so the idempotency keys are unique on their own.

-- BEGIN entddl
CREATE TABLE game.events (
  event_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(16) NOT NULL DEFAULT 'platform',
  code varchar(64) NOT NULL,
  event_kind varchar(32) NOT NULL,
  template_id uuid,
  school_id uuid,
  school_params jsonb,
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (event_id)
);

CREATE TABLE game.event_versions (
  event_version_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  event_id uuid NOT NULL,
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
  event_kind varchar(32) NOT NULL,
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  effective_from timestamptz,
  decoration_areas jsonb NOT NULL,
  PRIMARY KEY (event_version_id)
);
CREATE UNIQUE INDEX event_versions_event_no_uq ON game.event_versions (event_id, version_no);

CREATE TABLE game.event_templates (
  template_id uuid NOT NULL,
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
  PRIMARY KEY (template_id)
);

CREATE TABLE game.event_template_versions (
  template_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  template_id uuid NOT NULL,
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
  event_kind varchar(32) NOT NULL,
  allowed_params jsonb NOT NULL,
  max_duration_days bigint NOT NULL,
  PRIMARY KEY (template_version_id)
);

CREATE TABLE game.event_runs (
  event_run_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  event_id uuid NOT NULL,
  event_version_id uuid NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'scheduled',
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  claim_until timestamptz NOT NULL,
  suspended_reason varchar(1000),
  status_changed_by uuid,
  status_version bigint NOT NULL DEFAULT 1,
  audience jsonb NOT NULL,
  school_id uuid,
  cancelled_at timestamptz,
  archived_at timestamptz,
  PRIMARY KEY (event_run_id)
);
CREATE INDEX event_runs_event_idx ON game.event_runs (event_id);
CREATE INDEX event_runs_status_claim_idx ON game.event_runs (status, claim_until);
CREATE INDEX event_runs_status_ends_idx ON game.event_runs (status, ends_at);
CREATE INDEX event_runs_status_starts_idx ON game.event_runs (status, starts_at);

CREATE TABLE game.event_participation (
  participation_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  event_run_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  joined_at timestamptz NOT NULL,
  event_points bigint NOT NULL DEFAULT 0,
  reached_milestones jsonb NOT NULL,
  claimed_milestones jsonb NOT NULL,
  auto_granted_milestones jsonb NOT NULL,
  PRIMARY KEY (participation_id)
);
CREATE UNIQUE INDEX event_participation_run_student_uq ON game.event_participation (event_run_id, student_id) WHERE deleted_at IS NULL;

CREATE TABLE game.event_point_ledger (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  event_run_id uuid NOT NULL,
  student_id uuid NOT NULL,
  delta bigint NOT NULL,
  source_code varchar(64) NOT NULL,
  source_event_id uuid NOT NULL,
  capped boolean NOT NULL DEFAULT false,
  earned_on date NOT NULL,
  PRIMARY KEY (entry_id)
);
CREATE INDEX event_point_ledger_day_idx ON game.event_point_ledger (tenant_id, event_run_id, student_id, earned_on);
CREATE UNIQUE INDEX event_point_ledger_source_uq ON game.event_point_ledger (event_run_id, source_event_id, student_id);

CREATE TABLE game.community_goals (
  goal_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  event_run_id uuid NOT NULL,
  goal_code varchar(64) NOT NULL,
  metric varchar(32) NOT NULL,
  scope varchar(16) NOT NULL,
  target bigint NOT NULL,
  per_student_daily_cap bigint NOT NULL,
  show_grade_contributions boolean NOT NULL DEFAULT true,
  milestones jsonb NOT NULL,
  metric_filter jsonb,
  PRIMARY KEY (goal_id)
);
CREATE UNIQUE INDEX community_goals_run_code_uq ON game.community_goals (event_run_id, goal_code) WHERE deleted_at IS NULL;

CREATE TABLE game.community_goal_contributions (
  contribution_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  goal_id uuid NOT NULL,
  scope_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  grade_level bigint,
  amount bigint NOT NULL,
  source_event_id uuid NOT NULL,
  contributed_on date NOT NULL,
  PRIMARY KEY (contribution_id)
);
CREATE INDEX community_goal_contributions_day_idx ON game.community_goal_contributions (goal_id, student_id, contributed_on);
CREATE INDEX community_goal_contributions_scope_idx ON game.community_goal_contributions (goal_id, scope_id);
CREATE UNIQUE INDEX community_goal_contributions_source_uq ON game.community_goal_contributions (goal_id, source_event_id, student_id);

CREATE TABLE game.community_goal_progress (
  progress_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  goal_id uuid NOT NULL,
  scope_id uuid NOT NULL,
  total bigint NOT NULL DEFAULT 0,
  milestones_reached jsonb NOT NULL,
  recomputed_at timestamptz,
  PRIMARY KEY (progress_id)
);
CREATE UNIQUE INDEX community_goal_progress_scope_uq ON game.community_goal_progress (goal_id, scope_id) WHERE deleted_at IS NULL;

CREATE TABLE game.season_tracks (
  season_track_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  season_id uuid,
  code varchar(64),
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (season_track_id)
);

CREATE TABLE game.season_track_versions (
  season_track_version_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  season_track_id uuid NOT NULL,
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
  season_id uuid NOT NULL,
  PRIMARY KEY (season_track_version_id)
);

CREATE TABLE game.season_point_ledger (
  entry_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  season_id uuid NOT NULL,
  student_id uuid NOT NULL,
  delta bigint NOT NULL,
  source_code varchar(64) NOT NULL,
  source_event_id uuid NOT NULL,
  capped boolean NOT NULL DEFAULT false,
  earned_on date NOT NULL,
  PRIMARY KEY (entry_id)
);
CREATE INDEX season_point_ledger_day_idx ON game.season_point_ledger (tenant_id, season_id, student_id, earned_on);
CREATE UNIQUE INDEX season_point_ledger_source_uq ON game.season_point_ledger (season_id, source_event_id, student_id);

CREATE TABLE game.season_track_progress (
  progress_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  season_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  points bigint NOT NULL DEFAULT 0,
  tier bigint NOT NULL DEFAULT 0,
  claimed_tiers jsonb NOT NULL,
  auto_granted_tiers jsonb NOT NULL,
  auto_granted_at timestamptz,
  PRIMARY KEY (progress_id)
);
CREATE UNIQUE INDEX season_track_progress_student_uq ON game.season_track_progress (season_id, student_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- ref.seasons: projection of season.started / season.ended (module competition, owner F106) ----------
-- Minimal columns (DATABASE_DESIGN §2.17), platform rows (no tenant_id), no entx mixin; written only by the platform
-- consumer game.season_projection (game_platform), upserted by source_version (an older event never overwrites a newer one).
CREATE TABLE ref.seasons (
  season_id      uuid PRIMARY KEY,
  code           varchar(64) NOT NULL,
  name           varchar(128) NOT NULL DEFAULT '',
  theme_code     varchar(64),
  starts_at      timestamptz NOT NULL,
  ends_at        timestamptz NOT NULL,
  ended_at       timestamptz,
  status         varchar(64) NOT NULL CHECK (status IN ('started','ended')),
  source_version bigint NOT NULL
);
CREATE INDEX ref_seasons_started_idx ON ref.seasons (starts_at) WHERE status = 'started';
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON ref.seasons FROM game_app;
REVOKE DELETE, TRUNCATE ON ref.seasons FROM game_platform;
GRANT SELECT ON ref.seasons TO game_readonly;

-- ---------- events, versions (G/T): platform events (tenant_id NULL) and school events of a tenant ----------
-- Platform rows are written only under PlatformScope (game_platform: seed through PublishGameContent, Game Operations
-- Console F078); a tenant writes its own school events (RLS). Published versions are immutable (R11, R31).
ALTER TABLE game.events
  ADD CONSTRAINT events_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT events_owner_chk CHECK (owner_scope IN ('platform','tenant') AND ((owner_scope = 'platform') = (tenant_id IS NULL))),
  ADD CONSTRAINT events_template_chk CHECK (owner_scope = 'platform' OR template_id IS NOT NULL),
  ADD CONSTRAINT events_code_chk CHECK (code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT events_kind_chk CHECK (event_kind IN ('world_festival','weekend','school','community_goal','learning_campaign',
    'special_boss','limited_quest')),
  ADD CONSTRAINT events_params_chk CHECK (school_params IS NULL OR (jsonb_typeof(school_params) = 'object'
    AND octet_length(school_params::text) <= 16384)),
  ADD CONSTRAINT events_current_fk FOREIGN KEY (current_version_id) REFERENCES game.event_versions (event_version_id)
    DEFERRABLE INITIALLY DEFERRED;
CREATE UNIQUE INDEX events_platform_code_uq ON game.events (code) WHERE tenant_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX events_tenant_code_uq ON game.events (tenant_id, code) WHERE tenant_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX events_tenant_created_idx ON game.events (tenant_id, created_at, event_id) WHERE owner_scope = 'tenant' AND deleted_at IS NULL;
CREATE CONSTRAINT TRIGGER events_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.events
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('game', 'event_versions', 'event_version_id', 'event_id');

ALTER TABLE game.event_versions
  ADD CONSTRAINT event_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT event_versions_event_fk FOREIGN KEY (event_id) REFERENCES game.events (event_id),
  ADD CONSTRAINT event_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT event_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT event_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT event_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT event_versions_schedule_chk CHECK (start_at < end_at),
  ADD CONSTRAINT event_versions_areas_chk CHECK (jsonb_typeof(decoration_areas) = 'array' AND jsonb_array_length(decoration_areas) <= 20);
CREATE INDEX event_versions_areas_idx ON game.event_versions USING gin (decoration_areas) WHERE deleted_at IS NULL AND status = 'published';
CREATE TRIGGER event_versions_immutable BEFORE UPDATE ON game.event_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

-- ---------- event templates (G): platform content assigned to tenants (ADR-0029) ----------
ALTER TABLE game.event_templates
  ADD CONSTRAINT event_templates_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT event_templates_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.event_template_versions (template_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE CONSTRAINT TRIGGER event_templates_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.event_templates
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('game', 'event_template_versions', 'template_version_id', 'template_id');
ALTER TABLE game.event_template_versions
  ADD CONSTRAINT event_template_versions_template_fk FOREIGN KEY (template_id) REFERENCES game.event_templates (template_id),
  ADD CONSTRAINT event_template_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT event_template_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT event_template_versions_payload_chk CHECK (octet_length(payload::text) <= 65536),
  ADD CONSTRAINT event_template_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT event_template_versions_kind_chk CHECK (event_kind IN ('school','community_goal','learning_campaign')),
  ADD CONSTRAINT event_template_versions_days_chk CHECK (max_duration_days BETWEEN 1 AND 28),
  ADD CONSTRAINT event_template_versions_params_chk CHECK (jsonb_typeof(allowed_params) = 'array' AND jsonb_array_length(allowed_params) <= 30);
CREATE TRIGGER event_template_versions_immutable BEFORE UPDATE ON game.event_template_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

-- ---------- event runs (G/T): the run of a version — scheduled → live → ended → archived; suspended, cancelled ----------
-- status_version is the optimistic lock of the transitions (one transition whatever the scheduler instances, F077-AC-01).
ALTER TABLE game.event_runs
  ADD CONSTRAINT event_runs_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT event_runs_event_fk FOREIGN KEY (event_id) REFERENCES game.events (event_id),
  ADD CONSTRAINT event_runs_version_fk FOREIGN KEY (event_version_id) REFERENCES game.event_versions (event_version_id),
  ADD CONSTRAINT event_runs_status_chk CHECK (status IN ('scheduled','live','suspended','ended','cancelled','archived')),
  ADD CONSTRAINT event_runs_schedule_chk CHECK (starts_at < ends_at AND ends_at <= claim_until),
  ADD CONSTRAINT event_runs_status_version_chk CHECK (status_version >= 1),
  ADD CONSTRAINT event_runs_audience_chk CHECK (jsonb_typeof(audience) = 'object' AND octet_length(audience::text) <= 65536);
CREATE UNIQUE INDEX event_runs_active_uq ON game.event_runs (event_id) WHERE status IN ('scheduled','live','suspended') AND deleted_at IS NULL;

-- ---------- participation (T), event point ledger (TA) ----------
ALTER TABLE game.event_participation
  ADD CONSTRAINT event_participation_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT event_participation_run_fk FOREIGN KEY (event_run_id) REFERENCES game.event_runs (event_run_id),
  ADD CONSTRAINT event_participation_points_chk CHECK (event_points >= 0),
  ADD CONSTRAINT event_participation_lists_chk CHECK (jsonb_typeof(reached_milestones) = 'array' AND jsonb_array_length(reached_milestones) <= 50
    AND jsonb_typeof(claimed_milestones) = 'array' AND jsonb_array_length(claimed_milestones) <= 50
    AND jsonb_typeof(auto_granted_milestones) = 'array' AND jsonb_array_length(auto_granted_milestones) <= 50);
ALTER TABLE game.event_participation ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.event_participation FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.event_participation FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.event_participation FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.event_participation FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.event_participation FROM game_app, game_platform;

-- Event points (TA, ADR-0049): only credits (a missed day never takes points, F077-AC-29), never updated or deleted.
ALTER TABLE game.event_point_ledger
  ADD CONSTRAINT event_point_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT event_point_ledger_run_fk FOREIGN KEY (event_run_id) REFERENCES game.event_runs (event_run_id),
  ADD CONSTRAINT event_point_ledger_delta_chk CHECK (delta > 0);
ALTER TABLE game.event_point_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.event_point_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.event_point_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.event_point_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.event_point_ledger FROM game_app, game_platform;

-- ---------- community goal (G/T), contributions (TA), progress (T) ----------
ALTER TABLE game.community_goals
  ADD CONSTRAINT community_goals_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT community_goals_run_fk FOREIGN KEY (event_run_id) REFERENCES game.event_runs (event_run_id),
  ADD CONSTRAINT community_goals_metric_chk CHECK (metric IN ('correct_answers','activities_completed','quests_completed','discoveries_found',
    'boss_phases_cleared','coop_sessions')),
  ADD CONSTRAINT community_goals_scope_chk CHECK (scope IN ('school','tenant','platform') AND (scope <> 'platform' OR tenant_id IS NULL)),
  ADD CONSTRAINT community_goals_target_chk CHECK (target BETWEEN 1 AND 100000000 AND per_student_daily_cap BETWEEN 1 AND 10000),
  ADD CONSTRAINT community_goals_milestones_chk CHECK (jsonb_typeof(milestones) = 'array' AND jsonb_array_length(milestones) BETWEEN 1 AND 10);

-- Contributions (TA): one per goal, source event and student (F077-AC-13), capped per student per day; never changed.
ALTER TABLE game.community_goal_contributions
  ADD CONSTRAINT community_goal_contributions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT community_goal_contributions_goal_fk FOREIGN KEY (goal_id) REFERENCES game.community_goals (goal_id),
  ADD CONSTRAINT community_goal_contributions_amount_chk CHECK (amount > 0),
  ADD CONSTRAINT community_goal_contributions_grade_chk CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
ALTER TABLE game.community_goal_contributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.community_goal_contributions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.community_goal_contributions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.community_goal_contributions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.community_goal_contributions FROM game_app, game_platform;

-- Progress (T): the total of one scope (a cache of Σ contributions, recomputed by goal-recompute). No FORCE: the owner-run
-- function game.community_goal_platform_total sums a platform goal over every tenant (counts only, F077 Edge Cases);
-- application roles are never owners, so RLS applies to them (pattern of ref.projection_tenant).
ALTER TABLE game.community_goal_progress
  ADD CONSTRAINT community_goal_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT community_goal_progress_goal_fk FOREIGN KEY (goal_id) REFERENCES game.community_goals (goal_id),
  ADD CONSTRAINT community_goal_progress_total_chk CHECK (total >= 0),
  ADD CONSTRAINT community_goal_progress_reached_chk CHECK (jsonb_typeof(milestones_reached) = 'array'
    AND jsonb_array_length(milestones_reached) <= 10);
ALTER TABLE game.community_goal_progress ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.community_goal_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.community_goal_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.community_goal_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.community_goal_progress FROM game_app, game_platform;

CREATE FUNCTION game.community_goal_platform_total(p_goal uuid) RETURNS bigint
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, game AS $$
  SELECT coalesce(sum(p.total), 0)::bigint FROM game.community_goal_progress p
    JOIN game.community_goals g ON g.goal_id = p.goal_id AND g.deleted_at IS NULL AND g.scope = 'platform' AND g.tenant_id IS NULL
  WHERE p.goal_id = p_goal AND p.deleted_at IS NULL
$$;
REVOKE ALL ON FUNCTION game.community_goal_platform_total(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION game.community_goal_platform_total(uuid) TO game_app, game_platform;

-- ---------- G/T tables: platform rows readable by every tenant, tenant rows by their tenant ----------
ALTER TABLE game.events ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.events FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.events FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.event_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.event_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.event_versions FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.event_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.event_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.event_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.event_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.event_runs FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.event_runs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.event_runs FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE game.community_goals ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.community_goals FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.community_goals FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.community_goals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.community_goals FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.events, game.event_versions, game.event_runs, game.community_goals FROM game_app, game_platform;

-- ---------- season track (G), season point ledger (TA), season track progress (T) ----------
ALTER TABLE game.season_tracks
  ADD CONSTRAINT season_tracks_code_chk CHECK (code IS NULL OR code ~ '^[a-z0-9]+(\.[a-z0-9_]+)+$'),
  ADD CONSTRAINT season_tracks_current_fk FOREIGN KEY (current_version_id)
    REFERENCES game.season_track_versions (season_track_version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE UNIQUE INDEX season_tracks_season_uq ON game.season_tracks (season_id) WHERE deleted_at IS NULL;
CREATE CONSTRAINT TRIGGER season_tracks_current_published AFTER INSERT OR UPDATE OF current_version_id ON game.season_tracks
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
  EXECUTE FUNCTION game.check_current_version_in('game', 'season_track_versions', 'season_track_version_id', 'season_track_id');
ALTER TABLE game.season_track_versions
  ADD CONSTRAINT season_track_versions_track_fk FOREIGN KEY (season_track_id) REFERENCES game.season_tracks (season_track_id),
  ADD CONSTRAINT season_track_versions_no_chk CHECK (version_no >= 1),
  ADD CONSTRAINT season_track_versions_status_chk CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT season_track_versions_payload_chk CHECK (octet_length(payload::text) <= 65536 AND (payload->>'paid_track') = 'false'),
  ADD CONSTRAINT season_track_versions_sha_chk CHECK (payload_sha256 ~ '^[0-9a-f]{64}$');
CREATE TRIGGER season_track_versions_immutable BEFORE UPDATE ON game.season_track_versions
  FOR EACH ROW EXECUTE FUNCTION game.forbid_published_version_change();

ALTER TABLE game.season_point_ledger
  ADD CONSTRAINT season_point_ledger_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT season_point_ledger_delta_chk CHECK (delta > 0);
ALTER TABLE game.season_point_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.season_point_ledger FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.season_point_ledger FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.season_point_ledger FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.season_point_ledger FROM game_app, game_platform;

ALTER TABLE game.season_track_progress
  ADD CONSTRAINT season_track_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT season_track_progress_points_chk CHECK (points >= 0 AND tier BETWEEN 0 AND 50),
  ADD CONSTRAINT season_track_progress_lists_chk CHECK (jsonb_typeof(claimed_tiers) = 'array' AND jsonb_array_length(claimed_tiers) <= 50
    AND jsonb_typeof(auto_granted_tiers) = 'array' AND jsonb_array_length(auto_granted_tiers) <= 50);
ALTER TABLE game.season_track_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.season_track_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.season_track_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.season_track_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.season_track_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.season_track_progress FROM game_app, game_platform;

-- Platform content written only under PlatformScope (game_platform); students and schools only read it.
REVOKE INSERT, UPDATE ON game.event_templates, game.event_template_versions, game.season_tracks, game.season_track_versions FROM game_app;
REVOKE DELETE, TRUNCATE ON game.event_templates, game.event_template_versions, game.season_tracks, game.season_track_versions
  FROM game_app, game_platform;

-- The release history accepts the F077 item types; resource assignments accept event templates; reward grants of F077;
-- the achievement category event (F075 trigger catalog extended with the per-student live-ops events).
ALTER TABLE game.content_releases DROP CONSTRAINT content_releases_type_chk,
  ADD CONSTRAINT content_releases_type_chk CHECK (item_type IN ('world_area','story_chapter','npc_dialogue','level',
    'character_catalog','item','quest','minigame_config','progression_curve','reward_rule','shop_item','collection','achievement',
    'event','event_template','season_track'));
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck,
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('quest','minigame_config','item','shop_item','event_template'));
ALTER TABLE gamification.reward_grants DROP CONSTRAINT reward_grants_kind_chk,
  ADD CONSTRAINT reward_grants_kind_chk CHECK (trigger_kind IN ('event_rule','level_up','achievement','collection','knowledge_card',
    'event_participation','event_milestone','community_goal','season_tier'));
ALTER TABLE gamification.achievement_versions DROP CONSTRAINT achievement_versions_category_chk,
  ADD CONSTRAINT achievement_versions_category_chk CHECK (category IN ('first_time','progress','learning','exploration','social','team',
    'competition','seasonal','event','hidden'));

-- ---------- _live views for the readonly role (R27) ----------
CREATE OR REPLACE VIEW game.events_live WITH (security_barrier = true) AS
  SELECT * FROM game.events WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.event_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.event_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.event_templates_live WITH (security_barrier = true) AS
  SELECT * FROM game.event_templates WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.event_template_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.event_template_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.event_runs_live WITH (security_barrier = true) AS
  SELECT * FROM game.event_runs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.event_participation_live WITH (security_barrier = true) AS
  SELECT * FROM game.event_participation WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.community_goals_live WITH (security_barrier = true) AS
  SELECT * FROM game.community_goals WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.community_goal_progress_live WITH (security_barrier = true) AS
  SELECT * FROM game.community_goal_progress WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.season_tracks_live WITH (security_barrier = true) AS
  SELECT * FROM game.season_tracks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.season_track_versions_live WITH (security_barrier = true) AS
  SELECT * FROM game.season_track_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW game.season_track_progress_live WITH (security_barrier = true) AS
  SELECT * FROM game.season_track_progress WHERE deleted_at IS NULL;
GRANT SELECT ON game.events_live, game.event_versions_live, game.event_templates_live, game.event_template_versions_live, game.event_runs_live,
  game.event_participation_live, game.community_goals_live, game.community_goal_progress_live, game.season_tracks_live,
  game.season_track_versions_live, game.season_track_progress_live TO game_readonly;
GRANT SELECT ON game.event_point_ledger, game.community_goal_contributions, game.season_point_ledger TO game_readonly;
