-- F031 US6 — lesson sessions and lesson completion (specs/031-learning-events-mastery: data-model §1 LessonSession;
-- DATABASE_DESIGN §2.5; R31, ADR-0043). Applied by migrate --module=learning as learning_migrator. Hand-written table
-- (explicit-SQL repository, F031 Clarifications [Proposed]); expand only (AGENTS R12).

-- ---------- learning.lesson_sessions (T): one study of a lesson pinned to the version open when it started ----------
CREATE TABLE learning.lesson_sessions (
  lesson_session_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  lesson_id uuid NOT NULL,
  lesson_version_id uuid NOT NULL,
  lesson_version integer NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'in_progress',
  context_mode varchar(64) NOT NULL DEFAULT 'practice',
  assignment_id uuid,
  learning_path_id uuid,
  recommendation_id uuid,
  completed_activity_ids uuid[] NOT NULL DEFAULT '{}',
  completed_attempt_ids uuid[] NOT NULL DEFAULT '{}',
  active_ms bigint NOT NULL DEFAULT 0,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (lesson_session_id),
  CONSTRAINT lesson_sessions_status_ck CHECK (status IN ('in_progress','completed')),
  CONSTRAINT lesson_sessions_mode_ck CHECK (context_mode IN ('practice','assignment')),
  CONSTRAINT lesson_sessions_lists_ck CHECK (cardinality(completed_activity_ids) <= 50 AND cardinality(completed_attempt_ids) <= 50),
  CONSTRAINT lesson_sessions_active_ck CHECK (active_ms >= 0 AND lesson_version >= 1),
  CONSTRAINT lesson_sessions_end_ck CHECK ((status = 'completed') = (ended_at IS NOT NULL))
);
-- One open session per student and lesson; one completion per student and lesson version (F031-AC-27).
CREATE UNIQUE INDEX lesson_sessions_open_uq ON learning.lesson_sessions (tenant_id, student_id, lesson_id)
  WHERE status = 'in_progress' AND deleted_at IS NULL;
CREATE UNIQUE INDEX lesson_sessions_completed_uq ON learning.lesson_sessions (tenant_id, student_id, lesson_version_id)
  WHERE status = 'completed' AND deleted_at IS NULL;
ALTER TABLE learning.lesson_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.lesson_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.lesson_sessions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.lesson_sessions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.lesson_sessions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW learning.lesson_sessions_live WITH (security_barrier = true) AS
  SELECT * FROM learning.lesson_sessions WHERE deleted_at IS NULL;
GRANT SELECT ON learning.lesson_sessions_live TO learning_readonly;
REVOKE ALL ON learning.lesson_sessions FROM learning_readonly;
