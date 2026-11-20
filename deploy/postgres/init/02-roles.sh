#!/usr/bin/env bash
# F001 T011 — roles per module (DATABASE_DESIGN §1.1, ADR-0074 §4, ADR-0089).
# Roles are cluster-wide objects, so they are created here (cluster bootstrap) and never by
# migrations: <module>_migrator must not hold CREATEROLE. Migrations only GRANT.
#
#   <module>_migrator        owner of the module database and its schemas (DDL)
#   <module>_app             application role, subject to RLS, no DELETE/TRUNCATE
#   <module>_platform        BYPASSRLS for cross-tenant System Admin/platform jobs, no DELETE/TRUNCATE
#   <module>_readonly        reporting/BI on *_live views (granted by F003)
#   <module>_relay           outbox relay (granted by F003)
#   <module>_partition_admin partition maintenance of infra.* (granted by F003)
#   chat_purger              gameplay chat purge only (privileges granted by F117, ADR-0033)
#   learning_dashboard       School Learning Dashboard reads on the replica: SELECT on rollups/read models only (F043,
#                            ADR-0062, DASHBOARD_DATA §7); statement_timeout 5 s, idle in transaction 10 s, work_mem 32 MB
#
# Each role may CONNECT only to its own module database (03-revoke.sql removes PUBLIC CONNECT).
set -euo pipefail

PW="${EW_DB_ROLE_PASSWORD:-change-me-local}"
KC_PW="${KEYCLOAK_DB_PASSWORD:-change-me-local}"
MODULES=(identity learning game competition chat)

psql_su() { psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${1:-postgres}"; }

create_role() { # name attributes
  psql_su <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '$1') THEN
    CREATE ROLE "$1" LOGIN $2 PASSWORD '$PW';
  END IF;
END
\$\$;
SQL
}

for m in "${MODULES[@]}"; do
  create_role "${m}_migrator"        "NOBYPASSRLS"
  create_role "${m}_app"             "NOBYPASSRLS"
  create_role "${m}_platform"        "BYPASSRLS"
  create_role "${m}_readonly"        "NOBYPASSRLS"
  create_role "${m}_relay"           "NOBYPASSRLS"
  create_role "${m}_partition_admin" "NOBYPASSRLS"

  psql_su <<SQL
ALTER DATABASE eduworld_${m} OWNER TO ${m}_migrator;
GRANT CONNECT ON DATABASE eduworld_${m} TO ${m}_app, ${m}_platform, ${m}_readonly, ${m}_relay, ${m}_partition_admin;
SQL
done

create_role chat_purger "NOBYPASSRLS"
psql_su <<SQL
GRANT CONNECT ON DATABASE eduworld_chat TO chat_purger;
SQL

# Dashboard role of module learning (DASHBOARD_DATA §7): read-only, subject to RLS; the timeouts protect the replica
# (dashboard_statement_timeout = 5 s; the query also sets it per transaction from the configuration).
create_role learning_dashboard "NOBYPASSRLS"
psql_su <<SQL
GRANT CONNECT ON DATABASE eduworld_learning TO learning_dashboard;
ALTER ROLE learning_dashboard SET statement_timeout = '5s';
ALTER ROLE learning_dashboard SET idle_in_transaction_session_timeout = '10s';
ALTER ROLE learning_dashboard SET work_mem = '32MB';
ALTER ROLE learning_dashboard SET default_transaction_read_only = on;
SQL

# identity_membership_reader: NOLOGIN, BYPASSRLS, owner of the SECURITY DEFINER function
# identity.memberships_for_user used on the login path (ADR-0089 §3, F013 T044). Its SELECT privileges
# on the membership sources are granted by the identity migration; identity_migrator is a member so the
# migration can hand the function over to it.
psql_su <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'identity_membership_reader') THEN
    CREATE ROLE identity_membership_reader NOLOGIN BYPASSRLS;
  END IF;
END
\$\$;
GRANT identity_membership_reader TO identity_migrator;
SQL

# Keycloak keeps its own database and role.
psql_su <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'keycloak') THEN
    CREATE ROLE keycloak LOGIN PASSWORD '$KC_PW';
  END IF;
END
\$\$;
ALTER DATABASE keycloak OWNER TO keycloak;
SQL
