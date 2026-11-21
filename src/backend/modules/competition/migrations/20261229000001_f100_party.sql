-- F100 T008/T009 — party of module competition (specs/100-party-management: data-model §1; DATABASE_DESIGN §2.9,
-- §2.16, §2.17, §3.7; ADR-0004, ADR-0028, ADR-0041, ADR-0054, ADR-0089). Applied by `migrate --module=competition` as
-- competition_migrator. Order: new projections ref.* (hand-written, rule 5), ent tables (entddl block, golden test
-- modules/competition/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, RLS, soft-delete only grants, the
-- membership leave-once trigger, the append-only partitioned party history, _live views.

-- ---------- Projections (DATABASE_DESIGN §2.17), written only by the consumers of competition ----------
-- identity · enrollment.changed (consumer competition.projection_identity): the class of a student (same-class rules,
-- visibility = class). At most one active enrollment per student at the source.
CREATE TABLE ref.enrollments (
  student_id     uuid NOT NULL,
  class_id       uuid NOT NULL,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id      uuid,
  status         varchar(64) NOT NULL CHECK (status IN ('active','transferred','ended')),
  start_date     date,
  end_date       date,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, class_id)
);
CREATE INDEX ref_enrollments_active_idx ON ref.enrollments (tenant_id, student_id) WHERE status = 'active';

-- chat · sanction.applied / sanction.lifted (owner F121; consumer competition.party_safety): only the sanctions that
-- restrict multiplayer are read (restrict_multiplayer, account_lock). until NULL = no end.
CREATE TABLE ref.sanctions (
  sanction_id    uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  user_id        uuid NOT NULL,
  sanction_type  varchar(64) NOT NULL,
  until          timestamptz,
  active         boolean NOT NULL,
  source_version bigint NOT NULL
);
CREATE INDEX ref_sanctions_user_idx ON ref.sanctions (user_id) WHERE active;

ALTER TABLE ref.enrollments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.sanctions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.sanctions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.sanctions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.sanctions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- BEGIN entddl
CREATE TABLE multiplayer.parties (
  party_id uuid NOT NULL,
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
  name varchar(128) NOT NULL,
  leader_id uuid NOT NULL,
  party_type varchar(64) NOT NULL,
  visibility varchar(64) NOT NULL DEFAULT 'private',
  invitation_policy varchar(64) NOT NULL DEFAULT 'leader_only',
  max_members bigint NOT NULL,
  grade_band varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'open',
  current_activity_id uuid,
  current_game_session_ref uuid,
  state_version bigint NOT NULL DEFAULT 0,
  disbanded_at timestamptz,
  PRIMARY KEY (party_id)
);
CREATE INDEX parties_school_status_idx ON multiplayer.parties (school_id, status) WHERE deleted_at IS NULL;
CREATE INDEX parties_tenant_status_idx ON multiplayer.parties (tenant_id, status) WHERE deleted_at IS NULL;

CREATE TABLE multiplayer.party_members (
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
  party_id uuid NOT NULL,
  student_id uuid NOT NULL,
  school_id uuid NOT NULL,
  role varchar(64) NOT NULL,
  invitation_id uuid,
  joined_at timestamptz NOT NULL,
  left_at timestamptz,
  left_reason varchar(64),
  PRIMARY KEY (membership_id)
);
CREATE UNIQUE INDEX party_members_active_uq ON multiplayer.party_members (party_id, student_id) WHERE left_at IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX party_members_one_active_party_uq ON multiplayer.party_members (student_id) WHERE left_at IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX party_members_one_leader_uq ON multiplayer.party_members (party_id) WHERE left_at IS NULL AND role = 'leader' AND deleted_at IS NULL;
CREATE INDEX party_members_party_idx ON multiplayer.party_members (party_id, joined_at);
CREATE INDEX party_members_student_left_idx ON multiplayer.party_members (tenant_id, student_id, left_reason, left_at);

CREATE TABLE multiplayer.party_invitations (
  invitation_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  party_id uuid NOT NULL,
  school_id uuid NOT NULL,
  inviter_id uuid NOT NULL,
  invitee_id uuid NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'pending',
  expires_at timestamptz NOT NULL,
  responded_at timestamptz,
  end_reason varchar(64),
  PRIMARY KEY (invitation_id)
);
CREATE INDEX party_invitations_expiry_idx ON multiplayer.party_invitations (expires_at) WHERE status = 'pending' AND deleted_at IS NULL;
CREATE INDEX party_invitations_invitee_idx ON multiplayer.party_invitations (invitee_id, status, expires_at);
CREATE INDEX party_invitations_inviter_idx ON multiplayer.party_invitations (tenant_id, inviter_id, created_at);
CREATE UNIQUE INDEX party_invitations_pending_uq ON multiplayer.party_invitations (party_id, invitee_id) WHERE status = 'pending' AND deleted_at IS NULL;

CREATE TABLE multiplayer.party_policies (
  policy_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope_type varchar(64) NOT NULL,
  scope_id uuid NOT NULL,
  school_id uuid NOT NULL,
  rules jsonb NOT NULL,
  PRIMARY KEY (policy_id)
);
CREATE UNIQUE INDEX party_policies_scope_uq ON multiplayer.party_policies (scope_type, scope_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- multiplayer.parties (T): CHECKs, FK, RLS, soft-delete only ----------
ALTER TABLE multiplayer.parties
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT parties_name_chk CHECK (char_length(name) BETWEEN 1 AND 40),
  ADD CONSTRAINT parties_type_chk CHECK (party_type IN ('friend','study_group','quest','competition_team','custom')),
  ADD CONSTRAINT parties_visibility_chk CHECK (visibility IN ('private','friends','class')),
  ADD CONSTRAINT parties_invitation_policy_chk CHECK (invitation_policy IN ('leader_only','members')),
  ADD CONSTRAINT parties_max_members_chk CHECK (max_members BETWEEN 2 AND 8),
  ADD CONSTRAINT parties_grade_band_chk CHECK (grade_band IN ('g1_3','g4_5','g6_7','g8_9')),
  ADD CONSTRAINT parties_status_chk CHECK (status IN ('open','in_activity','disbanded')),
  ADD CONSTRAINT parties_disbanded_chk CHECK ((status = 'disbanded') = (disbanded_at IS NOT NULL)),
  ADD CONSTRAINT parties_state_version_chk CHECK (state_version >= 0),
  ADD CONSTRAINT parties_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.parties ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.parties FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.parties FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.parties FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.parties FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.parties FROM competition_app, competition_platform;

-- ---------- multiplayer.party_members (T): left_at/left_reason set once ----------
ALTER TABLE multiplayer.party_members
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT party_members_role_chk CHECK (role IN ('leader','member')),
  ADD CONSTRAINT party_members_left_reason_chk CHECK (left_reason IS NULL OR left_reason IN ('left','kicked','disbanded','timeout','policy')),
  ADD CONSTRAINT party_members_left_chk CHECK ((left_at IS NULL) = (left_reason IS NULL)),
  ADD CONSTRAINT party_members_party_fk FOREIGN KEY (party_id) REFERENCES multiplayer.parties (party_id),
  ADD CONSTRAINT party_members_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.party_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.party_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.party_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.party_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.party_members FROM competition_app, competition_platform;

CREATE FUNCTION multiplayer.party_members_leave_once() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.left_at IS NOT NULL AND (NEW.left_at IS DISTINCT FROM OLD.left_at OR NEW.left_reason IS DISTINCT FROM OLD.left_reason) THEN
    RAISE EXCEPTION 'PARTY_MEMBERSHIP_ALREADY_LEFT' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER party_members_leave_once BEFORE UPDATE ON multiplayer.party_members FOR EACH ROW
  EXECUTE FUNCTION multiplayer.party_members_leave_once();

-- ---------- multiplayer.party_invitations (T) ----------
ALTER TABLE multiplayer.party_invitations
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT party_invitations_status_chk CHECK (status IN ('pending','accepted','rejected','expired','cancelled')),
  ADD CONSTRAINT party_invitations_self_chk CHECK (inviter_id <> invitee_id),
  ADD CONSTRAINT party_invitations_responded_chk CHECK ((status = 'pending') = (responded_at IS NULL)),
  ADD CONSTRAINT party_invitations_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('ttl','party_disbanded',
    'joined_other_party','pending_overflow','cancelled','inviter_kicked','inviter_left','blocked','policy')),
  ADD CONSTRAINT party_invitations_party_fk FOREIGN KEY (party_id) REFERENCES multiplayer.parties (party_id),
  ADD CONSTRAINT party_invitations_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.party_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.party_invitations FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.party_invitations FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.party_invitations FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.party_invitations FROM competition_app, competition_platform;

-- ---------- multiplayer.party_policies (T, read only in F100) ----------
ALTER TABLE multiplayer.party_policies
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT party_policies_scope_chk CHECK (scope_type IN ('school','class')),
  ADD CONSTRAINT party_policies_rules_chk CHECK (jsonb_typeof(rules) = 'object' AND octet_length(rules::text) <= 4096),
  ADD CONSTRAINT party_policies_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
ALTER TABLE multiplayer.party_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.party_policies FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.party_policies FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON multiplayer.party_policies FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON multiplayer.party_policies FROM competition_app, competition_platform;

-- ---------- multiplayer.party_histories (TA, P monthly by occurred_at: DATABASE_DESIGN §2.16, F100-AC-22) ----------
-- Append-only history written in the transaction of every party change. data ≤ 4 KB, ids and enums only (no chat
-- content, no name, no PII). Monthly partitions come with the partition job of business tables; until then rows land
-- in the default partition (same as competition.integrity_signals).
CREATE TABLE multiplayer.party_histories (
  history_id  uuid NOT NULL,
  tenant_id   uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id   uuid NOT NULL,
  party_id    uuid NOT NULL REFERENCES multiplayer.parties (party_id),
  event_type  varchar(64) NOT NULL CHECK (event_type IN ('created','member_joined','member_left','member_kicked',
    'leader_transferred','settings_updated','disbanded')),
  actor_id    uuid,
  data        jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(data::text) <= 4096),
  occurred_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (history_id, occurred_at)
) PARTITION BY RANGE (occurred_at);
CREATE TABLE multiplayer.party_histories_default PARTITION OF multiplayer.party_histories DEFAULT;
CREATE INDEX party_histories_party_idx ON multiplayer.party_histories (tenant_id, party_id, occurred_at);
CREATE INDEX party_histories_actor_idx ON multiplayer.party_histories (tenant_id, actor_id, event_type, occurred_at);
ALTER TABLE multiplayer.party_histories ENABLE ROW LEVEL SECURITY;
ALTER TABLE multiplayer.party_histories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON multiplayer.party_histories FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON multiplayer.party_histories FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON multiplayer.party_histories, multiplayer.party_histories_default FROM competition_app, competition_platform;
REVOKE ALL ON multiplayer.party_histories_default FROM competition_app, competition_platform;
GRANT SELECT, INSERT ON multiplayer.party_histories TO competition_app, competition_platform;

-- ---------- _live views for reporting (competition_readonly) ----------
CREATE OR REPLACE VIEW multiplayer.parties_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.parties WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.party_members_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.party_members WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.party_invitations_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.party_invitations WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW multiplayer.party_policies_live WITH (security_barrier = true) AS
  SELECT * FROM multiplayer.party_policies WHERE deleted_at IS NULL;
GRANT SELECT ON multiplayer.parties_live, multiplayer.party_members_live, multiplayer.party_invitations_live,
  multiplayer.party_policies_live, multiplayer.party_histories TO competition_readonly;
