-- F105 T007 — question plans of competition matches (specs/105-battle-engine/data-model.md §1 QuestionPlan,
-- DATABASE_DESIGN §2.6; ADR-0016, ADR-0043). Expand only (AGENTS R12). items pins the question versions with the
-- presentation shown in the room (seed, shuffled orders) — never an answer key: grading reads content.question_versions.
-- The answers of a match go to assessment.attempt_answers through an attempt mode competition per student (context
-- match), created on the first scored answer.

-- BEGIN entddl
CREATE TABLE assessment.question_plans (
  plan_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  created_by uuid,
  updated_at timestamptz NOT NULL,
  updated_by uuid,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  version bigint NOT NULL DEFAULT 1,
  match_id uuid NOT NULL,
  mode varchar(64) NOT NULL,
  grade_level smallint NOT NULL,
  subject_id uuid NOT NULL,
  plan_version varchar(64) NOT NULL,
  seed bigint NOT NULL,
  items jsonb NOT NULL,
  participants jsonb NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (plan_id)
);
CREATE UNIQUE INDEX question_plans_match_uq ON assessment.question_plans (match_id) WHERE deleted_at IS NULL;
-- END entddl

ALTER TABLE assessment.question_plans
  ALTER COLUMN created_at SET DEFAULT now(), ALTER COLUMN updated_at SET DEFAULT now(),
  ADD CONSTRAINT question_plans_mode_ck CHECK (mode IN ('battle_1v1','team_battle','coop_challenge','timed_quiz','tournament')),
  ADD CONSTRAINT question_plans_grade_ck CHECK (grade_level BETWEEN 1 AND 9),
  ADD CONSTRAINT question_plans_items_ck CHECK (jsonb_typeof(items) = 'array' AND jsonb_array_length(items) BETWEEN 1 AND 30
    AND octet_length(items::text) <= 65536),
  ADD CONSTRAINT question_plans_participants_ck CHECK (jsonb_typeof(participants) = 'array'
    AND jsonb_array_length(participants) BETWEEN 1 AND 40),
  ADD CONSTRAINT question_plans_tenant_fk FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);

-- ---------- RLS by tenant (T) ----------
ALTER TABLE assessment.question_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.question_plans FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON assessment.question_plans FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.question_plans FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.question_plans FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW assessment.question_plans_live WITH (security_barrier = true) AS
  SELECT * FROM assessment.question_plans WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.question_plans_live TO learning_readonly;
REVOKE ALL ON assessment.question_plans FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON assessment.question_plans FROM learning_app, learning_platform;
