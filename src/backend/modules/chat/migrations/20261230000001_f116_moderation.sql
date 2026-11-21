-- F116 T006/T007/T008 — moderation pipeline, sensitive-term catalogue, moderation switches of module chat
-- (specs/116-chat-moderation-pipeline: data-model §1; DATABASE_DESIGN §2.12; MODERATION §7, §11, §12; ADR-0028, ADR-0030,
-- ADR-0041, ADR-0072, ADR-0078, ADR-0089). Applied by `migrate --module=chat` as chat_migrator.
-- Order: ent tables (entddl block, golden test modules/chat/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs,
-- numeric columns, expression unique indexes, the partitioned decision log (composite key), append-only triggers, the
-- platform seed (categories, version 1 of the catalogue with NO term — R15: terms are entered through the CMS —, the
-- all-on platform switch, the bilingual quick messages; before FORCE ROW LEVEL SECURITY), RLS, soft-delete only grants,
-- _live views. Seeds equal modules/chat/seed/moderation/*.json (TestSeedModeration).

-- BEGIN entddl
CREATE TABLE chat.sensitive_term_categories (
  category_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name varchar(128) NOT NULL,
  description varchar(1000),
  severity varchar(64) NOT NULL,
  default_action varchar(64) NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  allow_tenant_toggle boolean NOT NULL DEFAULT false,
  PRIMARY KEY (category_id)
);
CREATE UNIQUE INDEX sensitive_term_categories_code_uq ON chat.sensitive_term_categories (code) WHERE deleted_at IS NULL;

CREATE TABLE chat.sensitive_terms (
  term_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  category_id uuid NOT NULL,
  term varchar(255) NOT NULL,
  match_type varchar(64) NOT NULL,
  locale varchar(64) NOT NULL,
  variants jsonb,
  severity_override varchar(64),
  action_override varchar(64),
  enabled boolean NOT NULL DEFAULT true,
  status varchar(64) NOT NULL DEFAULT 'draft',
  note varchar(1000),
  published_in_version bigint,
  PRIMARY KEY (term_id)
);
CREATE INDEX sensitive_terms_tenant_category_idx ON chat.sensitive_terms (tenant_id, category_id);

CREATE TABLE chat.sensitive_allowlist (
  entry_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  phrase varchar(255) NOT NULL,
  locale varchar(64) NOT NULL,
  reason varchar(1000),
  PRIMARY KEY (entry_id)
);

CREATE TABLE chat.sensitive_catalog_versions (
  catalog_version_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  catalog_version bigint NOT NULL,
  scope varchar(64) NOT NULL,
  published_at timestamptz NOT NULL,
  published_by uuid,
  change_summary varchar(1000) NOT NULL,
  term_count bigint NOT NULL,
  checksum varchar(128) NOT NULL,
  snapshot jsonb NOT NULL,
  PRIMARY KEY (catalog_version_id)
);

CREATE TABLE chat.tenant_category_settings (
  setting_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  category_id uuid NOT NULL,
  enabled boolean NOT NULL,
  reason varchar(1000) NOT NULL,
  PRIMARY KEY (setting_id)
);
CREATE UNIQUE INDEX tenant_category_settings_uq ON chat.tenant_category_settings (tenant_id, category_id) WHERE deleted_at IS NULL;

CREATE TABLE chat.moderation_switches (
  switch_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope varchar(64) NOT NULL,
  enabled_all boolean NOT NULL DEFAULT true,
  steps jsonb NOT NULL,
  reason varchar(1000),
  expires_at timestamptz,
  PRIMARY KEY (switch_id)
);
CREATE INDEX moderation_switches_tenant_idx ON chat.moderation_switches (tenant_id);

CREATE TABLE chat.moderation_switch_requests (
  request_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope varchar(64) NOT NULL,
  enabled_all boolean NOT NULL,
  steps jsonb NOT NULL,
  reason varchar(1000) NOT NULL,
  switch_expires_at timestamptz NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'pending_approval',
  proposed_by uuid NOT NULL,
  approved_by uuid,
  decided_at timestamptz,
  request_expires_at timestamptz NOT NULL,
  PRIMARY KEY (request_id)
);
CREATE INDEX moderation_switch_requests_status_idx ON chat.moderation_switch_requests (status, request_expires_at);

CREATE TABLE chat.moderation_switch_changes (
  change_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  scope varchar(64) NOT NULL,
  enabled_all_before boolean NOT NULL,
  enabled_all_after boolean NOT NULL,
  before jsonb NOT NULL,
  after jsonb NOT NULL,
  reason varchar(1000),
  expires_at timestamptz,
  actor_id uuid,
  approved_by uuid,
  request_id uuid,
  source varchar(64) NOT NULL,
  PRIMARY KEY (change_id)
);
CREATE INDEX moderation_switch_changes_tenant_idx ON chat.moderation_switch_changes (tenant_id, created_at);

CREATE TABLE chat.sanctions (
  sanction_id uuid NOT NULL,
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
  user_id uuid NOT NULL,
  type varchar(64) NOT NULL,
  scope varchar(64) NOT NULL,
  channel_id uuid,
  reason varchar(1000) NOT NULL,
  case_id uuid,
  issued_by uuid,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz,
  revoked_at timestamptz,
  PRIMARY KEY (sanction_id)
);
CREATE INDEX sanctions_user_active_idx ON chat.sanctions (tenant_id, user_id, ends_at) WHERE revoked_at IS NULL AND deleted_at IS NULL;

CREATE TABLE chat.quick_messages (
  quick_message_id varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  category varchar(64) NOT NULL,
  text_vi varchar(255) NOT NULL,
  text_en varchar(255) NOT NULL,
  emoji varchar(64),
  sort_order bigint NOT NULL DEFAULT 0,
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (quick_message_id)
);
-- END entddl

-- ---------- CHECKs, FKs, defaults (data-model §1, DATABASE_DESIGN §1.4) ----------
ALTER TABLE chat.sensitive_term_categories
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT sensitive_term_categories_platform_chk CHECK (tenant_id IS NULL),
  ADD CONSTRAINT sensitive_term_categories_code_chk CHECK (code ~ '^[a-z][a-z0-9_]{1,40}$'),
  ADD CONSTRAINT sensitive_term_categories_severity_chk CHECK (severity IN ('watch','low','medium','high','critical')),
  ADD CONSTRAINT sensitive_term_categories_action_chk
    CHECK (default_action IN ('allow','mask','block','hold_for_review','block_and_alert'));

ALTER TABLE chat.sensitive_terms
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT sensitive_terms_match_chk CHECK (match_type IN ('word','phrase','contains','pattern')),
  ADD CONSTRAINT sensitive_terms_locale_chk CHECK (locale IN ('vi','en')),
  ADD CONSTRAINT sensitive_terms_status_chk CHECK (status IN ('draft','published')),
  ADD CONSTRAINT sensitive_terms_severity_chk
    CHECK (severity_override IS NULL OR severity_override IN ('watch','low','medium','high','critical')),
  ADD CONSTRAINT sensitive_terms_action_chk
    CHECK (action_override IS NULL OR action_override IN ('allow','mask','block','hold_for_review','block_and_alert')),
  ADD CONSTRAINT sensitive_terms_variants_chk CHECK (variants IS NULL OR (jsonb_typeof(variants) = 'array'
    AND jsonb_array_length(variants) <= 50 AND octet_length(variants::text) <= 16384)),
  ADD CONSTRAINT sensitive_terms_pattern_chk CHECK (match_type <> 'pattern' OR char_length(term) <= 200),
  ADD CONSTRAINT sensitive_terms_category_fk FOREIGN KEY (category_id) REFERENCES chat.sensitive_term_categories (category_id),
  ADD CONSTRAINT sensitive_terms_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE UNIQUE INDEX sensitive_terms_uq ON chat.sensitive_terms
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), category_id, lower(term), match_type) WHERE deleted_at IS NULL;

ALTER TABLE chat.sensitive_allowlist
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT sensitive_allowlist_locale_chk CHECK (locale IN ('vi','en')),
  ADD CONSTRAINT sensitive_allowlist_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE UNIQUE INDEX sensitive_allowlist_uq ON chat.sensitive_allowlist
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(phrase)) WHERE deleted_at IS NULL;

ALTER TABLE chat.sensitive_catalog_versions
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN published_at SET DEFAULT now(),
  ADD COLUMN regression_recall numeric(5,4) CHECK (regression_recall IS NULL OR regression_recall BETWEEN 0 AND 1),
  ADD COLUMN regression_fp_rate numeric(5,4) CHECK (regression_fp_rate IS NULL OR regression_fp_rate BETWEEN 0 AND 1),
  ADD CONSTRAINT sensitive_catalog_versions_scope_chk CHECK (scope IN ('platform','tenant') AND (scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT sensitive_catalog_versions_version_chk CHECK (catalog_version >= 1 AND term_count >= 0),
  ADD CONSTRAINT sensitive_catalog_versions_snapshot_chk
    CHECK (jsonb_typeof(snapshot) = 'object' AND octet_length(snapshot::text) <= 16777216),
  ADD CONSTRAINT sensitive_catalog_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE UNIQUE INDEX sensitive_catalog_versions_uq ON chat.sensitive_catalog_versions
  (scope, coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), catalog_version);

ALTER TABLE chat.tenant_category_settings
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT tenant_category_settings_category_fk FOREIGN KEY (category_id) REFERENCES chat.sensitive_term_categories (category_id),
  ADD CONSTRAINT tenant_category_settings_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- A disabled value needs a reason and an expiry (ADR-0078); steps keeps only boolean switches (pii is not a switch).
ALTER TABLE chat.moderation_switches
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_switches_scope_chk CHECK (scope IN ('platform','tenant') AND (scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT moderation_switches_steps_chk CHECK (jsonb_typeof(steps) = 'object' AND octet_length(steps::text) <= 1024),
  ADD CONSTRAINT moderation_switches_reason_chk CHECK ((enabled_all
    AND coalesce((steps->>'sensitive_terms')::boolean, true) AND coalesce((steps->>'spam')::boolean, true)
    AND coalesce((steps->>'bullying')::boolean, true) AND coalesce((steps->>'async_analysis')::boolean, true))
    OR (reason IS NOT NULL AND expires_at IS NOT NULL)),
  ADD CONSTRAINT moderation_switches_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE UNIQUE INDEX moderation_switches_uq ON chat.moderation_switches
  (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE deleted_at IS NULL;

ALTER TABLE chat.moderation_switch_requests
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_switch_requests_scope_chk
    CHECK (scope IN ('platform','tenant') AND (scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT moderation_switch_requests_status_chk CHECK (status IN ('pending_approval','approved','rejected','expired')),
  ADD CONSTRAINT moderation_switch_requests_steps_chk CHECK (jsonb_typeof(steps) = 'object' AND octet_length(steps::text) <= 1024),
  ADD CONSTRAINT moderation_switch_requests_approver_chk CHECK (approved_by IS NULL OR approved_by <> proposed_by),
  ADD CONSTRAINT moderation_switch_requests_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.moderation_switch_changes
  ALTER COLUMN created_at SET DEFAULT now(),
  ADD CONSTRAINT moderation_switch_changes_scope_chk CHECK (scope IN ('platform','tenant') AND (scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT moderation_switch_changes_source_chk CHECK (source IN ('manual','approved_request','auto_expire')),
  ADD CONSTRAINT moderation_switch_changes_json_chk
    CHECK (octet_length(before::text) <= 1024 AND octet_length(after::text) <= 1024);

ALTER TABLE chat.sanctions
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT sanctions_type_chk CHECK (type IN ('warning','mute','restrict_text','restrict_dm','restrict_multiplayer',
    'restrict_competition','account_lock')),
  ADD CONSTRAINT sanctions_scope_chk CHECK (scope IN ('channel','all') AND (scope = 'channel') = (channel_id IS NOT NULL)),
  ADD CONSTRAINT sanctions_period_chk CHECK (ends_at IS NULL OR ends_at > starts_at),
  ADD CONSTRAINT sanctions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

ALTER TABLE chat.quick_messages
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT quick_messages_id_chk CHECK (quick_message_id ~ '^[a-z][a-z0-9_]{1,63}$'),
  ADD CONSTRAINT quick_messages_category_chk
    CHECK (category IN ('greeting','teamwork','encouragement','thanks','strategy','safety','other')),
  ADD CONSTRAINT quick_messages_status_chk CHECK (status IN ('active','retired'));

-- ---------- chat.moderation_decisions (TA, P month): the decision log, metadata only (F116-AC-09, MODERATION §12.4) ----------
-- Hand-written: composite primary key of a partitioned table. Until the partition job of business tables exists rows
-- land in the default partition (precedent chat.messages). No content column (R14). decision_id = message_id for a
-- stored message (idempotent on redelivery); blocked messages have no message_id.
CREATE TABLE chat.moderation_decisions (
  decision_id              uuid NOT NULL,
  tenant_id                uuid NOT NULL,
  channel_id               uuid NOT NULL,
  message_id               uuid,
  sender_id                uuid NOT NULL,
  decision                 varchar(64) NOT NULL
                           CHECK (decision IN ('allow','unchecked','mask','block','hold_for_review','block_and_alert')),
  category_codes           varchar(64)[] NOT NULL DEFAULT '{}' CHECK (cardinality(category_codes) <= 20),
  steps_run                varchar(64)[] NOT NULL DEFAULT '{}' CHECK (cardinality(steps_run) <= 20),
  steps_skipped            varchar(64)[] NOT NULL DEFAULT '{}' CHECK (cardinality(steps_skipped) <= 20),
  catalog_version_platform bigint,
  catalog_version_tenant   bigint,
  switch_snapshot_id       uuid,
  latency_ms               int NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
  created_at               timestamptz NOT NULL,
  PRIMARY KEY (decision_id, created_at)
) PARTITION BY RANGE (created_at);
CREATE TABLE chat.moderation_decisions_default PARTITION OF chat.moderation_decisions DEFAULT;
CREATE INDEX moderation_decisions_tenant_idx ON chat.moderation_decisions (tenant_id, created_at DESC);
CREATE INDEX moderation_decisions_sender_idx ON chat.moderation_decisions (sender_id, created_at DESC);

-- ---------- Append-only: catalogue versions, switch history, decision log (no UPDATE/DELETE, even by the owner) ----------
CREATE OR REPLACE FUNCTION chat.f_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'chat.%: append-only table', TG_TABLE_NAME USING ERRCODE = 'insufficient_privilege';
END $$;
CREATE TRIGGER sensitive_catalog_versions_append_only BEFORE UPDATE OR DELETE ON chat.sensitive_catalog_versions
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();
CREATE TRIGGER moderation_switch_changes_append_only BEFORE UPDATE OR DELETE ON chat.moderation_switch_changes
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();
CREATE TRIGGER moderation_decisions_append_only BEFORE UPDATE OR DELETE ON chat.moderation_decisions
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();

-- ---------- Platform seed (T008; before FORCE ROW LEVEL SECURITY: tenant_id NULL rows) ----------
-- BEGIN seed moderation
INSERT INTO chat.sensitive_term_categories (category_id, tenant_id, created_at, updated_at, code, name, severity, default_action, enabled, allow_tenant_toggle) VALUES
  ('01936a00-0000-7000-8000-000000000101', NULL, now(), now(), 'profanity', 'Từ thô tục', 'low', 'mask', true, false),
  ('01936a00-0000-7000-8000-000000000102', NULL, now(), now(), 'insult', 'Xúc phạm', 'medium', 'block', true, false),
  ('01936a00-0000-7000-8000-000000000103', NULL, now(), now(), 'violence', 'Bạo lực', 'medium', 'block', true, false),
  ('01936a00-0000-7000-8000-000000000104', NULL, now(), now(), 'sexual', 'Nội dung tình dục', 'high', 'hold_for_review', true, false),
  ('01936a00-0000-7000-8000-000000000105', NULL, now(), now(), 'drugs', 'Chất cấm', 'medium', 'block', true, false),
  ('01936a00-0000-7000-8000-000000000106', NULL, now(), now(), 'self_harm', 'Tự làm hại bản thân', 'critical', 'block_and_alert', true, false),
  ('01936a00-0000-7000-8000-000000000107', NULL, now(), now(), 'grooming_scam', 'Dụ dỗ, lừa đảo', 'critical', 'block_and_alert', true, false),
  ('01936a00-0000-7000-8000-000000000108', NULL, now(), now(), 'politics_religion', 'Chính trị, tôn giáo', 'medium', 'block', true, true),
  ('01936a00-0000-7000-8000-000000000109', NULL, now(), now(), 'pii_pattern', 'Mẫu thông tin cá nhân', 'medium', 'block', true, false),
  ('01936a00-0000-7000-8000-000000000110', NULL, now(), now(), 'context_watch', 'Từ cần theo dõi theo ngữ cảnh', 'watch', 'allow', true, true);
INSERT INTO chat.quick_messages (quick_message_id, created_at, updated_at, category, text_vi, text_en, emoji, sort_order, status) VALUES
  ('qm_hello', now(), now(), 'greeting', 'Xin chào!', 'Hello!', 'wave', 10, 'active'),
  ('qm_hi_team', now(), now(), 'greeting', 'Chào cả đội!', 'Hi team!', 'wave', 20, 'active'),
  ('qm_go_for_it', now(), now(), 'encouragement', 'Cố lên!', 'Go for it!', 'muscle', 30, 'active'),
  ('qm_great_job', now(), now(), 'encouragement', 'Giỏi quá!', 'Great job!', 'star', 40, 'active'),
  ('qm_thanks', now(), now(), 'thanks', 'Cảm ơn bạn!', 'Thank you!', 'heart', 50, 'active'),
  ('qm_together', now(), now(), 'teamwork', 'Mình cùng làm nhé!', 'Let''s do it together!', 'handshake', 60, 'active'),
  ('qm_need_help', now(), now(), 'teamwork', 'Bạn cần giúp không?', 'Do you need help?', 'raised_hand', 70, 'active'),
  ('qm_wait', now(), now(), 'strategy', 'Chờ mình một chút!', 'Wait for me!', 'hourglass', 80, 'active'),
  ('qm_follow_me', now(), now(), 'strategy', 'Đi theo mình!', 'Follow me!', 'point_right', 90, 'active'),
  ('qm_done_question', now(), now(), 'strategy', 'Mình làm xong câu {n} rồi!', 'I finished question {n}!', 'check', 100, 'active'),
  ('qm_dont_like', now(), now(), 'safety', 'Mình không thích điều đó.', 'I don''t like that.', 'stop', 110, 'active'),
  ('qm_bye', now(), now(), 'other', 'Tạm biệt!', 'Goodbye!', 'wave', 120, 'active');
-- END seed moderation
-- Version 1 of the platform catalogue: the seeded categories and no term (terms come from the CMS, R15).
INSERT INTO chat.sensitive_catalog_versions (catalog_version_id, tenant_id, created_at, catalog_version, scope, published_at,
  published_by, change_summary, term_count, checksum, snapshot)
SELECT '01936a00-0000-7000-8000-0000000001f0', NULL, now(), 1, 'platform', now(), NULL,
  'F116 seed: category mặc định, chưa có term (R15)', 0, encode(sha256(convert_to(s::text, 'UTF8')), 'hex'), s
FROM (SELECT jsonb_build_object('scope', 'platform', 'version', 1, 'categories', jsonb_agg(jsonb_build_object(
    'category_id', c.category_id, 'code', c.code, 'severity', c.severity, 'default_action', c.default_action,
    'enabled', c.enabled, 'allow_tenant_toggle', c.allow_tenant_toggle) ORDER BY c.code),
    'terms', '[]'::jsonb, 'allowlist', '[]'::jsonb) AS s
  FROM chat.sensitive_term_categories c WHERE c.deleted_at IS NULL) x;
-- Platform switches: every step on (moderation_switches, CONFIG_DEFAULTS); pii and grooming have no switch (ADR-0078).
INSERT INTO chat.moderation_switches (switch_id, tenant_id, created_at, updated_at, scope, enabled_all, steps)
VALUES ('01936a00-0000-7000-8000-0000000001f1', NULL, now(), now(), 'platform', true,
  '{"sensitive_terms":true,"spam":true,"bullying":true,"async_analysis":true}'::jsonb);

-- ---------- RLS (FORCE): tenant condition only (ADR-0089 §1); T* tables also show the platform rows (tenant_id NULL) ----------
ALTER TABLE chat.sensitive_term_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.sensitive_term_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.sensitive_term_categories FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.sensitive_term_categories FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.sensitive_term_categories FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.sensitive_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.sensitive_terms FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.sensitive_terms FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.sensitive_terms FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.sensitive_terms FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.sensitive_allowlist ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.sensitive_allowlist FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.sensitive_allowlist FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.sensitive_allowlist FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.sensitive_allowlist FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.sensitive_catalog_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.sensitive_catalog_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.sensitive_catalog_versions FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.sensitive_catalog_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.sensitive_catalog_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_switches ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_switches FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_switches FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_switches FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_switches FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_switch_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_switch_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_switch_requests FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_switch_requests FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_switch_requests FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_switch_changes ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_switch_changes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_switch_changes FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_switch_changes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_switch_changes FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.tenant_category_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.tenant_category_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.tenant_category_settings FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.tenant_category_settings FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.tenant_category_settings FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.sanctions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.sanctions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.sanctions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.sanctions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.sanctions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.moderation_decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.moderation_decisions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.moderation_decisions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.moderation_decisions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.moderation_decisions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21): no DELETE/TRUNCATE; append-only tables get no UPDATE either ----------
REVOKE DELETE, TRUNCATE ON chat.sensitive_term_categories, chat.sensitive_terms, chat.sensitive_allowlist,
  chat.sensitive_catalog_versions, chat.tenant_category_settings, chat.moderation_switches, chat.moderation_switch_requests,
  chat.moderation_switch_changes, chat.moderation_decisions, chat.sanctions, chat.quick_messages FROM chat_app, chat_platform;
REVOKE UPDATE ON chat.sensitive_catalog_versions, chat.moderation_switch_changes, chat.moderation_decisions FROM chat_app, chat_platform;
REVOKE ALL ON chat.moderation_decisions_default FROM chat_app, chat_platform;
GRANT SELECT, INSERT ON chat.moderation_decisions TO chat_app, chat_platform;

-- ---------- _live views for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW chat.sensitive_term_categories_live WITH (security_barrier = true) AS
  SELECT * FROM chat.sensitive_term_categories WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.tenant_category_settings_live WITH (security_barrier = true) AS
  SELECT * FROM chat.tenant_category_settings WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.moderation_switches_live WITH (security_barrier = true) AS
  SELECT * FROM chat.moderation_switches WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.sanctions_live WITH (security_barrier = true) AS
  SELECT * FROM chat.sanctions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.quick_messages_live WITH (security_barrier = true) AS
  SELECT * FROM chat.quick_messages WHERE deleted_at IS NULL;
GRANT SELECT ON chat.sensitive_term_categories_live, chat.tenant_category_settings_live, chat.moderation_switches_live,
  chat.sanctions_live, chat.quick_messages_live TO chat_readonly;
