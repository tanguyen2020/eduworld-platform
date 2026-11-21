-- ROLLBACK of 20261231000003_adr0095_check_values.sql (module chat, ADR-0095 mục 8 bước 4). Written in advance, NEVER
-- applied automatically. The migration drops nothing; this script only widens the two CHECKs again, exactly as
-- 20261229000001_f115_chat_core.sql defined them. Run as the table owner, in one transaction:
--   psql -v ON_ERROR_STOP=1 --single-transaction -U chat_migrator -d eduworld_chat -f scripts/db/rollback/adr-0095-chat.sql
-- Verified by tests/integration/adr0095_rollback_test.go (rollback → re-apply the contract migration).

ALTER TABLE chat.channels DROP CONSTRAINT channels_type_chk,
  ADD CONSTRAINT channels_type_chk CHECK (channel_type IN ('world','area','match','party','team','class','study_group','direct'));
ALTER TABLE chat.chat_policies DROP CONSTRAINT chat_policies_scope_chk,
  ADD CONSTRAINT chat_policies_scope_chk CHECK (scope_type IN ('platform','tenant','school','class'));
