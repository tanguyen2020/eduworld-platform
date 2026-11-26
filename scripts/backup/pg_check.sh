#!/usr/bin/env bash
# Daily `pgbackrest check` on its own (F130-AC-13, BACKUP_RECOVERY §3.4): fails when WAL archiving is stuck or the
# repo is unreachable, independently of the nightly backup. Metric target="postgres",type="check".
set -euo pipefail
# shellcheck source=scripts/backup/lib-backup.sh
. "$(dirname "$0")/lib-backup.sh"
STANZA="${PGBACKREST_STANZA:-eduworld}"
load_repo_write_credentials
start=$(date +%s)
status=1
"${PGBACKREST_BIN:-pgbackrest}" --stanza="$STANZA" check || status=0
metrics_write postgres check "$status" "$start" "$(date +%s)"
[ "$status" = 1 ] || die "pgbackrest check failed"
log_json info "pgbackrest check ok" stanza="$STANZA"
