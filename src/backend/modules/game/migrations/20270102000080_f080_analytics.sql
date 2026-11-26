-- F080 T009 — Game Analytics of module game (specs/080-game-analytics-liveops: data-model §1; DATABASE_DESIGN §2.8b;
-- GAME_ANALYTICS §5–§6, §9; DASHBOARD_DATA §3–§10; ADR-0058, ADR-0062, ADR-0070, ADR-0089). Applied by
-- `migrate --module=game` as game_migrator. Hand-written like game.login_events (composite keys, no entx mixin: derived
-- and pseudonymous tables without deleted_at — DASHBOARD_DATA §3.1; explicit SQL only). No column holds student_id, a
-- name, a nickname or chat content (F080-AC-06): players are player_key / session_key = HMAC-SHA256 hex (FR-ANL-008).
-- Not partitioned yet (spec Clarifications 2026-11-26: range indexes on the date; monthly/yearly partitions when the
-- tables reach the DASHBOARD_DATA §11 sizes, like the F077 ledgers). Never deleted: retention anonymises (ADR-0070).

-- ---------- analytics.events_raw (T): one pseudonymous event, server (domain event) or client (telemetry) ----------
CREATE TABLE analytics.events_raw (
  occurred_on    date NOT NULL,
  event_id       uuid NOT NULL,
  occurred_at    timestamptz NOT NULL,
  ingested_at    timestamptz NOT NULL DEFAULT now(),
  tenant_id      uuid NOT NULL,
  school_id      uuid,
  grade_level    smallint CHECK (grade_level BETWEEN 1 AND 12),
  platform       varchar(16),
  locale         varchar(8),
  device_tier    varchar(8),
  client_version varchar(32),
  player_key     varchar(64),
  session_key    varchar(64),
  key_version    smallint NOT NULL DEFAULT 1,
  event_name     varchar(64) NOT NULL,
  source         varchar(8) NOT NULL CHECK (source IN ('server', 'client')),
  props          jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(props::text) <= 2048),
  PRIMARY KEY (occurred_on, event_id)
);
CREATE INDEX events_raw_name_idx ON analytics.events_raw (occurred_on, event_name);
CREATE INDEX events_raw_tenant_idx ON analytics.events_raw (occurred_on, tenant_id);
CREATE INDEX events_raw_player_idx ON analytics.events_raw (player_key) WHERE player_key IS NOT NULL;

-- ---------- analytics.player_days (T): activity of one pseudonymous player in one ICT day (job rollup) ----------
-- player_key becomes NULL when anonymised, so the key of the row is a surrogate; the upsert key is (activity_date,
-- player_key) (NULLs never conflict).
CREATE TABLE analytics.player_days (
  activity_date     date NOT NULL,
  day_id            uuid NOT NULL DEFAULT gen_random_uuid(),
  player_key        varchar(64),
  tenant_id         uuid NOT NULL,
  school_id         uuid,
  grade_level       smallint,
  platform          varchar(16),
  locale            varchar(8),
  device_tier       varchar(8),
  client_version    varchar(32),
  sessions          integer NOT NULL DEFAULT 0,
  active_seconds    integer NOT NULL DEFAULT 0,
  first_active_date date,
  capped_xp         boolean NOT NULL DEFAULT false,
  capped_currency   boolean NOT NULL DEFAULT false,
  reminders_shown   smallint NOT NULL DEFAULT 0,
  late_night        boolean NOT NULL DEFAULT false,
  social            boolean NOT NULL DEFAULT false,
  matches           integer NOT NULL DEFAULT 0,
  purchases         integer NOT NULL DEFAULT 0,
  level             smallint,
  computed_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (activity_date, day_id)
);
CREATE UNIQUE INDEX player_days_player_uq ON analytics.player_days (activity_date, player_key);
CREATE INDEX player_days_key_idx ON analytics.player_days (player_key, activity_date) WHERE player_key IS NOT NULL;
CREATE INDEX player_days_tenant_idx ON analytics.player_days (tenant_id, activity_date);

-- ---------- rollups (T): metric × fixed dimension set (dims / dims_hash) per day, ISO week (Monday) and month ----------
CREATE TABLE analytics.rollup_daily (
  metric_date    date NOT NULL,
  metric_code    varchar(64) NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  tenant_id      uuid NOT NULL,
  dims_hash      varchar(64) NOT NULL,
  dim_set        varchar(64) NOT NULL DEFAULT '',
  dims           jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(dims::text) <= 1024),
  value          numeric,
  numerator      bigint,
  denominator    bigint,
  players        integer NOT NULL DEFAULT 0,
  hist           jsonb CHECK (hist IS NULL OR octet_length(hist::text) <= 2048),
  is_final       boolean NOT NULL DEFAULT false,
  computed_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (metric_date, metric_code, metric_version, tenant_id, dims_hash)
);
CREATE INDEX rollup_daily_tenant_idx ON analytics.rollup_daily (tenant_id, metric_code, dim_set, metric_date);
CREATE TABLE analytics.rollup_weekly (LIKE analytics.rollup_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (metric_date, metric_code, metric_version, tenant_id, dims_hash));
CREATE INDEX rollup_weekly_tenant_idx ON analytics.rollup_weekly (tenant_id, metric_code, dim_set, metric_date);
CREATE TABLE analytics.rollup_monthly (LIKE analytics.rollup_daily INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
  PRIMARY KEY (metric_date, metric_code, metric_version, tenant_id, dims_hash));
CREATE INDEX rollup_monthly_tenant_idx ON analytics.rollup_monthly (tenant_id, metric_code, dim_set, metric_date);

-- ---------- analytics.cohort_retention (T): cohort of first active day × day N ----------
CREATE TABLE analytics.cohort_retention (
  cohort_date date NOT NULL,
  tenant_id   uuid NOT NULL,
  dims_hash   varchar(64) NOT NULL,
  dim_set     varchar(64) NOT NULL DEFAULT '',
  dims        jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(dims::text) <= 1024),
  day_n       smallint NOT NULL CHECK (day_n BETWEEN 0 AND 60),
  cohort_size integer NOT NULL,
  retained    integer NOT NULL,
  is_final    boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (cohort_date, tenant_id, dims_hash, day_n)
);
CREATE INDEX cohort_retention_tenant_idx ON analytics.cohort_retention (tenant_id, cohort_date);

-- ---------- analytics.funnel_daily (T): players of a cohort day reaching each step ----------
CREATE TABLE analytics.funnel_daily (
  bucket_date date NOT NULL,
  funnel_code varchar(64) NOT NULL,
  step_no     smallint NOT NULL CHECK (step_no BETWEEN 1 AND 50),
  tenant_id   uuid NOT NULL,
  dims_hash   varchar(64) NOT NULL,
  dim_set     varchar(64) NOT NULL DEFAULT '',
  dims        jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(dims::text) <= 1024),
  count       integer NOT NULL,
  is_final    boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bucket_date, funnel_code, step_no, tenant_id, dims_hash)
);
CREATE INDEX funnel_daily_tenant_idx ON analytics.funnel_daily (tenant_id, funnel_code, bucket_date);

-- ---------- analytics.content_stats_weekly (T): quest / challenge / boss version per ISO week (Monday) ----------
CREATE TABLE analytics.content_stats_weekly (
  week_start         date NOT NULL,
  tenant_id          uuid NOT NULL,
  content_kind       varchar(16) NOT NULL CHECK (content_kind IN ('quest', 'challenge', 'boss')),
  content_version_id uuid NOT NULL,
  content_id         uuid,
  started            integer NOT NULL DEFAULT 0,
  completed          integer NOT NULL DEFAULT 0,
  abandoned          integer NOT NULL DEFAULT 0,
  succeeded          integer NOT NULL DEFAULT 0,
  ended              integer NOT NULL DEFAULT 0,
  players            integer NOT NULL DEFAULT 0,
  duration_p50_s     integer,
  is_final           boolean NOT NULL DEFAULT false,
  computed_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (week_start, tenant_id, content_kind, content_version_id)
);
CREATE INDEX content_stats_weekly_tenant_idx ON analytics.content_stats_weekly (tenant_id, content_kind, week_start);

-- ---------- analytics.perf_daily (T): real performance per platform × device tier × preset × renderer × version ----------
CREATE TABLE analytics.perf_daily (
  bucket_date      date NOT NULL,
  tenant_id        uuid NOT NULL,
  platform         varchar(16) NOT NULL,
  device_tier      varchar(8) NOT NULL,
  preset           varchar(8) NOT NULL,
  renderer         varchar(16) NOT NULL,
  client_version   varchar(32) NOT NULL,
  samples          integer NOT NULL DEFAULT 0,
  sessions         integer NOT NULL DEFAULT 0,
  players          integer NOT NULL DEFAULT 0,
  fps_p50          numeric,
  fps_p10          numeric,
  budget_pass_rate numeric,
  load_cold_p50_ms integer,
  load_cold_p90_ms integer,
  load_area_p90_ms integer,
  crash_free_rate  numeric,
  is_final         boolean NOT NULL DEFAULT false,
  computed_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bucket_date, tenant_id, platform, device_tier, preset, renderer, client_version)
);
CREATE INDEX perf_daily_tenant_idx ON analytics.perf_daily (tenant_id, bucket_date);

-- ---------- analytics.rollup_days (T): closing state of a tenant day (jobs analytics-rollup-hourly / day close) ----------
CREATE TABLE analytics.rollup_days (
  tenant_id     uuid NOT NULL,
  bucket_date   date NOT NULL,
  is_final      boolean NOT NULL DEFAULT false,
  needs_reclose boolean NOT NULL DEFAULT false,
  closed_at     timestamptz,
  computed_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, bucket_date)
);
CREATE INDEX rollup_days_open_idx ON analytics.rollup_days (bucket_date) WHERE NOT is_final OR needs_reclose;

-- ---------- analytics.export_jobs (T, tenant_id NULL = platform export): asynchronous CSV exports ----------
CREATE TABLE analytics.export_jobs (
  export_id    uuid PRIMARY KEY,
  tenant_id    uuid,
  requested_by uuid NOT NULL,
  report       varchar(32) NOT NULL CHECK (report IN ('metrics', 'retention', 'funnel', 'content_stats', 'performance')),
  params       jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(params::text) <= 8192),
  status       varchar(16) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'expired')),
  row_count    integer NOT NULL DEFAULT 0,
  object_key   varchar(512),
  error_code   varchar(64),
  created_at   timestamptz NOT NULL DEFAULT now(),
  started_at   timestamptz,
  finished_at  timestamptz,
  expires_at   timestamptz
);
CREATE INDEX export_jobs_queue_idx ON analytics.export_jobs (created_at) WHERE status = 'queued';
CREATE INDEX export_jobs_user_idx ON analytics.export_jobs (requested_by, status);

-- ---------- RLS: tenant condition only (ADR-0089 §1); cross-tenant analytics run as game_platform (audited Bypass) ----------
ALTER TABLE analytics.events_raw ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.events_raw FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.player_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.player_days FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_daily FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_weekly FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_monthly ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_monthly FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.cohort_retention ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.cohort_retention FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.funnel_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.funnel_daily FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.content_stats_weekly ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.content_stats_weekly FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.perf_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.perf_daily FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.rollup_days FORCE ROW LEVEL SECURITY;
ALTER TABLE analytics.export_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE analytics.export_jobs FORCE ROW LEVEL SECURITY;
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['events_raw', 'player_days', 'rollup_daily', 'rollup_weekly', 'rollup_monthly', 'cohort_retention',
    'funnel_daily', 'content_stats_weekly', 'perf_daily', 'rollup_days', 'export_jobs'] LOOP
    EXECUTE format('CREATE POLICY tenant_read ON analytics.%I FOR SELECT USING (tenant_id = infra.current_tenant_id())', t);
    EXECUTE format('CREATE POLICY tenant_insert ON analytics.%I FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id())', t);
    EXECUTE format('CREATE POLICY tenant_update ON analytics.%I FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id())', t);
  END LOOP;
END $$;

-- ---------- privileges: no DELETE/TRUNCATE (R21); game_dashboard reads the rollups only (DASHBOARD_DATA §7) ----------
REVOKE DELETE, TRUNCATE ON analytics.events_raw, analytics.player_days, analytics.rollup_daily, analytics.rollup_weekly,
  analytics.rollup_monthly, analytics.cohort_retention, analytics.funnel_daily, analytics.content_stats_weekly,
  analytics.perf_daily, analytics.rollup_days, analytics.export_jobs FROM game_app, game_platform;
GRANT SELECT, INSERT, UPDATE ON analytics.events_raw, analytics.player_days, analytics.rollup_daily, analytics.rollup_weekly,
  analytics.rollup_monthly, analytics.cohort_retention, analytics.funnel_daily, analytics.content_stats_weekly,
  analytics.perf_daily, analytics.rollup_days, analytics.export_jobs TO game_app, game_platform;
GRANT USAGE ON SCHEMA analytics, ref, infra TO game_dashboard;
GRANT EXECUTE ON FUNCTION infra.current_tenant_id() TO game_dashboard;
GRANT SELECT ON analytics.rollup_daily, analytics.rollup_weekly, analytics.rollup_monthly, analytics.cohort_retention,
  analytics.funnel_daily, analytics.content_stats_weekly, analytics.perf_daily, analytics.rollup_days, ref.tenants, ref.schools TO game_dashboard;
