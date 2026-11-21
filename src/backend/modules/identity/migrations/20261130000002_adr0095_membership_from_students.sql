-- ADR-0095 mục 2 (D1, WP-I1, expand): a student belongs to one school with one grade (org.students.school_id,
-- grade_level); org.enrollments and org.classes are no longer written (DROP in WP-R, ADR-0095 mục 8).
-- identity.tenant_memberships_v is rebuilt from the effective students (status 'active', not soft-deleted, live school of
-- the same tenant) ∪ the valid user roles of staff. The columns of the view do not change, so the SECURITY DEFINER
-- function identity.memberships_for_user (F013) keeps working; its owner identity_membership_reader now reads
-- org.students and org.schools instead of org.enrollments. Applied as identity_migrator.

CREATE OR REPLACE VIEW identity.tenant_memberships_v WITH (security_invoker = true, security_barrier = true) AS
  SELECT s.tenant_id, s.student_id AS user_id, 'student'::varchar(64) AS member_kind, s.school_id
  FROM org.students s
  JOIN org.schools sc ON sc.school_id = s.school_id AND sc.tenant_id = s.tenant_id AND sc.deleted_at IS NULL
  WHERE s.status = 'active' AND s.deleted_at IS NULL
  UNION ALL
  SELECT ur.tenant_id, ur.user_id, 'staff'::varchar(64) AS member_kind, NULL::uuid AS school_id
  FROM identity.user_roles ur
  WHERE ur.tenant_id IS NOT NULL AND ur.deleted_at IS NULL
    AND ur.valid_from <= now() AND (ur.valid_to IS NULL OR ur.valid_to > now())
  GROUP BY ur.tenant_id, ur.user_id;
GRANT SELECT ON identity.tenant_memberships_v TO identity_app, identity_platform;

GRANT SELECT ON org.students, org.schools TO identity_membership_reader;
REVOKE SELECT ON org.enrollments FROM identity_membership_reader;

-- Data of every tenant (org.*, sis.* are FORCE RLS and identity_migrator has no BYPASSRLS: app.tenant_id is set per
-- tenant, as the F010 seed does). Updates only, never DELETE (AGENTS R21).
DO $$
DECLARE t uuid;
BEGIN
  FOR t IN SELECT tenant_id FROM platform.tenants LOOP
    PERFORM set_config('app.tenant_id', t::text, true);

    -- Students whose enrollment was closed (SIS inactive/graduated/transferred_out, manual end) were kept active on
    -- org.students; their effective status now lives there. Students without any enrollment (missing_class) become
    -- members of the tenant of their school, as ADR-0095 intends. Soft-deleted rows are untouched.
    UPDATE org.students s
       SET status = 'inactive', updated_at = now(), version = s.version + 1
     WHERE s.tenant_id = t AND s.status = 'active' AND s.deleted_at IS NULL
       AND EXISTS (SELECT 1 FROM org.enrollments e WHERE e.student_id = s.student_id AND e.tenant_id = t AND e.deleted_at IS NULL)
       AND NOT EXISTS (SELECT 1 FROM org.enrollments e
                        WHERE e.student_id = s.student_id AND e.tenant_id = t AND e.status = 'active' AND e.deleted_at IS NULL);

    -- SIS contract 1.3.0 (ADR-0095 D6): the links of classes and enrollments are history; soft-delete them so the source
    -- keys are no longer read. The CHECK values stay until WP-R (rows still carry them).
    UPDATE sis.sis_links SET deleted_at = now(), delete_reason = 'concept_removed', updated_at = now(), version = version + 1
     WHERE tenant_id = t AND deleted_at IS NULL AND entity_type IN ('class', 'enrollment');

    -- The reconciliation issues of the removed kinds can no longer be fixed: open ones are closed as ignored with a note.
    UPDATE sis.reconciliation_issues
       SET status = 'ignored', resolution = 'ignore', resolution_note = 'ADR-0095: lớp/enrollment không còn trong EduWorld',
           resolved_at = now(), updated_at = now(), version = version + 1
     WHERE tenant_id = t AND deleted_at IS NULL AND status = 'open'
       AND issue_type IN ('missing_class', 'class_school_mismatch', 'orphan_enrollment', 'multi_school_enrollment', 'grade_mismatch');

    -- class_leaderboard_enabled left the school settings (ADR-0095 D5).
    UPDATE org.schools SET settings = settings - 'class_leaderboard_enabled', updated_at = now(), version = version + 1
     WHERE tenant_id = t AND settings ? 'class_leaderboard_enabled';
  END LOOP;
  PERFORM set_config('app.tenant_id', '', true);
END $$;
