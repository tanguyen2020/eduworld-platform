-- OQ-17 (owner 2026-11-20), F013-AC-33 / T048 — daily play limit at the game login of module identity
-- (CHILD_SAFETY §4, CONFIG_DEFAULTS daily_play_limit_minutes). Hand-written table accessed by explicit SQL.
-- Applied as identity_migrator.

-- ---------- identity.daily_play_time (T, projection): minutes played per student and ICT day ----------
-- Written only by the consumer identity.daily_play_time from game_session.ended of module game (duration_ms split
-- over the ICT days it covers); read by the game login to refuse DAILY_PLAY_LIMIT_REACHED without calling module
-- game (R30, ADR-0045). A projection of another module's events: no soft-delete columns, rows are only added to
-- (one row per student and day played), never removed by the application.
CREATE TABLE identity.daily_play_time (
  tenant_id   uuid NOT NULL REFERENCES platform.tenants (tenant_id),
  user_id     uuid NOT NULL REFERENCES identity.users (user_id),
  play_date   date NOT NULL,
  played_ms   bigint NOT NULL DEFAULT 0 CHECK (played_ms >= 0 AND played_ms <= 86400000),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, user_id, play_date)
);
ALTER TABLE identity.daily_play_time ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.daily_play_time FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_rw ON identity.daily_play_time FOR ALL USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE ALL ON identity.daily_play_time FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON identity.daily_play_time TO identity_app, identity_platform;

-- ---------- login journal: refusal at the daily play limit ----------
ALTER TABLE identity.login_events DROP CONSTRAINT login_events_result_check;
ALTER TABLE identity.login_events ADD CONSTRAINT login_events_result_check CHECK (result IN ('success','invalid_credentials',
  'account_not_provisioned','game_access_student_only','account_locked','account_deactivated','no_active_enrollment',
  'tenant_not_selected','tenant_mismatch','rate_limited','queue_busy','client_version_unsupported','curfew_active','unavailable',
  'access_outside_allowed_hours','daily_play_limit_reached'));
