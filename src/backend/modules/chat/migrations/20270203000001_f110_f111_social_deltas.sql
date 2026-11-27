-- Server deltas for the Godot client specs F110 and F111 (contracts §6; owner chốt 2026-11-28 [Confirmed]). Hand-written
-- like the other social tables of module chat (explicit SQL, RLS, R27). Applied by `migrate --module=chat` as
-- chat_migrator. Additive only (expand): one nullable column, one new table, one widened CHECK. Nothing is deleted (R21).

-- ---------- F095: PlayerCard.title (ref.player_profiles, projection of module game) ----------
-- The level title of the progression curve (LocalizedName {vi, en}), projected by chat.projection_game from
-- level.reached.title; NULL until the student reaches a titled level (the client hides it). Public game data only (R16).
ALTER TABLE ref.player_profiles ADD COLUMN title jsonb
  CHECK (title IS NULL OR (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 1024));

-- ---------- F095: social.recent_coplayers ("Vừa chơi cùng" of listFriendSuggestions) ----------
-- One row per (student, other student) of one tenant who played together: same party activity (party.activity_started,
-- context party/coop) or same match (match.started, context match; > 8 participants: same team only), written by the
-- consumer chat.recent_coplayers (projection of module competition events, R29 — never a call to competition). The row
-- is updated in place with the newest time; listFriendSuggestions reads the rows of the caller within
-- friend_suggestion_recent_window (CONFIG_DEFAULTS) and filters friends, pending requests, blocks, mutes and scope at
-- read time. Soft-delete columns are the standard ones (never set by this feature; R21).
CREATE TABLE social.recent_coplayers (
  tenant_id       uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  user_id         uuid NOT NULL,
  other_id        uuid NOT NULL,
  context_kind    varchar(16) NOT NULL CHECK (context_kind IN ('party', 'match', 'club', 'coop')),
  last_played_at  timestamptz NOT NULL,
  source_event_id uuid NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  deleted_by      uuid,
  PRIMARY KEY (tenant_id, user_id, other_id),
  CHECK (user_id <> other_id)
);
-- listFriendSuggestions: the caller's rows, most recent first (keyset last_played_at, other_id).
CREATE INDEX recent_coplayers_recent_idx ON social.recent_coplayers (tenant_id, user_id, last_played_at DESC, other_id DESC)
  WHERE deleted_at IS NULL;

ALTER TABLE social.recent_coplayers ENABLE ROW LEVEL SECURITY;
ALTER TABLE social.recent_coplayers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON social.recent_coplayers FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON social.recent_coplayers FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON social.recent_coplayers FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

REVOKE DELETE, TRUNCATE ON social.recent_coplayers FROM chat_app, chat_platform;
GRANT SELECT, INSERT, UPDATE ON social.recent_coplayers TO chat_app, chat_platform;

CREATE OR REPLACE VIEW social.recent_coplayers_live WITH (security_barrier = true) AS
  SELECT * FROM social.recent_coplayers WHERE deleted_at IS NULL;
GRANT SELECT ON social.recent_coplayers_live TO chat_readonly;

-- ---------- F123/F120: notification kinds of the clubs (NotificationKind of specs/schemas/common/enums.schema.json) ----------
-- Widening only: every existing value stays allowed (expand, R12).
ALTER TABLE social.notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE social.notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN ('assignment_new','assignment_due',
  'party_invitation','friend_request','quest_available','reward_granted','tournament_update','moderation_notice','system',
  'assignment_results_released','level_reached','achievement_unlocked','club_invitation','club_join_request','club_approved'));
