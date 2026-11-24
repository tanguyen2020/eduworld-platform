-- F124 T009 — on-call rotation of p1 cases and break-glass grants of the Platform Safety Team (specs/124-safety-enforcement-
-- oncall: data-model §1; DATABASE_DESIGN §2.16; MODERATION §6.1, §6.3; CHILD_SAFETY §7, §11; ADR-0028, ADR-0041, ADR-0072,
-- ADR-0089). Applied by `migrate --module=chat` as chat_migrator. Expand only (R12): two new tables, two nullable columns on
-- chat.moderation_cases and a wider CHECK of chat.moderation_actions.action (+ 'oncall_page'); nothing dropped or deleted.
-- chat.moderation_oncall_shifts is platform data (G: no tenant_id, no student data). chat.break_glass_grants is tenant data
-- (T, RLS); the System Admin reads it across tenants and reads the messages of an approved grant only through
-- tenancy.Bypass(ctx, "break_glass") (chat_platform, audited and counted — ADR-0089 §3). No row is ever deleted (R21).

-- BEGIN entddl
CREATE TABLE chat.moderation_oncall_shifts (
  shift_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  moderator_user_id uuid NOT NULL,
  oncall_role varchar(16) NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  note varchar(500),
  PRIMARY KEY (shift_id)
);
CREATE INDEX moderation_oncall_shifts_role_idx ON chat.moderation_oncall_shifts (oncall_role, starts_at, ends_at) WHERE deleted_at IS NULL;

CREATE TABLE chat.break_glass_grants (
  grant_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope_type varchar(16) NOT NULL,
  scope_id uuid NOT NULL,
  window_from timestamptz NOT NULL,
  window_to timestamptz NOT NULL,
  duration_minutes bigint NOT NULL,
  reason varchar(1000) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  requested_by uuid NOT NULL,
  requested_at timestamptz NOT NULL,
  decided_by uuid,
  decided_at timestamptz,
  decision_note varchar(1000),
  expires_at timestamptz,
  revoked_by uuid,
  revoked_at timestamptz,
  revoke_reason varchar(1000),
  messages_viewed_count bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (grant_id)
);
CREATE INDEX break_glass_grants_active_idx ON chat.break_glass_grants (status, expires_at) WHERE status = 'approved' AND deleted_at IS NULL;
CREATE INDEX break_glass_grants_list_idx ON chat.break_glass_grants (requested_at);
CREATE INDEX break_glass_grants_tenant_idx ON chat.break_glass_grants (tenant_id, requested_at);
ALTER TABLE chat.moderation_cases ADD COLUMN oncall_page_level varchar(16);
ALTER TABLE chat.moderation_cases ADD COLUMN last_paged_at timestamptz;
CREATE INDEX moderation_cases_p1_open_idx ON chat.moderation_cases (severity, status, created_at) WHERE status = 'open' AND severity = 'p1' AND deleted_at IS NULL;
-- END entddl

-- ---------- CHECKs, defaults (data-model §1) ----------
ALTER TABLE chat.moderation_oncall_shifts
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_oncall_shifts_role_chk CHECK (oncall_role IN ('primary','secondary')),
  ADD CONSTRAINT moderation_oncall_shifts_window_chk CHECK (ends_at > starts_at);

ALTER TABLE chat.break_glass_grants
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT break_glass_grants_scope_chk CHECK (scope_type IN ('channel','user')),
  ADD CONSTRAINT break_glass_grants_status_chk CHECK (status IN ('pending','approved','rejected','expired','revoked')),
  ADD CONSTRAINT break_glass_grants_window_chk CHECK (window_to > window_from),
  ADD CONSTRAINT break_glass_grants_duration_chk CHECK (duration_minutes >= 1),
  ADD CONSTRAINT break_glass_grants_reason_chk CHECK (char_length(reason) >= 20),
  ADD CONSTRAINT break_glass_grants_two_person_chk CHECK (decided_by IS NULL OR decided_by <> requested_by),
  ADD CONSTRAINT break_glass_grants_approved_chk CHECK (status <> 'approved' OR expires_at IS NOT NULL),
  ADD CONSTRAINT break_glass_grants_viewed_chk CHECK (messages_viewed_count >= 0),
  ADD CONSTRAINT break_glass_grants_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.moderation_cases
  ADD CONSTRAINT moderation_cases_oncall_level_chk
    CHECK (oncall_page_level IS NULL OR oncall_page_level IN ('primary','secondary','lead','system_admin'));

-- The on-call ladder writes one history row per level paged (F124-AC-14).
ALTER TABLE chat.moderation_actions DROP CONSTRAINT moderation_actions_action_chk;
ALTER TABLE chat.moderation_actions
  ADD CONSTRAINT moderation_actions_action_chk CHECK (action IN ('report','signal','assign','warn','mute','restrict',
    'remove_message','escalate','dismiss','close','reopen','system_escalate','hold','release_hold','notify_school',
    'school_acknowledged','school_reminder','school_reminder_lead','oncall_page'));

-- ---------- RLS (FORCE): tenant condition only on the tenant table (ADR-0089 §1) ----------
ALTER TABLE chat.break_glass_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.break_glass_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.break_glass_grants FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.break_glass_grants FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.break_glass_grants FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21): no DELETE/TRUNCATE ----------
REVOKE DELETE, TRUNCATE ON chat.moderation_oncall_shifts, chat.break_glass_grants FROM chat_app, chat_platform;

-- ---------- _live views for reporting (chat_readonly): grant metadata without the reason ----------
CREATE OR REPLACE VIEW chat.moderation_oncall_shifts_live WITH (security_barrier = true) AS
  SELECT * FROM chat.moderation_oncall_shifts WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.break_glass_grants_live WITH (security_barrier = true) AS
  SELECT grant_id, tenant_id, scope_type, window_from, window_to, duration_minutes, status, requested_by, requested_at,
    decided_by, decided_at, expires_at, revoked_by, revoked_at, messages_viewed_count, created_at
  FROM chat.break_glass_grants WHERE deleted_at IS NULL;
GRANT SELECT ON chat.moderation_oncall_shifts_live, chat.break_glass_grants_live TO chat_readonly;
