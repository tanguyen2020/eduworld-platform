-- F041 — school assignments: assignments, targets and their student snapshot, submissions, per-student exceptions,
-- reminders, the read-only catalog of game activities (specs/041-school-assignments: data-model §1; DATABASE_DESIGN §2.14,
-- §2.16, §2.17; SCHOOL_LMS §2, §4). Applied by migrate --module=learning as learning_migrator. Hand-written tables
-- (explicit-SQL repositories, like the LMS of F040); expand only (AGENTS R12); soft-delete only (R21).

-- ---------- lms.assignments (T) ----------
CREATE TABLE lms.assignments (
  assignment_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  course_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  created_by uuid NOT NULL,
  created_by_role varchar(64) NOT NULL,
  title varchar(120) NOT NULL,
  instructions varchar(2000),
  content_type varchar(64) NOT NULL,
  content_id uuid NOT NULL,
  pinned_version_id uuid,
  follow_latest boolean NOT NULL DEFAULT false,
  activity_type varchar(64),
  mode varchar(64) NOT NULL,
  open_at timestamptz NOT NULL,
  due_at timestamptz NOT NULL,
  allow_late boolean NOT NULL DEFAULT false,
  late_window_hours smallint NOT NULL DEFAULT 0,
  max_attempts smallint NOT NULL DEFAULT 1,
  score_policy varchar(64) NOT NULL DEFAULT 'best',
  feedback_timing varchar(64) NOT NULL,
  result_release varchar(64) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  exam_session_id uuid,
  objective_ids uuid[] NOT NULL DEFAULT '{}',
  published_at timestamptz,
  closed_at timestamptz,
  results_released_at timestamptz,
  cancelled_at timestamptz,
  archived_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (assignment_id),
  CONSTRAINT assignments_role_ck CHECK (created_by_role IN ('school_admin','academic_manager','system_admin')),
  CONSTRAINT assignments_content_type_ck CHECK (content_type IN ('activity','question_set','assessment_blueprint','lesson','quest',
    'minigame_config')),
  CONSTRAINT assignments_mode_ck CHECK (mode IN ('practice','graded')),
  CONSTRAINT assignments_follow_ck CHECK (NOT (follow_latest AND mode = 'graded') AND (follow_latest OR pinned_version_id IS NOT NULL)),
  CONSTRAINT assignments_window_ck CHECK (due_at > open_at),
  CONSTRAINT assignments_late_ck CHECK (late_window_hours BETWEEN 0 AND 168),
  CONSTRAINT assignments_attempts_ck CHECK (max_attempts BETWEEN 1 AND 10),
  CONSTRAINT assignments_score_policy_ck CHECK (score_policy IN ('best','last','first')),
  CONSTRAINT assignments_feedback_ck CHECK (feedback_timing IN ('immediate','after_submit','after_close','never')),
  CONSTRAINT assignments_release_ck CHECK (result_release IN ('immediate','after_close','manual')),
  CONSTRAINT assignments_status_ck CHECK (status IN ('draft','scheduled','open','closed','results_released','archived','cancelled')),
  CONSTRAINT assignments_objectives_ck CHECK (cardinality(objective_ids) <= 50)
);
CREATE INDEX assignments_school_status_idx ON lms.assignments (tenant_id, school_id, status, due_at) WHERE deleted_at IS NULL;
CREATE INDEX assignments_course_idx ON lms.assignments (tenant_id, course_id) WHERE deleted_at IS NULL;
-- Scheduler: assignments to open or close (every tenant, job assignment-scheduler).
CREATE INDEX assignments_due_idx ON lms.assignments (status, due_at) WHERE deleted_at IS NULL AND status IN ('scheduled','open');

-- ---------- lms.assignment_targets (T): what the assignment was given to ----------
CREATE TABLE lms.assignment_targets (
  assignment_target_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  target_type varchar(64) NOT NULL,
  target_ref_id uuid,
  student_ids uuid[] NOT NULL DEFAULT '{}',
  position smallint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (assignment_target_id),
  CONSTRAINT assignment_targets_type_ck CHECK (target_type IN ('class','study_group','students')),
  CONSTRAINT assignment_targets_ref_ck CHECK ((target_type = 'students') = (target_ref_id IS NULL)),
  CONSTRAINT assignment_targets_students_ck CHECK (cardinality(student_ids) <= 200 AND (target_type = 'students' OR cardinality(student_ids) = 0)),
  CONSTRAINT assignment_targets_position_ck CHECK (position BETWEEN 1 AND 50)
);
CREATE INDEX assignment_targets_assignment_idx ON lms.assignment_targets (assignment_id) WHERE deleted_at IS NULL;
CREATE INDEX assignment_targets_ref_idx ON lms.assignment_targets (tenant_id, target_type, target_ref_id) WHERE deleted_at IS NULL;

-- ---------- lms.assignment_target_students (T): snapshot of the students at publish, and new members of a class ----------
CREATE TABLE lms.assignment_target_students (
  assignment_target_student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  student_id uuid NOT NULL,
  class_id_at_assign uuid,
  source varchar(64) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (assignment_target_student_id),
  CONSTRAINT assignment_target_students_source_ck CHECK (source IN ('publish','enrollment_added'))
);
-- A student is targeted once, whatever the number of targets holding them.
CREATE UNIQUE INDEX assignment_target_students_uq ON lms.assignment_target_students (assignment_id, student_id) WHERE deleted_at IS NULL;
CREATE INDEX assignment_target_students_student_idx ON lms.assignment_target_students (tenant_id, student_id) WHERE deleted_at IS NULL;

-- ---------- lms.assignment_submissions (T): read model of the attempts of each targeted student ----------
CREATE TABLE lms.assignment_submissions (
  assignment_submission_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  student_id uuid NOT NULL,
  class_id uuid,
  status varchar(64) NOT NULL DEFAULT 'not_started',
  attempts_used smallint NOT NULL DEFAULT 0,
  current_attempt_id uuid,
  best_attempt_id uuid,
  score numeric(7,2),
  max_score numeric(7,2),
  submitted_at timestamptz,
  rapid_guess_count int NOT NULL DEFAULT 0,
  due_at_override timestamptz,
  completed_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (assignment_submission_id),
  CONSTRAINT assignment_submissions_status_ck CHECK (status IN ('not_started','in_progress','submitted','late','missed','excused')),
  CONSTRAINT assignment_submissions_counts_ck CHECK (attempts_used BETWEEN 0 AND 10 AND rapid_guess_count >= 0)
);
CREATE UNIQUE INDEX assignment_submissions_uq ON lms.assignment_submissions (assignment_id, student_id) WHERE deleted_at IS NULL;
CREATE INDEX assignment_submissions_student_idx ON lms.assignment_submissions (tenant_id, student_id, status) WHERE deleted_at IS NULL;

-- ---------- lms.assignment_student_exceptions (T): extension (personal due date) or excuse of one student ----------
CREATE TABLE lms.assignment_student_exceptions (
  exception_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  student_id uuid NOT NULL,
  kind varchar(64) NOT NULL,
  due_at timestamptz,
  reason varchar(1000) NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (exception_id),
  CONSTRAINT assignment_student_exceptions_kind_ck CHECK (kind IN ('extend','excuse')),
  CONSTRAINT assignment_student_exceptions_due_ck CHECK ((kind = 'extend') = (due_at IS NOT NULL))
);
-- A later extension replaces the earlier one (soft-delete of the previous row).
CREATE UNIQUE INDEX assignment_student_exceptions_uq ON lms.assignment_student_exceptions (assignment_id, student_id, kind)
  WHERE deleted_at IS NULL;

-- ---------- lms.assignment_reminders (TA): reminders of an assignment, counted per day (ICT) ----------
CREATE TABLE lms.assignment_reminders (
  reminder_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  assignment_id uuid NOT NULL,
  reminded_on date NOT NULL,
  recipients int NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (reminder_id),
  CONSTRAINT assignment_reminders_recipients_ck CHECK (recipients >= 0)
);
CREATE INDEX assignment_reminders_day_idx ON lms.assignment_reminders (assignment_id, reminded_on);

-- ---------- ref.game_activities: quests and mini-games of module game published and assigned to a tenant ----------
-- Projection of the game content events (F078); empty until module game publishes them (no fake data).
CREATE TABLE ref.game_activities (
  item_type varchar(64) NOT NULL,
  item_id uuid NOT NULL,
  tenant_id uuid NOT NULL REFERENCES ref.tenants (tenant_id),
  current_version_id uuid NOT NULL,
  title_i18n jsonb NOT NULL DEFAULT '{}' CHECK (octet_length(title_i18n::text) <= 2048),
  objective_ids uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(objective_ids) <= 50),
  grade_levels smallint[] NOT NULL DEFAULT '{}' CHECK (cardinality(grade_levels) <= 12),
  thumbnail_key varchar(512),
  status varchar(64) NOT NULL,
  source_version bigint NOT NULL,
  PRIMARY KEY (item_type, item_id, tenant_id),
  CONSTRAINT game_activities_type_ck CHECK (item_type IN ('quest','minigame_config')),
  CONSTRAINT game_activities_status_ck CHECK (status IN ('published','unpublished','unassigned'))
);
CREATE INDEX ref_game_activities_tenant_idx ON ref.game_activities (tenant_id, status);
ALTER TABLE ref.game_activities ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON ref.game_activities FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON ref.game_activities FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON ref.game_activities FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- RLS of the T tables by tenant ----------
ALTER TABLE lms.assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignments FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignments FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.assignments FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.assignments_live WITH (security_barrier = true) AS SELECT * FROM lms.assignments WHERE deleted_at IS NULL;
GRANT SELECT ON lms.assignments_live TO learning_readonly;
REVOKE ALL ON lms.assignments FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.assignments FROM learning_app, learning_platform;

ALTER TABLE lms.assignment_targets ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignment_targets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignment_targets FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignment_targets FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.assignment_targets FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.assignment_targets_live WITH (security_barrier = true) AS SELECT * FROM lms.assignment_targets WHERE deleted_at IS NULL;
GRANT SELECT ON lms.assignment_targets_live TO learning_readonly;
REVOKE ALL ON lms.assignment_targets FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.assignment_targets FROM learning_app, learning_platform;

ALTER TABLE lms.assignment_target_students ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignment_target_students FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignment_target_students FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignment_target_students FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.assignment_target_students FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.assignment_target_students_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_target_students WHERE deleted_at IS NULL;
GRANT SELECT ON lms.assignment_target_students_live TO learning_readonly;
REVOKE ALL ON lms.assignment_target_students FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.assignment_target_students FROM learning_app, learning_platform;

ALTER TABLE lms.assignment_submissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignment_submissions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignment_submissions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignment_submissions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.assignment_submissions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.assignment_submissions_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_submissions WHERE deleted_at IS NULL;
GRANT SELECT ON lms.assignment_submissions_live TO learning_readonly;
REVOKE ALL ON lms.assignment_submissions FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.assignment_submissions FROM learning_app, learning_platform;

ALTER TABLE lms.assignment_student_exceptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignment_student_exceptions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignment_student_exceptions FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignment_student_exceptions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON lms.assignment_student_exceptions FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW lms.assignment_student_exceptions_live WITH (security_barrier = true) AS
  SELECT * FROM lms.assignment_student_exceptions WHERE deleted_at IS NULL;
GRANT SELECT ON lms.assignment_student_exceptions_live TO learning_readonly;
REVOKE ALL ON lms.assignment_student_exceptions FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON lms.assignment_student_exceptions FROM learning_app, learning_platform;

ALTER TABLE lms.assignment_reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE lms.assignment_reminders FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON lms.assignment_reminders FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON lms.assignment_reminders FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
GRANT SELECT ON lms.assignment_reminders TO learning_readonly;
REVOKE UPDATE, DELETE, TRUNCATE ON lms.assignment_reminders FROM learning_app, learning_platform;
