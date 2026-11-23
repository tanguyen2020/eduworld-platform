-- F120 T009 — persistent channels (world, area, club team, direct), clubs, the chat policy levels platform → tenant → school
-- and the retention of persistent channels (specs/120-persistent-channels-clubs: data-model §1; DATABASE_DESIGN §2.11,
-- §2.12; CHAT_SYSTEM §2–§6; SOCIAL_SYSTEM §6; ADR-0020, ADR-0028, ADR-0033, ADR-0041, ADR-0050, ADR-0054, ADR-0070, ADR-0078,
-- ADR-0089, ADR-0095). Applied by `migrate --module=chat` as chat_migrator. Order: columns added to the ent tables (entddl
-- block, golden test modules/chat/ent/ddl_test.go), then hand-written: CHECKs, the club tables of schema social, the retention
-- runs, the common-tenant overrides, RLS, soft-delete only grants, _live views. No table here holds gameplay chat: nothing
-- is granted to chat_purger (persistent channels are never purged — R21, ADR-0033; their content is anonymised). Nothing is
-- deleted by this migration (R12).

-- BEGIN entddl

ALTER TABLE chat.channels ADD COLUMN capabilities jsonb;
ALTER TABLE chat.channel_members ADD COLUMN source_event_id uuid;
ALTER TABLE chat.chat_policies ADD COLUMN reason varchar(500);
-- END entddl

-- ---------- chat.channels: status of persistent channels, no voice, one persistent channel per scope ----------
-- read_only: club locked/archived; frozen: direct channel of a blocked pair (CHAT_SYSTEM §4, F120-AC-15, AC-20). closed stays
-- the state of a finished gameplay channel (F117).
ALTER TABLE chat.channels DROP CONSTRAINT channels_status_chk;
ALTER TABLE chat.channels ADD CONSTRAINT channels_status_chk CHECK (status IN ('open','closed','read_only','frozen'));
ALTER TABLE chat.channels ALTER COLUMN capabilities SET DEFAULT '{"voice": false}'::jsonb;
ALTER TABLE chat.channels ADD CONSTRAINT channels_capabilities_chk CHECK (capabilities IS NULL
  OR (jsonb_typeof(capabilities) = 'object' AND octet_length(capabilities::text) <= 256
      AND (capabilities->>'voice') IS DISTINCT FROM 'true'));
CREATE UNIQUE INDEX channels_persistent_scope_uq ON chat.channels (tenant_id, channel_type, scope_ref)
  WHERE deleted_at IS NULL AND NOT is_gameplay AND scope_ref IS NOT NULL;

-- ---------- chat.messages: anonymisation of persistent messages (F120-AC-24) ----------
ALTER TABLE chat.messages ADD COLUMN anonymized_at timestamptz;
CREATE INDEX messages_retention_idx ON chat.messages (tenant_id, created_at) WHERE anonymized_at IS NULL;

-- ---------- chat.message_revisions: anonymisation of the earlier versions of a persistent message ----------
-- The table stays append-only except for the one update the retention job makes: body/body_display set to NULL with every
-- other column unchanged (chat.f_revisions_guard replaces chat.f_append_only on this table).
CREATE OR REPLACE FUNCTION chat.f_revisions_guard() RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
  IF TG_OP = 'UPDATE' AND NEW.body IS NULL AND NEW.body_display IS NULL
     AND (NEW.tenant_id, NEW.channel_id, NEW.message_id, NEW.revision, NEW.state, NEW.mentions, NEW.decision, NEW.edited_at,
          NEW.created_at) IS NOT DISTINCT FROM
         (OLD.tenant_id, OLD.channel_id, OLD.message_id, OLD.revision, OLD.state, OLD.mentions, OLD.decision, OLD.edited_at,
          OLD.created_at)
     AND NEW.filter_verdict IS NOT DISTINCT FROM OLD.filter_verdict THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'chat.%: append-only table (only anonymisation of body/body_display)', TG_TABLE_NAME
    USING ERRCODE = 'insufficient_privilege';
END $fn$;
DROP TRIGGER message_revisions_append_only ON chat.message_revisions;
CREATE TRIGGER message_revisions_append_only BEFORE UPDATE ON chat.message_revisions
  FOR EACH ROW EXECUTE FUNCTION chat.f_revisions_guard();
GRANT UPDATE (body, body_display) ON chat.message_revisions TO chat_app;
CREATE POLICY tenant_update ON chat.message_revisions FOR UPDATE TO chat_app USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- chat.chat_policies: one live document per level and tenant ----------
DROP INDEX IF EXISTS chat.chat_policies_scope_uq;
CREATE UNIQUE INDEX chat_policies_scope_uq ON chat.chat_policies
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), scope_type,
   coalesce(scope_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE deleted_at IS NULL;
-- The platform document (tenant NULL) is written by chat_platform under PlatformScope (putPlatformChatPolicy): RLS lets that
-- role update the platform row only.
CREATE POLICY platform_update ON chat.chat_policies FOR UPDATE TO chat_platform USING (tenant_id IS NULL) WITH CHECK (tenant_id IS NULL);
GRANT SELECT, UPDATE ON chat.chat_policies TO chat_platform;

-- ---------- chat.persistent_retention_runs (G, append-only): one row per tenant and run, no content ----------
CREATE TABLE chat.persistent_retention_runs (
  run_id              uuid PRIMARY KEY,
  tenant_id           uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  started_at          timestamptz NOT NULL,
  finished_at         timestamptz,
  channels_scanned    int NOT NULL DEFAULT 0 CHECK (channels_scanned >= 0),
  messages_anonymized int NOT NULL DEFAULT 0 CHECK (messages_anonymized >= 0),
  evidence_copied     int NOT NULL DEFAULT 0 CHECK (evidence_copied >= 0),
  status              varchar(16) NOT NULL CHECK (status IN ('running','succeeded','failed')),
  error_code          varchar(64),
  created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX persistent_retention_runs_tenant_idx ON chat.persistent_retention_runs (tenant_id, started_at DESC);
ALTER TABLE chat.persistent_retention_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.persistent_retention_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.persistent_retention_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.persistent_retention_runs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON chat.persistent_retention_runs FROM chat_app, chat_platform;
GRANT SELECT, INSERT ON chat.persistent_retention_runs TO chat_app;

-- ---------- chat.common_tenant_social_overrides (T): System Admin re-enables direct/world/free Area text of a common tenant ----------
-- Absent row = the defaults of common_tenant_social_defaults (ADR-0078, CHILD_SAFETY §3.6): direct and world off, Area quick
-- + emoji only. Written only by putCommonTenantSocialOverride (platform.settings.write), always with a target tenant.
CREATE TABLE chat.common_tenant_social_overrides (
  tenant_id              uuid PRIMARY KEY REFERENCES ref.tenants (tenant_id),
  direct_enabled         boolean NOT NULL DEFAULT false,
  world_enabled          boolean NOT NULL DEFAULT false,
  area_free_text_enabled boolean NOT NULL DEFAULT false,
  reason                 varchar(500) NOT NULL,
  version                bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at             timestamptz NOT NULL DEFAULT now(),
  created_by             uuid,
  updated_at             timestamptz NOT NULL DEFAULT now(),
  updated_by             uuid,
  deleted_at             timestamptz,
  deleted_by             uuid
);
ALTER TABLE chat.common_tenant_social_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.common_tenant_social_overrides FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.common_tenant_social_overrides FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.common_tenant_social_overrides FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.common_tenant_social_overrides FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON chat.common_tenant_social_overrides FROM chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON chat.common_tenant_social_overrides TO chat_app;

-- ---------- social.* clubs (DATABASE_DESIGN §2.11, SOCIAL_SYSTEM §6, ADR-0050): hand-written, explicit SQL ----------
-- club_type without study_group (ADR-0084/ADR-0095); no "whole class" option. chat_channel_id = the fixed team channel of
-- the club (scope_ref {club_id}, is_gameplay false, never purged).
CREATE TABLE social.clubs (
  club_id         uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id       uuid,
  club_type       varchar(32) NOT NULL CHECK (club_type IN ('school_club','subject_club','competition_team','interest_group')),
  name            varchar(64) NOT NULL CHECK (char_length(name) >= 2),
  description     varchar(500),
  icon_id         varchar(64),
  subject_id      uuid,
  owner_id        uuid NOT NULL,
  created_by_kind varchar(16) NOT NULL CHECK (created_by_kind IN ('student','staff')),
  status          varchar(32) NOT NULL CHECK (status IN ('pending_approval','active','locked','archived')),
  max_members     int NOT NULL CHECK (max_members BETWEEN 2 AND 200),
  member_count    int NOT NULL DEFAULT 0 CHECK (member_count >= 0),
  chat_channel_id uuid,
  approved_by     uuid,
  approved_at     timestamptz,
  archived_at     timestamptz,
  version         bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      uuid,
  updated_at      timestamptz NOT NULL DEFAULT now(),
  updated_by      uuid,
  deleted_at      timestamptz,
  deleted_by      uuid
);
CREATE INDEX clubs_tenant_school_idx ON social.clubs (tenant_id, school_id, status) WHERE deleted_at IS NULL;

CREATE TABLE social.club_members (
  club_member_id  uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  club_id         uuid NOT NULL REFERENCES social.clubs (club_id),
  user_id         uuid NOT NULL,
  school_id       uuid,
  role            varchar(16) NOT NULL CHECK (role IN ('owner','officer','member')),
  joined_at       timestamptz NOT NULL,
  left_at         timestamptz,
  left_reason     varchar(16) CHECK (left_reason IN ('left','removed','archived')),
  source_event_id uuid,
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      uuid,
  updated_at      timestamptz NOT NULL DEFAULT now(),
  updated_by      uuid,
  deleted_at      timestamptz,
  deleted_by      uuid
);
CREATE UNIQUE INDEX club_members_active_uq ON social.club_members (club_id, user_id) WHERE left_at IS NULL AND deleted_at IS NULL;
CREATE INDEX club_members_user_idx ON social.club_members (tenant_id, user_id) WHERE left_at IS NULL AND deleted_at IS NULL;

CREATE TABLE social.club_join_requests (
  request_id   uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  club_id      uuid NOT NULL REFERENCES social.clubs (club_id),
  requester_id uuid NOT NULL,
  status       varchar(16) NOT NULL CHECK (status IN ('pending','accepted','rejected','expired','cancelled')),
  decided_by   uuid,
  decided_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  created_by   uuid,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  updated_by   uuid,
  deleted_at   timestamptz,
  deleted_by   uuid
);
CREATE UNIQUE INDEX club_join_requests_pending_uq ON social.club_join_requests (club_id, requester_id)
  WHERE status = 'pending' AND deleted_at IS NULL;

CREATE TABLE social.club_invitations (
  invitation_id uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  club_id       uuid NOT NULL REFERENCES social.clubs (club_id),
  inviter_id    uuid NOT NULL,
  invitee_id    uuid NOT NULL,
  status        varchar(16) NOT NULL CHECK (status IN ('pending','accepted','declined','expired','cancelled')),
  expires_at    timestamptz NOT NULL,
  decided_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  created_by    uuid,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  updated_by    uuid,
  deleted_at    timestamptz,
  deleted_by    uuid
);
CREATE UNIQUE INDEX club_invitations_pending_uq ON social.club_invitations (club_id, invitee_id)
  WHERE status = 'pending' AND deleted_at IS NULL;

CREATE TABLE social.club_goals (
  goal_id      uuid PRIMARY KEY,
  tenant_id    uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  club_id      uuid NOT NULL REFERENCES social.clubs (club_id),
  title        varchar(128),
  metric       varchar(64) NOT NULL CHECK (metric IN ('activities_completed','quests_completed','minutes_learning','boss_defeated',
                 'discoveries_found')),
  subject_id   uuid,
  target       int NOT NULL CHECK (target BETWEEN 1 AND 100000),
  progress     int NOT NULL DEFAULT 0 CHECK (progress >= 0),
  week_start   date NOT NULL,
  status       varchar(16) NOT NULL CHECK (status IN ('active','completed','expired')),
  completed_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  created_by   uuid,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  updated_by   uuid,
  deleted_at   timestamptz,
  deleted_by   uuid
);
CREATE INDEX club_goals_club_idx ON social.club_goals (tenant_id, club_id, status) WHERE deleted_at IS NULL;

-- Contribution of one member to one goal; last_event_id + infra.processed_events make a redelivered event count once
-- (F120-AC-21). Never a score or mastery value.
CREATE TABLE social.club_goal_progress (
  tenant_id     uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  goal_id       uuid NOT NULL REFERENCES social.club_goals (goal_id),
  user_id       uuid NOT NULL,
  contribution  int NOT NULL DEFAULT 0 CHECK (contribution >= 0),
  last_event_id uuid,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (goal_id, user_id)
);

CREATE TABLE social.club_events (
  event_id               uuid PRIMARY KEY,
  tenant_id              uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  club_id                uuid NOT NULL REFERENCES social.clubs (club_id),
  title                  varchar(128) NOT NULL,
  description            varchar(1000),
  event_kind             varchar(32) NOT NULL DEFAULT 'other'
                           CHECK (event_kind IN ('practice_session','coop_quest','boss_raid','internal_tournament','other')),
  tournament_template_id uuid,
  starts_at              timestamptz NOT NULL,
  ends_at                timestamptz CHECK (ends_at IS NULL OR ends_at > starts_at),
  created_at             timestamptz NOT NULL DEFAULT now(),
  created_by             uuid,
  updated_at             timestamptz NOT NULL DEFAULT now(),
  updated_by             uuid,
  deleted_at             timestamptz,
  deleted_by             uuid
);
CREATE INDEX club_events_club_idx ON social.club_events (tenant_id, club_id, starts_at) WHERE deleted_at IS NULL;

-- ---------- RLS (FORCE) on every club table: tenant condition only (ADR-0089 §1); soft-delete only grants (R21) ----------
ALTER TABLE social.clubs ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.clubs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.clubs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.clubs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.clubs FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_join_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_join_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_join_requests FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_join_requests FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_join_requests FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_invitations FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_invitations FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_invitations FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_goals ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_goals FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_goals FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_goals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_goals FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_goal_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_goal_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_goal_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_goal_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_goal_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE social.club_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.club_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.club_events FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.club_events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.club_events FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON social.clubs, social.club_members, social.club_join_requests, social.club_invitations, social.club_goals,
  social.club_goal_progress, social.club_events FROM chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON social.clubs, social.club_members, social.club_join_requests, social.club_invitations, social.club_goals,
  social.club_goal_progress, social.club_events TO chat_app;

-- ---------- _live views for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW social.clubs_live WITH (security_barrier = true) AS SELECT * FROM social.clubs WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.club_members_live WITH (security_barrier = true) AS
  SELECT * FROM social.club_members WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.club_goals_live WITH (security_barrier = true) AS SELECT * FROM social.club_goals WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW social.club_events_live WITH (security_barrier = true) AS SELECT * FROM social.club_events WHERE deleted_at IS NULL;
GRANT SELECT ON social.clubs_live, social.club_members_live, social.club_goals_live, social.club_events_live TO chat_readonly;
