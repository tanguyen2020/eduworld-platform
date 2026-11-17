#!/usr/bin/env bash
# F001 T049 / F001-AC-20 — restore a dump made by db-backup.sh into the LOCAL module database.
# Usage: scripts/ops/db-restore.sh <module> <file.dump>
# Refuses to run unless EW_ENV is "local" (default when unset): never point this at staging/production.
set -euo pipefail
cd "$(dirname "$0")/../.."
m="${1:?usage: db-restore.sh <module> <file.dump>}"
f="${2:?usage: db-restore.sh <module> <file.dump>}"
case "$m" in identity|learning|game|competition|chat) ;; *) echo "unknown module: $m" >&2; exit 1 ;; esac
[ "${EW_ENV:-local}" = "local" ] || { echo "db-restore only runs with EW_ENV=local (got ${EW_ENV})" >&2; exit 1; }
[ -s "$f" ] || { echo "dump not found or empty: $f" >&2; exit 1; }

# --clean --if-exists drops then recreates the objects in the dump, including their grants (roles are
# cluster-wide and always exist), so app roles keep USAGE/SELECT exactly as in the source database.
docker compose -f deploy/compose/docker-compose.yml exec -T postgres \
  pg_restore -U postgres --clean --if-exists -d "eduworld_${m}" < "$f"
echo "restore: eduworld_${m} from $f"
