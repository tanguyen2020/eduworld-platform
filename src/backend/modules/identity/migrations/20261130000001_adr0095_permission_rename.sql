-- ADR-0095 mục 6 (WP-I1): the class permissions are renamed or merged, in the same commit as the catalog
-- (src/backend/pkg/authz/catalog.go, roles.go, ADMIN_PORTAL §2.2; every binary checks the catalog at startup):
--   tenant.class.read    → tenant.student.read
--   lms.class.read       → lms.student.read
--   analytics.class.read → analytics.school.read (merged)
-- The catalog sync (seed.SyncAuthz, cmd/migrate) writes the new codes and the grants of the system roles; this
-- migration adds the new codes to identity.permissions first (role_permissions.permission_code is a foreign key) and
-- moves the grants held by the custom roles of tenants, which the sync does not touch. Soft-delete only (AGENTS R21):
-- the old rows keep their history with delete_reason. Applied as identity_migrator (policy migrator_seed; trigger
-- role_permissions_scope_chk allows the migrator).

INSERT INTO identity.permissions (permission_code, created_at, updated_at, scope_kind, group_code, name_vi, name_en,
                                  catalog_version, synced_at)
SELECT m.new_code, now(), now(), p.scope_kind, p.group_code, m.new_code, m.new_code, p.catalog_version, now()
  FROM (VALUES ('tenant.class.read', 'tenant.student.read'),
               ('lms.class.read', 'lms.student.read'),
               ('analytics.class.read', 'analytics.school.read')) AS m(old_code, new_code)
  JOIN identity.permissions p ON p.permission_code = m.old_code
ON CONFLICT (permission_code) DO UPDATE SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now();

-- Custom roles of tenants (is_system = false): the grant moves to the new code unless the role already holds it.
INSERT INTO identity.role_permissions (role_permission_id, tenant_id, created_at, updated_at, role_id, permission_code)
SELECT DISTINCT ON (rp.role_id, m.new_code) gen_random_uuid(), rp.tenant_id, now(), now(), rp.role_id, m.new_code
  FROM identity.role_permissions rp
  JOIN identity.roles r ON r.role_id = rp.role_id AND r.is_system = false
  JOIN (VALUES ('tenant.class.read', 'tenant.student.read'),
               ('lms.class.read', 'lms.student.read'),
               ('analytics.class.read', 'analytics.school.read')) AS m(old_code, new_code) ON m.old_code = rp.permission_code
 WHERE rp.deleted_at IS NULL
   AND NOT EXISTS (SELECT 1 FROM identity.role_permissions x
                    WHERE x.role_id = rp.role_id AND x.permission_code = m.new_code AND x.deleted_at IS NULL);

UPDATE identity.role_permissions
   SET deleted_at = now(), delete_reason = 'permission_renamed', updated_at = now()
 WHERE deleted_at IS NULL
   AND permission_code IN ('tenant.class.read', 'lms.class.read', 'analytics.class.read');

UPDATE identity.permissions
   SET deleted_at = now(), delete_reason = 'catalog_removed', updated_at = now()
 WHERE deleted_at IS NULL
   AND permission_code IN ('tenant.class.read', 'lms.class.read', 'analytics.class.read');
