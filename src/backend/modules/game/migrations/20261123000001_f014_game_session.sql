-- F014 T006/T007 — game session of module game (specs/014-game-session-thin: data-model §1; DATABASE_DESIGN §2.8,
-- §2.16, §2.17, §3.12, §3.13, §4; ADR-0028, ADR-0031, ADR-0043, ADR-0089). Applied by `migrate --module=game` as
-- game_migrator. Order: projections ref.* (hand-written), ent tables (entddl block, golden test
-- modules/game/ent/ddl_test.go), then what ent cannot express: game.login_events (partitioned), game.content_manifests
-- (composite key), CHECKs, FKs, RLS, append-only grants, _live views, functions.

-- Projections of module identity (DATABASE_DESIGN §2.17): minimal columns, no entx mixin (no audit or soft-delete
-- columns of their own, rule 2); written only by the consumer game.projection_identity, upserted by source_version
-- (rule 1). ref.schools keeps settings_subset for the school curfew window (ADR-0078).
CREATE TABLE ref.tenants (
  tenant_id       uuid PRIMARY KEY,
  code            varchar(64),
  tenant_type     varchar(64) NOT NULL CHECK (tenant_type IN ('school','unit','common')),
  status          varchar(64) NOT NULL CHECK (status IN ('active','disabled')),
  is_qa           boolean NOT NULL DEFAULT false,
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);
CREATE INDEX ref_tenants_qa_idx ON ref.tenants (tenant_id) WHERE is_qa;

CREATE TABLE ref.schools (
  school_id       uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status          varchar(64) NOT NULL DEFAULT 'active',
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);
CREATE INDEX ref_schools_tenant_idx ON ref.schools (tenant_id);

CREATE TABLE ref.students (
  student_id     uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id      uuid NOT NULL,
  grade_level    smallint CHECK (grade_level BETWEEN 1 AND 9),
  status         varchar(64) NOT NULL DEFAULT 'active',
  deleted_at     timestamptz,
  source_version bigint NOT NULL
);
CREATE INDEX ref_students_tenant_school_idx ON ref.students (tenant_id, school_id);

-- RLS of the tenant projections: tenant condition only (ADR-0089 §1). No FORCE: the owner-run guard function
-- ref.projection_tenant must see the owning tenant of a row across tenants (F020-AC-04 pattern); application roles
-- are never owners, so RLS applies to them. ref.tenants is the tenant registry (not tenant-scoped).
ALTER TABLE ref.schools ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.students ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Owning tenant of a projected school/student whatever the session tenant (NULL when unknown): the consumer refuses
-- an event whose tenant differs from the projection row it references (MULTITENANCY §4.1). Returns only a tenant id.
CREATE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'schools' THEN (SELECT s.tenant_id FROM ref.schools s WHERE s.school_id = p_id)
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
  END
$$;
REVOKE ALL ON FUNCTION ref.projection_tenant(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ref.projection_tenant(text, uuid) TO game_app, game_platform;

-- BEGIN entddl
CREATE TABLE game.game_sessions (
  game_session_id uuid NOT NULL,
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
  school_id uuid,
  device_id uuid NOT NULL,
  device_model varchar(128),
  client_platform varchar(64) NOT NULL,
  client_version varchar(64) NOT NULL,
  keycloak_sid varchar(64),
  started_at timestamptz NOT NULL,
  last_heartbeat_at timestamptz NOT NULL,
  ended_at timestamptz,
  end_reason varchar(64),
  replaced_by_session_id uuid,
  PRIMARY KEY (game_session_id)
);
CREATE INDEX game_sessions_heartbeat_idx ON game.game_sessions (last_heartbeat_at) WHERE ended_at IS NULL;
CREATE UNIQUE INDEX game_sessions_one_active_uq ON game.game_sessions (student_id) WHERE ended_at IS NULL AND deleted_at IS NULL;
-- END entddl

-- ---------- game.game_sessions (T): CHECKs, FKs, RLS, soft-delete only ----------
-- replaced_by_session_id is deferred: a takeover ends the old session (unique active index) before it inserts the
-- new one, in the same transaction (DATABASE_DESIGN §3.12).
ALTER TABLE game.game_sessions
  ADD CONSTRAINT game_sessions_platform_chk CHECK (client_platform IN ('windows','android','ios','web','macos','linux')),
  ADD CONSTRAINT game_sessions_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('logout','replaced_by_other_device',
    'timeout','account_locked','access_revoked','cleanup','client_crash','curfew')),
  ADD CONSTRAINT game_sessions_ended_chk CHECK ((ended_at IS NULL) = (end_reason IS NULL)),
  ADD CONSTRAINT game_sessions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT game_sessions_replaced_by_fk FOREIGN KEY (replaced_by_session_id)
    REFERENCES game.game_sessions (game_session_id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE game.game_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.game_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.game_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.game_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.game_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON game.game_sessions FROM game_app, game_platform;

CREATE OR REPLACE VIEW game.game_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM game.game_sessions WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA game TO game_readonly;
GRANT SELECT ON game.game_sessions_live TO game_readonly;

-- ---------- game.login_events (TA, P month on login_at): append-only, no raw IP ----------
-- Monthly partitions are added by the partition job of business tables (later feature); until then rows land in the
-- default partition (same as identity.login_events, F013).
CREATE TABLE game.login_events (
  login_event_id    uuid NOT NULL,
  tenant_id         uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  created_at        timestamptz NOT NULL,
  created_by        uuid,
  student_id        uuid NOT NULL,
  game_session_id   uuid NOT NULL,
  device_id         uuid NOT NULL,
  platform          varchar(64) NOT NULL,
  device_model      varchar(128),
  client_version    varchar(64),
  ip_hash           bytea,
  login_at          timestamptz NOT NULL,
  logout_at         timestamptz,
  end_reason        varchar(64),
  sis_connection_id uuid,
  PRIMARY KEY (login_event_id, login_at),
  CHECK (platform IN ('windows','android','ios','web','macos','linux')),
  CHECK (end_reason IS NULL OR end_reason IN ('logout','replaced_by_other_device','timeout','account_locked','access_revoked',
    'cleanup','client_crash','curfew')),
  CHECK ((logout_at IS NULL) = (end_reason IS NULL)),
  CHECK (ip_hash IS NULL OR octet_length(ip_hash) <= 64)
) PARTITION BY RANGE (login_at);
CREATE TABLE game.login_events_default PARTITION OF game.login_events DEFAULT;
CREATE INDEX login_events_student_idx ON game.login_events (tenant_id, student_id, login_at DESC);
CREATE INDEX login_events_session_idx ON game.login_events (game_session_id);
CREATE INDEX login_events_tenant_idx ON game.login_events (tenant_id, login_at DESC);

ALTER TABLE game.login_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.login_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.login_events FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.login_events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON game.login_events FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
-- Append-only: the application may only INSERT, SELECT and record the end once (logout_at, end_reason).
REVOKE UPDATE, DELETE, TRUNCATE ON game.login_events, game.login_events_default FROM game_app, game_platform;
REVOKE ALL ON game.login_events_default FROM game_app, game_platform;
GRANT SELECT, INSERT ON game.login_events TO game_app, game_platform;
GRANT UPDATE (logout_at, end_reason) ON game.login_events TO game_app, game_platform;

-- The end of a login event is written once and nothing else changes (DATABASE_DESIGN §2.8).
CREATE FUNCTION game.login_events_close_once() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.logout_at IS NOT NULL THEN
    RAISE EXCEPTION 'LOGIN_EVENT_CLOSED' USING ERRCODE = 'P0001';
  END IF;
  IF (to_jsonb(NEW) - ARRAY['logout_at', 'end_reason']) <> (to_jsonb(OLD) - ARRAY['logout_at', 'end_reason']) THEN
    RAISE EXCEPTION 'LOGIN_EVENT_APPEND_ONLY' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER login_events_close_once BEFORE UPDATE ON game.login_events FOR EACH ROW
  EXECUTE FUNCTION game.login_events_close_once();

-- ---------- game.content_manifests (T, DATABASE_DESIGN §2.16, §3.13): tenant content manifest ----------
-- Written by the content release (F026/F078); F014 only reads the latest version of the session tenant.
CREATE TABLE game.content_manifests (
  tenant_id        uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  manifest_version bigint NOT NULL CHECK (manifest_version > 0),
  etag             varchar(64) NOT NULL,
  entries          jsonb NOT NULL CHECK (octet_length(entries::text) <= 4194304),
  generated_at     timestamptz NOT NULL DEFAULT now(),
  cause_release_id uuid,
  PRIMARY KEY (tenant_id, manifest_version)
);
ALTER TABLE game.content_manifests ENABLE ROW LEVEL SECURITY;
ALTER TABLE game.content_manifests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON game.content_manifests FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON game.content_manifests FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON game.content_manifests FROM game_app, game_platform;
