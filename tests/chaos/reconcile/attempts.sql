-- F132 T017 (F132-AC-12, TC-PERF-008; also AC-08, AC-10, AC-11, AC-25): every attempt the load bots got a 2xx submit
-- for is in PostgreSQL. READ ONLY, explicit tenant filter (synthetic tenants perf-*, F132-AC-07) and soft-delete filter
-- (AGENTS R21, R27). Runs with the read role of module learning (eduworld_learning) through the bastion/ops job.
--   psql "$DSN" -X -q -A -t -F '|' -v ON_ERROR_STOP=1 \
--     -v tenant_prefix=perf- -v from=2026-12-01T08:00:00Z -v to=2026-12-01T08:20:00Z -v ids=<uuid,uuid,…|empty> \
--     -f tests/chaos/reconcile/attempts.sql
-- Output (one row): db_submitted|bot_acked|missing
--   db_submitted  attempts of perf-* tenants submitted/graded in [from, to)  (≥ bot_acked: the worker also auto-submits)
--   bot_acked     distinct attempt ids of the bot ledger (2xx on submitActivityAttempt/submitExamAttempt)
--   missing       ledger ids with no submitted/graded attempt (must be 0)
BEGIN TRANSACTION READ ONLY;
WITH perf AS (
  SELECT tenant_id FROM ref.tenants WHERE code LIKE :'tenant_prefix' || '%'
), db AS (
  SELECT a.attempt_id
  FROM assessment.attempts a
  JOIN perf USING (tenant_id)
  WHERE a.deleted_at IS NULL
    AND a.status IN ('submitted', 'graded')
    AND a.submitted_at >= :'from'::timestamptz
    AND a.submitted_at < :'to'::timestamptz
), bot AS (
  SELECT DISTINCT x::uuid AS attempt_id
  FROM unnest(string_to_array(NULLIF(:'ids', ''), ',')) AS x
)
SELECT
  (SELECT count(*) FROM db) AS db_submitted,
  (SELECT count(*) FROM bot) AS bot_acked,
  (SELECT count(*) FROM bot b
    WHERE NOT EXISTS (
      SELECT 1 FROM assessment.attempts a
      JOIN perf USING (tenant_id)
      WHERE a.attempt_id = b.attempt_id
        AND a.deleted_at IS NULL
        AND a.status IN ('submitted', 'graded'))) AS missing;
COMMIT;
