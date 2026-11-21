-- F064 T004 (expand only, AGENTS R12): ExamSession.navigation copied from the pinned blueprint version at creation
-- (assessment-blueprint.schema.json `navigation`, EXAM_ENGINE §3). Rows created before this migration keep the
-- blueprint default `free`. No data is removed.
ALTER TABLE assessment.exam_sessions ADD COLUMN navigation varchar(32) NOT NULL DEFAULT 'free';
ALTER TABLE assessment.exam_sessions ADD CONSTRAINT exam_sessions_navigation_ck
  CHECK (navigation IN ('free','sequential_no_return'));
-- The `_live` view (R27) expands `*` at creation: re-create it so it carries the columns added since F042
-- (efficacy_wave_id from F035, navigation). New columns are appended, so CREATE OR REPLACE keeps the grants.
CREATE OR REPLACE VIEW assessment.exam_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.exam_sessions WHERE deleted_at IS NULL;
