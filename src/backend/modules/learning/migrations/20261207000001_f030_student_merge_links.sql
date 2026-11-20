-- F030 T056 — account merge step of module learning (OQ F016-2, F045-3, owner 2026-11-20; IDENTITY_MAPPING §7,
-- ACCOUNT_PROVISIONING §7b): attempts, learning events and mastery stay under the original student_id, a link original →
-- target user is written once per merge_id. Applied by migrate --module=learning as learning_migrator. Hand-written table
-- (explicit-SQL repository); expand only (AGENTS R12).

-- ---------- learning.student_merge_links (T) ----------
CREATE TABLE learning.student_merge_links (
  merge_id uuid NOT NULL,
  tenant_id uuid NOT NULL,
  source_student_id uuid NOT NULL,
  target_student_id uuid NOT NULL,
  reason varchar(1000) NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz,
  deleted_by uuid,
  delete_reason varchar(64),
  PRIMARY KEY (merge_id),
  CONSTRAINT student_merge_links_distinct_ck CHECK (source_student_id <> target_student_id),
  CONSTRAINT student_merge_links_reason_ck CHECK (char_length(reason) BETWEEN 3 AND 1000)
);
-- One live link per original student (R21: partial unique on live rows).
CREATE UNIQUE INDEX student_merge_links_source_uq ON learning.student_merge_links (tenant_id, source_student_id)
  WHERE deleted_at IS NULL;
CREATE INDEX student_merge_links_target_idx ON learning.student_merge_links (tenant_id, target_student_id)
  WHERE deleted_at IS NULL;
ALTER TABLE learning.student_merge_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning.student_merge_links FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON learning.student_merge_links FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON learning.student_merge_links FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON learning.student_merge_links FOR UPDATE USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE OR REPLACE VIEW learning.student_merge_links_live WITH (security_barrier = true) AS
  SELECT * FROM learning.student_merge_links WHERE deleted_at IS NULL;
GRANT SELECT ON learning.student_merge_links_live TO learning_readonly;
REVOKE ALL ON learning.student_merge_links FROM learning_readonly;
REVOKE DELETE, TRUNCATE ON learning.student_merge_links FROM learning_app, learning_platform;
