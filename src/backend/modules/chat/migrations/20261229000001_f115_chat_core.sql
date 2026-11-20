-- F115 T006/T007/T009 — chat core of module chat (specs/115-chat-core: data-model §1; DATABASE_DESIGN §2.12, §3.8, §3.8b;
-- ADR-0028, ADR-0033, ADR-0035, ADR-0041, ADR-0078, ADR-0089). Applied by `migrate --module=chat` as chat_migrator.
-- Order: projections ref.schools / ref.platform_settings (curfew, ADR-0078; hand-written, rule 5), ent tables (entddl
-- block, golden test modules/chat/ent/ddl_test.go), the platform default chat policy (seed, before RLS), then what ent
-- cannot express: CHECKs, FKs, the partitioned message tables (composite keys), unaccent wrapper, RLS, soft-delete
-- only grants, _live views. Message rows are written by chat-persister only (R25).

-- ---------- Projections (DATABASE_DESIGN §2.17) for the curfew of the send path (F115-AC-22) ----------
-- identity · school.synced/school.updated (consumer chat.projection_identity): settings_subset.curfew_window only is read.
CREATE TABLE ref.schools (
  school_id       uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status          varchar(64) NOT NULL DEFAULT 'active',
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);
CREATE INDEX ref_schools_tenant_idx ON ref.schools (tenant_id);
ALTER TABLE ref.schools ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- identity · platform_settings.changed (key platform_curfew, scope platform): a G table (no tenant), read by the send path,
-- written by the platform consumer only.
CREATE TABLE ref.platform_settings (
  key            varchar(128) PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]*$'),
  value          jsonb NOT NULL CHECK (octet_length(value::text) <= 16384),
  source_version bigint NOT NULL,
  updated_at     timestamptz NOT NULL DEFAULT now()
);
REVOKE ALL ON ref.platform_settings FROM chat_app, chat_platform;
GRANT SELECT ON ref.platform_settings TO chat_app;
GRANT SELECT, INSERT, UPDATE ON ref.platform_settings TO chat_platform;
REVOKE DELETE, TRUNCATE ON ref.schools FROM chat_app, chat_platform;

CREATE OR REPLACE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
    WHEN 'player_profiles' THEN (SELECT pp.tenant_id FROM ref.player_profiles pp WHERE pp.student_id = p_id)
    WHEN 'schools' THEN (SELECT sc.tenant_id FROM ref.schools sc WHERE sc.school_id = p_id)
  END
$$;
REVOKE ALL ON FUNCTION ref.projection_tenant(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ref.projection_tenant(text, uuid) TO chat_app, chat_platform;

-- BEGIN entddl
CREATE TABLE chat.channels (
  channel_id uuid NOT NULL,
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
  channel_type varchar(64) NOT NULL,
  is_gameplay boolean NOT NULL DEFAULT false,
  scope_ref jsonb,
  policy_id uuid,
  retention_days bigint,
  last_seq bigint NOT NULL DEFAULT 0,
  status varchar(64) NOT NULL DEFAULT 'open',
  opened_at timestamptz NOT NULL,
  closed_at timestamptz,
  purge_after timestamptz,
  PRIMARY KEY (channel_id)
);
CREATE INDEX channels_type_status_idx ON chat.channels (tenant_id, channel_type, status);

CREATE TABLE chat.channel_members (
  channel_member_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  channel_id uuid NOT NULL,
  user_id uuid NOT NULL,
  school_id uuid,
  role varchar(64) NOT NULL DEFAULT 'member',
  joined_at timestamptz NOT NULL,
  left_at timestamptz,
  muted_until timestamptz,
  last_delivered_seq bigint NOT NULL DEFAULT 0,
  last_read_seq bigint NOT NULL DEFAULT 0,
  unread_mentions bigint NOT NULL DEFAULT 0,
  show_read_receipts boolean NOT NULL DEFAULT true,
  notification_level varchar(64) NOT NULL DEFAULT 'all',
  PRIMARY KEY (channel_member_id)
);
CREATE UNIQUE INDEX channel_members_pair_uq ON chat.channel_members (channel_id, user_id) WHERE deleted_at IS NULL;
CREATE INDEX channel_members_user_idx ON chat.channel_members (tenant_id, user_id);

CREATE TABLE chat.chat_policies (
  policy_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope_type varchar(64) NOT NULL,
  scope_id uuid,
  school_id uuid,
  rules jsonb NOT NULL,
  PRIMARY KEY (policy_id)
);
CREATE INDEX chat_policies_tenant_scope_idx ON chat.chat_policies (tenant_id, scope_type);
-- END entddl

-- ---------- Platform default chat policy (T009, CHAT_SYSTEM §3.1 [Confirmed]) ----------
-- Source: modules/chat/seed/chat_policies.platform.json (= specs/examples/chat-policy.platform-default.json, schema v5);
-- the test seed_policy_test.go checks this literal equals the file. Inserted before FORCE ROW LEVEL SECURITY (tenant NULL).
-- BEGIN seed chat_policies.platform.json
INSERT INTO chat.chat_policies (policy_id, tenant_id, created_at, updated_at, version, scope_type, scope_id, school_id, rules)
VALUES ('01936a00-0000-7000-8000-000000000001', NULL, now(), now(), 1, 'platform', NULL, NULL, '{"schema_version":5,"policy_id":"01936a00-0000-7000-8000-000000000001","scope_type":"platform","scope_id":null,"version":1,"timezone":"Asia/Ho_Chi_Minh","global_rate":{"messages_per_minute":30,"burst":5},"world_scope":"school","moderation_switches":{"enabled_all":true,"steps":{"sensitive_terms":true,"pii":true,"spam":true,"bullying":true,"async_analysis":true}},"sensitive_term_policy":{"categories":[],"action_overrides":{}},"rules":[{"channel_type":"world","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":120,"rate":{"messages_per_minute":6,"burst":2},"slow_mode_seconds":5,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"21:30"}],"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":1,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":false,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":false,"typing":false,"receipts":"none","search":false,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"area","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":120,"rate":{"messages_per_minute":10,"burst":3},"slow_mode_seconds":0,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"21:30"}],"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":1,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":false,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":false,"typing":false,"receipts":"none","search":false,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"match","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":200,"rate":{"messages_per_minute":12,"burst":3},"slow_mode_seconds":0,"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":7,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"per_member","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"party","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":200,"rate":{"messages_per_minute":12,"burst":3},"slow_mode_seconds":0,"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":7,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"per_member","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"team","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":200,"rate":{"messages_per_minute":12,"burst":3},"slow_mode_seconds":0,"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":7,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"per_member","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"class","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":300,"rate":{"messages_per_minute":10,"burst":3},"slow_mode_seconds":0,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"21:30"}],"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":180,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"count","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"study_group","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":300,"rate":{"messages_per_minute":10,"burst":3},"slow_mode_seconds":0,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"21:30"}],"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":180,"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"count","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}},{"channel_type":"direct","enabled":true,"allowed_kinds":["text","emoji","quick","sticker","gif"],"reactions":true,"max_text_length":300,"rate":{"messages_per_minute":15,"burst":4},"slow_mode_seconds":0,"allowed_hours":[{"days":"mon-sun","from":"06:30","to":"21:30"}],"validation":{"strictness":"strict","profanity":"mask","pii":"block","unknown_link":"block","spam":"block","bullying_suspected":"hold_for_review"},"retention_days":180,"sensitive_term_policy":{"action_overrides":{"insult":"block"}},"features":{"edit":true,"recall":true,"reply":true,"quote":true,"forward":true,"sticker":true,"gif":true,"reaction":true,"mention":true,"pin":true,"typing":true,"receipts":"per_member","search":true,"edit_window_minutes":15,"recall_window_hours":24,"max_pins":3,"max_mentions":5,"max_forward_targets":5,"max_reactions_per_user":3}}],"quick_message_set_id":"qm-default-vi-v1","notes":"Mặc định platform — một bộ quy tắc cho mọi lớp 1–9 (ADR-0020). Giá trị số là [Proposed]. match/team/party là channel gameplay always-on (ADR-0020 mục 6): không có allowed_hours, cấp dưới không tắt được."}'::jsonb);
-- END seed chat_policies.platform.json

-- ---------- chat.channels, chat.channel_members, chat.chat_policies: CHECKs, FKs ----------
ALTER TABLE chat.channels
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(), ALTER COLUMN opened_at SET DEFAULT now(),
  ADD COLUMN tenant_ids uuid[],
  ADD CONSTRAINT channels_type_chk CHECK (channel_type IN ('world','area','match','party','team','class','study_group','direct')),
  ADD CONSTRAINT channels_gameplay_chk CHECK (is_gameplay = (channel_type IN ('match','party')
    OR (channel_type = 'team' AND scope_ref->>'match_id' IS NOT NULL))),
  ADD CONSTRAINT channels_status_chk CHECK (status IN ('open','closed')),
  ADD CONSTRAINT channels_scope_ref_chk CHECK (scope_ref IS NULL OR octet_length(scope_ref::text) <= 2048),
  ADD CONSTRAINT channels_last_seq_chk CHECK (last_seq >= 0),
  ADD CONSTRAINT channels_tenant_ids_chk CHECK (tenant_ids IS NULL OR cardinality(tenant_ids) <= 64),
  ADD CONSTRAINT channels_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE INDEX channels_purge_idx ON chat.channels (purge_after) WHERE is_gameplay AND closed_at IS NOT NULL;

ALTER TABLE chat.channel_members
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(), ALTER COLUMN joined_at SET DEFAULT now(),
  ADD CONSTRAINT channel_members_role_chk CHECK (role IN ('member','leader')),
  ADD CONSTRAINT channel_members_notification_chk CHECK (notification_level IN ('all','mentions','none')),
  ADD CONSTRAINT channel_members_cursor_chk CHECK (last_delivered_seq >= 0 AND last_read_seq >= 0 AND unread_mentions >= 0),
  ADD CONSTRAINT channel_members_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT channel_members_channel_fk FOREIGN KEY (channel_id) REFERENCES chat.channels (channel_id);

ALTER TABLE chat.chat_policies
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT chat_policies_scope_chk CHECK (scope_type IN ('platform','tenant','school','class')),
  ADD CONSTRAINT chat_policies_platform_chk CHECK ((scope_type = 'platform') = (tenant_id IS NULL)
    AND (scope_type = 'platform') = (scope_id IS NULL)),
  ADD CONSTRAINT chat_policies_rules_chk CHECK (octet_length(rules::text) <= 65536 AND jsonb_typeof(rules) = 'object'),
  ADD CONSTRAINT chat_policies_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
CREATE UNIQUE INDEX chat_policies_scope_uq ON chat.chat_policies
  (scope_type, coalesce(scope_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE deleted_at IS NULL;

-- ---------- chat.messages / chat.gameplay_messages (P): DDL of DATABASE_DESIGN §3.8, §3.8b ----------
-- Hand-written: composite primary keys of partitioned tables are outside ent (precedent competition.integrity_signals).
-- Until the partition job of business tables exists rows land in the default partitions. quick_message_id is the
-- string code of the contract (varchar(64), contracts §6). F115 creates the GIN index used by search (F120).
-- unaccent lives in schema chat (not public): the application roles have USAGE on chat only (least privilege).
CREATE EXTENSION IF NOT EXISTS unaccent SCHEMA chat;
CREATE OR REPLACE FUNCTION chat.f_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT chat.unaccent('chat.unaccent'::regdictionary, $1) $$;
GRANT EXECUTE ON FUNCTION chat.f_unaccent(text) TO chat_app, chat_platform;

CREATE TABLE chat.messages (
  message_id          uuid NOT NULL,
  channel_id          uuid NOT NULL,
  seq                 bigint NOT NULL CHECK (seq >= 1),
  tenant_id           uuid NOT NULL,
  school_id           uuid,
  sender_id           uuid NOT NULL,
  client_message_id   uuid,
  kind                varchar(64) NOT NULL CHECK (kind IN ('text','emoji','quick','sticker','gif','system')),
  body                varchar(1000),
  body_display        varchar(1000),
  quick_message_id    varchar(64),
  sticker_id          uuid,
  gif_id              uuid,
  reply_to_message_id uuid,
  quote               jsonb CHECK (quote IS NULL OR octet_length(quote::text) <= 2048),
  forwarded_from      jsonb CHECK (forwarded_from IS NULL OR octet_length(forwarded_from::text) <= 2048),
  mentions            uuid[] NOT NULL DEFAULT '{}',
  revision            int NOT NULL DEFAULT 1,
  edit_count          int NOT NULL DEFAULT 0,
  edited_at           timestamptz,
  recalled_at         timestamptz,
  recalled_by         uuid,
  moderation_state    varchar(64) NOT NULL DEFAULT 'allowed'
                      CHECK (moderation_state IN ('allowed','masked','unchecked','held','removed')),
  filter_verdict      jsonb CHECK (filter_verdict IS NULL OR octet_length(filter_verdict::text) <= 4096),
  search_tsv          tsvector GENERATED ALWAYS AS
                      (to_tsvector('simple', chat.f_unaccent(lower(coalesce(body_display, ''))))) STORED,
  created_at          timestamptz NOT NULL,
  deleted_at          timestamptz,
  deleted_by          uuid,
  PRIMARY KEY (message_id, created_at),
  UNIQUE (channel_id, seq, created_at),
  CHECK (cardinality(mentions) <= 5)
) PARTITION BY RANGE (created_at);
CREATE TABLE chat.messages_default PARTITION OF chat.messages DEFAULT;
CREATE INDEX messages_channel_seq_idx ON chat.messages (channel_id, seq DESC) WHERE deleted_at IS NULL;
CREATE INDEX messages_sender_idx      ON chat.messages (sender_id, created_at DESC);
CREATE INDEX messages_search_idx      ON chat.messages USING gin (search_tsv)
  WHERE deleted_at IS NULL AND recalled_at IS NULL AND moderation_state IN ('allowed','masked','unchecked');
CREATE INDEX messages_mentions_idx    ON chat.messages USING gin (mentions);

CREATE TABLE chat.gameplay_messages (
  message_id          uuid NOT NULL,
  channel_id          uuid NOT NULL,
  seq                 bigint NOT NULL CHECK (seq >= 1),
  tenant_id           uuid NOT NULL,
  school_id           uuid,
  sender_id           uuid NOT NULL,
  client_message_id   uuid,
  kind                varchar(64) NOT NULL CHECK (kind IN ('text','emoji','quick','sticker','gif','system')),
  body                varchar(1000),
  body_display        varchar(1000),
  quick_message_id    varchar(64),
  sticker_id          uuid,
  gif_id              uuid,
  reply_to_message_id uuid,
  quote               jsonb CHECK (quote IS NULL OR octet_length(quote::text) <= 2048),
  forwarded_from      jsonb CHECK (forwarded_from IS NULL OR octet_length(forwarded_from::text) <= 2048),
  mentions            uuid[] NOT NULL DEFAULT '{}',
  revision            int NOT NULL DEFAULT 1,
  edit_count          int NOT NULL DEFAULT 0,
  edited_at           timestamptz,
  recalled_at         timestamptz,
  recalled_by         uuid,
  moderation_state    varchar(64) NOT NULL DEFAULT 'allowed'
                      CHECK (moderation_state IN ('allowed','masked','unchecked','held','removed')),
  filter_verdict      jsonb CHECK (filter_verdict IS NULL OR octet_length(filter_verdict::text) <= 4096),
  search_tsv          tsvector GENERATED ALWAYS AS
                      (to_tsvector('simple', chat.f_unaccent(lower(coalesce(body_display, ''))))) STORED,
  created_at          timestamptz NOT NULL,
  deleted_at          timestamptz,
  deleted_by          uuid,
  created_on          date NOT NULL,
  PRIMARY KEY (message_id, created_on),
  UNIQUE (channel_id, seq, created_on),
  CHECK (cardinality(mentions) <= 5)
) PARTITION BY RANGE (created_on);
CREATE TABLE chat.gameplay_messages_default PARTITION OF chat.gameplay_messages DEFAULT;
CREATE INDEX gameplay_messages_channel_idx ON chat.gameplay_messages (channel_id, seq DESC) WHERE deleted_at IS NULL;
CREATE INDEX gameplay_messages_search_idx  ON chat.gameplay_messages USING gin (search_tsv)
  WHERE deleted_at IS NULL AND recalled_at IS NULL;

-- Which channel has messages in which daily partition (purge of F117, ADR-0033); written by chat-persister.
CREATE TABLE chat.gameplay_partition_channels (
  partition_day date NOT NULL,
  channel_id    uuid NOT NULL,
  tenant_id     uuid NOT NULL,
  purged_at     timestamptz,
  PRIMARY KEY (partition_day, channel_id)
);

-- ---------- RLS (FORCE) on every tenant table: tenant condition only (ADR-0089 §1) ----------
ALTER TABLE chat.channels ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.channels FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.channels FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.channels FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.channels FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.channel_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.channel_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.channel_members FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.channel_members FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.channel_members FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
-- T*: a tenant session reads its own rows and the platform default (tenant_id NULL); writes only its own rows.
ALTER TABLE chat.chat_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.chat_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.chat_policies FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.chat_policies FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.chat_policies FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.messages FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.messages FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.messages FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.gameplay_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.gameplay_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.gameplay_messages FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.gameplay_messages FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.gameplay_messages FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE chat.gameplay_partition_channels ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.gameplay_partition_channels FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.gameplay_partition_channels FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.gameplay_partition_channels FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.gameplay_partition_channels FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (R21, TC-TEN-016): no DELETE/TRUNCATE for application roles; partitions via parents only ----------
REVOKE DELETE, TRUNCATE ON chat.channels, chat.channel_members, chat.chat_policies, chat.messages, chat.gameplay_messages,
  chat.gameplay_partition_channels FROM chat_app, chat_platform;
REVOKE ALL ON chat.messages_default, chat.gameplay_messages_default FROM chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON chat.messages, chat.gameplay_messages, chat.gameplay_partition_channels TO chat_app, chat_platform;

-- ---------- _live views for reporting (chat_readonly) ----------
CREATE OR REPLACE VIEW chat.channels_live WITH (security_barrier = true) AS
  SELECT * FROM chat.channels WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.channel_members_live WITH (security_barrier = true) AS
  SELECT * FROM chat.channel_members WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.chat_policies_live WITH (security_barrier = true) AS
  SELECT * FROM chat.chat_policies WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.messages_live WITH (security_barrier = true) AS
  SELECT * FROM chat.messages WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW chat.gameplay_messages_live WITH (security_barrier = true) AS
  SELECT * FROM chat.gameplay_messages WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA chat TO chat_readonly;
GRANT SELECT ON chat.channels_live, chat.channel_members_live, chat.chat_policies_live, chat.messages_live,
  chat.gameplay_messages_live TO chat_readonly;
