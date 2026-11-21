-- F121 T008 — reports, moderation cases, evidence and school safety notifications of the Platform Safety Team, and the
-- sanction lifecycle columns (specs/121-moderation-safety-team: data-model §1; DATABASE_DESIGN §2.12, §2.16; MODERATION
-- §5–§7, §10; ADR-0028, ADR-0033, ADR-0041, ADR-0072, ADR-0089). Applied by `migrate --module=chat` as chat_migrator.
-- Order: ent tables and the columns added to chat.sanctions (entddl block, golden test modules/chat/ent/ddl_test.go),
-- then what ent cannot express: CHECKs, FKs, defaults, append-only triggers (actions, evidence), RLS, soft-delete only
-- grants, _live views. Cross-tenant reads of these tables happen only in the moderation use cases through
-- tenancy.Bypass(ctx, "moderation_case") (chat_platform, audited and counted, ADR-0072); no row is ever deleted (R21).
-- chat.moderation_evidence (DATABASE_DESIGN §2.16) is created here because F121 writes the report snapshots; the
-- gameplay purge of F117 adds its own copies before purging (grant to chat_purger with F117).

-- BEGIN entddl
CREATE TABLE chat.reports (
  report_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid,
  reporter_id uuid NOT NULL,
  target_type varchar(16) NOT NULL,
  target_id uuid NOT NULL,
  target_user_id uuid,
  reason varchar(32) NOT NULL,
  note varchar(500),
  context jsonb,
  case_id uuid NOT NULL,
  weight numeric(3,2) NOT NULL DEFAULT 1,
  status varchar(16) NOT NULL DEFAULT 'open',
  evidence_id uuid,
  PRIMARY KEY (report_id)
);
CREATE INDEX reports_case_idx ON chat.reports (case_id);
CREATE UNIQUE INDEX reports_reporter_case_uq ON chat.reports (reporter_id, target_type, target_id, case_id) WHERE deleted_at IS NULL;
CREATE INDEX reports_target_idx ON chat.reports (tenant_id, target_type, target_id, created_at);

CREATE TABLE chat.moderation_cases (
  case_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_ids jsonb NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'open',
  severity varchar(4) NOT NULL,
  source varchar(32) NOT NULL,
  category_code varchar(64),
  channel_id uuid,
  channel_type varchar(16),
  target_type varchar(16) NOT NULL,
  target_id uuid NOT NULL,
  subject_user_ids jsonb NOT NULL,
  assignee_moderator_id uuid,
  assigned_via varchar(16),
  report_count bigint NOT NULL DEFAULT 0,
  first_response_due_at timestamptz NOT NULL,
  sla_due_at timestamptz NOT NULL,
  first_response_at timestamptz,
  escalated_at timestamptz,
  hold_sanction_id uuid,
  school_notified_at timestamptz,
  school_ack_at timestamptz,
  acknowledged_by uuid,
  contact_outcome varchar(32),
  contact_path varchar(32) NOT NULL DEFAULT 'school',
  resolved_at timestamptz,
  source_event_id uuid,
  PRIMARY KEY (case_id)
);
CREATE INDEX moderation_cases_queue_idx ON chat.moderation_cases (status, severity, sla_due_at);
CREATE UNIQUE INDEX moderation_cases_source_event_uq ON chat.moderation_cases (source_event_id) WHERE source_event_id IS NOT NULL;
CREATE INDEX moderation_cases_target_open_idx ON chat.moderation_cases (tenant_id, target_type, target_id) WHERE status IN ('open','in_review') AND deleted_at IS NULL;

CREATE TABLE chat.moderation_actions (
  action_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  case_id uuid NOT NULL,
  action varchar(32) NOT NULL,
  actor_id uuid,
  actor_kind varchar(16) NOT NULL,
  note varchar(1000),
  sanction_id uuid,
  message_id uuid,
  PRIMARY KEY (action_id)
);
CREATE INDEX moderation_actions_case_idx ON chat.moderation_actions (case_id, created_at);

CREATE TABLE chat.moderation_evidence (
  evidence_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  case_id uuid NOT NULL,
  report_id uuid,
  school_id uuid,
  channel_id uuid,
  snapshot jsonb NOT NULL,
  sha256 varchar(64) NOT NULL,
  PRIMARY KEY (evidence_id)
);
CREATE INDEX moderation_evidence_case_idx ON chat.moderation_evidence (case_id, created_at);

CREATE TABLE chat.school_safety_notifications (
  notification_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  case_id uuid NOT NULL,
  school_id uuid NOT NULL,
  severity varchar(4) NOT NULL,
  category_code varchar(64),
  students jsonb NOT NULL,
  measures_taken varchar(500),
  summary varchar(1000) NOT NULL,
  school_notified_at timestamptz NOT NULL,
  ack_due_at timestamptz NOT NULL,
  reminder_count smallint NOT NULL DEFAULT 0,
  last_reminded_at timestamptz,
  school_ack_at timestamptz,
  acknowledged_by uuid,
  contact_outcome varchar(32),
  ack_note varchar(500),
  PRIMARY KEY (notification_id)
);
CREATE INDEX school_safety_notifications_case_idx ON chat.school_safety_notifications (case_id);
CREATE INDEX school_safety_notifications_due_idx ON chat.school_safety_notifications (ack_due_at) WHERE school_ack_at IS NULL AND deleted_at IS NULL;
CREATE INDEX school_safety_notifications_school_idx ON chat.school_safety_notifications (tenant_id, school_id, school_ack_at);
ALTER TABLE chat.sanctions ADD COLUMN reason_code varchar(64);
ALTER TABLE chat.sanctions ADD COLUMN issued_by_kind varchar(16) NOT NULL DEFAULT 'system';
ALTER TABLE chat.sanctions ADD COLUMN revoked_by uuid;
ALTER TABLE chat.sanctions ADD COLUMN revoke_reason varchar(1000);
ALTER TABLE chat.sanctions ADD COLUMN lifted_at timestamptz;
CREATE INDEX sanctions_expiry_idx ON chat.sanctions (ends_at) WHERE lifted_at IS NULL AND ends_at IS NOT NULL AND deleted_at IS NULL;
-- END entddl

-- ---------- CHECKs, FKs, defaults (data-model §1, MODERATION §5–§7) ----------
ALTER TABLE chat.reports
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT reports_target_type_chk CHECK (target_type IN ('message','user')),
  ADD CONSTRAINT reports_reason_chk CHECK (reason IN ('rude_language','bullying','asked_personal_info','spam',
    'inappropriate_content','cheating','other')),
  ADD CONSTRAINT reports_status_chk CHECK (status IN ('open','linked','dismissed')),
  ADD CONSTRAINT reports_weight_chk CHECK (weight >= 0 AND weight <= 1),
  ADD CONSTRAINT reports_context_chk CHECK (context IS NULL OR (jsonb_typeof(context) = 'object'
    AND octet_length(context::text) <= 1024)),
  ADD CONSTRAINT reports_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.moderation_cases
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_cases_status_chk CHECK (status IN ('open','in_review','escalated','resolved','reopened')),
  ADD CONSTRAINT moderation_cases_severity_chk CHECK (severity IN ('p1','p2','p3')),
  ADD CONSTRAINT moderation_cases_source_chk CHECK (source IN ('report','filter_alert','async_analysis','self_harm_signal')),
  ADD CONSTRAINT moderation_cases_target_type_chk CHECK (target_type IN ('message','user')),
  ADD CONSTRAINT moderation_cases_assigned_via_chk CHECK (assigned_via IS NULL OR assigned_via IN ('manual','rotation')),
  ADD CONSTRAINT moderation_cases_contact_path_chk CHECK (contact_path IN ('school','platform_direct')),
  ADD CONSTRAINT moderation_cases_contact_outcome_chk
    CHECK (contact_outcome IS NULL OR contact_outcome IN ('parent_contacted','unable_to_contact','not_needed')),
  ADD CONSTRAINT moderation_cases_school_ids_chk CHECK (jsonb_typeof(school_ids) = 'array' AND jsonb_array_length(school_ids) <= 20),
  ADD CONSTRAINT moderation_cases_subjects_chk
    CHECK (jsonb_typeof(subject_user_ids) = 'array' AND jsonb_array_length(subject_user_ids) <= 20),
  ADD CONSTRAINT moderation_cases_report_count_chk CHECK (report_count >= 0),
  ADD CONSTRAINT moderation_cases_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.moderation_actions
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_actions_action_chk CHECK (action IN ('report','signal','assign','warn','mute','restrict',
    'remove_message','escalate','dismiss','close','reopen','system_escalate','hold','release_hold','notify_school',
    'school_acknowledged','school_reminder','school_reminder_lead')),
  ADD CONSTRAINT moderation_actions_actor_chk CHECK (actor_kind IN ('moderator','system','school')),
  ADD CONSTRAINT moderation_actions_case_fk FOREIGN KEY (case_id) REFERENCES chat.moderation_cases (case_id),
  ADD CONSTRAINT moderation_actions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.moderation_evidence
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_evidence_snapshot_chk
    CHECK (jsonb_typeof(snapshot) = 'object' AND octet_length(snapshot::text) <= 1048576),
  ADD CONSTRAINT moderation_evidence_sha_chk CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  ADD CONSTRAINT moderation_evidence_case_fk FOREIGN KEY (case_id) REFERENCES chat.moderation_cases (case_id),
  ADD CONSTRAINT moderation_evidence_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.school_safety_notifications
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT school_safety_notifications_severity_chk CHECK (severity IN ('p1','p2','p3')),
  ADD CONSTRAINT school_safety_notifications_students_chk
    CHECK (jsonb_typeof(students) = 'array' AND jsonb_array_length(students) BETWEEN 1 AND 20),
  ADD CONSTRAINT school_safety_notifications_outcome_chk
    CHECK (contact_outcome IS NULL OR contact_outcome IN ('parent_contacted','unable_to_contact','not_needed')),
  ADD CONSTRAINT school_safety_notifications_reminders_chk CHECK (reminder_count >= 0),
  ADD CONSTRAINT school_safety_notifications_case_fk FOREIGN KEY (case_id) REFERENCES chat.moderation_cases (case_id),
  ADD CONSTRAINT school_safety_notifications_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- F116 rows keep reason_code NULL (read as coalesce(reason_code, reason): auto_spam, block_and_alert_hold).
ALTER TABLE chat.sanctions
  ADD CONSTRAINT sanctions_issued_by_kind_chk CHECK (issued_by_kind IN ('system','moderator'));

-- ---------- Append-only: case history and evidence (no UPDATE/DELETE, even by the owner) ----------
CREATE TRIGGER moderation_actions_append_only BEFORE UPDATE OR DELETE ON chat.moderation_actions
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();
CREATE TRIGGER moderation_evidence_append_only BEFORE UPDATE OR DELETE ON chat.moderation_evidence
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();

-- ---------- RLS (FORCE): tenant condition only (ADR-0089 §1) ----------
ALTER TABLE chat.reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.reports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.reports FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.reports FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.reports FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_cases FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_cases FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_cases FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_cases FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_actions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_actions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_actions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_actions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_evidence FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_evidence FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_evidence FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_evidence FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.school_safety_notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.school_safety_notifications FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.school_safety_notifications FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.school_safety_notifications FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.school_safety_notifications FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21): no DELETE/TRUNCATE; append-only tables get no UPDATE either ----------
REVOKE DELETE, TRUNCATE ON chat.reports, chat.moderation_cases, chat.moderation_actions, chat.moderation_evidence,
  chat.school_safety_notifications FROM chat_app, chat_platform;
REVOKE UPDATE ON chat.moderation_actions, chat.moderation_evidence FROM chat_app, chat_platform;

-- ---------- _live views for reporting (chat_readonly): case metadata only, no evidence or summary ----------
CREATE OR REPLACE VIEW chat.moderation_cases_live WITH (security_barrier = true) AS
  SELECT * FROM chat.moderation_cases WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.school_safety_notifications_live WITH (security_barrier = true) AS
  SELECT notification_id, tenant_id, case_id, school_id, severity, category_code, school_notified_at, ack_due_at,
    reminder_count, school_ack_at, contact_outcome, created_at FROM chat.school_safety_notifications WHERE deleted_at IS NULL;
GRANT SELECT ON chat.moderation_cases_live, chat.school_safety_notifications_live TO chat_readonly;
