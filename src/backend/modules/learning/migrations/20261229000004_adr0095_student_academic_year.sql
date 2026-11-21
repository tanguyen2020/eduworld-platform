-- ADR-0095 mục 2 (WP-I1, coordinated with module learning, expand): ref.students keeps the current academic year of the
-- school of the student as student.placement_changed.academic_year_id carries it (owner F010). The default curriculum
-- resolver of the school settings (FR-CUR-008, F021) reads the academic years of the students of the school (and grade)
-- instead of the years of ref.classes, which module identity no longer feeds (DROP in WP-R). Nullable: a school without a
-- declared academic year keeps the rule without year. Applied by migrate --module=learning as learning_migrator.

ALTER TABLE ref.students ADD COLUMN academic_year_id uuid;
CREATE INDEX ref_students_school_year_idx ON ref.students (tenant_id, school_id, grade_level, academic_year_id)
  WHERE academic_year_id IS NOT NULL AND deleted_at IS NULL;

-- Existing rows: the academic year of the current class of the student (still projected from class.synced), so the
-- resolver keeps its answer until the next placement event.
UPDATE ref.students st
   SET academic_year_id = (SELECT k.academic_year_id FROM ref.enrollments e
                             JOIN ref.classes k ON k.class_id = e.class_id AND k.tenant_id = e.tenant_id
                            WHERE e.student_id = st.student_id AND e.tenant_id = st.tenant_id AND e.status = 'active'
                              AND k.academic_year_id IS NOT NULL
                            ORDER BY e.start_date DESC LIMIT 1)
 WHERE st.academic_year_id IS NULL;
