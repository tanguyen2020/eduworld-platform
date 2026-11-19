-- F013 T005/T035/T044 — game login of module identity (DATABASE_DESIGN §2.1, §4, ADR-0086, ADR-0089).
-- The entddl block (game_auth_sessions) is rendered from the ent schema (golden test ent/ddl_test.go). Hand-written:
-- identity.login_events (TA*, monthly range partitions, composite key with the partition key), CHECKs, foreign
-- keys, RLS, append-only grants, <table>_live view, and the SECURITY DEFINER function identity.memberships_for_user
-- owned by identity_membership_reader (NOLOGIN BYPASSRLS, created by deploy/postgres/init). Applied as identity_migrator.

-- BEGIN entddl
CREATE TABLE identity.game_auth_sessions (
  game_auth_session_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_id uuid NOT NULL,
  keycloak_sid varchar(64) NOT NULL,
  device_id uuid NOT NULL,
  platform varchar(16) NOT NULL,
  refresh_token_hash varchar(64) NOT NULL,
  previous_token_hashes jsonb NOT NULL,
  last_refreshed_at timestamptz NOT NULL,
  revoked_at timestamptz,
  revoke_reason varchar(32),
  PRIMARY KEY (game_auth_session_id)
);
CREATE INDEX game_auth_sessions_device_idx ON identity.game_auth_sessions (device_id) WHERE revoked_at IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX game_auth_sessions_sid_uq ON identity.game_auth_sessions (keycloak_sid) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX game_auth_sessions_token_uq ON identity.game_auth_sessions (refresh_token_hash) WHERE revoked_at IS NULL AND deleted_at IS NULL;
CREATE INDEX game_auth_sessions_user_idx ON identity.game_auth_sessions (tenant_id, user_id) WHERE revoked_at IS NULL AND deleted_at IS NULL;
-- END entddl

-- ---------- identity.login_events (TA*, P): append-only login journal, no username/password/token/raw IP ----------
CREATE TABLE identity.login_events (
  login_event_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  user_id uuid,
  occurred_at timestamptz NOT NULL,
  client varchar(64) NOT NULL,
  ip_hash bytea,
  sis_connection_id uuid,
  device_id uuid,
  result varchar(64) NOT NULL,
  tenant_resolution varchar(64),
  PRIMARY KEY (login_event_id, occurred_at),
  CHECK (result IN ('success','invalid_credentials','account_not_provisioned','game_access_student_only','account_locked',
    'account_deactivated','no_active_enrollment','tenant_not_selected','tenant_mismatch','rate_limited','queue_busy',
    'client_version_unsupported','curfew_active','unavailable')),
  CHECK (tenant_resolution IS NULL OR tenant_resolution IN ('claim','single_membership','common','selected','mismatch')),
  CHECK (ip_hash IS NULL OR octet_length(ip_hash) <= 64)
) PARTITION BY RANGE (occurred_at);
CREATE TABLE identity.login_events_default PARTITION OF identity.login_events DEFAULT;
CREATE INDEX login_events_user_idx ON identity.login_events (user_id, occurred_at);
CREATE INDEX login_events_tenant_idx ON identity.login_events (tenant_id, occurred_at);

ALTER TABLE identity.login_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.login_events FORCE ROW LEVEL SECURITY;
-- identity_app reads/writes only rows of its tenant; rows with tenant_id NULL only through identity_platform (BYPASSRLS).
CREATE POLICY tenant_read ON identity.login_events FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.login_events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON identity.login_events FROM identity_app, identity_platform;
GRANT SELECT, INSERT ON identity.login_events TO identity_app, identity_platform;

-- ---------- identity.game_auth_sessions (T): CHECKs, FK, RLS, soft-delete only ----------
ALTER TABLE identity.game_auth_sessions
  ADD CONSTRAINT game_auth_sessions_platform_chk CHECK (platform IN ('windows','macos','linux','android','ios','web')),
  ADD CONSTRAINT game_auth_sessions_reason_chk CHECK (revoke_reason IS NULL OR revoke_reason IN ('logout','takeover',
    'password_changed','account_locked','sanction','access_revoked','cleanup','refresh_reuse','admin')),
  ADD CONSTRAINT game_auth_sessions_revoked_chk CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL)),
  ADD CONSTRAINT game_auth_sessions_hashes_chk CHECK (jsonb_typeof(previous_token_hashes) = 'array'
    AND jsonb_array_length(previous_token_hashes) <= 20),
  ADD CONSTRAINT game_auth_sessions_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id),
  ADD CONSTRAINT game_auth_sessions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);
CREATE INDEX game_auth_sessions_hashes_gin ON identity.game_auth_sessions USING gin (previous_token_hashes);

ALTER TABLE identity.game_auth_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.game_auth_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.game_auth_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.game_auth_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.game_auth_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON identity.game_auth_sessions FROM identity_app, identity_platform;

CREATE OR REPLACE VIEW identity.game_auth_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM identity.game_auth_sessions WHERE deleted_at IS NULL;
GRANT SELECT ON identity.game_auth_sessions_live TO identity_readonly;

-- ---------- Login path membership read (ADR-0089 §3, F013-AC-30) ----------
-- The function runs as identity_membership_reader (BYPASSRLS) and returns the memberships of exactly one user;
-- identity_app (no BYPASSRLS) may only EXECUTE it, inside tenancy.Bypass(ctx, "auth_login").
GRANT USAGE ON SCHEMA identity, org, platform TO identity_membership_reader;
GRANT SELECT ON org.enrollments, identity.user_roles, identity.tenant_memberships_v TO identity_membership_reader;
CREATE FUNCTION identity.memberships_for_user(p_user_id uuid)
RETURNS SETOF identity.tenant_memberships_v
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = identity, pg_temp AS $fn$
  SELECT m.* FROM identity.tenant_memberships_v m WHERE m.user_id = p_user_id
$fn$;
-- Changing the owner needs CREATE on the schema at that moment only: granted and revoked around the ALTER.
GRANT CREATE ON SCHEMA identity TO identity_membership_reader;
ALTER FUNCTION identity.memberships_for_user(uuid) OWNER TO identity_membership_reader;
REVOKE CREATE ON SCHEMA identity FROM identity_membership_reader;
REVOKE ALL ON FUNCTION identity.memberships_for_user(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.memberships_for_user(uuid) TO identity_app;
