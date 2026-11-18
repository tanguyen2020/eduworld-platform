-- F010 T005/T006 — tenancy, catalog, organization and users of module identity (DATABASE_DESIGN §2.0–§2.2a,
-- DDL §3.0–§3.2, ADR-0005, ADR-0060, ADR-0081, ADR-0089). The block between the entddl markers is rendered
-- from the ent schema (entx.DDL, golden test modules/identity/ent/ddl_test.go); the rest is hand-written:
-- CHECK constraints, foreign keys (composite enrollments → classes), expression indexes, RLS (tenant only,
-- no DELETE policy), <table>_live views and identity.tenant_memberships_v. Applied as identity_migrator.

-- BEGIN entddl
CREATE TABLE org.academic_years (
  academic_year_id uuid NOT NULL,
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
  name varchar(32) NOT NULL,
  start_date date NOT NULL,
  end_date date NOT NULL,
  is_current boolean NOT NULL DEFAULT false,
  PRIMARY KEY (academic_year_id)
);
CREATE UNIQUE INDEX academic_years_current_uq ON org.academic_years (school_id) WHERE is_current AND deleted_at IS NULL;

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

CREATE TABLE catalog.countries (
  country_code varchar(2) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  name_i18n jsonb NOT NULL,
  default_locale varchar(16) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (country_code)
);

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

CREATE TABLE identity.external_identities (
  external_identity_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  issuer varchar(512) NOT NULL,
  subject varchar(255) NOT NULL,
  user_id uuid NOT NULL,
  idp_alias varchar(64),
  status varchar(64) NOT NULL DEFAULT 'active',
  superseded_at timestamptz,
  superseded_by uuid,
  last_login_at timestamptz,
  PRIMARY KEY (external_identity_id)
);
CREATE UNIQUE INDEX external_identities_active_uq ON identity.external_identities (user_id, issuer) WHERE status = 'active' AND deleted_at IS NULL;
CREATE UNIQUE INDEX external_identities_iss_sub_uq ON identity.external_identities (issuer, subject) WHERE deleted_at IS NULL;
CREATE INDEX external_identities_user_idx ON identity.external_identities (user_id);

CREATE TABLE catalog.provinces (
  province_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  country_code varchar(2) NOT NULL,
  code varchar(64) NOT NULL,
  official_code varchar(16),
  name_i18n jsonb NOT NULL,
  short_name_i18n jsonb,
  province_type varchar(64) NOT NULL,
  effective_from date,
  effective_to date,
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (province_id)
);
CREATE UNIQUE INDEX provinces_code_uq ON catalog.provinces (code);
CREATE INDEX provinces_country_status_idx ON catalog.provinces (country_code, status);

CREATE TABLE catalog.province_successions (
  old_province_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  new_province_id uuid NOT NULL,
  effective_date date NOT NULL,
  kind varchar(64) NOT NULL,
  PRIMARY KEY (old_province_id)
);

CREATE TABLE org.schools (
  school_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name varchar(255) NOT NULL,
  name_i18n jsonb,
  country_code varchar(2) NOT NULL,
  province_id uuid,
  moet_code varchar(32),
  school_level varchar(64) NOT NULL,
  grade_min smallint NOT NULL,
  grade_max smallint NOT NULL,
  address varchar(500),
  status varchar(64) NOT NULL DEFAULT 'active',
  is_virtual boolean NOT NULL DEFAULT false,
  login_code varchar(16),
  settings jsonb NOT NULL,
  PRIMARY KEY (school_id)
);
CREATE UNIQUE INDEX schools_code_uq ON org.schools (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX schools_login_code_uq ON org.schools (login_code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX schools_moet_code_uq ON org.schools (moet_code) WHERE moet_code IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX schools_province_level_idx ON org.schools (province_id, school_level);
CREATE INDEX schools_tenant_status_idx ON org.schools (tenant_id, status) WHERE deleted_at IS NULL;

CREATE TABLE org.students (
  student_id uuid NOT NULL,
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
  grade_level smallint NOT NULL,
  student_code varchar(32),
  full_name varchar(128) NOT NULL,
  date_of_birth_enc bytea,
  national_id_hash varchar(64),
  status varchar(64) NOT NULL DEFAULT 'active',
  PRIMARY KEY (student_id)
);
CREATE UNIQUE INDEX students_code_uq ON org.students (school_id, student_code) WHERE student_code IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE platform.tenants (
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name varchar(255) NOT NULL,
  tenant_type varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'active',
  keycloak_org_id uuid,
  keycloak_org_alias varchar(64),
  login_code varchar(16),
  settings jsonb NOT NULL,
  is_qa boolean NOT NULL DEFAULT false,
  PRIMARY KEY (tenant_id)
);
CREATE UNIQUE INDEX tenants_code_uq ON platform.tenants (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_kc_alias_uq ON platform.tenants (keycloak_org_alias) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_kc_org_uq ON platform.tenants (keycloak_org_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_login_code_uq ON platform.tenants (login_code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_single_common_uq ON platform.tenants (tenant_type) WHERE tenant_type = 'common';

CREATE TABLE identity.users (
  user_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_type varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'pending',
  username varchar(128) NOT NULL,
  email varchar(320),
  display_name varchar(128),
  locale varchar(64) NOT NULL DEFAULT 'vi-VN',
  primary_tenant_id uuid,
  primary_school_id uuid,
  deactivated_at timestamptz,
  anonymized_at timestamptz,
  provisioned_at timestamptz NOT NULL,
  last_login_at timestamptz,
  PRIMARY KEY (user_id)
);
CREATE INDEX users_tenant_type_idx ON identity.users (primary_tenant_id, user_type) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- CHECK constraints (DDL §3.0–§3.2) ----------
ALTER TABLE platform.tenants
  ADD CONSTRAINT tenants_type_chk CHECK (tenant_type IN ('school','unit','common')),
  ADD CONSTRAINT tenants_status_chk CHECK (status IN ('active','disabled')),
  ADD CONSTRAINT tenants_alias_chk CHECK (keycloak_org_alias ~ '^[a-z0-9-]{3,63}$'),
  ADD CONSTRAINT tenants_login_code_chk CHECK (login_code ~ '^[A-HJ-NP-Z2-9]{6}$'),
  ADD CONSTRAINT tenants_qa_chk CHECK (NOT (is_qa AND tenant_type = 'common'));
ALTER TABLE catalog.countries
  ADD CONSTRAINT countries_code_chk CHECK (country_code ~ '^[A-Z]{2}$'),
  ADD CONSTRAINT countries_status_chk CHECK (status IN ('active','inactive'));
ALTER TABLE catalog.provinces
  ADD CONSTRAINT provinces_status_chk CHECK (status IN ('active','inactive')),
  ADD CONSTRAINT provinces_type_chk CHECK (province_type IN ('municipality','province')),
  ADD CONSTRAINT provinces_effective_chk CHECK ((status = 'inactive') = (effective_to IS NOT NULL)),
  ADD CONSTRAINT provinces_period_chk CHECK (effective_to IS NULL OR effective_to >= effective_from);
ALTER TABLE catalog.province_successions
  ADD CONSTRAINT province_successions_kind_chk CHECK (kind IN ('merged','unchanged','renamed','split')),
  ADD CONSTRAINT province_successions_self_chk CHECK (old_province_id <> new_province_id);
ALTER TABLE org.schools
  ADD CONSTRAINT schools_level_chk CHECK (school_level IN ('primary','lower_secondary','multi_level')),
  ADD CONSTRAINT schools_grades_chk CHECK (grade_min BETWEEN 1 AND 9 AND grade_max BETWEEN 1 AND 9 AND grade_min <= grade_max),
  ADD CONSTRAINT schools_status_chk CHECK (status IN ('active','pending_review','disabled')),
  ADD CONSTRAINT schools_province_chk CHECK (is_virtual OR province_id IS NOT NULL OR status = 'pending_review');
ALTER TABLE org.academic_years
  ADD CONSTRAINT academic_years_period_chk CHECK (end_date > start_date);
ALTER TABLE org.classes
  ADD CONSTRAINT classes_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT classes_status_chk CHECK (status IN ('active','inactive'));
ALTER TABLE org.students
  ADD CONSTRAINT students_grade_chk CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT students_status_chk CHECK (status IN ('active','inactive'));
ALTER TABLE org.enrollments
  ADD CONSTRAINT enrollments_status_chk CHECK (status IN ('active','transferred','ended')),
  ADD CONSTRAINT enrollments_period_chk CHECK (end_date IS NULL OR end_date >= start_date),
  ADD CONSTRAINT enrollments_closed_chk CHECK ((status = 'active') = (end_date IS NULL)),
  ADD CONSTRAINT enrollments_source_chk CHECK (source IN ('sis','manual','platform_common'));
ALTER TABLE identity.users
  ADD CONSTRAINT users_type_chk CHECK (user_type IN ('student','staff')),
  ADD CONSTRAINT users_status_chk CHECK (status IN ('pending','active','locked','deactivated','anonymized')),
  ADD CONSTRAINT users_display_name_chk CHECK (status = 'anonymized' OR display_name IS NOT NULL),
  ADD CONSTRAINT users_username_chk CHECK (username = lower(btrim(username))),
  ADD CONSTRAINT users_staff_email_chk CHECK (user_type = 'student' OR status = 'anonymized' OR email IS NOT NULL),
  ADD CONSTRAINT users_student_tenant_chk CHECK (user_type = 'staff' OR primary_tenant_id IS NOT NULL);
ALTER TABLE identity.external_identities
  ADD CONSTRAINT external_identities_status_chk CHECK (status IN ('active','superseded')),
  ADD CONSTRAINT external_identities_superseded_chk CHECK ((status = 'superseded') = (superseded_at IS NOT NULL));

-- ---------- Foreign keys ----------
ALTER TABLE catalog.provinces ADD CONSTRAINT provinces_country_fk FOREIGN KEY (country_code) REFERENCES catalog.countries (country_code);
ALTER TABLE catalog.province_successions
  ADD CONSTRAINT province_successions_old_fk FOREIGN KEY (old_province_id) REFERENCES catalog.provinces (province_id),
  ADD CONSTRAINT province_successions_new_fk FOREIGN KEY (new_province_id) REFERENCES catalog.provinces (province_id);
ALTER TABLE org.schools
  ADD CONSTRAINT schools_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT schools_country_fk FOREIGN KEY (country_code) REFERENCES catalog.countries (country_code),
  ADD CONSTRAINT schools_province_fk FOREIGN KEY (province_id) REFERENCES catalog.provinces (province_id);
ALTER TABLE org.academic_years
  ADD CONSTRAINT academic_years_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT academic_years_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id);
ALTER TABLE org.classes
  ADD CONSTRAINT classes_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT classes_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT classes_year_fk FOREIGN KEY (academic_year_id) REFERENCES org.academic_years (academic_year_id);
ALTER TABLE org.students
  ADD CONSTRAINT students_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT students_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT students_user_fk FOREIGN KEY (student_id) REFERENCES identity.users (user_id);
ALTER TABLE org.enrollments
  ADD CONSTRAINT enrollments_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id),
  ADD CONSTRAINT enrollments_school_fk FOREIGN KEY (school_id) REFERENCES org.schools (school_id),
  ADD CONSTRAINT enrollments_student_fk FOREIGN KEY (student_id) REFERENCES org.students (student_id),
  -- The class belongs to the school of the enrollment (ADR-0060, F010-AC-12).
  ADD CONSTRAINT enrollments_class_school_fk FOREIGN KEY (class_id, school_id) REFERENCES org.classes (class_id, school_id);
ALTER TABLE identity.users ADD CONSTRAINT users_primary_tenant_fk FOREIGN KEY (primary_tenant_id) REFERENCES platform.tenants (tenant_id);
ALTER TABLE identity.external_identities
  ADD CONSTRAINT external_identities_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id),
  ADD CONSTRAINT external_identities_superseded_by_fk FOREIGN KEY (superseded_by) REFERENCES identity.external_identities (external_identity_id);

-- ---------- Expression and conditional indexes ent cannot express ----------
-- Username is unique on every row, soft-deleted included, until anonymization frees it (ADR-0067, ADR-0069).
CREATE UNIQUE INDEX users_username_uq ON identity.users (lower(username));
CREATE UNIQUE INDEX users_email_uq ON identity.users (lower(email)) WHERE email IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX users_cleanup_idx ON identity.users (primary_tenant_id, (COALESCE(last_login_at, provisioned_at)))
  WHERE user_type = 'student' AND deleted_at IS NULL;
CREATE UNIQUE INDEX provinces_official_code_uq ON catalog.provinces (country_code, official_code) WHERE official_code IS NOT NULL;
CREATE INDEX students_school_idx ON org.students (tenant_id, school_id) WHERE deleted_at IS NULL;

-- ---------- Row-Level Security: T tables, tenant condition only (DATABASE_DESIGN §4, ADR-0089 §1) ----------
ALTER TABLE org.schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.schools FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE org.academic_years ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.academic_years FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.academic_years FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.academic_years FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.academic_years FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE org.classes ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.classes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.classes FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE org.students ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.students FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE org.enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE org.enrollments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON org.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON org.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON org.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Soft-delete only (ADR-0028) ----------
REVOKE DELETE, TRUNCATE ON platform.tenants, catalog.countries, catalog.provinces, catalog.province_successions, org.schools, org.academic_years, org.classes, org.students, org.enrollments, identity.users, identity.external_identities FROM identity_app, identity_platform;

-- ---------- <table>_live views for identity_readonly (ADR-0089 §1) ----------
CREATE OR REPLACE VIEW platform.tenants_live WITH (security_barrier = true) AS
  SELECT * FROM platform.tenants WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA platform TO identity_readonly;
GRANT SELECT ON platform.tenants_live TO identity_readonly;
CREATE OR REPLACE VIEW catalog.countries_live WITH (security_barrier = true) AS
  SELECT * FROM catalog.countries WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA catalog TO identity_readonly;
GRANT SELECT ON catalog.countries_live TO identity_readonly;
CREATE OR REPLACE VIEW catalog.provinces_live WITH (security_barrier = true) AS
  SELECT * FROM catalog.provinces WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA catalog TO identity_readonly;
GRANT SELECT ON catalog.provinces_live TO identity_readonly;
CREATE OR REPLACE VIEW catalog.province_successions_live WITH (security_barrier = true) AS
  SELECT * FROM catalog.province_successions WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA catalog TO identity_readonly;
GRANT SELECT ON catalog.province_successions_live TO identity_readonly;
CREATE OR REPLACE VIEW org.schools_live WITH (security_barrier = true) AS
  SELECT * FROM org.schools WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA org TO identity_readonly;
GRANT SELECT ON org.schools_live TO identity_readonly;
CREATE OR REPLACE VIEW org.academic_years_live WITH (security_barrier = true) AS
  SELECT * FROM org.academic_years WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA org TO identity_readonly;
GRANT SELECT ON org.academic_years_live TO identity_readonly;
CREATE OR REPLACE VIEW org.classes_live WITH (security_barrier = true) AS
  SELECT * FROM org.classes WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA org TO identity_readonly;
GRANT SELECT ON org.classes_live TO identity_readonly;
CREATE OR REPLACE VIEW org.students_live WITH (security_barrier = true) AS
  SELECT * FROM org.students WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA org TO identity_readonly;
GRANT SELECT ON org.students_live TO identity_readonly;
CREATE OR REPLACE VIEW org.enrollments_live WITH (security_barrier = true) AS
  SELECT * FROM org.enrollments WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA org TO identity_readonly;
GRANT SELECT ON org.enrollments_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.users_live WITH (security_barrier = true) AS
  SELECT * FROM identity.users WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA identity TO identity_readonly;
GRANT SELECT ON identity.users_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.external_identities_live WITH (security_barrier = true) AS
  SELECT * FROM identity.external_identities WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA identity TO identity_readonly;
GRANT SELECT ON identity.external_identities_live TO identity_readonly;

-- ---------- Tenant membership (DATABASE_DESIGN §2.1): students from their active enrollment. F012 replaces the
-- view to add staff from identity.user_roles (CREATE OR REPLACE, same columns). ----------
-- security_invoker: RLS of org.enrollments applies to the caller (identity_app: its tenant; identity_platform:
-- BYPASSRLS for the login path and jobs), not to the view owner.
CREATE OR REPLACE VIEW identity.tenant_memberships_v WITH (security_invoker = true, security_barrier = true) AS
  SELECT e.tenant_id, e.student_id AS user_id, 'student'::varchar(64) AS member_kind, e.school_id
  FROM org.enrollments e
  WHERE e.status = 'active' AND e.deleted_at IS NULL;
GRANT SELECT ON identity.tenant_memberships_v TO identity_app, identity_platform;
