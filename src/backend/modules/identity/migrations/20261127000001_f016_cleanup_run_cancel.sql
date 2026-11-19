-- F016 follow-up (OQ F057-2): cancelStudentCleanupRun moves a run awaiting approval to `cancelled` (nothing is
-- removed; a later job tick starts a fresh run). Hand-written table: expand the status CHECK only (every
-- existing value stays valid).
ALTER TABLE platform.student_cleanup_runs
  DROP CONSTRAINT student_cleanup_runs_status_check,
  ADD CONSTRAINT student_cleanup_runs_status_check
    CHECK (status IN ('running', 'awaiting_approval', 'approved', 'cancelled', 'completed', 'failed'));
