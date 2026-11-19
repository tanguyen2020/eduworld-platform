-- F012 T005/T006 — authorization tables of module identity (DATABASE_DESIGN §3.1, §3.15, ADR-0066, ADR-0072,
-- ADR-0081, ADR-0091). The entddl block is rendered from the ent schema (golden test ent/ddl_test.go). Hand-written:
-- CHECKs, foreign keys, the two-layer scope triggers (layer 2 after the use cases), protection of system roles,
-- RLS (shared roles readable by every tenant), the staff branch of identity.tenant_memberships_v, <table>_live views.
-- identity.permissions and the system roles are written only by identity_migrator (cmd/migrate seed). The composite
-- key of role_permissions is a surrogate id plus a unique index. Applied as identity_migrator.

-- BEGIN entddl
CREATE TABLE identity.permissions (
  permission_code varchar(64) NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  scope_kind varchar(64) NOT NULL,
  group_code varchar(64) NOT NULL,
  name_vi varchar(255) NOT NULL,
  name_en varchar(255) NOT NULL,
  catalog_version varchar(64) NOT NULL,
  synced_at timestamptz NOT NULL,
  PRIMARY KEY (permission_code)
);

CREATE TABLE identity.portal_password_reset_tokens (
  reset_token_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_id uuid NOT NULL,
  token_hash varchar(64) NOT NULL,
  locale varchar(8) NOT NULL DEFAULT 'vi',
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  requested_device_hash varchar(64),
  PRIMARY KEY (reset_token_id)
);
CREATE UNIQUE INDEX portal_password_reset_tokens_active_uq ON identity.portal_password_reset_tokens (user_id) WHERE used_at IS NULL AND deleted_at IS NULL;
CREATE INDEX portal_password_reset_tokens_expires_idx ON identity.portal_password_reset_tokens (expires_at);
CREATE UNIQUE INDEX portal_password_reset_tokens_hash_uq ON identity.portal_password_reset_tokens (token_hash);

CREATE TABLE identity.role_permissions (
  role_permission_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  role_id uuid NOT NULL,
  permission_code varchar(64) NOT NULL,
  PRIMARY KEY (role_permission_id)
);
CREATE UNIQUE INDEX role_permissions_uq ON identity.role_permissions (role_id, permission_code) WHERE deleted_at IS NULL;

CREATE TABLE identity.roles (
  role_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  code varchar(64) NOT NULL,
  name_vi varchar(255) NOT NULL,
  name_en varchar(255) NOT NULL,
  description varchar(1000),
  role_scope_kind varchar(64) NOT NULL,
  is_system boolean NOT NULL DEFAULT false,
  PRIMARY KEY (role_id)
);
CREATE UNIQUE INDEX roles_shared_code_uq ON identity.roles (code) WHERE tenant_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX roles_tenant_code_uq ON identity.roles (tenant_id, code) WHERE tenant_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE identity.user_roles (
  user_role_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  user_id uuid NOT NULL,
  role_id uuid NOT NULL,
  scope_type varchar(64) NOT NULL,
  scope_id uuid,
  school_id uuid,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz,
  granted_by uuid NOT NULL,
  source varchar(64) NOT NULL DEFAULT 'manual',
  training_completed_at timestamptz,
  PRIMARY KEY (user_role_id)
);
CREATE UNIQUE INDEX user_roles_active_uq ON identity.user_roles (user_id, role_id, scope_type, scope_id) WHERE deleted_at IS NULL AND valid_to IS NULL;
CREATE INDEX user_roles_tenant_role_idx ON identity.user_roles (tenant_id, role_id) WHERE deleted_at IS NULL;
CREATE INDEX user_roles_user_tenant_idx ON identity.user_roles (user_id, tenant_id) WHERE deleted_at IS NULL;
-- END entddl

-- ---------- CHECK constraints ----------
ALTER TABLE identity.permissions
  ADD CONSTRAINT permissions_scope_kind_chk CHECK (scope_kind IN ('tenant','platform')),
  -- platform-only prefixes are always platform permissions (AGENTS R20)
  ADD CONSTRAINT permissions_platform_prefix_chk CHECK (scope_kind = 'platform' OR NOT (permission_code LIKE 'game.%'
    OR permission_code LIKE 'competition.config.%' OR permission_code LIKE 'platform.%'
    OR permission_code LIKE 'moderation.case.%' OR permission_code LIKE 'moderation.sanction.%'));
ALTER TABLE identity.roles
  ADD CONSTRAINT roles_scope_kind_chk CHECK (role_scope_kind IN ('tenant','platform')),
  ADD CONSTRAINT roles_custom_tenant_chk CHECK (role_scope_kind = 'tenant' OR tenant_id IS NULL),
  ADD CONSTRAINT roles_system_shared_chk CHECK (NOT is_system OR tenant_id IS NULL);
ALTER TABLE identity.user_roles
  ADD CONSTRAINT user_roles_scope_type_chk CHECK (scope_type IN ('platform','tenant','school','subject')),  -- ADR-0072: no class
  ADD CONSTRAINT user_roles_platform_tenant_chk CHECK ((scope_type = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT user_roles_validity_chk CHECK (valid_to IS NULL OR valid_to > valid_from),
  ADD CONSTRAINT user_roles_source_chk CHECK (source IN ('manual'));  -- SIS never grants roles (ADR-0072)

-- ---------- Foreign keys ----------
ALTER TABLE identity.roles ADD CONSTRAINT roles_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);
ALTER TABLE identity.role_permissions
  ADD CONSTRAINT role_permissions_role_fk FOREIGN KEY (role_id) REFERENCES identity.roles (role_id),
  ADD CONSTRAINT role_permissions_permission_fk FOREIGN KEY (permission_code) REFERENCES identity.permissions (permission_code);
ALTER TABLE identity.user_roles
  ADD CONSTRAINT user_roles_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id),
  ADD CONSTRAINT user_roles_role_fk FOREIGN KEY (role_id) REFERENCES identity.roles (role_id),
  ADD CONSTRAINT user_roles_tenant_fk FOREIGN KEY (tenant_id) REFERENCES platform.tenants (tenant_id);
ALTER TABLE identity.portal_password_reset_tokens
  ADD CONSTRAINT portal_password_reset_tokens_user_fk FOREIGN KEY (user_id) REFERENCES identity.users (user_id);

-- ---------- Layer 2 after CreateRole/UpdateRole: a tenant role never holds a platform permission ----------
CREATE FUNCTION identity.enforce_role_permission_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE rkind text; rsystem boolean; pkind text;
BEGIN
  SELECT role_scope_kind, is_system INTO rkind, rsystem FROM identity.roles WHERE role_id = NEW.role_id;
  SELECT scope_kind INTO pkind FROM identity.permissions WHERE permission_code = NEW.permission_code;
  IF rkind = 'tenant' AND pkind = 'platform' THEN
    RAISE EXCEPTION 'tenant role % must not hold platform permission %', NEW.role_id, NEW.permission_code
      USING ERRCODE = 'check_violation';
  END IF;
  IF rsystem AND current_user <> 'identity_migrator' THEN
    RAISE EXCEPTION 'permissions of system role % are written by the catalog seed only', NEW.role_id
      USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER role_permissions_scope_chk
  BEFORE INSERT OR UPDATE ON identity.role_permissions
  FOR EACH ROW EXECUTE FUNCTION identity.enforce_role_permission_scope();

-- ---------- System roles are read-only outside the catalog seed (F012-AC-17, layer 2) ----------
CREATE FUNCTION identity.protect_system_roles() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.is_system AND current_user <> 'identity_migrator' THEN
    RAISE EXCEPTION 'system role % is read-only', OLD.code USING ERRCODE = 'insufficient_privilege';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER roles_system_readonly
  BEFORE UPDATE ON identity.roles
  FOR EACH ROW EXECUTE FUNCTION identity.protect_system_roles();

-- ---------- Layer 2 after GrantUserRole (ADR-0066, ADR-0072, ADR-0081) ----------
CREATE FUNCTION identity.enforce_user_role_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE r identity.roles%ROWTYPE; utype text; uprimary uuid;
BEGIN
  SELECT * INTO r FROM identity.roles WHERE role_id = NEW.role_id AND deleted_at IS NULL;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'unknown role %', NEW.role_id USING ERRCODE = 'check_violation';
  END IF;
  IF r.role_scope_kind = 'platform' AND NEW.scope_type <> 'platform' THEN
    RAISE EXCEPTION 'platform role % cannot be assigned at scope %', r.code, NEW.scope_type
      USING ERRCODE = 'check_violation';
  END IF;
  IF r.role_scope_kind = 'tenant' AND NEW.scope_type = 'platform' THEN
    RAISE EXCEPTION 'tenant role % cannot be assigned at platform scope', r.code USING ERRCODE = 'check_violation';
  END IF;
  IF r.tenant_id IS NOT NULL AND r.tenant_id IS DISTINCT FROM NEW.tenant_id THEN
    RAISE EXCEPTION 'custom role % belongs to another tenant', r.code USING ERRCODE = 'check_violation';
  END IF;
  SELECT user_type, primary_tenant_id INTO utype, uprimary FROM identity.users WHERE user_id = NEW.user_id;
  IF utype = 'student' THEN   -- students never hold a portal role (ADR-0066)
    RAISE EXCEPTION 'student cannot hold portal role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  -- ADR-0081: no home tenant => platform roles only; a home tenant => tenant roles only
  IF uprimary IS NULL AND r.role_scope_kind <> 'platform' THEN
    RAISE EXCEPTION 'platform staff cannot hold tenant role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  IF uprimary IS NOT NULL AND r.role_scope_kind = 'platform' THEN
    RAISE EXCEPTION 'tenant user cannot hold platform role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER user_roles_scope_chk
  BEFORE INSERT OR UPDATE OF role_id, scope_type, tenant_id, user_id ON identity.user_roles
  FOR EACH ROW EXECUTE FUNCTION identity.enforce_user_role_scope();

-- ---------- Catalog copy: written by identity_migrator only (ADR-0066) ----------
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON identity.permissions FROM identity_app, identity_platform;
REVOKE DELETE, TRUNCATE ON identity.roles, identity.role_permissions, identity.user_roles, identity.portal_password_reset_tokens
  FROM identity_app, identity_platform;

-- ---------- Row-Level Security (tenant condition only, ADR-0089 §1). Shared rows (tenant_id NULL: system and
-- shared roles) are readable in every tenant; they are written by identity_migrator (seed) or identity_platform
-- (BYPASSRLS, platform roles). Platform user roles (tenant_id NULL) are visible to identity_platform only. ----------
ALTER TABLE identity.roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.roles FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.roles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.roles FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY migrator_seed ON identity.roles TO identity_migrator USING (true) WITH CHECK (true);
ALTER TABLE identity.role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.role_permissions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.role_permissions FOR SELECT USING (tenant_id IS NULL OR tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.role_permissions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.role_permissions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY migrator_seed ON identity.role_permissions TO identity_migrator USING (true) WITH CHECK (true);
ALTER TABLE identity.user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.user_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.user_roles FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.user_roles FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON identity.user_roles FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- Tenant membership: students (F010) + staff with a valid tenant user role (F012). One row per
-- (tenant_id, user_id); same columns as F010. security_invoker: RLS of the caller applies. ----------
CREATE OR REPLACE VIEW identity.tenant_memberships_v WITH (security_invoker = true, security_barrier = true) AS
  SELECT e.tenant_id, e.student_id AS user_id, 'student'::varchar(64) AS member_kind, e.school_id
  FROM org.enrollments e
  WHERE e.status = 'active' AND e.deleted_at IS NULL
  UNION ALL
  SELECT ur.tenant_id, ur.user_id, 'staff'::varchar(64) AS member_kind, NULL::uuid AS school_id
  FROM identity.user_roles ur
  WHERE ur.tenant_id IS NOT NULL AND ur.deleted_at IS NULL
    AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())
  GROUP BY ur.tenant_id, ur.user_id;
GRANT SELECT ON identity.tenant_memberships_v TO identity_app, identity_platform;

-- ---------- <table>_live views for identity_readonly (ADR-0089 §1) ----------
CREATE OR REPLACE VIEW identity.permissions_live WITH (security_barrier = true) AS
  SELECT * FROM identity.permissions WHERE deleted_at IS NULL;
GRANT SELECT ON identity.permissions_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.roles_live WITH (security_barrier = true) AS
  SELECT * FROM identity.roles WHERE deleted_at IS NULL;
GRANT SELECT ON identity.roles_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.role_permissions_live WITH (security_barrier = true) AS
  SELECT * FROM identity.role_permissions WHERE deleted_at IS NULL;
GRANT SELECT ON identity.role_permissions_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.user_roles_live WITH (security_barrier = true) AS
  SELECT * FROM identity.user_roles WHERE deleted_at IS NULL;
GRANT SELECT ON identity.user_roles_live TO identity_readonly;
CREATE OR REPLACE VIEW identity.portal_password_reset_tokens_live WITH (security_barrier = true) AS
  SELECT * FROM identity.portal_password_reset_tokens WHERE deleted_at IS NULL;
GRANT SELECT ON identity.portal_password_reset_tokens_live TO identity_readonly;
