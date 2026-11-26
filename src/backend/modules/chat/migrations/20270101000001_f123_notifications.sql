-- F123 T008/T009 — in-game notifications of the students (specs/123-in-game-notifications: data-model §1; DATABASE_DESIGN
-- §2.11; SOCIAL_SYSTEM §8; CHAT_SYSTEM §10; ADR-0004, ADR-0028, ADR-0041, ADR-0070, ADR-0089). Applied by
-- `migrate --module=chat` as chat_migrator.
-- social.notifications is partitioned by month on created_at: hand-written like chat.messages (composite primary key of a
-- partitioned table is outside ent, precedent F115). Until the partition job of business tables exists rows land in the
-- default partition. The idempotency key of the consumers is (recipient_id, source_event_id, kind) — created_at is the
-- occurred_at of the source event, so a redelivered event hits the same unique row (F123-AC-03). No row is ever deleted
-- (R21, ADR-0070): read state lives here, Valkey only caches the unread count (R9).

CREATE TABLE social.notifications (
  notification_id uuid NOT NULL,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  recipient_id    uuid NOT NULL,
  school_id       uuid,
  kind            varchar(64) NOT NULL CHECK (kind IN ('assignment_new','assignment_due','party_invitation','friend_request',
                    'quest_available','reward_granted','tournament_update','moderation_notice','system',
                    'assignment_results_released','level_reached','achievement_unlocked')),
  message_code    varchar(128) NOT NULL,
  params          jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 4096),
  title           jsonb CHECK (title IS NULL OR octet_length(title::text) <= 2048),
  body            jsonb CHECK (body IS NULL OR octet_length(body::text) <= 8192),
  ref_type        varchar(64),
  ref_id          uuid,
  action          jsonb CHECK (action IS NULL OR (jsonb_typeof(action) = 'object' AND octet_length(action::text) <= 1024)),
  source_event_id uuid NOT NULL,
  created_at      timestamptz NOT NULL,
  read_at         timestamptz,
  deleted_at      timestamptz,
  deleted_by      uuid,
  delete_reason   varchar(64),
  PRIMARY KEY (notification_id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE social.notifications_default PARTITION OF social.notifications DEFAULT;
-- Idempotency of the notifier consumers (with infra.processed_events): one notification per recipient, event and kind.
CREATE UNIQUE INDEX notifications_source_uq ON social.notifications (recipient_id, source_event_id, kind, created_at);
-- Inbox of one user, newest first (listNotifications).
CREATE INDEX notifications_inbox_idx ON social.notifications (tenant_id, recipient_id, created_at DESC, notification_id DESC)
  WHERE deleted_at IS NULL;
-- Unread count of one user.
CREATE INDEX notifications_unread_idx ON social.notifications (recipient_id) WHERE read_at IS NULL AND deleted_at IS NULL;

ALTER TABLE social.notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.notifications FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.notifications FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.notifications FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.notifications FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21, F123-AC-12): no DELETE/TRUNCATE for application roles; the partition via its parent only ----------
REVOKE DELETE, TRUNCATE ON social.notifications FROM chat_app, chat_platform;
REVOKE ALL ON social.notifications_default FROM chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON social.notifications TO chat_app, chat_platform;

-- ---------- _live view for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW social.notifications_live WITH (security_barrier = true) AS
  SELECT * FROM social.notifications WHERE deleted_at IS NULL;
GRANT SELECT ON social.notifications_live TO chat_readonly;
