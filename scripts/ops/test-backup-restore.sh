#!/usr/bin/env bash
# F001 T048 / F001-AC-20 — backup one module database, lose data, restore, compare.
# Requires the local stack (`make up`, `make migrate`). Uses a probe table in eduworld_identity.
set -euo pipefail
cd "$(dirname "$0")/../.."
PSQL=(docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d eduworld_identity -tAq)

"${PSQL[@]}" -c "SET ROLE identity_migrator; CREATE TABLE IF NOT EXISTS infra.f001_backup_probe (id int PRIMARY KEY); INSERT INTO infra.f001_backup_probe SELECT generate_series(1, 42) ON CONFLICT DO NOTHING;"
before_rows=$("${PSQL[@]}" -c "SELECT count(*) FROM infra.f001_backup_probe")
before_rev=$("${PSQL[@]}" -c "SELECT max(version) FROM atlas_schema_revisions.atlas_schema_revisions")

dump=$(bash scripts/ops/db-backup.sh identity | sed -n 's/^backup: //p')
[ -f "$dump" ] || { echo "backup file missing: $dump" >&2; exit 1; }

# Data loss
"${PSQL[@]}" -c "SET ROLE identity_migrator; DROP TABLE infra.f001_backup_probe;"

bash scripts/ops/db-restore.sh identity "$dump"

after_rows=$("${PSQL[@]}" -c "SELECT count(*) FROM infra.f001_backup_probe")
after_rev=$("${PSQL[@]}" -c "SELECT max(version) FROM atlas_schema_revisions.atlas_schema_revisions")
# Grants must survive the restore, otherwise the binaries lose readiness (atlas:<module>, schema USAGE).
grants=$("${PSQL[@]}" -c "SELECT has_schema_privilege('identity_app','infra','USAGE') AND has_schema_privilege('identity_app','atlas_schema_revisions','USAGE') AND has_table_privilege('identity_app','atlas_schema_revisions.atlas_schema_revisions','SELECT')")
[ "$grants" = "t" ] || { "${PSQL[@]}" -c "SET ROLE identity_migrator; DROP TABLE IF EXISTS infra.f001_backup_probe;"; echo "restore lost grants of identity_app" >&2; exit 1; }

"${PSQL[@]}" -c "SET ROLE identity_migrator; DROP TABLE infra.f001_backup_probe;"

if [ "$before_rows" != "$after_rows" ] || [ "$before_rev" != "$after_rev" ]; then
  echo "restore mismatch: rows $before_rows→$after_rows, revision $before_rev→$after_rev" >&2; exit 1
fi
echo "backup/restore: ok (rows $after_rows, revision $after_rev, dump $(basename "$dump"))"
