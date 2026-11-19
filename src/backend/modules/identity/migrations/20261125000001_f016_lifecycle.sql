-- F016 T005 — account lifecycle of module identity (DATABASE_DESIGN §2.0, §2.2, §2.16, ADR-0028, ADR-0032,
-- ADR-0070): saga engine (shared with F045), cleanup exclusions and runs, data requests, erasure log and the
-- history of user status changes. Hand-written tables accessed by explicit SQL; soft-delete only, no row is
-- ever removed. Applied as identity_migrator.

-- ---------- Saga engine (G): cross-module operations step by step (cleanup, restore, anonymize, F045 clone) ----------
CREATE TABLE platform.saga_runs (
  saga_id          uuid PRIMARY KEY,
  saga_type        varchar(64) NOT NULL CHECK (saga_type ~ '^[a-z][a-z0-9_]{2,63}$'),
  tenant_id        uuid,
  subject          jsonb NOT NULL CHECK (jsonb_typeof(subject) = 'object' AND octet_length(subject::text) <= 16384),
  status           varchar(32) NOT NULL DEFAULT 'running'
                   CHECK (status IN ('running', 'compensating', 'succeeded', 'blocked', 'failed')),
  requested_by     uuid,
  idempotency_key  varchar(128) NOT NULL,
  attempts         integer NOT NULL DEFAULT 0,
  next_attempt_at  timestamptz,
  last_error       varchar(1000),
  started_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  finished_at      timestamptz,
  version          bigint NOT NULL DEFAULT 1,
  CHECK ((status IN ('succeeded', 'blocked', 'failed')) = (finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX saga_runs_idem_uq ON platform.saga_runs (saga_type, idempotency_key);
CREATE INDEX saga_runs_due_idx ON platform.saga_runs (next_attempt_at) WHERE status IN ('running', 'compensating');
ALTER TABLE platform.saga_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.saga_runs FORCE ROW LEVEL SECURITY;
-- identity_app has no privilege; identity_platform (BYPASSRLS) runs the jobs and the platform scope.
CREATE POLICY tenant_read ON platform.saga_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());

CREATE TABLE platform.saga_steps (
  saga_id     uuid NOT NULL REFERENCES platform.saga_runs (saga_id),
  step_no     integer NOT NULL CHECK (step_no >= 0 AND step_no < 50),
  service     varchar(64) NOT NULL,
  action      varchar(64) NOT NULL,
  status      varchar(32) NOT NULL DEFAULT 'pending'
              CHECK (status IN ('pending', 'done', 'blocked', 'nothing_to_do', 'failed', 'compensated')),
  attempts    integer NOT NULL DEFAULT 0,
  response    jsonb CHECK (response IS NULL OR octet_length(response::text) <= 16384),
  error       varchar(1000),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (saga_id, step_no)
);

REVOKE ALL ON platform.saga_runs, platform.saga_steps FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.saga_runs, platform.saga_steps TO identity_platform; -- jobs/platform scope only

-- ---------- Cleanup exclusions (T): a student kept out of the inactive cleanup until a date ----------
CREATE TABLE platform.cleanup_exclusions (
  exclusion_id  uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  student_id    uuid NOT NULL REFERENCES org.students (student_id),
  school_id     uuid NOT NULL,
  until         timestamptz,
  reason        varchar(1000) NOT NULL CHECK (length(btrim(reason)) >= 3),
  created_by    uuid NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  deleted_by    uuid,
  delete_reason varchar(64)
);
CREATE UNIQUE INDEX cleanup_exclusions_live_uq ON platform.cleanup_exclusions (tenant_id, student_id) WHERE deleted_at IS NULL;
ALTER TABLE platform.cleanup_exclusions ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.cleanup_exclusions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_rw ON platform.cleanup_exclusions FOR ALL USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE ALL ON platform.cleanup_exclusions FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.cleanup_exclusions TO identity_app, identity_platform;
CREATE OR REPLACE VIEW platform.cleanup_exclusions_live WITH (security_barrier = true) AS
  SELECT * FROM platform.cleanup_exclusions WHERE deleted_at IS NULL;
GRANT SELECT ON platform.cleanup_exclusions_live TO identity_readonly;

-- ---------- Cleanup runs (G): one run of the inactive-student worker per tenant ----------
CREATE TABLE platform.student_cleanup_runs (
  run_id          uuid PRIMARY KEY,
  tenant_id       uuid NOT NULL,
  mode            varchar(16) NOT NULL CHECK (mode IN ('dry_run', 'apply')),
  status          varchar(32) NOT NULL CHECK (status IN ('running', 'awaiting_approval', 'approved', 'completed', 'failed')),
  cutoff_days     integer NOT NULL CHECK (cutoff_days > 0),
  candidates      integer NOT NULL DEFAULT 0,
  cleaned         integer NOT NULL DEFAULT 0,
  excluded        integer NOT NULL DEFAULT 0,
  skipped_active  integer NOT NULL DEFAULT 0,
  approved_by     uuid,
  approved_at     timestamptz,
  started_at      timestamptz NOT NULL DEFAULT now(),
  finished_at     timestamptz,
  version         bigint NOT NULL DEFAULT 1
);
CREATE INDEX student_cleanup_runs_tenant_idx ON platform.student_cleanup_runs (tenant_id, started_at DESC);
ALTER TABLE platform.student_cleanup_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.student_cleanup_runs FORCE ROW LEVEL SECURITY;
-- identity_app has no privilege; identity_platform (BYPASSRLS) runs the jobs and the platform scope.
CREATE POLICY tenant_read ON platform.student_cleanup_runs FOR SELECT USING (tenant_id = infra.current_tenant_id());
REVOKE ALL ON platform.student_cleanup_runs FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.student_cleanup_runs TO identity_platform;

-- ---------- Data requests and erasure log (G, §2.16): anonymization on request or retention ----------
CREATE TABLE platform.deletion_requests (
  request_id    uuid PRIMARY KEY,
  subject_type  varchar(64) NOT NULL CHECK (subject_type = 'user'),
  subject_id    uuid NOT NULL,
  tenant_id     uuid,
  action        varchar(64) NOT NULL CHECK (action = 'anonymize'),
  status        varchar(32) NOT NULL CHECK (status IN ('pending', 'approved', 'executed', 'rejected')),
  reason        varchar(1000) NOT NULL,
  requested_by  uuid NOT NULL,
  approved_by   uuid,
  due_at        timestamptz,
  executed_at   timestamptz,
  erasure_id    uuid,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  version       bigint NOT NULL DEFAULT 1
);
CREATE INDEX deletion_requests_status_idx ON platform.deletion_requests (status, due_at);
CREATE UNIQUE INDEX deletion_requests_open_uq ON platform.deletion_requests (subject_id) WHERE status IN ('pending', 'approved');
ALTER TABLE platform.deletion_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.deletion_requests FORCE ROW LEVEL SECURITY;
-- identity_app has no privilege; identity_platform (BYPASSRLS) runs the jobs and the platform scope.
CREATE POLICY tenant_read ON platform.deletion_requests FOR SELECT USING (tenant_id = infra.current_tenant_id());

CREATE TABLE platform.erasure_log (
  erasure_id    uuid PRIMARY KEY,
  user_id       uuid NOT NULL,
  kind          varchar(64) NOT NULL CHECK (kind = 'anonymize'),
  requested_by  uuid,
  legal_basis   varchar(255) NOT NULL,
  executed_at   timestamptz NOT NULL DEFAULT now(),
  scope         jsonb NOT NULL CHECK (jsonb_typeof(scope) = 'object' AND octet_length(scope::text) <= 16384)
);
REVOKE ALL ON platform.deletion_requests, platform.erasure_log FROM identity_app, identity_platform;
GRANT SELECT, INSERT, UPDATE ON platform.deletion_requests TO identity_platform;
GRANT SELECT, INSERT ON platform.erasure_log TO identity_platform; -- append-only

-- ---------- identity.user_status_history (TA*): who changed a status, when and why ----------
-- The reason of a lock is visible to staff in scope only (F016-AC-06); the student sees a neutral message.
CREATE TABLE identity.user_status_history (
  history_id   uuid PRIMARY KEY,
  tenant_id    uuid,
  user_id      uuid NOT NULL REFERENCES identity.users (user_id),
  from_status  varchar(64) NOT NULL,
  to_status    varchar(64) NOT NULL,
  reason       varchar(1000),
  source       varchar(64) NOT NULL CHECK (source IN ('staff', 'sis', 'cleanup', 'restore', 'anonymize', 'retention')),
  changed_by   uuid,
  changed_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_status_history_user_idx ON identity.user_status_history (user_id, changed_at DESC);
ALTER TABLE identity.user_status_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.user_status_history FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_read ON identity.user_status_history FOR SELECT USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON identity.user_status_history FOR INSERT WITH CHECK (tenant_id = infra.current_tenant_id());
REVOKE ALL ON identity.user_status_history FROM identity_app, identity_platform;
GRANT SELECT, INSERT ON identity.user_status_history TO identity_app, identity_platform;
