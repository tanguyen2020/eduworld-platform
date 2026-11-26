#!/usr/bin/env bash
# Integration tests of the F130 backup/restore/runbook scripts against THROWAWAY containers (F130-AC-12…AC-17, AC-20 b,
# AC-23 proxy part). Real images of F130 (backup-tools, postgres-patroni) with the committed pgBackRest configuration;
# S3, the second site, the new RB-05 cluster and the Swarm manager are STAND-INS (compose.yml, R17).
#   tests/ops/integration/run.sh [--keep]        (Docker required; unique project name; removes its containers/volumes)
# On this shared machine run it through the test lock: bash /d/game/tc-lock.sh run f130 -- tests/ops/integration/run.sh
set -uo pipefail
# Git Bash (Windows): never rewrite container paths; host paths are converted explicitly with hp().
export MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL="*"
hp() { if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf "%s" "$1"; fi; }
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
KEEP=false; [ "${1:-}" = "--keep" ] && KEEP=true
P="ew-f130-it-$(date +%s)-$RANDOM"
export COMPOSE_PROJECT_NAME="$P"
DC=(docker compose -p "$P" -f "$(hp "$here/compose.yml")")
pass=0 fail=0 results=()
ok() { pass=$((pass + 1)); results+=("PASS $1"); echo "PASS $1"; }
ko() { fail=$((fail + 1)); results+=("FAIL $1"); echo "FAIL $1" >&2; }
check() { local name=$1; shift; if "$@"; then ok "$name"; else ko "$name"; fi; }
bt() { "${DC[@]}" exec -T backup-tools "$@"; }
psql_pg() { "${DC[@]}" exec -T pg psql -v ON_ERROR_STOP=1 -U postgres -h /var/run/postgresql -tAX "$@"; }
cleanup() {
  if [ "$KEEP" = true ]; then echo "kept project $P"; return; fi
  "${DC[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

if [ "${EW_IT_SKIP_BUILD:-}" != 1 ]; then
  echo "== build images (backup-tools, postgres-patroni) =="
  docker build -q -f "$(hp "$root/deploy/backup/Dockerfile")" --build-arg PG_IMAGE="${PG_IMAGE:-postgres:17.12-trixie}" --build-arg VERSION=it -t ew-f130/backup-tools:test "$(hp "$root")" >/dev/null || exit 1
  docker build -q -f "$(hp "$root/deploy/postgres/patroni.Dockerfile")" --build-arg PG_IMAGE="${PG_IMAGE:-postgres:17.12-trixie}" --build-arg VERSION=it -t ew-f130/postgres-patroni:test "$(hp "$root")" >/dev/null || exit 1
fi

echo "== certificates and test secrets (generated in the volumes, never on the host) =="
"${DC[@]}" create --quiet-pull >/dev/null 2>&1 || "${DC[@]}" create >/dev/null || exit 1
vol() { echo "${P}_$1"; }
docker run --rm -u 0 -v "$(vol certs-server):/s" -v "$(vol certs-client):/c" -v "$(vol certs-manager):/m" -v "$(vol certs-proxy):/p" \
  -v "$(vol restore-creds):/r" -v "$(vol restore):/restore" ew-f130/backup-tools:test bash -euc '
  cd /tmp
  gen() { openssl req -new -newkey rsa:2048 -nodes -keyout "$1.key" -out "$1.csr" -subj "/CN=$2" 2>/dev/null
          printf "subjectAltName=%s\n" "$3" > "$1.ext"
          openssl x509 -req -in "$1.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -days 2 -out "$1.crt" -extfile "$1.ext" 2>/dev/null; }
  openssl req -x509 -newkey rsa:2048 -nodes -keyout ca.key -out ca.crt -days 2 -subj "/CN=ew-f130-it-ca" 2>/dev/null
  gen server pgbackrest-1 "DNS:pgbackrest-1,DNS:pgbackrest-2,DNS:pgbackrest-3,DNS:s3-tls"
  gen client backup-tools "DNS:backup-tools"
  gen manager fake-manager "DNS:fake-manager"
  gen proxy docker-api-proxy "DNS:docker-api-proxy"
  cp ca.crt /s/backup_pgbackrest_ca; cp server.crt /s/backup_pgbackrest_server_cert; cp server.key /s/backup_pgbackrest_server_key
  cp server.crt /s/nginx.crt; cp server.key /s/nginx.key
  chown 999:999 /s/backup_pgbackrest_*; chmod 0400 /s/backup_pgbackrest_server_key; chmod 0444 /s/nginx.*
  cp ca.crt /c/backup_pgbackrest_ca_client; cp client.crt /c/backup_pgbackrest_client_cert; cp client.key /c/backup_pgbackrest_client_key
  for k in backup_repo_write_access_key backup_repo_write_secret_key backup_source_access_key backup_source_secret_key \
           backup_offsite_access_key backup_offsite_secret_key; do printf test > "/c/$k"; done
  printf it-only-cipher-passphrase > /c/backup_repo_cipher_pass
  printf it-only-offsite-crypt-password > /c/backup_offsite_crypt_password
  printf "pg:5432:*:backup_reader:it-only-reader\n" > /c/backup_pgpass
  printf "pg:5432:*:postgres:it-only-superuser\n" > /c/ops_pg_admin_pgpass
  printf "%s\n" test > /c/new_target_access_key; printf "%s\n" test > /c/new_target_secret_key
  chown -R 10001:10001 /c; chmod 0400 /c/backup_pgbackrest_client_key /c/backup_pgpass /c/ops_pg_admin_pgpass
  printf test > /r/backup_restore_access_key; printf test > /r/backup_restore_secret_key; chown -R 10001:10001 /r
  chown 10001:10001 /restore
  cp ca.crt /m/ca.crt; cp manager.crt /m/server.crt; cp manager.key /m/server.key; chmod 0444 /m/*
  cat ca.crt > /p/ca.pem; cat proxy.crt proxy.key > /p/client.pem; chmod 0444 /p/*
  cat > /m/fake_manager.py <<PY
# STAND-IN for the Swarm manager Engine API (mTLS): answers 200 with the request line for any call.
import http.server, json, ssl
class H(http.server.BaseHTTPRequestHandler):
    def _r(self):
        b = json.dumps({"method": self.command, "path": self.path}).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.send_header("Content-Length", str(len(b)))
        self.end_headers(); self.wfile.write(b)
    do_GET = do_POST = do_DELETE = do_PUT = _r
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); ctx.load_cert_chain("/certs/server.crt", "/certs/server.key")
ctx.verify_mode = ssl.CERT_REQUIRED; ctx.load_verify_locations("/certs/ca.crt")
s = http.server.HTTPServer(("0.0.0.0", 2376), H); s.socket = ctx.wrap_socket(s.socket, server_side=True); s.serve_forever()
PY
' || { echo "certificate setup failed" >&2; exit 1; }

echo "== committed configuration into the containers =="
tmp=$(mktemp -d)
cat > "$tmp/nginx.conf" <<'NGINX'
server {
  listen 443 ssl;
  ssl_certificate /certs/nginx.crt;
  ssl_certificate_key /certs/nginx.key;
  client_max_body_size 0;
  proxy_request_buffering off;
  location / { proxy_pass http://s3-repo:4566; proxy_set_header Host $http_host; proxy_http_version 1.1; }
}
NGINX
# Repo host: the committed production configuration minus the pg2/pg3 hosts this stand-in does not have.
awk '!/^pg[23]-/' "$root/deploy/backup/pgbackrest.conf" > "$tmp/pgbackrest-repo.conf"
cp_in() { docker cp "$(hp "$1")" "$("${DC[@]}" ps -a -q "$2")":"$3"; }
cp_in "$tmp/nginx.conf" s3-tls /etc/nginx/conf.d/default.conf
cp_in "$root/deploy/backup/pgbackrest-db.conf" pg /etc/pgbackrest/pgbackrest.conf
cp_in "$root/deploy/backup/pgbackrest-server.conf" pgbackrest-1 /etc/pgbackrest/pgbackrest.conf
cp_in "$tmp/pgbackrest-repo.conf" backup-tools /etc/pgbackrest/pgbackrest.conf
cp_in "$root/deploy/ops-scheduler/socket-proxy.cfg" docker-api-proxy /usr/local/etc/haproxy/haproxy.cfg
"${DC[@]}" start >/dev/null || exit 1
for _ in $(seq 1 60); do psql_pg -c 'SELECT 1' >/dev/null 2>&1 && break; sleep 2; done
ARCHIVE_ON="ALTER SYSTEM SET archive_command = 'pgbackrest --stanza=eduworld archive-push %p'"
psql_pg -d postgres -c "$ARCHIVE_ON" -c "SELECT pg_reload_conf()" >/dev/null || ko "set archive_command"

echo "== buckets and schema =="
bt bash -c 'export RCLONE_CONFIG_R_TYPE=s3 RCLONE_CONFIG_R_PROVIDER=Other RCLONE_CONFIG_R_ENDPOINT=http://s3-repo:4566 RCLONE_CONFIG_R_ACCESS_KEY_ID=test RCLONE_CONFIG_R_SECRET_ACCESS_KEY=test RCLONE_CONFIG_R_FORCE_PATH_STYLE=true
  for _ in $(seq 1 30); do rclone mkdir r:eduworld-backups 2>/dev/null && break; sleep 1; done; rclone mkdir r:eduworld-objects-offsite' || ko "create repo buckets"
psql_pg -d postgres <<'SQL' >/dev/null
CREATE DATABASE eduworld_identity; CREATE DATABASE eduworld_learning; CREATE DATABASE eduworld_game;
CREATE DATABASE eduworld_competition; CREATE DATABASE eduworld_chat; CREATE DATABASE keycloak;
CREATE ROLE learning_app LOGIN PASSWORD 'it-only-app';
CREATE ROLE backup_reader LOGIN PASSWORD 'it-only-reader'; GRANT pg_read_all_data TO backup_reader;
GRANT CONNECT ON DATABASE eduworld_learning TO learning_app;
SQL
for db in eduworld_identity eduworld_learning eduworld_game eduworld_competition eduworld_chat; do
  psql_pg -d "$db" -c "CREATE SCHEMA atlas_schema_revisions; CREATE TABLE atlas_schema_revisions.atlas_schema_revisions (version varchar(255) PRIMARY KEY); INSERT INTO atlas_schema_revisions.atlas_schema_revisions VALUES ('20261201000001');" >/dev/null
done
psql_pg -d eduworld_learning <<'SQL' >/dev/null
CREATE SCHEMA assessment;
CREATE TABLE assessment.attempts (attempt_id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL DEFAULT gen_random_uuid(),
  status varchar(64) NOT NULL DEFAULT 'submitted', submitted_at timestamptz, deleted_at timestamptz, note varchar(32));
INSERT INTO assessment.attempts (submitted_at, note) SELECT now() - interval '2 hours', 'before-full' FROM generate_series(1, 50);
SQL

echo "== US3: stanza, full backup, WAL, differential, logical dumps, check (F130-AC-12, AC-13) =="
check "pgBackRest stanza-create over TLS server + S3 repo (encrypted)" bt bash -c '. /opt/eduworld/backup/lib-backup.sh; load_repo_write_credentials; pgbackrest --stanza=eduworld stanza-create'
check "pg_backup.sh full → status 1" bt pg_backup.sh full
check "metric full: last_success_timestamp{target=postgres,type=full}" bt grep -q '^eduworld_backup_last_success_timestamp{target="postgres",type="full"} [0-9]' /var/lib/node_exporter/textfile/eduworld_backup_postgres_full.prom
psql_pg -d eduworld_learning -c "INSERT INTO assessment.attempts (submitted_at, note) SELECT now() - interval '90 minutes', 'before-T' FROM generate_series(1, 30);" >/dev/null
sleep 3
T=$(psql_pg -d postgres -c "SELECT to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') || '+00'")
sleep 3
psql_pg -d eduworld_learning -c "INSERT INTO assessment.attempts (submitted_at, note) SELECT now(), 'after-T' FROM generate_series(1, 20);" >/dev/null
psql_pg -d postgres -c "SELECT pg_switch_wal();" >/dev/null
check "pg_backup.sh diff → status 1 (pgbackrest check: WAL archived)" bt pg_backup.sh diff
check "pgbackrest info shows full + diff, repo encrypted" bt bash -c '. /opt/eduworld/backup/lib-backup.sh; load_repo_write_credentials; i=$(pgbackrest --stanza=eduworld --output=json info); echo "$i" | jq -e "[.[0].backup[].type] | index(\"full\") != null and index(\"diff\") != null" >/dev/null && echo "$i" | jq -e ".[0].cipher == \"aes-256-cbc\"" >/dev/null'
check "WAL archive reachable from the repo (archive min/max present)" bt bash -c '. /opt/eduworld/backup/lib-backup.sh; load_repo_write_credentials; pgbackrest --stanza=eduworld --output=json info | jq -e ".[0].archive[0].max != null" >/dev/null'
check "pg_logical_dump.sh → one encrypted pg_dump per module database" bt env EW_BACKUP_DATABASES="eduworld_identity eduworld_learning eduworld_game eduworld_competition eduworld_chat" PGBACKREST_REPO1_S3_REGION=us-east-1 pg_logical_dump.sh
check "logical dumps stored under logical/<database>/<day>.dump" bt bash -c 'export RCLONE_CONFIG_R_TYPE=s3 RCLONE_CONFIG_R_PROVIDER=Other RCLONE_CONFIG_R_ENDPOINT=http://s3-repo:4566 RCLONE_CONFIG_R_ACCESS_KEY_ID=test RCLONE_CONFIG_R_SECRET_ACCESS_KEY=test RCLONE_CONFIG_R_FORCE_PATH_STYLE=true; [ "$(rclone lsf -R r:eduworld-backups/logical | grep -cE "\.dump(\.bin)?$")" = 5 ]'

echo "== AC-13: WAL archiving stuck → check fails, metric status 0 =="
psql_pg -d postgres -c "ALTER SYSTEM SET archive_command = 'false'" -c "SELECT pg_reload_conf()" >/dev/null
sleep 1
if bt env PGBACKREST_ARCHIVE_TIMEOUT=10 pg_check.sh >/dev/null 2>&1; then ko "pg_check.sh fails while archiving is stuck"; else ok "pg_check.sh fails while archiving is stuck"; fi
check "metric check status 0 while archiving is stuck" bt grep -q 'eduworld_backup_last_status{target="postgres",type="check"} 0' /var/lib/node_exporter/textfile/eduworld_backup_postgres_check.prom
psql_pg -d postgres -c "$ARCHIVE_ON" -c "SELECT pg_reload_conf()" >/dev/null
sleep 1
check "pg_check.sh recovers once archiving works again" bt pg_check.sh

echo "== US4: restore tests in an isolated scratch cluster (F130-AC-16, AC-15 report) at T=$T =="
check "restore_test.sh --mode=full --target=T → PASS (reconciliation restored == live)" \
  bt restore_test.sh --mode=full --target="$T" --actor=it-runner --purpose="integration full PITR"
check "restore_test.sh --mode=module --db=eduworld_learning --target=T → PASS" \
  bt restore_test.sh --mode=module --db=eduworld_learning --target="$T" --actor=it-runner --purpose="integration module PITR"
check "restore report has actor, purpose, T and PASS for both runs" bt bash -c 'f=$(ls /restore/reports/*.md); [ "$(grep -c "| Kết quả | \*\*PASS\*\* |" "$f")" = 2 ] && grep -q "| Người thực hiện | it-runner |" "$f" && grep -q "| Mục đích | integration module PITR |" "$f"'
check "scratch cluster removed after the restore test" bt bash -c '[ -z "$(ls -d /restore/run-* 2>/dev/null)" ]'
check "restore test with the write credential only is refused (restore credential required)" \
  bt bash -c '! env EW_RESTORE_CREDENTIALS_DIR=/nonexistent restore_test.sh --mode=full --target="'"$T"'" --actor=a --purpose=p >/dev/null 2>&1'

echo "== AC-17: RB-01 restore of eduworld_learning to T into a new database, then swap (no data dropped) =="
check "rb01 --phase=restore → eduworld_learning_restored_<stamp> created" \
  bt rb01-pitr-restore.sh --phase=restore --db=eduworld_learning --target="$T" --actor=it-runner --purpose="integration RB-01"
restored=$(psql_pg -d postgres -c "SELECT datname FROM pg_database WHERE datname LIKE 'eduworld_learning_restored_%' ORDER BY 1 DESC LIMIT 1")
check "restored database stops exactly at T (80 rows: before-full + before-T, no after-T)" \
  test "$(psql_pg -d "$restored" -c "SELECT count(*) FILTER (WHERE note <> 'after-T') || '/' || count(*) FILTER (WHERE note = 'after-T') FROM assessment.attempts")" = "80/0"
check "database privileges copied (learning_app may CONNECT)" \
  test "$(psql_pg -d postgres -c "SELECT has_database_privilege('learning_app', '$restored', 'CONNECT')")" = t
check "rb01 --phase=swap → live eduworld_learning is the restored copy, the old one kept as _pre_" \
  bt rb01-pitr-restore.sh --phase=swap --db=eduworld_learning --restored="$restored" --actor=it-runner --purpose="integration RB-01" --yes
check "after swap: live has 80 rows, eduworld_learning_pre_* still has 100 (nothing deleted)" bash -c '
  live=$("$@" -d eduworld_learning -c "SELECT count(*) FROM assessment.attempts")
  pre=$("$@" -d postgres -c "SELECT datname FROM pg_database WHERE datname LIKE '"'"'eduworld_learning_pre_%'"'"'")
  old=$("$@" -d "$pre" -c "SELECT count(*) FROM assessment.attempts")
  [ "$live" = 80 ] && [ "$old" = 100 ]' _ "${DC[@]}" exec -T pg psql -U postgres -h /var/run/postgresql -tAX
check "other module databases untouched by the module restore" test "$(psql_pg -d eduworld_game -c 'SELECT count(*) FROM atlas_schema_revisions.atlas_schema_revisions')" = 1

echo "== US3/US4: object storage — versioning, off-site copy, replication lag, RB-05 (F130-AC-14, AC-20) =="
bt bash -c '. /opt/eduworld/backup/lib-backup.sh
  export K=test; rclone_s3_remote src http://s3-primary:4566 us-east-1 K K; export RCLONE_CONFIG_SRC_NO_CHECK_BUCKET=false
  for b in ew-media-quarantine ew-media-public ew-media-private ew-game-bundles ew-exports; do rclone mkdir "src:$b"; done
  for i in 1 2 3; do printf "media-$i-content" > /tmp/o$i; s=$(sha256sum /tmp/o$i | cut -d" " -f1)
    rclone copyto /tmp/o$i "src:ew-media-public/media/$s/original.png"; printf "thumb-$i" | rclone rcat "src:ew-media-public/media/$s/thumb.webp"
    printf "private-$i" > /tmp/p$i; p=$(sha256sum /tmp/p$i | cut -d" " -f1); rclone copyto /tmp/p$i "src:ew-media-private/media/$p/original.pdf"; done
  printf bundle | rclone rcat src:ew-game-bundles/bundles/v1/game.pck
  printf infected | rclone rcat src:ew-media-quarantine/upload/x.bin' >/dev/null || ko "seed object storage"
check "object_storage_setup.sh enables versioning on public/private/bundles (SigV4 S3 API)" bt object_storage_setup.sh
check "object_storage_backup.sh → off-site encrypted copy verified" bt env EW_BACKUP_BUCKETS="ew-media-public ew-media-private ew-game-bundles ew-media-quarantine" object_storage_backup.sh
check "off-site copy holds no ew-media-quarantine object and a manifest per bucket" bt bash -c 'export RCLONE_CONFIG_R_TYPE=s3 RCLONE_CONFIG_R_PROVIDER=Other RCLONE_CONFIG_R_ENDPOINT=http://s3-repo:4566 RCLONE_CONFIG_R_ACCESS_KEY_ID=test RCLONE_CONFIG_R_SECRET_ACCESS_KEY=test RCLONE_CONFIG_R_FORCE_PATH_STYLE=true
  l=$(rclone lsf -R r:eduworld-objects-offsite); ! echo "$l" | grep -q quarantine && [ "$(echo "$l" | grep -c "manifests/.*\.sha256$")" = 3 ]'
check "off-site objects are encrypted at rest (raw bytes differ from the source)" bt bash -c 'export RCLONE_CONFIG_R_TYPE=s3 RCLONE_CONFIG_R_PROVIDER=Other RCLONE_CONFIG_R_ENDPOINT=http://s3-repo:4566 RCLONE_CONFIG_R_ACCESS_KEY_ID=test RCLONE_CONFIG_R_SECRET_ACCESS_KEY=test RCLONE_CONFIG_R_FORCE_PATH_STYLE=true
  f=$(rclone lsf -R r:eduworld-objects-offsite/object-storage/current/ew-game-bundles | grep pck); ! rclone cat "r:eduworld-objects-offsite/object-storage/current/ew-game-bundles/$f" | grep -q "^bundle$"'
check "metric object_storage/offsite status 1" bt grep -q 'eduworld_backup_last_status{target="object_storage",type="offsite"} 1' /var/lib/node_exporter/textfile/eduworld_backup_object_storage_offsite.prom
check "replication lag of ew-media-private ≤ RPO (stand-in replicator)" bt env EW_RPO_SECONDS=300 EW_REPLICATION_POLL_SECONDS=1 object_storage_replication_check.sh
check "replication lag metric exported" bt grep -q '^eduworld_object_storage_replication_lag_seconds{bucket="ew-media-private"} [0-9]' /var/lib/node_exporter/textfile/eduworld_backup_object_storage_replication.prom
bt bash -c '. /opt/eduworld/backup/lib-backup.sh; export K=test; rclone_s3_remote src http://s3-primary:4566 us-east-1 K K
  for b in ew-media-public ew-media-private ew-game-bundles; do rclone lsf -R --files-only "src:$b" | grep -v "^_replication-canary/" | sed "s#^#$b #"; done' > "$tmp/referenced.txt"
cp_in "$tmp/referenced.txt" backup-tools /restore/referenced.txt
check "RB-05: new cluster restored from replica (private) + off-site copy (others), 100 % referenced objects verified" \
  bt env EW_TARGET_URL=http://s3-new:4566 EW_TARGET_ACCESS_KEY_FILE=/run/secrets/new_target_access_key EW_TARGET_SECRET_KEY_FILE=/run/secrets/new_target_secret_key \
     EW_REPLICA_ACCESS_KEY_FILE=/run/secrets/backup_source_access_key EW_REPLICA_SECRET_KEY_FILE=/run/secrets/backup_source_secret_key \
     rb05-object-storage-restore.sh --referenced=/restore/referenced.txt --buckets="ew-media-public ew-media-private ew-game-bundles" --yes

echo "== AC-23: Docker API allowlist proxy (HAProxy config of the ops stack) in front of an mTLS manager stand-in =="
code() { bt curl -s -o /dev/null -w '%{http_code}' -X "$1" "http://docker-api-proxy:2375$2"; }
for _ in $(seq 1 20); do [ "$(code GET /v1.44/services)" = 200 ] && break; sleep 1; done
check "GET /v1.44/services allowed" test "$(code GET /v1.44/services)" = 200
check "GET /nodes allowed" test "$(code GET /nodes)" = 200
check "GET /v1.44/tasks allowed" test "$(code GET /v1.44/tasks)" = 200
check "POST /v1.44/services/abc123/update allowed" test "$(code POST /v1.44/services/abc123/update)" = 200
check "POST /v1.44/services/create denied 403" test "$(code POST /v1.44/services/create)" = 403
check "DELETE /v1.44/services/abc123 denied 403" test "$(code DELETE /v1.44/services/abc123)" = 403
check "GET /v1.44/secrets denied 403" test "$(code GET /v1.44/secrets)" = 403
check "POST /containers/x/exec denied 403" test "$(code POST /containers/x/exec)" = 403
check "GET /v1.44/services/abc123 (inspect, not in the allowlist) denied 403" test "$(code GET /v1.44/services/abc123)" = 403

echo
echo "== summary ($P) =="
printf '%s\n' "${results[@]}"
echo "integration: $pass passed, $fail failed"
[ "$fail" = 0 ]
