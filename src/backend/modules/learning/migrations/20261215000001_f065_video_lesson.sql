-- F065 — video lessons: progress, branch choices, integrity signals (specs/065-video-lesson: data-model §1;
-- DATABASE_DESIGN §2.6a, §2.16; ADR-0005, ADR-0028, ADR-0041, ANTI_CHEAT §6). Applied by migrate --module=learning as
-- learning_migrator. Hand-written tables (explicit-SQL repository like F031 Clarifications [Proposed]); expand only
-- (AGENTS R12).

-- ---------- assessment.video_progress (T): one row per media attempt (video_lesson; audio_lesson in F066) ----------
CREATE TABLE assessment.video_progress (
  attempt_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  position_ms bigint NOT NULL DEFAULT 0,
  watched_ranges jsonb NOT NULL DEFAULT '[]',
  watched_ratio numeric(5,4) NOT NULL DEFAULT 0,
  last_heartbeat_at timestamptz NOT NULL,
  client_seq bigint NOT NULL DEFAULT 0,
  milestones_emitted smallint NOT NULL DEFAULT 0,
  processed_cue_ids varchar(64)[] NOT NULL DEFAULT '{}',
  segment_id varchar(64),
  reached_ending boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (attempt_id),
  CONSTRAINT video_progress_ranges_ck CHECK (jsonb_typeof(watched_ranges) = 'array' AND jsonb_array_length(watched_ranges) <= 64),
  CONSTRAINT video_progress_ratio_ck CHECK (watched_ratio >= 0 AND watched_ratio <= 1),
  CONSTRAINT video_progress_counters_ck CHECK (position_ms >= 0 AND client_seq >= 0 AND milestones_emitted BETWEEN 0 AND 15),
  CONSTRAINT video_progress_cues_ck CHECK (cardinality(processed_cue_ids) <= 30)
);
CREATE INDEX video_progress_heartbeat_idx ON assessment.video_progress (tenant_id, last_heartbeat_at) WHERE deleted_at IS NULL;
ALTER TABLE assessment.video_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.video_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.video_progress FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.video_progress FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.video_progress FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.video_progress_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.video_progress WHERE deleted_at IS NULL;

-- ---------- assessment.video_branch_choices (T): one choice per branch point of an attempt (F065-AC-25) ----------
CREATE TABLE assessment.video_branch_choices (
  choice_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  attempt_id uuid NOT NULL,
  branch_point_id varchar(64) NOT NULL,
  option_id varchar(64) NOT NULL,
  target_segment_id varchar(64) NOT NULL,
  chosen_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (choice_id)
);
CREATE UNIQUE INDEX video_branch_choices_point_uq ON assessment.video_branch_choices (attempt_id, branch_point_id)
  WHERE deleted_at IS NULL;
ALTER TABLE assessment.video_branch_choices ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.video_branch_choices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.video_branch_choices FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.video_branch_choices FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.video_branch_choices FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.video_branch_choices_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.video_branch_choices WHERE deleted_at IS NULL;

-- ---------- assessment.integrity_signals (T, append-only): signals of the attempt engine, never a penalty ----------
-- Not partitioned yet [Proposed]: one signal per trimmed heartbeat at most; partitions follow when the volume needs it.
CREATE TABLE assessment.integrity_signals (
  signal_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  attempt_id uuid NOT NULL,
  signal_type varchar(64) NOT NULL,
  score numeric(8,3) NOT NULL DEFAULT 0,
  evidence jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL,
  PRIMARY KEY (signal_id),
  CONSTRAINT integrity_signals_evidence_ck CHECK (pg_column_size(evidence) <= 4096)
);
CREATE INDEX integrity_signals_student_idx ON assessment.integrity_signals (tenant_id, student_id, created_at DESC);
ALTER TABLE assessment.integrity_signals ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.integrity_signals FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.integrity_signals FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.integrity_signals FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON assessment.integrity_signals FROM learning_app, learning_platform;
GRANT SELECT, INSERT ON assessment.integrity_signals TO learning_app, learning_platform;

GRANT SELECT ON assessment.video_progress_live, assessment.video_branch_choices_live TO learning_readonly;
REVOKE ALL ON assessment.video_progress, assessment.video_branch_choices, assessment.integrity_signals FROM learning_readonly;
