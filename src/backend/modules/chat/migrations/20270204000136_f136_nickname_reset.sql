-- F136 T034 — sanction nickname_reset of module chat (specs/136-free-text-nickname: data-model §1; ADR-0105 §7;
-- MODERATION §7; DATABASE_DESIGN §2.11). Applied by `migrate --module=chat` as chat_migrator. Widening of two CHECKs only
-- (no column, no data change — R12): the type nickname_reset, its period — immediate, ends_at = starts_at (applied
-- once, never active, never lifted; every other type keeps ends_at IS NULL OR ends_at > starts_at) — and the case action
-- nickname_reset of chat.moderation_actions.
ALTER TABLE chat.sanctions DROP CONSTRAINT sanctions_type_chk;
ALTER TABLE chat.sanctions ADD CONSTRAINT sanctions_type_chk CHECK (type IN ('warning','mute','restrict_text','restrict_dm',
  'restrict_multiplayer','restrict_competition','account_lock','nickname_reset'));
ALTER TABLE chat.sanctions DROP CONSTRAINT sanctions_period_chk;
ALTER TABLE chat.sanctions ADD CONSTRAINT sanctions_period_chk CHECK (
  (type <> 'nickname_reset' AND (ends_at IS NULL OR ends_at > starts_at))
  OR (type = 'nickname_reset' AND ends_at = starts_at AND scope = 'all'));
-- The case action nickname_reset in the append-only history (F136-AC-19).
ALTER TABLE chat.moderation_actions DROP CONSTRAINT moderation_actions_action_chk;
ALTER TABLE chat.moderation_actions
  ADD CONSTRAINT moderation_actions_action_chk CHECK (action IN ('report','signal','assign','warn','mute','restrict',
    'remove_message','escalate','dismiss','close','reopen','system_escalate','hold','release_hold','notify_school',
    'school_acknowledged','school_reminder','school_reminder_lead','oncall_page','nickname_reset'));
