-- Projection of the platform settings of module identity that module game applies (OQ F014-2, F015 Clarifications
-- 2026-11-20; DATABASE_DESIGN §2.17): the consumer game.platform_settings keeps the latest value of each key from
-- platform_settings.changed (scope platform). Only key platform_curfew is projected: value = {start, end} "HH:MM"
-- (ICT, ADR-0078). Platform data, not tenant data: no tenant_id, no RLS. Written only by the consumer, which runs
-- under SystemPlatformScope as game_platform; upserted by source_version (envelope occurred_at, rule 1). While the
-- key has no row, module game uses its start-up default EW_PLATFORM_CURFEW (CONFIG_DEFAULTS platform_curfew).
CREATE TABLE ref.platform_settings (
  key            varchar(128) PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]*$'),
  value          jsonb NOT NULL CHECK (octet_length(value::text) <= 16384),
  source_version bigint NOT NULL,
  updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Use cases (game_app) only read it; the platform consumer (game_platform) writes it. No DELETE (R21).
REVOKE ALL ON ref.platform_settings FROM game_app, game_platform;
GRANT SELECT ON ref.platform_settings TO game_app;
GRANT SELECT, INSERT, UPDATE ON ref.platform_settings TO game_platform;
