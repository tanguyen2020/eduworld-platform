-- ROLLBACK of 20261231000004_adr0095_drop_enrollments.sql (module competition, ADR-0095 mục 8 bước 4).
-- Written in advance, NEVER applied automatically. Recreates ref.enrollments EMPTY with the DDL of the applied migration
-- 20261229000001_f100_party.sql (copied verbatim) and widens the party CHECKs again (the expand migration
-- 20261231000002 left parties_type_chk with both study_group and study_party). The rows come back from the dump:
--   scripts/db/adr-0095-backup.sh competition --restore-data <dump>      (pg_restore --data-only --table=enrollments)
-- Run as the table owner so the default privileges of the baseline apply, in one transaction:
--   psql -v ON_ERROR_STOP=1 --single-transaction -U competition_migrator -d eduworld_competition -f scripts/db/rollback/adr-0095-competition.sql
-- Code since WP-C1 never reads the projection: rolling back the structure restores the data for audit/analysis only.
-- Verified by tests/integration/adr0095_rollback_test.go (rollback → re-apply the contract migration).

CREATE TABLE ref.enrollments (
  student_id     uuid NOT NULL,
  class_id       uuid NOT NULL,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id      uuid,
  status         varchar(64) NOT NULL CHECK (status IN ('active','transferred','ended')),
  start_date     date,
  end_date       date,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, class_id)
);
CREATE INDEX ref_enrollments_active_idx ON ref.enrollments (tenant_id, student_id) WHERE status = 'active';

ALTER TABLE ref.enrollments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

ALTER TABLE multiplayer.parties DROP CONSTRAINT parties_type_chk,
  ADD CONSTRAINT parties_type_chk CHECK (party_type IN ('friend','study_party','study_group','quest','competition_team','custom'));
ALTER TABLE multiplayer.parties DROP CONSTRAINT parties_visibility_chk,
  ADD CONSTRAINT parties_visibility_chk CHECK (visibility IN ('private','friends','class'));
ALTER TABLE multiplayer.party_policies DROP CONSTRAINT party_policies_scope_chk,
  ADD CONSTRAINT party_policies_scope_chk CHECK (scope_type IN ('school','class'));
