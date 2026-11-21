-- ADR-0095 mục 3 (WP-L1, expand): school assignments and exam sessions are given to (school_id, grade_level) of a school
-- plus included students of another grade of the same school minus excluded students, or to a list of students of the
-- school; subject_id is set directly (no course). F040 is removed: courses, learning paths, the schedule and study groups
-- are no longer written (their tables are dropped by WP-R, ADR-0095 mục 8, with the backup plan of R12).
--
-- Existing targets of a class or a study group become a FROZEN list of students (ADR-0095 mục 3: never widened to the
-- whole grade): the students of the snapshot taken at publish (published assignments) or the current members of the
-- class/group (drafts); target_ref_id keeps the old class/group id as history. Old values are converted on every row (live and soft-deleted), so the CHECK keeps only the new
-- values; a row left with an old value (tenant missing from ref.tenants) makes the CHECK fail and the migration stop.
-- Old columns (assignments.course_id, assignment_targets.target_ref_id, assignment_target_students.class_id_at_assign,
-- assignment_submissions.class_id, exam_sessions.course_id) stay, are no longer written, and are dropped by WP-R.
-- Applied by migrate --module=learning as learning_migrator (NOBYPASSRLS, FORCE RLS on lms.* and assessment.*: the
-- conversion runs tenant by tenant with app.tenant_id set, so every row is visible to its tenant policy).

-- ---------- lms.assignments: subject directly, course no longer required ----------
ALTER TABLE lms.assignments ALTER COLUMN course_id DROP NOT NULL;
COMMENT ON COLUMN lms.assignments.course_id IS 'ADR-0095: no longer written (subject_id is set directly); dropped by WP-R';
CREATE INDEX assignments_school_subject_idx ON lms.assignments (tenant_id, school_id, subject_id) WHERE deleted_at IS NULL;

-- ---------- lms.assignment_targets: grade targets with include/exclude ----------
ALTER TABLE lms.assignment_targets
  ADD COLUMN grade_level smallint,
  ADD COLUMN include_student_ids uuid[] NOT NULL DEFAULT '{}',
  ADD COLUMN exclude_student_ids uuid[] NOT NULL DEFAULT '{}';
ALTER TABLE lms.assignment_targets DROP CONSTRAINT assignment_targets_type_ck;
ALTER TABLE lms.assignment_targets DROP CONSTRAINT assignment_targets_ref_ck;

-- ---------- lms.assignment_target_students: the grade when targeted, students who join the grade after publish ----------
ALTER TABLE lms.assignment_target_students DROP CONSTRAINT assignment_target_students_source_ck;
ALTER TABLE lms.assignment_target_students ADD CONSTRAINT assignment_target_students_source_ck
  CHECK (source IN ('publish','enrollment_added','grade_added'));
-- The grade of the student when targeted (ADR-0095 mục 2: grade recorded at event time, never derived from the current
-- grade); NULL on the rows written before.
ALTER TABLE lms.assignment_target_students ADD COLUMN grade_level smallint,
  ADD CONSTRAINT assignment_target_students_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
COMMENT ON COLUMN lms.assignment_target_students.class_id_at_assign IS 'ADR-0095: no longer written; dropped by WP-R';
COMMENT ON COLUMN lms.assignment_submissions.class_id IS 'ADR-0095: no longer written; dropped by WP-R';
COMMENT ON COLUMN assessment.exam_sessions.course_id IS 'ADR-0095: no longer written; dropped by WP-R';

-- BEGIN adr0095-freeze (re-run by tests/integration/assignment_freeze_test.go)
DO $freeze$
DECLARE
  tid uuid;
BEGIN
  FOR tid IN SELECT tenant_id FROM ref.tenants ORDER BY tenant_id LOOP
    PERFORM set_config('app.tenant_id', tid::text, true);
    -- Published assignment: the students of its snapshot from this class (class_id_at_assign) or, for a study group,
    -- the snapshot students who were members of the group.
    UPDATE lms.assignment_targets t
       SET target_type = 'students', updated_at = now(), student_ids = COALESCE((
             SELECT array_agg(s.student_id ORDER BY s.student_id) FROM lms.assignment_target_students s
              WHERE s.assignment_id = t.assignment_id AND s.tenant_id = tid AND s.deleted_at IS NULL
                AND CASE WHEN t.target_type = 'class' THEN s.class_id_at_assign = t.target_ref_id
                         ELSE EXISTS (SELECT 1 FROM lms.study_group_members m WHERE m.study_group_id = t.target_ref_id
                                        AND m.student_id = s.student_id AND m.tenant_id = tid) END), '{}')
     WHERE t.tenant_id = tid AND t.target_type IN ('class', 'study_group')
       AND EXISTS (SELECT 1 FROM lms.assignment_target_students s WHERE s.assignment_id = t.assignment_id AND s.tenant_id = tid);
    -- Draft (no snapshot yet): the active members of the class or the live members of the study group now.
    UPDATE lms.assignment_targets t
       SET target_type = 'students', updated_at = now(), student_ids = COALESCE(CASE WHEN t.target_type = 'class' THEN (
             SELECT array_agg(DISTINCT e.student_id) FROM ref.enrollments e
              WHERE e.class_id = t.target_ref_id AND e.tenant_id = tid AND e.status = 'active')
           ELSE (
             SELECT array_agg(DISTINCT m.student_id) FROM lms.study_group_members m
              WHERE m.study_group_id = t.target_ref_id AND m.tenant_id = tid AND m.deleted_at IS NULL) END, '{}')
     WHERE t.tenant_id = tid AND t.target_type IN ('class', 'study_group')
       AND NOT EXISTS (SELECT 1 FROM lms.assignment_target_students s WHERE s.assignment_id = t.assignment_id AND s.tenant_id = tid);
    -- Exam sessions of a class/study group: the audience snapshot (participations) becomes their list of students.
    UPDATE assessment.exam_sessions s
       SET target = jsonb_build_object('targets', jsonb_build_array(jsonb_build_object('Type', 'students',
             'StudentIDs', COALESCE((SELECT jsonb_agg(p.student_id ORDER BY p.student_id) FROM assessment.exam_participations p
                                      WHERE p.exam_session_id = s.exam_session_id AND p.tenant_id = tid), '[]'::jsonb),
             'StudentCount', COALESCE((s.target->>'student_count')::int, 0))),
             'student_count', COALESCE((s.target->>'student_count')::int, 0)),
           updated_at = now()
     WHERE s.tenant_id = tid AND jsonb_path_exists(s.target, '$.targets[*] ? (@.Type == "class" || @.Type == "study_group")');
  END LOOP;
  PERFORM set_config('app.tenant_id', '', true);
END
$freeze$;
-- END adr0095-freeze

ALTER TABLE lms.assignment_targets
  ADD CONSTRAINT assignment_targets_type_ck CHECK (target_type IN ('grade','students')),
  ADD CONSTRAINT assignment_targets_grade_ck CHECK ((target_type = 'grade') = (grade_level IS NOT NULL)
    AND (grade_level IS NULL OR grade_level BETWEEN 1 AND 9)),
  ADD CONSTRAINT assignment_targets_adjust_ck CHECK (cardinality(include_student_ids) <= 200 AND cardinality(exclude_student_ids) <= 200
    AND (target_type = 'grade' OR (cardinality(include_student_ids) = 0 AND cardinality(exclude_student_ids) = 0)));
COMMENT ON COLUMN lms.assignment_targets.target_ref_id IS 'ADR-0095: class/study group of a converted target (history only); dropped by WP-R';
CREATE INDEX assignment_targets_grade_idx ON lms.assignment_targets (tenant_id, school_id, grade_level)
  WHERE deleted_at IS NULL AND target_type = 'grade';

-- New columns appear in the _live views (SELECT * is expanded when a view is created).
CREATE OR REPLACE VIEW lms.assignment_targets_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_targets WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW lms.assignment_target_students_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_target_students WHERE deleted_at IS NULL;

-- ---------- ref.students: the students of a grade of a school (grade targets, F041) ----------
CREATE INDEX ref_students_school_grade_idx ON ref.students (tenant_id, school_id, grade_level) WHERE deleted_at IS NULL;
