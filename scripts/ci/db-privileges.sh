#!/usr/bin/env bash
# F001-AC-11 — fail when an application role has DELETE/TRUNCATE or cross-database CONNECT.
# Usage: scripts/ci/db-privileges.sh            (local compose: runs psql inside the postgres container)
#        PGHOST=... PGPASSWORD=... scripts/ci/db-privileges.sh --direct   (CI service container)
set -euo pipefail
cd "$(dirname "$0")/../.."

run_psql() { # database < sql
  if [ "${1:-}" = "--direct" ]; then shift; psql -v ON_ERROR_STOP=1 -U "${PGUSER:-postgres}" -d "$1" -tA -F '|'
  else docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d "$1" -tA -F '|'
  fi
}

mode="${1:-}"
violations=""
for db in eduworld_identity eduworld_learning eduworld_game eduworld_competition eduworld_chat; do
  out=$(run_psql $mode "$db" < scripts/ci/db-privileges.sql)
  [ -n "$out" ] && violations+="$db: $out"$'\n'
done

if [ -n "$violations" ]; then
  echo "db-privileges: violations found (violation|role|object|privilege):" >&2
  printf '%s' "$violations" | sort -u >&2
  exit 1
fi
echo "db-privileges: ok (no DELETE/TRUNCATE on app roles, no cross-database CONNECT)"
