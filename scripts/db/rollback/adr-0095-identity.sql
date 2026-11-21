-- ROLLBACK of 20261201000001_adr0095_drop_class_enrollment.sql (module identity, ADR-0095 mục 8 bước 4).
-- Written in advance, NEVER applied automatically. Recreates the dropped structures EMPTY, exactly as the applied
-- migrations left them (20261119000001_f010_org.sql; the grant of 20261122000001 was revoked by 20261130000002), and
-- widens the SIS CHECKs again. The rows come back from the dump taken before the migration:
--   scripts/db/adr-0095-backup.sh identity --restore-data <dump>      (pg_restore --data-only --table=classes,enrollments)
-- Run as the table owner so the default privileges of the baseline apply, in one transaction:
--   psql -v ON_ERROR_STOP=1 --single-transaction -U identity_migrator -d eduworld_identity -f scripts/db/rollback/adr-0095-identity.sql
-- Code since WP-I1 never reads these tables: rolling back the structure restores the data for audit/analysis only.
-- Verified by tests/integration/adr0095_rollback_test.go (rollback → re-apply the contract migration).

CREATE TABLE org.classes (
  class_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  academic_year_id uuid NOT NULL,
  grade_level smallint NOT NULL,
  name varchar(128) NOT NULL,
  is_virtual boolean NOT NULL DEFAULT false,
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (class_id)
);
CREATE UNIQUE INDEX classes_class_school_uq ON org.classes (class_id, school_id);
CREATE UNIQUE INDEX classes_name_uq ON org.classes (school_id, academic_year_id, name) WHERE deleted_at IS NULL;
CREATE INDEX classes_school_grade_idx ON org.classes (school_id, grade_level);

CREATE TABLE org.enrollments (
  enrollment_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  class_id uuid NOT NULL,
  status varchar(64) NOT NULL,
  start_date date NOT NULL,
  end_date date,
  source varchar(64) NOT NULL DEFAULT 'sis',
  PRIMARY KEY (enrollment_id)
);
CREATE INDEX enrollments_class_idx ON org.enrollments (tenant_id, class_id, status) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX enrollments_one_active_uq ON org.enrollments (student_id) WHERE status = 'active' AND deleted_at IS NULL;

ALTER TABLE org.classes
  ADD CONSTRAINT classes_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT classes_status_chk CHECK (status IN ('active','inactive')),
  ADD CONSTRAINT classes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT classes_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT classes_year_fk FOREIGN KEY (academic_year_id) REFERENCES org.academic_years (academic_year_id);
ALTER TABLE org.enrollments
  ADD CONSTRAINT enrollments_status_chk CHECK (status IN ('active','transferred','ended')),
  ADD CONSTRAINT enrollments_period_chk CHECK (end_date IS NULL OR end_date >= start_date),
  ADD CONSTRAINT enrollments_closed_chk CHECK ((status = 'active') = (end_date IS NULL)),
  ADD CONSTRAINT enrollments_source_chk CHECK (source IN ('sis','manual','platform_common')),
  ADD CONSTRAINT enrollments_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT enrollments_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT enrollments_student_fk FOREIGN KEY (student_id) REFERENCES org.students (student_id),
  ADD CONSTRAINT enrollments_class_school_fk FOREIGN KEY (class_id, school_id) REFERENCES org.classes (class_id, school_id);

ALTER TABLE org.classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.classes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.classes FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE org.enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.enrollments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

REVOKE DELETE, TRUNCATE ON org.classes, org.enrollments FROM identity_app, identity_platform;

CREATE VIEW org.classes_live WITH (security_barrier = true) AS
  SELECT * FROM org.classes WHERE deleted_at IS NULL;
GRANT SELECT ON org.classes_live TO identity_readonly;
CREATE VIEW org.enrollments_live WITH (security_barrier = true) AS
  SELECT * FROM org.enrollments WHERE deleted_at IS NULL;
GRANT SELECT ON org.enrollments_live TO identity_readonly;

-- The SIS CHECKs accept the old kinds again (rows restored from the dump may carry them).
ALTER TABLE sis.sis_links DROP CONSTRAINT sis_links_type_chk;
ALTER TABLE sis.sis_links ADD CONSTRAINT sis_links_type_chk CHECK (entity_type IN ('school','class','enrollment'));
ALTER TABLE sis.staging_records DROP CONSTRAINT staging_records_entity_kind_check;
ALTER TABLE sis.staging_records ADD CONSTRAINT staging_records_entity_kind_check
  CHECK (entity_kind IN ('school','class','student','enrollment'));
