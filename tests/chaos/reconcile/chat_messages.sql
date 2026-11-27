-- F132 T040 (F132-AC-25, also AC-10): every chat message the load bots got a JetStream ack for (chat.message.acked /
-- REST 2xx) is written by chat-persister once worker-chat is back. READ ONLY, explicit tenant filter (perf-*) and
-- soft-delete filter (AGENTS R21, R27). Persistent channels land in chat.messages, gameplay channels (match/party/team,
-- ADR-0033) in chat.gameplay_messages; the window is short, so gameplay-chat-purge cannot have run on these rows.
-- The ledger holds the client_message_id the bot generated for each acked send. Runs with the read role of module chat.
--   psql "$DSN" -X -q -A -t -F '|' -v ON_ERROR_STOP=1 \
--     -v tenant_prefix=perf- -v from=… -v to=… -v ids=<uuid,uuid,…|empty> -f tests/chaos/reconcile/chat_messages.sql
-- Output (one row): db_messages|bot_acked|missing|duplicates
--   duplicates  client_message_id written more than once (must be 0: the persister is idempotent)
BEGIN TRANSACTION READ ONLY;
WITH perf AS (
  SELECT tenant_id FROM ref.tenants WHERE code LIKE :'tenant_prefix' || '%'
), msgs AS (
  SELECT m.client_message_id FROM chat.messages m JOIN perf USING (tenant_id)
  WHERE m.deleted_at IS NULL AND m.created_at >= :'from'::timestamptz AND m.created_at < :'to'::timestamptz
  UNION ALL
  SELECT g.client_message_id FROM chat.gameplay_messages g JOIN perf USING (tenant_id)
  WHERE g.deleted_at IS NULL AND g.created_at >= :'from'::timestamptz AND g.created_at < :'to'::timestamptz
), bot AS (
  SELECT DISTINCT x::uuid AS client_message_id
  FROM unnest(string_to_array(NULLIF(:'ids', ''), ',')) AS x
)
SELECT
  (SELECT count(*) FROM msgs) AS db_messages,
  (SELECT count(*) FROM bot) AS bot_acked,
  (SELECT count(*) FROM bot b WHERE NOT EXISTS (SELECT 1 FROM msgs m WHERE m.client_message_id = b.client_message_id)) AS missing,
  (SELECT count(*) FROM (SELECT client_message_id FROM msgs WHERE client_message_id IS NOT NULL
     GROUP BY client_message_id HAVING count(*) > 1) d) AS duplicates;
COMMIT;
