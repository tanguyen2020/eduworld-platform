-- ADR-0095 mục 2 (WP-L1, expand): the grade of the student is recorded AT THE TIME OF THE EVENT on the learning events
-- and the daily progress (no history table of grades; never derived from the current grade): learning.learning_events
-- and learning.progress_daily gain grade_level, written by the learning-event writer and the progress consumer from
-- ref.students when the row is written. Existing rows are backfilled from the class of the row (ref.classes via
-- class_id), otherwise the current grade of the student (ref.students). class_id stays (rollups by class until WP-L2,
-- DROP at WP-R). Applied by migrate --module=learning as learning_migrator.
--
-- learning_events is append-only (no UPDATE policy, UPDATE revoked from the app roles): the one-off backfill uses a
-- temporary UPDATE policy for learning_migrator only, dropped in the same migration. The local/dev volume is small; a
-- production backfill would run by partition in batches (ADR-0095 is applied before the first staging deployment).

ALTER TABLE learning.learning_events ADD COLUMN grade_level smallint;
ALTER TABLE learning.learning_events ADD CONSTRAINT learning_events_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
ALTER TABLE learning.progress_daily ADD COLUMN grade_level smallint;
ALTER TABLE learning.progress_daily ADD CONSTRAINT progress_daily_grade_ck CHECK (grade_level IS NULL OR grade_level BETWEEN 1 AND 9);
COMMENT ON COLUMN learning.learning_events.grade_level IS 'ADR-0095: grade of the student when the event happened (ref.students at write time)';
COMMENT ON COLUMN learning.progress_daily.grade_level IS 'ADR-0095: grade of the student on the bucket day (ref.students at write time)';

-- BEGIN adr0095-grade-backfill
CREATE POLICY adr0095_backfill ON learning.learning_events FOR UPDATE TO learning_migrator USING (true) WITH CHECK (true);
CREATE POLICY adr0095_backfill_read ON learning.learning_events FOR SELECT TO learning_migrator USING (true);
CREATE POLICY adr0095_backfill ON learning.progress_daily FOR UPDATE TO learning_migrator USING (true) WITH CHECK (true);
CREATE POLICY adr0095_backfill_read ON learning.progress_daily FOR SELECT TO learning_migrator USING (true);

-- tenancyguard:ok one-off backfill as learning_migrator over every tenant; projections joined on the tenant of the row
UPDATE learning.learning_events le
   SET grade_level = COALESCE(
         (SELECT k.grade_level FROM ref.classes k WHERE k.class_id = le.class_id AND k.tenant_id = le.tenant_id),
         (SELECT st.grade_level FROM ref.students st WHERE st.student_id = le.student_id AND st.tenant_id = le.tenant_id))
 WHERE le.grade_level IS NULL;

-- tenancyguard:ok one-off backfill as learning_migrator over every tenant; projections joined on the tenant of the row
UPDATE learning.progress_daily pd
   SET grade_level = COALESCE(
         (SELECT k.grade_level FROM ref.classes k WHERE k.class_id = pd.class_id AND k.tenant_id = pd.tenant_id),
         (SELECT st.grade_level FROM ref.students st WHERE st.student_id = pd.student_id AND st.tenant_id = pd.tenant_id))
 WHERE pd.grade_level IS NULL;

DROP POLICY adr0095_backfill ON learning.learning_events;
DROP POLICY adr0095_backfill_read ON learning.learning_events;
DROP POLICY adr0095_backfill ON learning.progress_daily;
DROP POLICY adr0095_backfill_read ON learning.progress_daily;
-- END adr0095-grade-backfill
