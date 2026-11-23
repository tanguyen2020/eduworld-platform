-- F073 — Mini-game Engine, part of module learning (specs/073-minigame-engine: data-model §1; DATABASE_DESIGN §2.17,
-- §3.4; MINIGAME_ENGINE §3, §6; ADR-0043). Applied by `migrate --module=learning` as learning_migrator.
--   * ref.minigame_config_versions: immutable projection of the published MiniGameConfigVersion of module game
--     (event content.published, item_type minigame_config, platform data: no tenant_id, no deleted_at — DATABASE_DESIGN
--     §2.17 rule 2). Insert-only by the consumer learning.minigame_config_projection (learning_platform); a replayed
--     event inserts nothing (F073-AC-04). The snapshot holds no question and no answer (F073-AC-03).
--   * assessment.attempts: game_score, game_streak (current correct streak, needed by the streak bonus — not in the
--     data-model, F073 Clarifications) and minigame_config_version_id (the pinned configuration version); source_kind
--     minigame_config for the plays started by module game through createAttempt.

CREATE TABLE ref.minigame_config_versions (
  minigame_config_version_id uuid PRIMARY KEY,
  minigame_config_id         uuid NOT NULL,
  version                    integer NOT NULL,
  minigame_type              varchar(64) NOT NULL,
  grade_levels               smallint[] NOT NULL,
  snapshot                   jsonb NOT NULL,
  published_at               timestamptz NOT NULL,
  source_version             bigint NOT NULL,
  CONSTRAINT minigame_config_versions_version_ck CHECK (version >= 1),
  CONSTRAINT minigame_config_versions_type_ck CHECK (minigame_type IN ('multiple_choice','true_false','fill_in_blank',
    'drag_and_drop','matching','ordering','calculation','puzzle','word_game','science_simulation','timed_challenge')),
  CONSTRAINT minigame_config_versions_grades_ck CHECK (cardinality(grade_levels) BETWEEN 1 AND 9
    AND grade_levels <@ ARRAY[1,2,3,4,5,6,7,8,9]::smallint[]),
  CONSTRAINT minigame_config_versions_snapshot_ck CHECK (jsonb_typeof(snapshot) = 'object' AND octet_length(snapshot::text) <= 65536)
);
CREATE UNIQUE INDEX ref_minigame_config_versions_no_uq ON ref.minigame_config_versions (minigame_config_id, version);
CREATE INDEX ref_minigame_config_versions_current_idx ON ref.minigame_config_versions (minigame_config_id, published_at DESC);
-- Immutable: the application role reads it; only the platform role of the consumer inserts (never UPDATE, DELETE).
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON ref.minigame_config_versions FROM learning_app;
REVOKE UPDATE, DELETE, TRUNCATE ON ref.minigame_config_versions FROM learning_platform;
GRANT SELECT ON ref.minigame_config_versions TO learning_app;
GRANT SELECT, INSERT ON ref.minigame_config_versions TO learning_platform;

-- ---------- assessment.attempts: mini-game columns (DATABASE_DESIGN §3.4) ----------
-- BEGIN entddl

ALTER TABLE assessment.attempts ADD COLUMN game_score integer;
ALTER TABLE assessment.attempts ADD COLUMN game_streak integer;
ALTER TABLE assessment.attempts ADD COLUMN minigame_config_version_id uuid;
-- END entddl

ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_game_score_ck CHECK (game_score IS NULL OR game_score >= 0),
  ADD CONSTRAINT attempts_game_streak_ck CHECK (game_streak IS NULL OR game_streak >= 0),
  ADD CONSTRAINT attempts_minigame_ck CHECK (minigame_config_version_id IS NULL OR activity_type = 'minigame');
ALTER TABLE assessment.attempts DROP CONSTRAINT attempts_source_kind_ck;
ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_source_kind_ck CHECK (source_kind IN ('activity','question_set','selection','exam_paper','minigame_config'));
