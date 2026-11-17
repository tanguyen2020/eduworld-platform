#!/usr/bin/env bash
# F001 T049 / F001-AC-20 — dump ONE module database (local) to data/backups/<module>-<UTC>.dump.
# Usage: scripts/ops/db-backup.sh <module>      (identity|learning|game|competition|chat)
# Production backups (pgBackRest, PITR) are F130 — BACKUP_RECOVERY.
set -euo pipefail
cd "$(dirname "$0")/../.."
m="${1:?usage: db-backup.sh <module>}"
case "$m" in identity|learning|game|competition|chat) ;; *) echo "unknown module: $m" >&2; exit 1 ;; esac

mkdir -p data/backups
out="data/backups/${m}-$(date -u +%Y%m%dT%H%M%SZ).dump"
docker compose -f deploy/compose/docker-compose.yml exec -T postgres \
  pg_dump -U postgres -Fc "eduworld_${m}" > "$out"
[ -s "$out" ] || { echo "backup is empty: $out" >&2; exit 1; }
echo "backup: $out"
