-- F091 T007/T008 — hub instance, presence, position sync of module competition (specs/091-hub-instance-presence:
-- data-model §1; DATABASE_DESIGN §2.9, §2.16, §2.17; ADR-0004, ADR-0028, ADR-0041, ADR-0054, ADR-0089). Applied by
-- `migrate --module=competition` as competition_migrator. Order: projections ref.* (hand-written, rule 5), ent tables
-- (entddl block, golden test modules/competition/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, RLS,
-- soft-delete only grants, the append-only integrity signals, _live views, functions.

-- ---------- Projections (DATABASE_DESIGN §2.17): minimal columns, written only by the consumers of competition ----------
-- identity · tenant.*, student.*, user.status_changed (consumer competition.projection_identity)
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

-- game · game_profile.updated (consumer competition.projection_game): public profile only (R16), avatar ≤ 4 KB.
CREATE TABLE ref.player_profiles (
  student_id       uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  nickname         varchar(32),
  avatar_thumb_key varchar(512),
  level            int NOT NULL DEFAULT 1 CHECK (level >= 1),
  avatar_config    jsonb CHECK (avatar_config IS NULL OR octet_length(avatar_config::text) <= 4096),
  source_version   bigint NOT NULL
);
CREATE INDEX ref_player_profiles_tenant_idx ON ref.player_profiles (tenant_id);

-- game · content.published/unpublished (item_type = world_area, owner F071/F078): runtime data of an area. Global (G)
-- content: no tenant. bounds = {"min":[x,y,z],"max":[x,y,z]} (metres), spawn_points = {"<name>":[x,y,z]} [Proposed F091].
CREATE TABLE ref.world_areas (
  area_id             varchar(64) PRIMARY KEY,
  capacity            int NOT NULL CHECK (capacity BETWEEN 1 AND 40),
  max_speed_mps       numeric(5,2) NOT NULL CHECK (max_speed_mps > 0),
  bounds              jsonb NOT NULL CHECK (octet_length(bounds::text) <= 16384),
  spawn_points        jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(spawn_points::text) <= 8192),
  allowed_grade_bands varchar(64)[] NOT NULL CHECK (cardinality(allowed_grade_bands) BETWEEN 1 AND 4
    AND allowed_grade_bands <@ ARRAY['g1_3','g4_5','g6_7','g8_9']::varchar(64)[]),
  requires_unlock     boolean NOT NULL DEFAULT false,
  status              varchar(64) NOT NULL CHECK (status IN ('active','inactive')),
  source_version      bigint NOT NULL
);

-- game · world.area_unlocked (owner F071)
CREATE TABLE ref.area_unlocks (
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  student_id     uuid NOT NULL,
  area_id        varchar(64) NOT NULL,
  unlocked_at    timestamptz NOT NULL,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, area_id)
);
CREATE INDEX ref_area_unlocks_tenant_idx ON ref.area_unlocks (tenant_id);

-- chat · friend.added/removed, user.blocked/unblocked (owner F095). Owner F091; F100, F104 read only.
CREATE TABLE ref.friendships (
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  user_low_id    uuid NOT NULL,
  user_high_id   uuid NOT NULL,
  active         boolean NOT NULL,
  source_version bigint NOT NULL,
  PRIMARY KEY (user_low_id, user_high_id),
  CHECK (user_low_id < user_high_id)
);
CREATE INDEX ref_friendships_high_idx ON ref.friendships (user_high_id) WHERE active;

CREATE TABLE ref.blocks (
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  blocker_id     uuid NOT NULL,
  blocked_id     uuid NOT NULL,
  active         boolean NOT NULL,
  source_version bigint NOT NULL,
  PRIMARY KEY (blocker_id, blocked_id),
  CHECK (blocker_id <> blocked_id)
);
CREATE INDEX ref_blocks_blocked_idx ON ref.blocks (blocked_id) WHERE active;

-- RLS of the tenant projections: tenant condition only, no FORCE (rule 5: the owner-run ref.projection_tenant sees
-- the owning tenant of a row across tenants). ref.tenants and ref.world_areas are not tenant-scoped.
ALTER TABLE ref.students ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.player_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.player_profiles FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.player_profiles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.player_profiles FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.area_unlocks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.area_unlocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.area_unlocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.area_unlocks FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.friendships ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.friendships FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.friendships FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.friendships FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.blocks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.blocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.blocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.blocks FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ref.world_areas is platform content: written by the platform consumer only (competition_platform), read by both.
REVOKE INSERT, UPDATE ON ref.world_areas FROM competition_app;

-- Owning tenant of a projected row whatever the session tenant (NULL when unknown, MULTITENANCY §4.1).
CREATE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
    WHEN 'player_profiles' THEN (SELECT pp.tenant_id FROM ref.player_profiles pp WHERE pp.student_id = p_id)
  END
$$;
REVOKE ALL ON FUNCTION ref.projection_tenant(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ref.projection_tenant(text, uuid) TO competition_app, competition_platform;

-- BEGIN entddl
CREATE TABLE multiplayer.rooms (
  room_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  room_type varchar(64) NOT NULL,
  area_id varchar(64) NOT NULL,
  school_id uuid,
  grade_band varchar(64),
  capacity bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'open',
  closed_at timestamptz,
  PRIMARY KEY (room_id)
);
CREATE INDEX rooms_partition_open_idx ON multiplayer.rooms (tenant_id, area_id, school_id, grade_band) WHERE status = 'open' AND deleted_at IS NULL;

CREATE TABLE multiplayer.session_memberships (
  membership_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  room_id uuid NOT NULL,
  room_type varchar(64) NOT NULL,
  school_id uuid,
  student_id uuid NOT NULL,
  game_session_id uuid NOT NULL,
  party_id uuid,
  match_id uuid,
  encounter_id uuid,
  joined_at timestamptz NOT NULL,
  left_at timestamptz,
  leave_reason varchar(64),
  disconnects bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (membership_id)
);
CREATE UNIQUE INDEX session_memberships_active_uq ON multiplayer.session_memberships (room_id, student_id) WHERE left_at IS NULL AND deleted_at IS NULL;
CREATE INDEX session_memberships_student_idx ON multiplayer.session_memberships (tenant_id, student_id, joined_at);
-- END entddl

-- ---------- multiplayer.rooms (T): CHECKs, FK, RLS, soft-delete only ----------
ALTER TABLE multiplayer.rooms
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT rooms_type_chk CHECK (room_type IN ('hub_instance','private_instance','lobby','battle','coop')),
  ADD CONSTRAINT rooms_grade_band_chk CHECK (grade_band IS NULL OR grade_band IN ('g1_3','g4_5','g6_7','g8_9')),
  ADD CONSTRAINT rooms_capacity_chk CHECK (capacity BETWEEN 1 AND 40),
  ADD CONSTRAINT rooms_status_chk CHECK (status IN ('open','closed')),
  ADD CONSTRAINT rooms_closed_chk CHECK ((status = 'closed') = (closed_at IS NOT NULL)),
  ADD CONSTRAINT rooms_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.rooms ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.rooms FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.rooms FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.rooms FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.rooms FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.rooms FROM competition_app, competition_platform;

-- ---------- multiplayer.session_memberships (T): left_at/leave_reason set once ----------
ALTER TABLE multiplayer.session_memberships
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT session_memberships_room_type_chk CHECK (room_type IN ('hub_instance','private_instance','lobby','battle','coop','boss')),
  ADD CONSTRAINT session_memberships_leave_reason_chk CHECK (leave_reason IS NULL OR leave_reason IN ('left','disconnected','kicked',
    'timeout','room_closed','transferred','replaced_by_other_device')),
  ADD CONSTRAINT session_memberships_left_chk CHECK ((left_at IS NULL) = (leave_reason IS NULL)),
  ADD CONSTRAINT session_memberships_disconnects_chk CHECK (disconnects >= 0),
  ADD CONSTRAINT session_memberships_room_fk FOREIGN KEY (room_id) REFERENCES multiplayer.rooms (room_id),
  ADD CONSTRAINT session_memberships_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.session_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.session_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.session_memberships FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.session_memberships FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.session_memberships FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.session_memberships FROM competition_app, competition_platform;

CREATE FUNCTION multiplayer.session_memberships_leave_once() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.left_at IS NOT NULL AND (NEW.left_at IS DISTINCT FROM OLD.left_at OR NEW.leave_reason IS DISTINCT FROM OLD.leave_reason) THEN
    RAISE EXCEPTION 'MEMBERSHIP_ALREADY_LEFT' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER session_memberships_leave_once BEFORE UPDATE ON multiplayer.session_memberships FOR EACH ROW
  EXECUTE FUNCTION multiplayer.session_memberships_leave_once();

-- ---------- competition.integrity_signals (T, P, append-only: DATABASE_DESIGN §2.16, ANTI_CHEAT §6) ----------
-- Never an automatic sanction. evidence ≤ 4 KB, ids and counters only (no PII). Monthly partitions come with the
-- partition job of business tables; until then rows land in the default partition (same as game.login_events).
CREATE TABLE competition.integrity_signals (
  signal_id   uuid NOT NULL,
  tenant_id   uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id   uuid,
  student_id  uuid NOT NULL,
  match_id    uuid,
  room_id     uuid,
  signal_type varchar(64) NOT NULL CHECK (signal_type IN ('hub_speed','hub_bounds')),
  score       numeric(6,2) NOT NULL DEFAULT 0,
  evidence    jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(evidence::text) <= 4096),
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (signal_id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE competition.integrity_signals_default PARTITION OF competition.integrity_signals DEFAULT;
CREATE INDEX integrity_signals_student_idx ON competition.integrity_signals (tenant_id, student_id, created_at DESC);
ALTER TABLE competition.integrity_signals ENABLE ROW LEVEL SECURITY;
ALTER TABLE competition.integrity_signals FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON competition.integrity_signals FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON competition.integrity_signals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON competition.integrity_signals, competition.integrity_signals_default FROM competition_app, competition_platform;
REVOKE ALL ON competition.integrity_signals_default FROM competition_app, competition_platform;
GRANT SELECT, INSERT ON competition.integrity_signals TO competition_app, competition_platform;

-- ---------- _live views for reporting (competition_readonly) ----------
CREATE OR REPLACE VIEW multiplayer.rooms_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.rooms WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.session_memberships_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.session_memberships WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA multiplayer, competition TO competition_readonly;
GRANT SELECT ON multiplayer.rooms_live, multiplayer.session_memberships_live TO competition_readonly;
