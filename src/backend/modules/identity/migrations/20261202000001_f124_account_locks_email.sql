-- F124 T008 — account locks by source and staff email deliveries (specs/124-safety-enforcement-oncall: data-model §1;
-- DATABASE_DESIGN §2.2; ACCOUNT_PROVISIONING §6; MODERATION §6.3–§7; ADR-0028, ADR-0041, ADR-0089). Applied by
-- `migrate --module=identity` as identity_migrator. Expand only (R12): two new tables, a backfill of one lock per account
-- that is locked today and a wider CHECK of identity.user_status_history.source (+ 'moderation'); nothing dropped or
-- deleted. identity.users.status = 'locked' ⇔ at least one row of identity.account_locks with released_at IS NULL (except
-- deactivated/anonymized accounts): a lock of the moderation source (sanction account_lock of the Safety Team) is released
-- only by sanction.lifted, a lock of the school, the platform or the SIS only by that source (F124-AC-02, AC-05).
-- identity.email_deliveries keeps the recipient id only, never the address (R14, NFR-PRIV-002).

-- BEGIN entddl
CREATE TABLE identity.account_locks (
  lock_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_id uuid NOT NULL,
  source varchar(32) NOT NULL,
  source_ref_id uuid,
  case_id uuid,
  reason varchar(1000),
  released_at timestamptz,
  released_by uuid,
  release_reason varchar(1000),
  source_event_id uuid,
  PRIMARY KEY (lock_id)
);
CREATE INDEX account_locks_active_idx ON identity.account_locks (user_id) WHERE released_at IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX account_locks_admin_source_uq ON identity.account_locks (user_id, source) WHERE source_ref_id IS NULL AND released_at IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX account_locks_source_ref_uq ON identity.account_locks (source, source_ref_id) WHERE source_ref_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE identity.email_deliveries (
  delivery_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  source_event_id uuid NOT NULL,
  template_code varchar(64) NOT NULL,
  recipient_user_id uuid NOT NULL,
  params jsonb NOT NULL,
  locale varchar(8) NOT NULL DEFAULT 'vi',
  status varchar(16) NOT NULL DEFAULT 'pending',
  attempts smallint NOT NULL DEFAULT 0,
  next_attempt_at timestamptz,
  sent_at timestamptz,
  last_error_code varchar(64),
  PRIMARY KEY (delivery_id)
);
CREATE INDEX email_deliveries_due_idx ON identity.email_deliveries (status, next_attempt_at) WHERE status = 'pending';
CREATE UNIQUE INDEX email_deliveries_once_uq ON identity.email_deliveries (source_event_id, recipient_user_id, template_code);
-- END entddl

-- ---------- CHECKs, FKs, defaults (data-model §1) ----------
ALTER TABLE identity.account_locks
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT account_locks_source_chk CHECK (source IN ('school_admin','platform_admin','sis','moderation_sanction')),
  ADD CONSTRAINT account_locks_ref_chk CHECK (source <> 'moderation_sanction' OR source_ref_id IS NOT NULL),
  ADD CONSTRAINT account_locks_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id),
  ADD CONSTRAINT account_locks_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);

ALTER TABLE identity.email_deliveries
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT email_deliveries_template_chk CHECK (template_code IN ('school_safety_notification','school_safety_reminder',
    'moderation_oncall_page','break_glass_request')),
  ADD CONSTRAINT email_deliveries_status_chk CHECK (status IN ('pending','sent','failed','skipped')),
  ADD CONSTRAINT email_deliveries_attempts_chk CHECK (attempts >= 0),
  ADD CONSTRAINT email_deliveries_params_chk CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 2048),
  ADD CONSTRAINT email_deliveries_locale_chk CHECK (locale IN ('vi','en')),
  ADD CONSTRAINT email_deliveries_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);

-- A lock of the Safety Team is recorded in the status history with its own source.
ALTER TABLE identity.user_status_history DROP CONSTRAINT IF EXISTS user_status_history_source_check;
ALTER TABLE identity.user_status_history ADD CONSTRAINT user_status_history_source_check
  CHECK (source IN ('staff', 'sis', 'cleanup', 'restore', 'anonymize', 'retention', 'moderation'));

-- ---------- Backfill (expand): one lock per account locked today (before RLS on the new table) ----------
-- Source from the last history entry to locked: sis → sis; otherwise a student → school_admin, staff → platform_admin.
-- identity_migrator has no BYPASSRLS and user_status_history forces RLS: a policy for the migrator reads it during the
-- backfill only and is dropped right after (same transaction).
CREATE POLICY migrator_backfill_f124 ON identity.user_status_history FOR SELECT TO identity_migrator USING (true);
INSERT INTO identity.account_locks (lock_id, tenant_id, created_at, created_by, updated_at, user_id, source, reason)
SELECT gen_random_uuid(), u.primary_tenant_id, coalesce(h.changed_at, now()), h.changed_by, now(), u.user_id,
  CASE WHEN h.source = 'sis' THEN 'sis' WHEN u.user_type = 'student' THEN 'school_admin' ELSE 'platform_admin' END,
  'backfill F124: lock recorded before identity.account_locks'
FROM identity.users u
LEFT JOIN LATERAL (SELECT hh.source, hh.changed_at, hh.changed_by FROM identity.user_status_history hh
  WHERE hh.user_id = u.user_id AND hh.to_status = 'locked' ORDER BY hh.changed_at DESC LIMIT 1) h ON true
WHERE u.status = 'locked' AND u.deleted_at IS NULL;
DROP POLICY migrator_backfill_f124 ON identity.user_status_history;

-- ---------- RLS (FORCE): tenant condition only (ADR-0089 §1); platform rows (tenant_id NULL) through identity_platform ----------
ALTER TABLE identity.account_locks ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.account_locks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.account_locks FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.account_locks FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.account_locks FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE identity.email_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.email_deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.email_deliveries FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.email_deliveries FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.email_deliveries FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21): no DELETE/TRUNCATE ----------
REVOKE DELETE, TRUNCATE ON identity.account_locks, identity.email_deliveries FROM identity_app, identity_platform;

-- ---------- _live views (identity_readonly): no lock reason, no email params ----------
CREATE OR REPLACE VIEW identity.account_locks_live WITH (security_barrier = true) AS
  SELECT lock_id, tenant_id, user_id, source, source_ref_id, created_at, released_at FROM identity.account_locks WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW identity.email_deliveries_live WITH (security_barrier = true) AS
  SELECT delivery_id, tenant_id, template_code, recipient_user_id, status, attempts, sent_at, created_at
  FROM identity.email_deliveries WHERE deleted_at IS NULL;
GRANT SELECT ON identity.account_locks_live, identity.email_deliveries_live TO identity_readonly;
