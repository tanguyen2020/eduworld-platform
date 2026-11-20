-- F095 T005/T006 — friends, block, mute of module chat (specs/095-friends-block-mute: data-model §1; DATABASE_DESIGN
-- §2.11, §2.17; ADR-0028, ADR-0041, ADR-0054, ADR-0089). Applied by `migrate --module=chat` as chat_migrator. Order:
-- projections ref.* (hand-written, rule 5), ent tables (entddl block, golden test modules/chat/ent/ddl_test.go), then
-- what ent cannot express: CHECKs, FKs, RLS, soft-delete only grants, _live views, functions.

-- ---------- Projections (DATABASE_DESIGN §2.17): minimal columns, written only by the consumers of chat ----------
-- identity · tenant.*, student.*, user.status_changed (consumer chat.projection_identity). settings_subset carries
-- cross_school_social_enabled (tenant unit, ADR-0054) and common_social_enabled (tenant common re-enabled by System
-- Admin, ADR-0078) [Proposed F095].
CREATE TABLE ref.tenants (
  tenant_id       uuid PRIMARY KEY,
  code            varchar(64),
  tenant_type     varchar(64) NOT NULL CHECK (tenant_type IN ('school','unit','common')),
  status          varchar(64) NOT NULL CHECK (status IN ('active','disabled')),
  is_qa           boolean NOT NULL DEFAULT false,
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);

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

-- game · game_profile.updated (consumer chat.projection_game): public profile only (R16), avatar ≤ 4 KB.
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

-- RLS of the tenant projections: tenant condition only, no FORCE (rule 5: the owner-run ref.projection_tenant sees
-- the owning tenant of a row across tenants). ref.tenants is not tenant-scoped.
ALTER TABLE ref.students ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.player_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.player_profiles FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.player_profiles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.player_profiles FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Owning tenant of a projected row whatever the session tenant (NULL when unknown, MULTITENANCY §4.1).
CREATE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
    WHEN 'player_profiles' THEN (SELECT pp.tenant_id FROM ref.player_profiles pp WHERE pp.student_id = p_id)
  END
$$;
REVOKE ALL ON FUNCTION ref.projection_tenant(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ref.projection_tenant(text, uuid) TO chat_app, chat_platform;

-- BEGIN entddl
CREATE TABLE social.friend_requests (
  request_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  requester_id uuid NOT NULL,
  addressee_id uuid NOT NULL,
  status varchar(32) NOT NULL DEFAULT 'pending',
  source varchar(64) NOT NULL DEFAULT 'direct',
  expires_at timestamptz NOT NULL,
  responded_at timestamptz,
  PRIMARY KEY (request_id)
);
CREATE INDEX friend_requests_addressee_idx ON social.friend_requests (tenant_id, addressee_id, status, created_at);
CREATE INDEX friend_requests_expiry_idx ON social.friend_requests (status, expires_at) WHERE status = 'pending';
CREATE UNIQUE INDEX friend_requests_pending_uq ON social.friend_requests (requester_id, addressee_id) WHERE status = 'pending' AND deleted_at IS NULL;
CREATE INDEX friend_requests_requester_idx ON social.friend_requests (tenant_id, requester_id, created_at);

CREATE TABLE social.friendships (
  friendship_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_low_id uuid NOT NULL,
  user_high_id uuid NOT NULL,
  PRIMARY KEY (friendship_id)
);
CREATE INDEX friendships_high_idx ON social.friendships (tenant_id, user_high_id);
CREATE INDEX friendships_low_idx ON social.friendships (tenant_id, user_low_id);
CREATE UNIQUE INDEX friendships_pair_uq ON social.friendships (user_low_id, user_high_id) WHERE deleted_at IS NULL;

CREATE TABLE social.friend_codes (
  friend_code_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(12) NOT NULL,
  owner_id uuid NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (friend_code_id)
);
CREATE UNIQUE INDEX friend_codes_code_uq ON social.friend_codes (tenant_id, code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX friend_codes_owner_uq ON social.friend_codes (owner_id) WHERE deleted_at IS NULL;

CREATE TABLE social.blocks (
  block_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  blocker_id uuid NOT NULL,
  blocked_id uuid NOT NULL,
  PRIMARY KEY (block_id)
);
CREATE INDEX blocks_blocked_idx ON social.blocks (tenant_id, blocked_id);
CREATE UNIQUE INDEX blocks_pair_uq ON social.blocks (blocker_id, blocked_id) WHERE deleted_at IS NULL;

CREATE TABLE social.mutes (
  mute_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  muter_id uuid NOT NULL,
  muted_id uuid NOT NULL,
  PRIMARY KEY (mute_id)
);
CREATE INDEX mutes_muted_idx ON social.mutes (tenant_id, muted_id);
CREATE UNIQUE INDEX mutes_pair_uq ON social.mutes (muter_id, muted_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- social.* (T): CHECKs, FK, RLS (FORCE), soft-delete only ----------
ALTER TABLE social.friend_requests
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT friend_requests_status_chk CHECK (status IN ('pending','accepted','rejected','cancelled','expired')),
  ADD CONSTRAINT friend_requests_source_chk CHECK (source IN ('direct','friend_code')),
  ADD CONSTRAINT friend_requests_pair_chk CHECK (requester_id <> addressee_id),
  ADD CONSTRAINT friend_requests_responded_chk CHECK (status = 'pending' OR responded_at IS NOT NULL),
  ADD CONSTRAINT friend_requests_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE social.friendships
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT friendships_order_chk CHECK (user_low_id < user_high_id),
  ADD CONSTRAINT friendships_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE social.friend_codes
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT friend_codes_code_chk CHECK (code ~ '^[A-Z0-9]{6,12}$'),
  ADD CONSTRAINT friend_codes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE social.blocks
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT blocks_pair_chk CHECK (blocker_id <> blocked_id),
  ADD CONSTRAINT blocks_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE social.mutes
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT mutes_pair_chk CHECK (muter_id <> muted_id),
  ADD CONSTRAINT mutes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE social.friend_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.friend_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.friend_requests FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.friend_requests FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.friend_requests FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.friendships ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.friendships FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.friendships FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.friendships FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.friendships FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.friend_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.friend_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.friend_codes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.friend_codes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.friend_codes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.blocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.blocks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.blocks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.blocks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.blocks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.mutes ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.mutes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.mutes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.mutes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.mutes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON social.friend_requests, social.friendships, social.friend_codes, social.blocks, social.mutes
  FROM chat_app, chat_platform;
REVOKE DELETE, TRUNCATE ON ref.tenants, ref.students, ref.player_profiles FROM chat_app, chat_platform;

-- ---------- _live views for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW social.friend_requests_live WITH (security_barrier = true) AS
  SELECT * FROM social.friend_requests WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.friendships_live WITH (security_barrier = true) AS
  SELECT * FROM social.friendships WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.friend_codes_live WITH (security_barrier = true) AS
  SELECT * FROM social.friend_codes WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.blocks_live WITH (security_barrier = true) AS
  SELECT * FROM social.blocks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.mutes_live WITH (security_barrier = true) AS
  SELECT * FROM social.mutes WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA social TO chat_readonly;
GRANT SELECT ON social.friend_requests_live, social.friendships_live, social.friend_codes_live, social.blocks_live,
  social.mutes_live TO chat_readonly;
