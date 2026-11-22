-- F117 T007/T008 — gameplay chat channels (match/team/party, always-on) and the gameplay chat purge (specs/117-gameplay-chat-purge:
-- data-model §1, §3; DATABASE_DESIGN §2.12, §3.0, §3.8b; CHAT_SYSTEM §4.1, §6.1; ADR-0020 §6, ADR-0028, ADR-0033, ADR-0041,
-- ADR-0070, ADR-0089). Applied by `migrate --module=chat` as chat_migrator. Every table here is hand-written (projection,
-- partitioned table, aggregates of the purge job: no ent entity). Nothing is deleted by this migration (R12): it only
-- creates tables, functions, policies and grants. The ONLY physical delete of the application stays the job
-- gameplay-chat-purge, connected as chat_purger (R21 exception, ADR-0033), whose rights are granted at the end.

-- ---------- Projections (DATABASE_DESIGN §2.17): retention of the tenant, parties ----------
-- identity · platform_settings.changed (key tenant_settings, scope tenant): gameplay_chat_retention_days of the tenant
-- (NULL = inherit the platform chat_retention, projected in ref.platform_settings). Fixed into purge_after when a channel
-- closes (research R4: never applied retroactively).
ALTER TABLE ref.tenants
  ADD COLUMN gameplay_chat_retention_days smallint CHECK (gameplay_chat_retention_days IS NULL
    OR gameplay_chat_retention_days BETWEEN 1 AND 365),
  ADD COLUMN retention_version bigint NOT NULL DEFAULT 0;

-- competition · party.* (consumer chat.gameplay_channels): the per-member version of the last applied change, so that a
-- late or redelivered event never undoes a newer one (F117-AC-03); status disbanded closes the party channel for good.
CREATE TABLE ref.parties (
  party_id        uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status          varchar(64) NOT NULL DEFAULT 'open' CHECK (status IN ('open','disbanded')),
  member_versions jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(member_versions) = 'object'
    AND octet_length(member_versions::text) <= 16384),
  source_version  bigint NOT NULL DEFAULT 0,
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ref_parties_tenant_idx ON ref.parties (tenant_id);
ALTER TABLE ref.parties ENABLE ROW LEVEL SECURITY;
ALTER TABLE ref.parties FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.parties FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.parties FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.parties FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON ref.parties FROM chat_app, chat_platform;

-- ---------- chat.gameplay_chat_daily_stats (T): rollup before the purge, kept for analytics (no content, no user) ----------
CREATE TABLE chat.gameplay_chat_daily_stats (
  tenant_id    uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  day          date NOT NULL,
  channel_type varchar(64) NOT NULL CHECK (channel_type IN ('match','party','team')),
  channels     int NOT NULL DEFAULT 0 CHECK (channels >= 0),
  messages     int NOT NULL DEFAULT 0 CHECK (messages >= 0),
  masked       int NOT NULL DEFAULT 0 CHECK (masked >= 0),
  blocked      int NOT NULL DEFAULT 0 CHECK (blocked >= 0),
  reported     int NOT NULL DEFAULT 0 CHECK (reported >= 0),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, day, channel_type)
);
ALTER TABLE chat.gameplay_chat_daily_stats ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.gameplay_chat_daily_stats FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.gameplay_chat_daily_stats FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.gameplay_chat_daily_stats FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.gameplay_chat_daily_stats FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- chat.gameplay_purge_runs (append-only): one row per tenant per run, or a run-level row (tenant NULL) ----------
CREATE TABLE chat.gameplay_purge_runs (
  row_id             uuid PRIMARY KEY,
  run_id             uuid NOT NULL,
  tenant_id          uuid REFERENCES ref.tenants (tenant_id),
  kind               varchar(64) NOT NULL CHECK (kind IN ('gameplay','ephemeral')),
  started_at         timestamptz NOT NULL,
  finished_at        timestamptz NOT NULL,
  channels_purged    int NOT NULL DEFAULT 0 CHECK (channels_purged >= 0),
  messages_purged    bigint NOT NULL DEFAULT 0 CHECK (messages_purged >= 0),
  partitions_dropped int NOT NULL DEFAULT 0 CHECK (partitions_dropped >= 0),
  deferred_open_case int NOT NULL DEFAULT 0 CHECK (deferred_open_case >= 0),
  evidence_copied    int NOT NULL DEFAULT 0 CHECK (evidence_copied >= 0),
  status             varchar(64) NOT NULL CHECK (status IN ('succeeded','partial','failed','blocked'))
);
-- One row per (run, kind, tenant); tenant_id NULL is the run-level row (blocked run, partitions dropped across tenants).
CREATE UNIQUE INDEX gameplay_purge_runs_uq ON chat.gameplay_purge_runs
  (run_id, kind, coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX gameplay_purge_runs_started_idx ON chat.gameplay_purge_runs (started_at);
CREATE TRIGGER gameplay_purge_runs_append_only BEFORE UPDATE OR DELETE ON chat.gameplay_purge_runs
  FOR EACH ROW EXECUTE FUNCTION chat.f_append_only();
ALTER TABLE chat.gameplay_purge_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.gameplay_purge_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.gameplay_purge_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.gameplay_purge_runs FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON chat.gameplay_purge_runs FROM chat_app, chat_platform;

-- ---------- chat.ephemeral_messages (T, P day): world/area messages (written by F120), purged by partition ----------
-- Hand-written: composite primary key of a partitioned table (precedent chat.gameplay_messages).
CREATE TABLE chat.ephemeral_messages (
  message_id       uuid NOT NULL,
  channel_id       uuid NOT NULL,
  tenant_id        uuid NOT NULL,
  school_id        uuid,
  sender_id        uuid NOT NULL,
  kind             varchar(64) NOT NULL CHECK (kind IN ('text','emoji','quick','sticker','gif','system')),
  body             varchar(1000),
  quick_message_id varchar(64),
  moderation_state varchar(64) NOT NULL DEFAULT 'allowed'
                   CHECK (moderation_state IN ('allowed','masked','unchecked','held','removed')),
  filter_verdict   jsonb CHECK (filter_verdict IS NULL OR octet_length(filter_verdict::text) <= 4096),
  created_at       timestamptz NOT NULL,
  deleted_at       timestamptz,
  deleted_by       uuid,
  created_on       date NOT NULL,
  PRIMARY KEY (message_id, created_on)
) PARTITION BY RANGE (created_on);
CREATE TABLE chat.ephemeral_messages_default PARTITION OF chat.ephemeral_messages DEFAULT;
CREATE INDEX ephemeral_messages_channel_idx ON chat.ephemeral_messages (channel_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ephemeral_messages_created_idx ON chat.ephemeral_messages (created_at);
ALTER TABLE chat.ephemeral_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat.ephemeral_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON chat.ephemeral_messages FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON chat.ephemeral_messages FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON chat.ephemeral_messages FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON chat.ephemeral_messages FROM chat_app, chat_platform;
REVOKE ALL ON chat.ephemeral_messages_default FROM chat_app, chat_platform;

-- Moderation cases of a channel (holds of the purge, F117-AC-18).
CREATE INDEX moderation_cases_channel_idx ON chat.moderation_cases (channel_id) WHERE channel_id IS NOT NULL;

-- ---------- Always-on rule in the database (F117-AC-09: every write path, whatever its code) ----------
-- A rule of match/team/party at any level: never disabled, keeps text/emoji/quick, no allowed_hours, slow mode ≤ 10 s
-- (= policy.ValidateAlwaysOn, chat-policy.schema.json v5 "ADR-0020 mục 6"); the platform level keeps a rule of each.
CREATE OR REPLACE FUNCTION chat.f_policy_always_on_ok(p_scope text, p_rules jsonb) RETURNS boolean
  LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(p_rules->'rules') = 'array' THEN p_rules->'rules' ELSE '[]'::jsonb END) r
      WHERE r->>'channel_type' IN ('match','team','party')
        AND (r->'enabled' = 'false'::jsonb
          OR (jsonb_typeof(r->'allowed_kinds') = 'array' AND NOT (r->'allowed_kinds' @> '["text","emoji","quick"]'::jsonb))
          OR (r ? 'allowed_hours' AND r->'allowed_hours' <> 'null'::jsonb)
          OR (jsonb_typeof(r->'slow_mode_seconds') = 'number' AND (r->>'slow_mode_seconds')::numeric > 10)))
    AND (p_scope <> 'platform' OR (
      SELECT count(DISTINCT r->>'channel_type') FROM jsonb_array_elements(CASE WHEN jsonb_typeof(p_rules->'rules') = 'array'
        THEN p_rules->'rules' ELSE '[]'::jsonb END) r WHERE r->>'channel_type' IN ('match','team','party')) = 3)
$$;
-- NOT VALID then VALIDATE: a stored row that breaks the rule (none expected: the resolver already ignored it) keeps the
-- migration applicable and is reported; every new write is checked either way.
ALTER TABLE chat.chat_policies
  ADD CONSTRAINT chat_policies_always_on_chk CHECK (chat.f_policy_always_on_ok(scope_type, rules)) NOT VALID;
DO $$
BEGIN
  ALTER TABLE chat.chat_policies VALIDATE CONSTRAINT chat_policies_always_on_chk;
EXCEPTION WHEN check_violation THEN
  RAISE WARNING 'chat_policies_always_on_chk left NOT VALID: a stored policy restricts match/team/party (ignored by the resolver)';
END $$;

-- ---------- Daily partitions of the gameplay and ephemeral message tables (job partition-maintainer, worker-chat) ----------
-- Runs as the table owner; new partitions get no grant (access goes through the parent, like infra.ensure_partitions).
-- A day whose rows already sit in the default partition is skipped with a warning.
CREATE FUNCTION chat.ensure_message_partitions(p_from date, p_days int) RETURNS int
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, chat AS $$
DECLARE
  t text; d date; part text; g record; created int := 0;
BEGIN
  IF p_days < 1 OR p_days > 62 THEN
    RAISE EXCEPTION 'p_days must be within 1..62';
  END IF;
  FOREACH t IN ARRAY ARRAY['gameplay_messages', 'ephemeral_messages'] LOOP
    FOR i IN 0 .. p_days - 1 LOOP
      d := p_from + i;
      part := t || '_p' || to_char(d, 'YYYYMMDD');
      CONTINUE WHEN to_regclass('chat.' || part) IS NOT NULL;
      BEGIN
        EXECUTE format('CREATE TABLE chat.%I PARTITION OF chat.%I FOR VALUES FROM (%L) TO (%L)', part, t, d::text, (d + 1)::text);
        FOR g IN SELECT DISTINCT grantee FROM information_schema.role_table_grants
                 WHERE table_schema = 'chat' AND table_name = part AND grantee <> current_user LOOP
          EXECUTE format('REVOKE ALL ON chat.%I FROM %I', part, g.grantee);
        END LOOP;
        created := created + 1;
      EXCEPTION WHEN check_violation THEN
        RAISE WARNING 'chat partition % skipped: rows for that day are in the default partition', part;
      END;
    END LOOP;
  END LOOP;
  RETURN created;
END $$;

-- Number of consecutive daily partitions of p_table from p_from (gauge of the job).
CREATE FUNCTION chat.message_partition_days_ahead(p_table text, p_from date) RETURNS int
  LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, chat AS $$
DECLARE n int := 0;
BEGIN
  IF p_table NOT IN ('gameplay_messages', 'ephemeral_messages') THEN
    RAISE EXCEPTION 'not a chat partitioned message table: %', p_table;
  END IF;
  WHILE n < 62 AND to_regclass('chat.' || p_table || '_p' || to_char(p_from + n, 'YYYYMMDD')) IS NOT NULL LOOP
    n := n + 1;
  END LOOP;
  RETURN n;
END $$;

-- Detach + drop the daily partition of chat.gameplay_messages of p_day (DATABASE_DESIGN §3.8b step 4): only a day at least
-- 3 days old (minimum retention, JetStream redelivery) whose partition holds no row any more — every channel with messages
-- that day was purged row by row (evidence copied first). Returns dropped | refused | absent. Never the default partition.
CREATE FUNCTION chat.drop_gameplay_partition(p_day date) RETURNS text
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, chat AS $$
DECLARE part text := 'gameplay_messages_p' || to_char(p_day, 'YYYYMMDD'); left_rows boolean;
BEGIN
  IF to_regclass('chat.' || part) IS NULL THEN
    RETURN 'absent';
  END IF;
  IF p_day > (now() AT TIME ZONE 'UTC')::date - 3 THEN
    RETURN 'refused';
  END IF;
  EXECUTE format('SELECT EXISTS (SELECT 1 FROM chat.%I)', part) INTO left_rows;
  IF left_rows THEN
    RETURN 'refused';
  END IF;
  EXECUTE format('ALTER TABLE chat.gameplay_messages DETACH PARTITION chat.%I', part);
  EXECUTE format('DROP TABLE chat.%I', part);
  RETURN 'dropped';
END $$;

-- Detach + drop the daily partition of chat.ephemeral_messages of p_day when the whole day is older than p_before
-- (now − ephemeral_chat_retention, at least one hour ago). The job copies the evidence of the reported messages of the day
-- before calling it and does not call it when that copy failed (fail-closed, F117-AC-19, AC-21).
CREATE FUNCTION chat.drop_ephemeral_partition(p_day date, p_before timestamptz) RETURNS text
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, chat AS $$
DECLARE part text := 'ephemeral_messages_p' || to_char(p_day, 'YYYYMMDD');
BEGIN
  IF to_regclass('chat.' || part) IS NULL THEN
    RETURN 'absent';
  END IF;
  IF p_before > now() - interval '1 hour' OR (p_day + 1)::timestamp AT TIME ZONE 'UTC' > p_before THEN
    RETURN 'refused';
  END IF;
  EXECUTE format('ALTER TABLE chat.ephemeral_messages DETACH PARTITION chat.%I', part);
  EXECUTE format('DROP TABLE chat.%I', part);
  RETURN 'dropped';
END $$;

REVOKE ALL ON FUNCTION chat.ensure_message_partitions(date, int), chat.message_partition_days_ahead(text, date),
  chat.drop_gameplay_partition(date), chat.drop_ephemeral_partition(date, timestamptz) FROM PUBLIC;

-- ---------- Role chat_purger (ADR-0033, DATABASE_DESIGN §3.0): the only role with DELETE, on the gameplay tables only ----------
GRANT USAGE ON SCHEMA chat, ref, infra TO chat_purger;
GRANT EXECUTE ON FUNCTION infra.current_tenant_id() TO chat_purger;
GRANT SELECT ON atlas_schema_revisions.atlas_schema_revisions TO chat_purger;
GRANT USAGE ON SCHEMA atlas_schema_revisions TO chat_purger;
GRANT SELECT, UPDATE, DELETE ON chat.gameplay_messages, chat.channels, chat.channel_members, chat.ephemeral_messages,
  chat.gameplay_partition_channels TO chat_purger;
-- Holds and evidence sources (read only): reports, cases, case actions, sanctions.
GRANT SELECT ON chat.reports, chat.moderation_cases, chat.moderation_actions, chat.sanctions TO chat_purger;
-- chat_retention of the platform (ephemeral_retention_hours), projected from identity.
GRANT SELECT ON ref.platform_settings TO chat_purger;
GRANT SELECT, INSERT ON chat.moderation_evidence, chat.gameplay_purge_runs TO chat_purger;
GRANT SELECT, INSERT, UPDATE ON chat.gameplay_chat_daily_stats TO chat_purger;
GRANT INSERT ON infra.outbox_events TO chat_purger; -- chat.gameplay_purged, chat.daily_stats, audit.recorded (outbox, §1.3)
GRANT EXECUTE ON FUNCTION chat.ensure_message_partitions(date, int), chat.message_partition_days_ahead(text, date),
  chat.drop_gameplay_partition(date), chat.drop_ephemeral_partition(date, timestamptz) TO chat_purger;
REVOKE TRUNCATE ON ALL TABLES IN SCHEMA chat, ref, infra FROM chat_purger;

-- RLS of the purger (permissive, OR'ed with the tenant policies): it sees and deletes only CLOSED gameplay channels, their
-- members and messages, in every tenant (the job lists the expired channels across tenants, then works one channel per
-- transaction with app.tenant_id of that channel). A club team channel (no match_id) or a direct channel is never
-- is_gameplay (CHECK channels_gameplay_chk), so no policy ever lets the purger delete it (F117-AC-20).
CREATE POLICY purger_gameplay_only ON chat.channels FOR ALL TO chat_purger
  USING (is_gameplay AND status = 'closed' AND closed_at IS NOT NULL)
  WITH CHECK (is_gameplay AND status = 'closed' AND closed_at IS NOT NULL);
CREATE POLICY purger_gameplay_only ON chat.channel_members FOR ALL TO chat_purger
  USING (EXISTS (SELECT 1 FROM chat.channels c WHERE c.channel_id = channel_members.channel_id AND c.is_gameplay
    AND c.status = 'closed' AND c.closed_at IS NOT NULL));
CREATE POLICY purger_gameplay_only ON chat.gameplay_messages FOR ALL TO chat_purger
  USING (EXISTS (SELECT 1 FROM chat.channels c WHERE c.channel_id = gameplay_messages.channel_id AND c.is_gameplay
    AND c.status = 'closed' AND c.closed_at IS NOT NULL));
CREATE POLICY purger_gameplay_only ON chat.gameplay_partition_channels FOR SELECT TO chat_purger USING (true);
CREATE POLICY purger_mark_purged ON chat.gameplay_partition_channels FOR UPDATE TO chat_purger USING (true) WITH CHECK (true);
-- Ephemeral rows of the default partition: only rows older than one hour (floor of ephemeral_chat_retention).
CREATE POLICY purger_expired_only ON chat.ephemeral_messages FOR ALL TO chat_purger
  USING (created_at < now() - interval '1 hour');
-- Run-level rows of the purge (tenant NULL) and their reading by the job.
CREATE POLICY purger_runs_read ON chat.gameplay_purge_runs FOR SELECT TO chat_purger USING (true);
CREATE POLICY purger_runs_insert ON chat.gameplay_purge_runs FOR INSERT TO chat_purger WITH CHECK (true);
