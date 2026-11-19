-- F030 — Activity Registry and Attempt API (specs/030-activity-attempt-api: data-model §1–§4; ACTIVITY_ENGINE §4, §6, §7;
-- DATABASE_DESIGN §2.6, §2.6a, §2.7, §3.4–§3.6; ADR-0016, ADR-0043, ADR-0048, ADR-0059, ADR-0089). Applied by migrate
-- --module=learning as learning_migrator. Order: ent tables (entddl block, golden test modules/learning/ent/ddl_test.go),
-- CHECKs, FKs, RLS (H for activities, T for attempts), immutability and release-target triggers, the partitioned append-only
-- tables assessment.attempt_answers and learning.learning_events (monthly partitions; the maintenance job is F031),
-- _live views, grants. Expand only (AGENTS R12): the assignment CHECK gains the resource type activity.

-- BEGIN entddl
CREATE TABLE content.activities (
  activity_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  current_version_id uuid,
  latest_version_no bigint NOT NULL DEFAULT 0,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  PRIMARY KEY (activity_id)
);

CREATE TABLE content.activity_versions (
  activity_version_id uuid NOT NULL,
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  owner_scope varchar(64) NOT NULL DEFAULT 'tenant',
  version_no bigint NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  author_id uuid NOT NULL,
  reviewer_id uuid,
  published_at timestamptz,
  content_language varchar(64) NOT NULL DEFAULT 'vi',
  curriculum_version_id uuid,
  change_note varchar(500),
  publish_at timestamptz,
  snapshot_key varchar(512),
  snapshot_sha256 varchar(64),
  based_on_version_id uuid,
  cloned_from_id uuid,
  cloned_from_tenant_id uuid,
  cloned_at timestamptz,
  cloned_by uuid,
  translation_status varchar(64),
  translation_meta jsonb,
  audio_exempt_reason varchar(255),
  audio_exempt_approved_by uuid,
  published_by uuid,
  activity_id uuid NOT NULL,
  title jsonb NOT NULL,
  summary jsonb,
  subject_id uuid,
  grade_levels jsonb NOT NULL,
  topic_ids jsonb NOT NULL,
  objective_ids jsonb NOT NULL,
  media_refs jsonb NOT NULL,
  worked_example_ids jsonb NOT NULL,
  body jsonb NOT NULL,
  PRIMARY KEY (activity_version_id)
);
CREATE UNIQUE INDEX activity_versions_no_uq ON content.activity_versions (activity_id, version_no);
CREATE UNIQUE INDEX activity_versions_open_uq ON content.activity_versions (activity_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL;
CREATE INDEX activity_versions_subject_idx ON content.activity_versions (tenant_id, subject_id) WHERE deleted_at IS NULL;

CREATE TABLE assessment.attempts (
  attempt_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  mode varchar(64) NOT NULL,
  context_type varchar(64) NOT NULL,
  context_id uuid NOT NULL,
  activity_id uuid,
  activity_type varchar(32) NOT NULL,
  source_kind varchar(32) NOT NULL,
  source_version_id uuid NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'in_progress',
  seed bigint NOT NULL,
  started_at timestamptz NOT NULL,
  deadline_at timestamptz,
  submitted_at timestamptz,
  graded_at timestamptz,
  paused_at timestamptz,
  paused_ms bigint NOT NULL DEFAULT 0,
  last_activity_at timestamptz NOT NULL,
  feedback_timing varchar(64) NOT NULL,
  display_language varchar(64) NOT NULL,
  submit_reason varchar(64),
  score numeric(7,2),
  max_score numeric(7,2),
  duration_ms bigint,
  client_elapsed_ms bigint,
  item_count bigint NOT NULL,
  objective_ids jsonb NOT NULL,
  recommendation_id uuid,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (attempt_id)
);
CREATE INDEX attempts_activity_idx ON assessment.attempts (status, last_activity_at) WHERE status = 'in_progress' AND deleted_at IS NULL;
CREATE INDEX attempts_context_idx ON assessment.attempts (tenant_id, context_type, context_id) WHERE deleted_at IS NULL;
CREATE INDEX attempts_deadline_idx ON assessment.attempts (status, deadline_at) WHERE status = 'in_progress' AND deadline_at IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX attempts_idem_uq ON assessment.attempts (student_id, idempotency_key);
CREATE UNIQUE INDEX attempts_one_open_uq ON assessment.attempts (student_id, context_type, context_id) WHERE status = 'in_progress' AND mode IN ('exam','assignment') AND deleted_at IS NULL;
CREATE UNIQUE INDEX attempts_practice_open_uq ON assessment.attempts (student_id, source_version_id) WHERE status = 'in_progress' AND mode = 'practice' AND deleted_at IS NULL;
CREATE INDEX attempts_student_idx ON assessment.attempts (tenant_id, student_id, started_at) WHERE deleted_at IS NULL;

CREATE TABLE assessment.attempt_items (
  attempt_item_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  attempt_id uuid NOT NULL,
  position bigint NOT NULL,
  question_version_id uuid NOT NULL,
  question_type varchar(64) NOT NULL,
  objective_ids jsonb NOT NULL,
  presentation jsonb NOT NULL,
  presented_payload_hash varchar(64) NOT NULL,
  max_score numeric(6,2) NOT NULL,
  answer_count bigint NOT NULL DEFAULT 0,
  final_is_correct boolean,
  final_score numeric(6,2),
  hints_revealed bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (attempt_item_id)
);
CREATE UNIQUE INDEX attempt_items_position_uq ON assessment.attempt_items (attempt_id, position);

CREATE TABLE assessment.evaluations (
  evaluation_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  attempt_id uuid NOT NULL,
  source varchar(16) NOT NULL,
  regrade_request_id uuid,
  supersedes_evaluation_id uuid,
  details jsonb,
  status varchar(16) NOT NULL DEFAULT 'final',
  score numeric(7,2) NOT NULL,
  max_score numeric(7,2) NOT NULL,
  passed boolean,
  feedback varchar(4000),
  triggered_by uuid,
  reason varchar(500),
  effective boolean NOT NULL DEFAULT true,
  PRIMARY KEY (evaluation_id)
);
CREATE INDEX evaluations_attempt_idx ON assessment.evaluations (tenant_id, attempt_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX evaluations_effective_uq ON assessment.evaluations (attempt_id) WHERE effective AND deleted_at IS NULL;
-- END entddl

-- ---------- content.activities / content.activity_versions (H): a composed content type (F024 columns) ----------
ALTER TABLE content.activities
  ADD CONSTRAINT activities_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT activities_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT activities_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT activities_latest_ck CHECK (latest_version_no >= 0);
ALTER TABLE content.activity_versions
  ADD CONSTRAINT activity_versions_owner_scope_ck CHECK (owner_scope IN ('platform','tenant')),
  ADD CONSTRAINT activity_versions_owner_ck CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  ADD CONSTRAINT activity_versions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT activity_versions_item_fk FOREIGN KEY (activity_id) REFERENCES content.activities (activity_id),
  ADD CONSTRAINT activity_versions_subject_fk FOREIGN KEY (subject_id) REFERENCES curriculum.subjects (subject_id),
  ADD CONSTRAINT activity_versions_curriculum_fk FOREIGN KEY (curriculum_version_id) REFERENCES curriculum.curriculum_versions (curriculum_version_id),
  ADD CONSTRAINT activity_versions_status_ck CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  ADD CONSTRAINT activity_versions_language_ck CHECK (content_language IN ('vi','en')),
  ADD CONSTRAINT activity_versions_title_ck CHECK (jsonb_typeof(title) = 'object' AND octet_length(title::text) <= 2048),
  ADD CONSTRAINT activity_versions_summary_ck CHECK (summary IS NULL OR (jsonb_typeof(summary) = 'object' AND octet_length(summary::text) <= 8192)),
  ADD CONSTRAINT activity_versions_grades_ck CHECK (jsonb_typeof(grade_levels) = 'array' AND jsonb_array_length(grade_levels) <= 9
    AND NOT jsonb_path_exists(grade_levels, '$[*] ? (@ < 1 || @ > 9)')),
  ADD CONSTRAINT activity_versions_topics_ck CHECK (jsonb_typeof(topic_ids) = 'array' AND jsonb_array_length(topic_ids) <= 50),
  ADD CONSTRAINT activity_versions_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 20),
  ADD CONSTRAINT activity_versions_media_ck CHECK (jsonb_typeof(media_refs) = 'array' AND jsonb_array_length(media_refs) <= 50),
  ADD CONSTRAINT activity_versions_worked_examples_ck CHECK (jsonb_typeof(worked_example_ids) = 'array' AND jsonb_array_length(worked_example_ids) <= 100),
  -- body holds activity_type, the activity fields and config (content/interaction/evaluation/feedback/completion_rule): <= 256 KB.
  ADD CONSTRAINT activity_versions_body_ck CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 262144);
-- activity_type is part of the version document; the generated column serves filters and the type CHECK (ActivityType).
ALTER TABLE content.activity_versions ADD COLUMN activity_type varchar(32) GENERATED ALWAYS AS (body->>'activity_type') STORED;
ALTER TABLE content.activity_versions ADD CONSTRAINT activity_versions_type_ck CHECK (activity_type IS NULL OR activity_type IN
  ('quiz','interactive_exercise','minigame','video_lesson','audio_lesson','simulation'));
ALTER TABLE content.activity_versions ADD COLUMN search_tsv tsvector
  GENERATED ALWAYS AS (to_tsvector('simple', content.f_search_text(title, summary))) STORED;
CREATE INDEX activity_versions_type_idx ON content.activity_versions (tenant_id, activity_type) WHERE deleted_at IS NULL;
CREATE INDEX activity_versions_search_gin ON content.activity_versions USING gin (search_tsv) WHERE deleted_at IS NULL;
CREATE INDEX activity_versions_topics_gin ON content.activity_versions USING gin (topic_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX activity_versions_objectives_gin ON content.activity_versions USING gin (objective_ids jsonb_path_ops) WHERE deleted_at IS NULL;
ALTER TABLE content.activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.activities FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.activities FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'activity' AND a.resource_id = activities.activity_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.activities FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.activities FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
ALTER TABLE content.activity_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE content.activity_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY content_read ON content.activity_versions FOR SELECT USING (
  tenant_id = infra.current_tenant_id()
  OR (tenant_id IS NULL AND EXISTS (
    SELECT 1 FROM infra.tenant_resource_assignments a
    WHERE a.resource_type = 'activity' AND a.resource_id = activity_versions.activity_id
      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
);
CREATE POLICY tenant_insert ON content.activity_versions FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON content.activity_versions FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE TRIGGER activity_versions_immutable BEFORE UPDATE ON content.activity_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();
CREATE FUNCTION content.activities_release_target() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.activity_versions v
       WHERE v.activity_version_id = NEW.current_version_id AND v.activity_id = NEW.activity_id AND v.status = 'published') THEN
    RAISE EXCEPTION 'RELEASE_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER activities_release_target BEFORE INSERT OR UPDATE OF current_version_id ON content.activities
  FOR EACH ROW EXECUTE FUNCTION content.activities_release_target();

-- Platform activities are assigned to tenants like the other learning resources (ADR-0029).
ALTER TABLE infra.tenant_resource_assignments DROP CONSTRAINT tra_resource_type_ck;
ALTER TABLE infra.tenant_resource_assignments
  ADD CONSTRAINT tra_resource_type_ck CHECK (resource_type IN ('curriculum','lesson','question','question_set',
    'assessment_blueprint','media','assignment','course','learning_path','activity'));

-- ---------- assessment.attempts (T): server-authoritative attempt, pinned version ----------
ALTER TABLE assessment.attempts
  ADD CONSTRAINT attempts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT attempts_mode_ck CHECK (mode IN ('practice','assignment','quest','minigame','challenge','exam','competition',
    'retry_wrong','diagnostic')),
  ADD CONSTRAINT attempts_context_type_ck CHECK (context_type IN ('free_play','lesson','learning_path','recommendation',
    'question_set','selection','assignment','quest_step','minigame_session','exam_session','match','challenge_attempt','boss_encounter')),
  ADD CONSTRAINT attempts_status_ck CHECK (status IN ('in_progress','submitted','graded','expired','abandoned')),
  ADD CONSTRAINT attempts_type_ck CHECK (activity_type IN ('quiz','interactive_exercise','minigame','video_lesson','audio_lesson','simulation')),
  ADD CONSTRAINT attempts_source_kind_ck CHECK (source_kind IN ('activity','question_set','selection')),
  ADD CONSTRAINT attempts_feedback_ck CHECK (feedback_timing IN ('immediate','after_submit','after_close','never')),
  ADD CONSTRAINT attempts_language_ck CHECK (display_language IN ('vi','en')),
  ADD CONSTRAINT attempts_submit_reason_ck CHECK (submit_reason IS NULL OR submit_reason IN ('manual','timeout','auto_complete')),
  ADD CONSTRAINT attempts_graded_ck CHECK (status <> 'graded' OR (score IS NOT NULL AND graded_at IS NOT NULL)),
  ADD CONSTRAINT attempts_delete_ck CHECK (deleted_at IS NULL OR status NOT IN ('submitted','graded')),
  ADD CONSTRAINT attempts_paused_ck CHECK (paused_ms >= 0),
  ADD CONSTRAINT attempts_items_ck CHECK (item_count BETWEEN 0 AND 200),
  ADD CONSTRAINT attempts_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50);
ALTER TABLE assessment.attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.attempts FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.attempts FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.attempts FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- assessment.attempt_items (T) ----------
ALTER TABLE assessment.attempt_items
  ADD CONSTRAINT attempt_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT attempt_items_attempt_fk FOREIGN KEY (attempt_id) REFERENCES assessment.attempts (attempt_id),
  ADD CONSTRAINT attempt_items_position_ck CHECK (position BETWEEN 1 AND 200),
  ADD CONSTRAINT attempt_items_type_ck CHECK (question_type IN ('single_choice','multiple_choice','true_false','fill_in_blank',
    'short_answer','numeric','matching','ordering','drag_drop','image_based','interactive')),
  ADD CONSTRAINT attempt_items_objectives_ck CHECK (jsonb_typeof(objective_ids) = 'array' AND jsonb_array_length(objective_ids) <= 50),
  ADD CONSTRAINT attempt_items_presentation_ck CHECK (jsonb_typeof(presentation) = 'object' AND octet_length(presentation::text) <= 16384),
  ADD CONSTRAINT attempt_items_score_ck CHECK (max_score > 0 AND (final_score IS NULL OR final_score BETWEEN 0 AND max_score)),
  ADD CONSTRAINT attempt_items_counts_ck CHECK (answer_count >= 0 AND hints_revealed BETWEEN 0 AND 5);
ALTER TABLE assessment.attempt_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.attempt_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.attempt_items FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.attempt_items FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.attempt_items FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- assessment.evaluations (T): exactly one effective evaluation per attempt ----------
ALTER TABLE assessment.evaluations
  ADD CONSTRAINT evaluations_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id),
  ADD CONSTRAINT evaluations_attempt_fk FOREIGN KEY (attempt_id) REFERENCES assessment.attempts (attempt_id),
  ADD CONSTRAINT evaluations_source_ck CHECK (source IN ('system','re_evaluation')),
  ADD CONSTRAINT evaluations_status_ck CHECK (status IN ('final','superseded')),
  ADD CONSTRAINT evaluations_effective_ck CHECK (NOT effective OR status = 'final'),
  ADD CONSTRAINT evaluations_score_ck CHECK (score >= 0 AND max_score >= 0 AND score <= max_score),
  ADD CONSTRAINT evaluations_details_ck CHECK (details IS NULL OR octet_length(details::text) <= 65536);
ALTER TABLE assessment.evaluations ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.evaluations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.evaluations FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.evaluations FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.evaluations FOR UPDATE USING (tenant_id = infra.current_tenant_id()) WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- assessment.attempt_answers (TA, P): every response, append-only, server times ----------
CREATE TABLE assessment.attempt_answers (
  attempt_answer_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  attempt_id uuid NOT NULL,
  position int NOT NULL,
  school_id uuid NOT NULL,
  question_version_id uuid NOT NULL,
  response jsonb NOT NULL,
  is_correct boolean,
  score numeric(6,2),
  answered_at timestamptz NOT NULL,
  client_elapsed_ms int,
  response_ms int,
  attempt_no smallint NOT NULL,
  is_scored boolean NOT NULL DEFAULT true,
  hints_used smallint NOT NULL DEFAULT 0,
  rapid_guess boolean NOT NULL DEFAULT false,
  idempotency_key varchar(64) NOT NULL,
  PRIMARY KEY (attempt_answer_id, answered_at),
  UNIQUE (attempt_id, idempotency_key, answered_at),
  CHECK (position BETWEEN 1 AND 200),
  CHECK (attempt_no >= 1),
  CHECK (hints_used >= 0),
  CHECK (client_elapsed_ms IS NULL OR client_elapsed_ms >= 0),
  CHECK (response_ms IS NULL OR response_ms >= 0),
  CHECK (octet_length(response::text) <= 32768)
) PARTITION BY RANGE (answered_at);
CREATE INDEX attempt_answers_attempt_idx ON assessment.attempt_answers (attempt_id, position, answered_at DESC);
CREATE INDEX attempt_answers_tenant_idx ON assessment.attempt_answers (tenant_id, answered_at);
ALTER TABLE assessment.attempt_answers ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.attempt_answers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.attempt_answers FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.attempt_answers FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON assessment.attempt_answers FROM learning_app, learning_platform;
GRANT SELECT, INSERT ON assessment.attempt_answers TO learning_app, learning_platform;

-- ---------- learning.learning_events (TA, P): internal LRS, append-only (EVENT_DESIGN §5) ----------
CREATE TABLE learning.learning_events (
  event_id uuid NOT NULL,
  occurred_at timestamptz NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  event_type varchar(64) NOT NULL,
  student_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  class_id uuid,
  game_session_id uuid,
  activity_id uuid,
  content_id uuid,
  content_version_id uuid,
  objective_ids uuid[] NOT NULL DEFAULT '{}',
  context varchar(64) NOT NULL,
  actor_type varchar(16) NOT NULL DEFAULT 'student',
  payload jsonb NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  schema_version smallint NOT NULL DEFAULT 1,
  PRIMARY KEY (event_id, occurred_at),
  UNIQUE (idempotency_key, occurred_at),
  CHECK (event_type IN ('activity.started','activity.submitted','activity.graded','activity.abandoned','activity.completed',
    'lesson.completed','question.viewed','question.answered','video.progressed','video.cue_answered','simulation.goal_met',
    'exam.started','exam.submitted','exam.graded','attempt.re_evaluated','quest.completed','learning_objective.progressed',
    'learning_objective.mastered')),
  CHECK (context IN ('practice','assignment','quest','minigame','challenge','exam','competition','retry_wrong','diagnostic')),
  CHECK (actor_type IN ('student','system','staff')),
  CHECK (cardinality(objective_ids) <= 50),
  CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 32768)
) PARTITION BY RANGE (occurred_at);
CREATE INDEX learning_events_student_idx ON learning.learning_events (tenant_id, student_id, occurred_at DESC);
CREATE INDEX learning_events_school_type_idx ON learning.learning_events (tenant_id, school_id, event_type, occurred_at);
CREATE INDEX learning_events_objectives_gin ON learning.learning_events USING gin (objective_ids);
ALTER TABLE learning.learning_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.learning_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.learning_events FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.learning_events FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON learning.learning_events FROM learning_app, learning_platform;
GRANT SELECT, INSERT ON learning.learning_events TO learning_app, learning_platform;

-- Monthly partitions from the release month through 2027-12 (data-model §1: current and next months; the daily
-- partition job keeping two months ahead is F031-AC-04).
DO $$
DECLARE
  m date := date '2026-11-01';
BEGIN
  WHILE m < date '2028-01-01' LOOP
    EXECUTE format('CREATE TABLE assessment.attempt_answers_%s PARTITION OF assessment.attempt_answers FOR VALUES FROM (%L) TO (%L)',
      to_char(m, 'YYYY_MM'), m, (m + interval '1 month')::date);
    EXECUTE format('CREATE TABLE learning.learning_events_%s PARTITION OF learning.learning_events FOR VALUES FROM (%L) TO (%L)',
      to_char(m, 'YYYY_MM'), m, (m + interval '1 month')::date);
    m := (m + interval '1 month')::date;
  END LOOP;
END $$;

-- ---------- _live views and the read-only role ----------
CREATE OR REPLACE VIEW content.activities_live WITH (security_barrier = true) AS
  SELECT * FROM content.activities WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW content.activity_versions_live WITH (security_barrier = true) AS
  SELECT * FROM content.activity_versions WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW assessment.attempts_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.attempts WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW assessment.attempt_items_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.attempt_items WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW assessment.evaluations_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.evaluations WHERE deleted_at IS NULL;
GRANT USAGE ON SCHEMA assessment, learning TO learning_readonly;
GRANT SELECT ON content.activities_live, content.activity_versions_live, assessment.attempts_live, assessment.attempt_items_live,
  assessment.evaluations_live TO learning_readonly;
REVOKE ALL ON content.activities, content.activity_versions, assessment.attempts, assessment.attempt_items, assessment.evaluations,
  assessment.attempt_answers, learning.learning_events FROM learning_readonly;
