-- OQ-17 (owner 2026-11-20), F014 T045 / F014-AC-27: game sessions ended at the daily play limit of the school
-- (end_reason daily_limit, CHILD_SAFETY §4.2). OQ F014-1, F014 T044: ref.students keeps the SIS connection of the
-- student (student.sis_link_created / student.sis_link_superseded / student.synced) so game.login_events records it
-- without reading module identity (R30). Expand only: every existing value stays valid. Applied as game_migrator.
ALTER TABLE game.game_sessions
  DROP CONSTRAINT game_sessions_end_reason_chk,
  ADD CONSTRAINT game_sessions_end_reason_chk CHECK (end_reason IS NULL OR end_reason IN ('logout','replaced_by_other_device',
    'timeout','account_locked','access_revoked','cleanup','client_crash','curfew','daily_limit'));
ALTER TABLE game.login_events
  DROP CONSTRAINT login_events_end_reason_check,
  ADD CONSTRAINT login_events_end_reason_check CHECK (end_reason IS NULL OR end_reason IN ('logout','replaced_by_other_device',
    'timeout','account_locked','access_revoked','cleanup','client_crash','curfew','daily_limit'));

ALTER TABLE ref.students ADD COLUMN sis_connection_id uuid;
