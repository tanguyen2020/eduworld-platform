-- F031 — learning events, mastery v1 (specs/031-learning-events-mastery: data-model §1; LEARNING_ANALYTICS §2–§4;
-- DATABASE_DESIGN §2.7, §5; ADR-0004, ADR-0017, ADR-0070, ADR-0089). Applied by migrate --module=learning as
-- learning_migrator. Hand-written tables (explicit-SQL repositories, OQ F031 [Proposed]): mastery_model_configs (G),
-- objective_masteries (T), mastery_history (TA, monthly partitions) and progress_daily (T derived, monthly partitions,
-- no deleted_at — DATABASE_DESIGN §2.7). Monthly partitions of the partitioned learning tables are kept ahead by the job
-- learning-events-partitions through SECURITY DEFINER functions (role learning_partition_admin: EXECUTE only).
-- Expand only (AGENTS R12).

-- ---------- learning.mastery_model_configs (G): versioned parameters of the mastery model (ADR-0017) ----------
CREATE TABLE learning.mastery_model_configs (
  config_id uuid NOT NULL,
  model_version varchar(64) NOT NULL,
  algorithm varchar(64) NOT NULL,
  params jsonb NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'draft',
  activated_at timestamptz,
  activated_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (config_id),
  CONSTRAINT mastery_model_configs_algorithm_ck CHECK (algorithm IN ('weighted_evidence_ewma')),
  CONSTRAINT mastery_model_configs_status_ck CHECK (status IN ('draft','shadow','active','retired')),
  CONSTRAINT mastery_model_configs_params_ck CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 16384)
);
CREATE UNIQUE INDEX mastery_model_configs_version_uq ON learning.mastery_model_configs (model_version) WHERE deleted_at IS NULL;
-- Exactly one active version at a time (F031-AC-12).
CREATE UNIQUE INDEX mastery_model_configs_active_uq ON learning.mastery_model_configs ((true))
  WHERE status = 'active' AND deleted_at IS NULL;
-- Platform configuration: the application reads it; changes go through the operations CLI (platform role).
REVOKE INSERT, UPDATE ON learning.mastery_model_configs FROM learning_app;

-- we-1.0: the defaults of LEARNING_ANALYTICS §3.1 (T003; a negative guess coefficient means 1/k, k = options shown).
INSERT INTO learning.mastery_model_configs (config_id, model_version, algorithm, params, status, activated_at)
VALUES ('01935f2a-0000-7000-8000-000000000031', 'we-1.0', 'weighted_evidence_ewma', '{
  "m0": 0.30, "alpha_base": 0.35, "alpha_max": 0.60,
  "w_ctx": {"exam": 1.0, "assignment": 0.9, "practice": 0.7, "retry_wrong": 0.7, "quest": 0.6, "challenge": 0.6,
            "minigame": 0.6, "timed_challenge": 0.4, "competition": 0.4, "diagnostic": 0.7},
  "w_ctx_default": 0.7, "w_rep": 0.5, "w_rep_window_hours": 24, "w_hint": [1.0, 0.6, 0.3],
  "guess_coefficient": {"true_false": 0.5, "single_choice": -1, "image_based": -1, "multiple_choice": 0.1,
                        "matching": 0.1, "ordering": 0.1, "drag_drop": 0.1},
  "default_option_count": 4, "correct_threshold": 0.5, "grace_days": 14, "half_life_days": 90, "n_min": 5, "d_min": 2,
  "level_thresholds": {"developing": 0.40, "proficient": 0.60, "mastered": 0.80},
  "day_location": "Asia/Ho_Chi_Minh"}'::jsonb, 'active', now());

-- ---------- learning.objective_masteries (T): current mastery of (student, learning objective) ----------
CREATE TABLE learning.objective_masteries (
  objective_mastery_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  value numeric(4,3) NOT NULL,
  level varchar(64) NOT NULL,
  evidence_count integer NOT NULL DEFAULT 0,
  distinct_days integer NOT NULL DEFAULT 0,
  last_evidence_at timestamptz,
  last_evidence_date date,
  first_mastered_at timestamptz,
  model_version varchar(64) NOT NULL,
  prior_source varchar(64) NOT NULL DEFAULT 'default',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (objective_mastery_id),
  CONSTRAINT objective_masteries_value_ck CHECK (value >= 0 AND value <= 1),
  CONSTRAINT objective_masteries_level_ck CHECK (level IN ('not_started','beginning','developing','proficient','mastered')),
  CONSTRAINT objective_masteries_counts_ck CHECK (evidence_count >= 0 AND distinct_days >= 0 AND distinct_days <= evidence_count),
  CONSTRAINT objective_masteries_prior_ck CHECK (prior_source IN ('default','diagnostic'))
);
CREATE UNIQUE INDEX objective_masteries_student_uq ON learning.objective_masteries (tenant_id, student_id, objective_id)
  WHERE deleted_at IS NULL;
CREATE INDEX objective_masteries_school_idx ON learning.objective_masteries (tenant_id, school_id, objective_id) WHERE deleted_at IS NULL;
ALTER TABLE learning.objective_masteries ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.objective_masteries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.objective_masteries FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.objective_masteries FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.objective_masteries FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());

-- ---------- learning.mastery_history (TA, P month): every change of a mastery value, append-only ----------
CREATE TABLE learning.mastery_history (
  history_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  student_id uuid NOT NULL,
  objective_id uuid NOT NULL,
  value_before numeric(4,3),
  value_after numeric(4,3) NOT NULL,
  level_before varchar(64),
  level_after varchar(64) NOT NULL,
  evidence_count integer NOT NULL,
  source_event_id uuid,
  source_kind varchar(64) NOT NULL,
  model_version varchar(64) NOT NULL,
  occurred_at timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (history_id, occurred_at),
  CHECK (source_kind IN ('evidence','re_evaluation','recompute')),
  CHECK (value_after >= 0 AND value_after <= 1 AND (value_before IS NULL OR (value_before >= 0 AND value_before <= 1)))
) PARTITION BY RANGE (occurred_at);
CREATE INDEX mastery_history_student_idx ON learning.mastery_history (tenant_id, student_id, objective_id, occurred_at DESC);
ALTER TABLE learning.mastery_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.mastery_history FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.mastery_history FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.mastery_history FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE UPDATE, DELETE, TRUNCATE ON learning.mastery_history FROM learning_app, learning_platform;
GRANT SELECT, INSERT ON learning.mastery_history TO learning_app, learning_platform;

-- ---------- learning.progress_daily (T, P month): counters per student × subject × day (derived, no deleted_at) ----------
-- subject_id 00000000-0000-0000-0000-000000000000 = activity without subject (ad-hoc practice) [Proposed].
CREATE TABLE learning.progress_daily (
  tenant_id uuid NOT NULL,
  student_id uuid NOT NULL,
  subject_id uuid NOT NULL,
  bucket_date date NOT NULL,
  metric_version smallint NOT NULL DEFAULT 1,
  school_id uuid NOT NULL,
  class_id uuid,
  activities_completed integer NOT NULL DEFAULT 0,
  items_answered integer NOT NULL DEFAULT 0,
  items_correct integer NOT NULL DEFAULT 0,
  items_rapid_guess integer NOT NULL DEFAULT 0,
  items_with_hint integer NOT NULL DEFAULT 0,
  active_seconds bigint NOT NULL DEFAULT 0,
  lessons_completed integer NOT NULL DEFAULT 0,
  is_final boolean NOT NULL DEFAULT false,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, student_id, subject_id, bucket_date, metric_version),
  CHECK (activities_completed >= 0 AND items_answered >= 0 AND items_correct >= 0 AND items_correct <= items_answered
    AND items_rapid_guess >= 0 AND items_with_hint >= 0 AND active_seconds >= 0 AND lessons_completed >= 0)
) PARTITION BY RANGE (bucket_date);
ALTER TABLE learning.progress_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.progress_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.progress_daily FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.progress_daily FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.progress_daily FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE DELETE, TRUNCATE ON learning.progress_daily FROM learning_app, learning_platform;
GRANT SELECT, INSERT, UPDATE ON learning.progress_daily TO learning_app, learning_platform;

-- ---------- monthly partitions (F031-AC-04, DATABASE_DESIGN §5) ----------
-- Monthly partitions [first day of p_from's month + i months), i in 0..p_months-1, of the partitioned learning tables,
-- named <table>_YYYY_MM (F030 naming). Runs as the table owner with UTC bounds; every direct privilege of other roles
-- on a new partition is revoked (the parent's grants, RLS and append-only rules are the only path, F031-AC-03/AC-07).
-- No DEFAULT partition: a missing month must fail loudly (alert LearningPartitionsMissing).
CREATE FUNCTION learning.ensure_monthly_partitions(p_from date, p_months int) RETURNS int
  LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, learning, assessment SET timezone = 'UTC' AS $$
DECLARE
  t text; m date; part text; created int := 0; g record; sch text; tbl text;
BEGIN
  IF p_months < 1 OR p_months > 24 THEN
    RAISE EXCEPTION 'p_months must be within 1..24';
  END IF;
  FOREACH t IN ARRAY ARRAY['learning.learning_events', 'assessment.attempt_answers', 'learning.mastery_history', 'learning.progress_daily'] LOOP
    sch := split_part(t, '.', 1); tbl := split_part(t, '.', 2);
    FOR i IN 0 .. p_months - 1 LOOP
      m := (date_trunc('month', p_from) + make_interval(months => i))::date;
      part := tbl || '_' || to_char(m, 'YYYY_MM');
      CONTINUE WHEN to_regclass(sch || '.' || part) IS NOT NULL;
      EXECUTE format('CREATE TABLE %I.%I PARTITION OF %I.%I FOR VALUES FROM (%L) TO (%L)', sch, part, sch, tbl,
        m::text, (m + interval '1 month')::date::text);
      FOR g IN SELECT DISTINCT grantee FROM information_schema.role_table_grants
               WHERE table_schema = sch AND table_name = part AND grantee <> current_user LOOP
        EXECUTE format('REVOKE ALL ON %I.%I FROM %I', sch, part, g.grantee);
      END LOOP;
      created := created + 1;
    END LOOP;
  END LOOP;
  RETURN created;
END $$;

-- Number of consecutive monthly partitions of p_table from the month of p_from (metric
-- eduworld_learning_events_partition_months_ahead; alert when below 2, F031-AC-04).
CREATE FUNCTION learning.monthly_partitions_ahead(p_table text, p_from date) RETURNS int
  LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog, learning, assessment AS $$
DECLARE n int := 0;
BEGIN
  IF p_table NOT IN ('learning.learning_events', 'assessment.attempt_answers', 'learning.mastery_history', 'learning.progress_daily') THEN
    RAISE EXCEPTION 'not a monthly partitioned learning table: %', p_table;
  END IF;
  WHILE n < 36 AND to_regclass(p_table || '_' || to_char(date_trunc('month', p_from) + make_interval(months => n), 'YYYY_MM')) IS NOT NULL LOOP
    n := n + 1;
  END LOOP;
  RETURN n;
END $$;

REVOKE ALL ON FUNCTION learning.ensure_monthly_partitions(date, int), learning.monthly_partitions_ahead(text, date) FROM PUBLIC;
GRANT USAGE ON SCHEMA learning, assessment TO learning_partition_admin;
GRANT EXECUTE ON FUNCTION learning.ensure_monthly_partitions(date, int), learning.monthly_partitions_ahead(text, date)
  TO learning_partition_admin;

-- First partitions of the new tables (same months as F030: 2026-11 … 2027-12).
SELECT learning.ensure_monthly_partitions(date '2026-11-01', 14);

-- Partitions created by F030 kept the default privileges of the migrator (SELECT, INSERT, UPDATE): direct access to a
-- partition bypasses the RLS and the append-only grants of its parent, so it is revoked (F031-AC-03, F031-AC-07).
DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT c.relnamespace::regnamespace::text AS sch, c.relname AS part
           FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
           WHERE i.inhparent IN ('learning.learning_events'::regclass, 'assessment.attempt_answers'::regclass) LOOP
    EXECUTE format('REVOKE ALL ON %I.%I FROM learning_app, learning_platform, learning_readonly', r.sch, r.part);
  END LOOP;
END $$;

-- ---------- _live views and the read-only role ----------
CREATE OR REPLACE VIEW learning.objective_masteries_live WITH (security_barrier = true) AS
  SELECT * FROM learning.objective_masteries WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW learning.mastery_model_configs_live WITH (security_barrier = true) AS
  SELECT * FROM learning.mastery_model_configs WHERE deleted_at IS NULL;
GRANT SELECT ON learning.objective_masteries_live, learning.mastery_model_configs_live TO learning_readonly;
REVOKE ALL ON learning.objective_masteries, learning.mastery_model_configs, learning.mastery_history, learning.progress_daily
  FROM learning_readonly;
