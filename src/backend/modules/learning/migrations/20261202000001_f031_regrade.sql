-- F031 US4 — re-evaluation (specs/031-learning-events-mastery: data-model §1 RegradeRequest, Evaluation; ACTIVITY_ENGINE
-- §7.3; DATABASE_DESIGN §2.16; ADR-0072, ADR-0089). Applied by migrate --module=learning as learning_migrator. Hand-written
-- table (explicit-SQL repository, F031 Clarifications 2026-11-20 [Proposed]); expand only (AGENTS R12).

-- ---------- assessment.regrade_requests (H): a re-evaluation run by the system ----------
-- tenant_id NULL = request of content_admin on platform content (rows reachable through learning_platform only);
-- otherwise the request of an academic_manager of that tenant.
CREATE TABLE assessment.regrade_requests (
  regrade_request_id uuid NOT NULL,
  tenant_id uuid,
  school_id uuid,
  subject_id uuid,
  scope varchar(64) NOT NULL,
  question_version_id uuid,
  activity_version_id uuid,
  exam_session_id uuid,
  corrected_version_id uuid,
  action varchar(64) NOT NULL,
  reason varchar(1000) NOT NULL,
  status varchar(64) NOT NULL DEFAULT 'queued',
  affected_attempts integer NOT NULL DEFAULT 0,
  re_evaluated_attempts integer NOT NULL DEFAULT 0,
  requested_by uuid NOT NULL,
  requested_role varchar(64) NOT NULL,
  idempotency_key varchar(64) NOT NULL,
  last_error varchar(500),
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by uuid,
  PRIMARY KEY (regrade_request_id),
  CONSTRAINT regrade_requests_scope_ck CHECK (scope IN ('question_version','activity_version','exam_session')),
  CONSTRAINT regrade_requests_target_ck CHECK (
    (scope = 'question_version' AND question_version_id IS NOT NULL)
    OR (scope = 'activity_version' AND activity_version_id IS NOT NULL)
    OR (scope = 'exam_session' AND exam_session_id IS NOT NULL)),
  CONSTRAINT regrade_requests_action_ck CHECK (action IN ('accept_all','use_new_key','drop_item')),
  CONSTRAINT regrade_requests_key_ck CHECK (action <> 'use_new_key' OR corrected_version_id IS NOT NULL),
  CONSTRAINT regrade_requests_status_ck CHECK (status IN ('queued','running','completed','failed')),
  CONSTRAINT regrade_requests_role_ck CHECK (requested_role IN ('content_admin','academic_manager')),
  CONSTRAINT regrade_requests_reason_ck CHECK (char_length(reason) >= 10),
  CONSTRAINT regrade_requests_counts_ck CHECK (affected_attempts >= 0 AND re_evaluated_attempts >= 0
    AND re_evaluated_attempts <= affected_attempts)
);
CREATE UNIQUE INDEX regrade_requests_idem_uq ON assessment.regrade_requests (requested_by, idempotency_key);
-- One queued/running request per target version and tenant (platform = zero UUID), F031-AC-22.
CREATE UNIQUE INDEX regrade_requests_active_uq ON assessment.regrade_requests
  ((coalesce(question_version_id, activity_version_id, exam_session_id)), (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)))
  WHERE status IN ('queued','running') AND deleted_at IS NULL;
CREATE INDEX regrade_requests_queue_idx ON assessment.regrade_requests (created_at) WHERE status = 'queued' AND deleted_at IS NULL;
ALTER TABLE assessment.regrade_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.regrade_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.regrade_requests FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.regrade_requests FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.regrade_requests FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.regrade_requests_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.regrade_requests WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.regrade_requests_live TO learning_readonly;
REVOKE ALL ON assessment.regrade_requests FROM learning_readonly;

-- ---------- assessment.evaluations: a superseded evaluation is never written again (F031-AC-22) ----------
CREATE FUNCTION assessment.evaluations_superseded_immutable() RETURNS trigger
  LANGUAGE plpgsql SET search_path = pg_catalog, assessment AS $$
BEGIN
  IF OLD.status = 'superseded' AND (NEW.status IS DISTINCT FROM OLD.status OR NEW.effective IS DISTINCT FROM OLD.effective
      OR NEW.score IS DISTINCT FROM OLD.score OR NEW.max_score IS DISTINCT FROM OLD.max_score OR NEW.details IS DISTINCT FROM OLD.details) THEN
    RAISE EXCEPTION 'EVALUATION_NOT_EDITABLE: evaluation % is superseded', OLD.evaluation_id USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER evaluations_superseded_immutable BEFORE UPDATE ON assessment.evaluations
  FOR EACH ROW EXECUTE FUNCTION assessment.evaluations_superseded_immutable();
