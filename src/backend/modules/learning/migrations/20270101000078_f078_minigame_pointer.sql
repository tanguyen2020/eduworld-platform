-- F078 — the release pointer of the mini-game configs of module game in module learning (specs/078-game-ops-console-backend:
-- F073 handoff; DATABASE_DESIGN §2.17 rule 2; ADR-0043). Applied by `migrate --module=learning` as learning_migrator.
--   * ref.minigame_config_pointers: the current version of each mini-game config as module game released it
--     (content.published / content.rolled_back move it, content.unpublished empties it); a learning activity that plays a
--     config without a version follows rollbacks through it. Platform data: no tenant_id, no deleted_at (projection).
--     Written only by the consumer learning.minigame_config_projection (learning_platform); an older event never
--     overwrites a newer one (source_version = release time in microseconds).

CREATE TABLE ref.minigame_config_pointers (
  minigame_config_id         uuid PRIMARY KEY,
  minigame_config_version_id uuid,
  source_version             bigint NOT NULL,
  updated_at                 timestamptz NOT NULL
);
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON ref.minigame_config_pointers FROM learning_app;
REVOKE DELETE, TRUNCATE ON ref.minigame_config_pointers FROM learning_platform;
GRANT SELECT ON ref.minigame_config_pointers TO learning_app;
GRANT SELECT, INSERT, UPDATE ON ref.minigame_config_pointers TO learning_platform;
