-- F001 T029, F001-AC-11 — run in every module database as a superuser. Each returned row is a violation.
-- 1) application roles holding DELETE or TRUNCATE on any table (chat_purger is granted later by F117
--    on the gameplay chat tables of DATABASE_DESIGN §3.0 only).
SELECT 'delete_or_truncate' AS violation, grantee AS role, table_schema || '.' || table_name AS object, privilege_type
FROM information_schema.role_table_grants
WHERE privilege_type IN ('DELETE', 'TRUNCATE')
  AND grantee ~ '^(identity|learning|game|competition|chat)_(app|platform|readonly|relay)$'
UNION ALL
SELECT 'purger_outside_gameplay_chat', grantee, table_schema || '.' || table_name, privilege_type
FROM information_schema.role_table_grants
WHERE grantee = 'chat_purger' AND privilege_type IN ('DELETE', 'TRUNCATE')
  AND (table_schema || '.' || table_name) NOT IN (
    'chat.gameplay_messages', 'chat.ephemeral_messages', 'chat.channel_members', 'chat.channels',
    'chat.message_reactions', 'chat.read_states', 'chat.message_revisions', 'chat.pinned_messages',
    'chat.gameplay_partition_channels')
UNION ALL
-- 2) any module role that may CONNECT to another module's database (or keycloak).
SELECT 'cross_database_connect', r.rolname, d.datname, 'CONNECT'
FROM pg_roles r
CROSS JOIN pg_database d
WHERE r.rolname ~ '^((identity|learning|game|competition|chat)_(app|platform|migrator|readonly|relay|partition_admin)|chat_purger)$'
  AND d.datname IN ('eduworld_identity', 'eduworld_learning', 'eduworld_game', 'eduworld_competition', 'eduworld_chat', 'keycloak')
  AND d.datname <> 'eduworld_' || CASE WHEN r.rolname = 'chat_purger' THEN 'chat' ELSE split_part(r.rolname, '_', 1) END
  AND has_database_privilege(r.rolname, d.datname, 'CONNECT');
