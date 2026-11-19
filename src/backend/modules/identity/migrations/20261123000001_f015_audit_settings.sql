-- F015 T005/T006/T039 — audit log, platform settings, portal menu catalog and feature flags of module identity
-- (DATABASE_DESIGN §2.15, §3.11, §3.15, ADR-0028, ADR-0070, ADR-0089, ADR-0090). Hand-written: the tables use
-- monthly range partitions without default (audit), arrays and a generated column, which ent does not render;
-- module identity reads and writes them with explicit SQL only. Applied as identity_migrator.

-- ---------- audit.audit_logs (TA*, P month, no default partition): append-only ----------
CREATE TABLE audit.audit_logs (
  audit_id    uuid NOT NULL,                          -- = event_id of audit.recorded (idempotent)
  occurred_at timestamptz NOT NULL,
  actor_id    uuid,                                   -- NULL = system/job
  actor_type  varchar(64) NOT NULL CHECK (actor_type IN ('user','service','system')),
  tenant_id   uuid,                                   -- NULL for platform operations
  school_id   uuid,
  action      varchar(64) NOT NULL,
  target_type varchar(64) NOT NULL,
  target_id   varchar(128) NOT NULL,
  result      varchar(64) NOT NULL CHECK (result IN ('success','denied','error')),
  reason      varchar(1000),
  metadata    jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object' AND octet_length(metadata::text) <= 65536),
  trace_id    varchar(64),
  ip_hash     bytea CHECK (ip_hash IS NULL OR octet_length(ip_hash) <= 64),
  producer    varchar(64) NOT NULL,                   -- <module>/<component> of the envelope
  PRIMARY KEY (audit_id, occurred_at)
) PARTITION BY RANGE (occurred_at);
CREATE INDEX audit_tenant_time_idx ON audit.audit_logs (tenant_id, occurred_at DESC);
CREATE INDEX audit_school_time_idx ON audit.audit_logs (school_id, occurred_at DESC) WHERE school_id IS NOT NULL;
CREATE INDEX audit_target_idx ON audit.audit_logs (target_type, target_id);
CREATE INDEX audit_action_time_idx ON audit.audit_logs (action, occurred_at DESC);

ALTER TABLE audit.audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.audit_logs FORCE ROW LEVEL SECURITY;
-- identity_app reads the rows of its tenant; the sink and System Admin reads go through identity_platform.
CREATE POLICY tenant_read ON audit.audit_logs FOR SELECT USING (tenant_id = infra.current_tenant_id());
-- FORCE RLS applies to the owner too: the SECURITY DEFINER retention function (owner) sees and updates every row.
CREATE POLICY owner_all ON audit.audit_logs FOR ALL TO identity_migrator USING (true) WITH CHECK (true);
REVOKE ALL ON audit.audit_logs FROM identity_app, identity_platform;
GRANT USAGE ON SCHEMA audit TO identity_app, identity_platform, identity_partition_admin;
GRANT SELECT ON audit.audit_logs TO identity_app;
GRANT SELECT, INSERT ON audit.audit_logs TO identity_platform;  -- no UPDATE/DELETE/TRUNCATE for anyone but the owner (AC-02)

-- Monthly partitions [first day of p_from's month + i months), i in 0..p_months-1, of the identity tables
-- partitioned by month (audit.audit_logs, identity.login_events). Runs as the table owner; a month whose rows
-- already sit in a default partition (login_events) is skipped with a warning. No default partition is ever
-- created for audit (F015-AC-03: a missing partition must fail loudly, never fill a default).
CREATE FUNCTION audit.ensure_monthly_partitions(p_from date, p_months int) RETURNS int
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, audit, identity AS $$
DECLARE
  t text; m date; part text; created int := 0; g record; sch text; tbl text;
BEGIN
  IF p_months < 1 OR p_months > 24 THEN
    RAISE EXCEPTION 'p_months must be within 1..24';
  END IF;
  FOREACH t IN ARRAY ARRAY['audit.audit_logs', 'identity.login_events'] LOOP
    sch := split_part(t, '.', 1); tbl := split_part(t, '.', 2);
    FOR i IN 0 .. p_months - 1 LOOP
      m := (date_trunc('month', p_from) + make_interval(months => i))::date;
      part := tbl || '_p' || to_char(m, 'YYYYMM');
      CONTINUE WHEN to_regclass(sch || '.' || part) IS NOT NULL;
      BEGIN
        EXECUTE format('CREATE TABLE %I.%I PARTITION OF %I.%I FOR VALUES FROM (%L) TO (%L)', sch, part, sch, tbl,
          m::text || ' 00:00:00+00', (m + interval '1 month')::date::text || ' 00:00:00+00');
        FOR g IN SELECT DISTINCT grantee FROM information_schema.role_table_grants
                 WHERE table_schema = sch AND table_name = part AND grantee <> current_user LOOP
          EXECUTE format('REVOKE ALL ON %I.%I FROM %I', sch, part, g.grantee);
        END LOOP;
        created := created + 1;
      EXCEPTION WHEN check_violation THEN
        RAISE WARNING 'partition % skipped: rows of that month are in the default partition', part;
      END;
    END LOOP;
  END LOOP;
  RETURN created;
END $$;

-- Number of consecutive monthly partitions of p_table from the month of p_from (alert when the next month is
-- missing 7 days ahead, plan F015 "Quan sát").
CREATE FUNCTION audit.monthly_partitions_ahead(p_table text, p_from date) RETURNS int
  LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, audit, identity AS $$
DECLARE n int := 0;
BEGIN
  IF p_table NOT IN ('audit.audit_logs', 'identity.login_events') THEN
    RAISE EXCEPTION 'not a monthly partitioned table: %', p_table;
  END IF;
  WHILE n < 24 AND to_regclass(p_table || '_p' || to_char(date_trunc('month', p_from) + make_interval(months => n), 'YYYYMM')) IS NOT NULL LOOP
    n := n + 1;
  END LOOP;
  RETURN n;
END $$;

-- Retention (F015-AC-06, DATA_PRIVACY §2, ADR-0070): rows older than p_before are anonymized in place —
-- actor_id and ip_hash cleared, identifier keys (*_id, *_ids, *_hash) removed from metadata. Never deletes,
-- never drops a partition. Returns the number of rows changed (idempotent: anonymized rows are skipped).
-- A definer function instead of a role with column UPDATE keeps identity_app/identity_platform without any
-- UPDATE on the table [Proposed in place of the identity_retention role of the data model].
CREATE FUNCTION audit.anonymize_expired(p_before timestamptz) RETURNS bigint
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, audit AS $$
DECLARE n bigint;
BEGIN
  UPDATE audit.audit_logs a SET actor_id = NULL, ip_hash = NULL,
    metadata = (SELECT coalesce(jsonb_object_agg(k, v), '{}'::jsonb) FROM jsonb_each(a.metadata) AS e(k, v)
                WHERE k !~ '(_id|_ids|_hash)$')
  WHERE a.occurred_at < p_before
    AND (a.actor_id IS NOT NULL OR a.ip_hash IS NOT NULL
         OR EXISTS (SELECT 1 FROM jsonb_object_keys(a.metadata) k WHERE k ~ '(_id|_ids|_hash)$'));
  GET DIAGNOSTICS n = ROW_COUNT;
  RETURN n;
END $$;

REVOKE ALL ON FUNCTION audit.ensure_monthly_partitions(date, int), audit.monthly_partitions_ahead(text, date),
  audit.anonymize_expired(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION audit.ensure_monthly_partitions(date, int), audit.monthly_partitions_ahead(text, date),
  audit.anonymize_expired(timestamptz) TO identity_partition_admin;

-- First partitions: the previous month through eleven months ahead (UTC).
SELECT audit.ensure_monthly_partitions(((now() AT TIME ZONE 'UTC') - interval '1 month')::date, 13);

-- ---------- platform.platform_settings (G): one JSON value per key, version for If-Match ----------
-- No seed: a missing key means the default of CONFIG_DEFAULTS read from configuration (AGENTS R33).
CREATE TABLE platform.platform_settings (
  key        varchar(64) PRIMARY KEY CHECK (key IN ('student_cleanup', 'chat_retention', 'platform_curfew')),
  value      jsonb NOT NULL CHECK (jsonb_typeof(value) = 'object' AND octet_length(value::text) <= 16384),
  version    integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid
);
REVOKE ALL ON platform.platform_settings FROM identity_app, identity_platform;
GRANT SELECT ON platform.platform_settings TO identity_app;
GRANT SELECT, INSERT, UPDATE ON platform.platform_settings TO identity_platform;  -- System Admin writes (platform scope)

-- ---------- platform.portal_menu_catalog (G, read-only copy of specs/portal/menu-catalog.yaml, ADR-0090) ----------
CREATE TABLE platform.portal_menu_catalog (
  item_key             varchar(128) PRIMARY KEY
                       CHECK (item_key ~ '^(school_lms|content_studio|game_ops|live_ops|platform_admin)$'
                           OR item_key ~ '^menu\.[a-z_]+\.[a-z0-9_]+$'
                           OR item_key ~ '^action\.[a-z_]+\.[a-z0-9_]+\.[a-z0-9_]+$'),
  item_kind            varchar(16)  NOT NULL CHECK (item_kind IN ('workspace','menu','action')),
  workspace_key        varchar(32)  NOT NULL CHECK (workspace_key IN ('school_lms','content_studio','game_ops','live_ops','platform_admin')),
  parent_key           varchar(128) REFERENCES platform.portal_menu_catalog(item_key),
  path                 varchar(256),
  label_key            varchar(128) NOT NULL,
  title                varchar(200),
  required_permissions varchar(64)[] NOT NULL DEFAULT '{}',
  operation_ids        varchar(128)[] NOT NULL DEFAULT '{}',
  sort_order           integer NOT NULL DEFAULT 0,
  can_disable          boolean NOT NULL DEFAULT true,
  catalog_version      varchar(64) NOT NULL,
  synced_at            timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,
  deleted_by           uuid,
  delete_reason        varchar(64),
  CHECK ((item_kind = 'workspace') = (parent_key IS NULL)),
  CHECK (item_kind = 'action' OR cardinality(operation_ids) = 0),
  CHECK (item_kind = 'menu' OR path IS NULL)
);
CREATE INDEX portal_menu_catalog_parent_idx ON platform.portal_menu_catalog (parent_key) WHERE deleted_at IS NULL;
REVOKE ALL ON platform.portal_menu_catalog FROM identity_app, identity_platform;
GRANT SELECT ON platform.portal_menu_catalog TO identity_app, identity_platform;  -- only identity_migrator writes

-- ---------- platform.feature_flags (G, ADR-0090): no row = enabled ----------
CREATE TABLE platform.feature_flags (
  flag_key          varchar(128) PRIMARY KEY CHECK (flag_key ~ '^[a-z0-9_.]{3,128}$'),
  flag_kind         varchar(16)  NOT NULL DEFAULT 'feature' CHECK (flag_kind IN ('feature','menu','action')),
  parent_key        varchar(128),
  description       varchar(1000),
  default_enabled   boolean NOT NULL DEFAULT true,
  tenant_overrides  jsonb NOT NULL DEFAULT '{}'
                    CHECK (jsonb_typeof(tenant_overrides) = 'object' AND octet_length(tenant_overrides::text) <= 65536),
  catalog_key       varchar(128) GENERATED ALWAYS AS (CASE WHEN flag_kind <> 'feature' THEN flag_key END) STORED,
  version           integer NOT NULL DEFAULT 1,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  updated_by        uuid,
  deleted_at        timestamptz,
  deleted_by        uuid,
  delete_reason     varchar(64),
  CHECK ((flag_kind = 'action') = (parent_key IS NOT NULL)),
  FOREIGN KEY (catalog_key) REFERENCES platform.portal_menu_catalog(item_key),
  FOREIGN KEY (parent_key)  REFERENCES platform.portal_menu_catalog(item_key)
);
CREATE INDEX feature_flags_kind_idx ON platform.feature_flags (flag_kind) WHERE deleted_at IS NULL;
REVOKE ALL ON platform.feature_flags FROM identity_app, identity_platform;
GRANT SELECT ON platform.feature_flags TO identity_app;
GRANT SELECT, INSERT, UPDATE ON platform.feature_flags TO identity_platform;  -- System Admin writes; soft-delete only

CREATE OR REPLACE VIEW platform.feature_flags_live WITH (security_barrier = true) AS
  SELECT * FROM platform.feature_flags WHERE deleted_at IS NULL;
GRANT SELECT ON platform.feature_flags_live TO identity_readonly;
CREATE OR REPLACE VIEW platform.portal_menu_catalog_live WITH (security_barrier = true) AS
  SELECT * FROM platform.portal_menu_catalog WHERE deleted_at IS NULL;
GRANT SELECT ON platform.portal_menu_catalog_live TO identity_readonly;
