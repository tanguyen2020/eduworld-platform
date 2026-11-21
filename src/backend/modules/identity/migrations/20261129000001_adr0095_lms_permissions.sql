-- ADR-0095 mục 6 (WP-L1): the LMS permissions of courses, learning paths, the schedule and study groups leave the
-- catalog (src/backend/pkg/authz/catalog.go, roles.go and ADMIN_PORTAL §2.2 change in the same commit; every binary
-- checks the catalog at startup). The grants of the system roles and the catalog copy are retired again by the catalog
-- sync (seed.SyncAuthz, cmd/migrate); this migration also retires the grants held by the custom roles of tenants, which
-- the sync does not touch. Soft-delete only (AGENTS R21): rows keep their history with delete_reason. Applied as
-- identity_migrator (policy migrator_seed, trigger role_permissions_scope_chk allows the migrator).

UPDATE identity.role_permissions
   SET deleted_at = now(), delete_reason = 'concept_removed', updated_at = now()
 WHERE deleted_at IS NULL
   AND permission_code IN ('lms.course.write', 'lms.learning_path.write', 'lms.schedule.write', 'lms.study_group.write');

UPDATE identity.permissions
   SET deleted_at = now(), delete_reason = 'catalog_removed', updated_at = now()
 WHERE deleted_at IS NULL
   AND permission_code IN ('lms.course.write', 'lms.learning_path.write', 'lms.schedule.write', 'lms.study_group.write');
