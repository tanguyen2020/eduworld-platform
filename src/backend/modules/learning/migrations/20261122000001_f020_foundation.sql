-- F020 — foundation of module learning (specs/020-learning-foundation: data-model §1, §3; DATABASE_DESIGN §2.5,
-- §2.17, §3.0, §3.13, §4; ADR-0028, ADR-0029, ADR-0043, ADR-0089). Applied by `migrate --module=learning` as
-- learning_migrator. Order: projections ref.* (hand-written), ent tables (entddl block, golden test
-- modules/learning/ent/ddl_test.go), then what ent cannot express: CHECKs, FKs, RLS, _live view, grants,
-- functions.

-- Projections of module identity (DATABASE_DESIGN §2.17): minimal columns, no entx mixin (no audit or
-- soft-delete columns of their own, rule 2); written only by the consumer learning.projection_identity,
-- upserted by source_version (rule 1).
CREATE TABLE ref.tenants (
  tenant_id       uuid PRIMARY KEY,
  code            varchar(64),
  tenant_type     varchar(64) NOT NULL CHECK (tenant_type IN ('school','unit','common')),
  status          varchar(64) NOT NULL CHECK (status IN ('active','disabled')),
  is_qa           boolean NOT NULL DEFAULT false,
  settings_subset jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version  bigint NOT NULL
);
CREATE INDEX ref_tenants_qa_idx ON ref.tenants (tenant_id) WHERE is_qa;

CREATE TABLE ref.provinces (
  code           varchar(64) PRIMARY KEY,
  country_code   varchar(2) NOT NULL,
  name_i18n      jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(name_i18n::text) <= 4096),
  status         varchar(64) NOT NULL CHECK (status IN ('active','inactive')),
  source_version bigint NOT NULL
);

CREATE TABLE ref.schools (
  school_id          uuid PRIMARY KEY,
  tenant_id          uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  name               varchar(255),
  name_i18n          jsonb CHECK (octet_length(name_i18n::text) <= 4096),
  province_code      varchar(64),
  province_name_i18n jsonb CHECK (octet_length(province_name_i18n::text) <= 4096),
  school_level       varchar(64),
  is_virtual         boolean NOT NULL DEFAULT false,
  status             varchar(64) NOT NULL DEFAULT 'active',
  settings_subset    jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(settings_subset::text) <= 16384),
  source_version     bigint NOT NULL
);
CREATE INDEX ref_schools_tenant_idx ON ref.schools (tenant_id);

CREATE TABLE ref.classes (
  class_id         uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id        uuid NOT NULL,
  grade_level      smallint NOT NULL CHECK (grade_level BETWEEN 1 AND 9),
  name             varchar(128) NOT NULL,
  academic_year_id uuid,
  is_virtual       boolean NOT NULL DEFAULT false,
  source_version   bigint NOT NULL
);
CREATE INDEX ref_classes_tenant_school_idx ON ref.classes (tenant_id, school_id);

CREATE TABLE ref.students (
  student_id     uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  school_id      uuid NOT NULL,
  grade_level    smallint CHECK (grade_level BETWEEN 1 AND 9),
  status         varchar(64) NOT NULL DEFAULT 'active',
  deleted_at     timestamptz,
  source_version bigint NOT NULL
);
CREATE INDEX ref_students_tenant_school_idx ON ref.students (tenant_id, school_id);

CREATE TABLE ref.enrollments (
  student_id     uuid NOT NULL,
  class_id       uuid NOT NULL,
  start_date     date NOT NULL,
  tenant_id      uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  status         varchar(64) NOT NULL CHECK (status IN ('active','transferred','ended')),
  end_date       date,
  source_version bigint NOT NULL,
  PRIMARY KEY (student_id, class_id, start_date)
);
CREATE INDEX ref_enrollments_class_idx ON ref.enrollments (tenant_id, class_id) WHERE status = 'active';

-- RLS of the tenant projections: tenant condition only (ADR-0089 §1). No FORCE: the owner-run guard function
-- ref.projection_tenant below must see the owning tenant of a row across tenants (F020-AC-04); application
-- roles are never owners, so RLS applies to them. ref.tenants and ref.provinces are not tenant-scoped.
ALTER TABLE ref.schools ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.schools FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.schools FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.schools FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.classes ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.classes FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.classes FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.classes FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.students ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.students FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE ref.enrollments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.enrollments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.enrollments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.enrollments FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- Owning tenant of a projected school/class/student whatever the session tenant (NULL when unknown): the consumer
-- refuses an event whose tenant differs from the projection row it references (F020-AC-04, MULTITENANCY §4.1).
-- Returns only a tenant id, never row data.
CREATE FUNCTION ref.projection_tenant(p_table text, p_id uuid) RETURNS uuid
  LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, ref AS $$
  SELECT CASE p_table
    WHEN 'schools' THEN (SELECT s.tenant_id FROM ref.schools s WHERE s.school_id = p_id)
    WHEN 'classes' THEN (SELECT c.tenant_id FROM ref.classes c WHERE c.class_id = p_id)
    WHEN 'students' THEN (SELECT st.tenant_id FROM ref.students st WHERE st.student_id = p_id)
  END
$$;
REVOKE ALL ON FUNCTION ref.projection_tenant(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ref.projection_tenant(text, uuid) TO learning_app, learning_platform;

-- BEGIN entddl
CREATE TABLE content.content_releases (
  release_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  item_type varchar(64) NOT NULL,
  item_id uuid NOT NULL,
  action varchar(16) NOT NULL,
  from_version_id uuid,
  to_version_id uuid,
  reason varchar(500),
  scheduled_for timestamptz,
  released_by uuid NOT NULL,
  released_at timestamptz NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (release_id)
);
CREATE UNIQUE INDEX content_releases_idem_uq ON content.content_releases (item_type, item_id, idempotency_key);
CREATE INDEX content_releases_item_idx ON content.content_releases (item_type, item_id, released_at);

CREATE TABLE content.content_transitions (
  transition_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  content_type varchar(64) NOT NULL,
  content_version_id uuid NOT NULL,
  from_status varchar(64),
  to_status varchar(64) NOT NULL,
  actor_id uuid NOT NULL,
  at timestamptz NOT NULL,
  note varchar(2000),
  PRIMARY KEY (transition_id)
);
CREATE INDEX content_transitions_version_idx ON content.content_transitions (content_version_id);

CREATE TABLE infra.tenant_resource_assignments (
  assignment_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  resource_type varchar(64) NOT NULL,
  resource_id uuid NOT NULL,
  assigned_by uuid NOT NULL,
  assigned_at timestamptz NOT NULL,
  PRIMARY KEY (assignment_id)
);
CREATE UNIQUE INDEX tra_active_uq ON infra.tenant_resource_assignments (resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX tra_tenant_idx ON infra.tenant_resource_assignments (tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL;
-- END entddl

-- content.content_releases (TA*, DATABASE_DESIGN §3.13): CHECKs of the DDL template, FK to the tenant projection.
ALTER TABLE content.content_releases
  ADD CONSTRAINT content_releases_action_ck CHECK (action IN ('publish','rollback','unpublish')),
  ADD CONSTRAINT content_releases_reason_ck CHECK (action = 'publish' OR reason IS NOT NULL),
  ADD CONSTRAINT content_releases_target_ck CHECK ((action = 'unpublish') = (to_version_id IS NULL)),
  ADD CONSTRAINT content_releases_item_type_ck CHECK (item_type IN ('curriculum','question','question_set',
    'assessment_blueprint','lesson','learning_material','learning_path','activity')),
  ADD CONSTRAINT content_releases_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- content.content_transitions (TA*, DATABASE_DESIGN §2.5, CONTENT_MODEL §6).
ALTER TABLE content.content_transitions
  ADD CONSTRAINT content_transitions_type_ck CHECK (content_type IN ('curriculum','question','question_set',
    'assessment_blueprint','lesson','learning_material','learning_path','activity')),
  ADD CONSTRAINT content_transitions_from_ck CHECK (from_status IS NULL OR from_status IN ('draft','in_review',
    'approved','published','archived','rejected')),
  ADD CONSTRAINT content_transitions_to_ck CHECK (to_status IN ('draft','in_review','approved','published',
    'archived','rejected')),
  ADD CONSTRAINT content_transitions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- Append-only (TA*): application roles only INSERT and SELECT (no UPDATE; DELETE/TRUNCATE never granted).
REVOKE UPDATE ON content.content_releases, content.content_transitions FROM learning_app, learning_platform;

-- RLS TA*: tenant condition only; rows with tenant_id NULL are written by learning_platform (BYPASSRLS) under
-- PlatformScope only, so learning_app can neither read nor write them (ADR-0089 §1–2, F020-AC-32).
ALTER TABLE content.content_releases ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.content_releases FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.content_releases FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.content_releases FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.content_transitions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.content_transitions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON content.content_transitions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON content.content_transitions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());

-- infra.tenant_resource_assignments (DATABASE_DESIGN §3.0, ADR-0029): resource types owned by module learning,
-- tenant from the projection. No RLS (schema infra, ADR-0089 §4); learning_app only reads it (H policies, entx
-- hybrid filter); System Admin writes it through learning_platform.
ALTER TABLE infra.tenant_resource_assignments
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('curriculum','lesson','question','question_set',
    'assessment_blueprint','media','assignment','course','learning_path')),
  ADD CONSTRAINT tra_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);
REVOKE INSERT, UPDATE ON infra.tenant_resource_assignments FROM learning_app;
GRANT SELECT, INSERT, UPDATE ON infra.tenant_resource_assignments TO learning_platform;
-- Second layer for deleted rows (ADR-0089 §1): learning_readonly reads the live view only, never the table.
CREATE OR REPLACE VIEW infra.tenant_resource_assignments_live WITH (security_barrier = true) AS
  SELECT * FROM infra.tenant_resource_assignments WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA infra TO learning_readonly;
GRANT SELECT ON infra.tenant_resource_assignments_live TO learning_readonly;
REVOKE ALL ON infra.tenant_resource_assignments FROM learning_readonly;

-- Immutability of published content (DATABASE_DESIGN §3.3, CONTENT_MODEL §5 rule 1): a published or archived
-- version is never updated, except published → archived changing only status and the bookkeeping columns.
-- Generic over the version tables: F021/F022/F024 attach it with
--   CREATE TRIGGER <table>_immutable BEFORE UPDATE ON <schema>.<table> FOR EACH ROW
--     EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.forbid_published_update() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  bookkeeping text[] := ARRAY['status', 'updated_at', 'updated_by', 'version'];
BEGIN
  IF OLD.status IN ('published', 'archived') AND NOT (
       OLD.status = 'published' AND NEW.status = 'archived'
       AND (to_jsonb(NEW) - bookkeeping) = (to_jsonb(OLD) - bookkeeping)) THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
