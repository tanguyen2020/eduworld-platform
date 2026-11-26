#!/usr/bin/env bash
# Shared helpers of the backup/restore scripts (F130 US3/US4, BACKUP_RECOVERY §3.4, §10). Sourced, never executed.
#  - load_secret VAR FILE: Docker secret → process environment only (never argv, never logs; F130-AC-08/11).
#  - metrics_write: Prometheus textfile for node_exporter, one file per target/type, written atomically (F130-AC-13).
#  - log_json / audit: one JSON line per event on stderr/stdout, no secret, no PII (AGENTS R14).
# Runtime image: eduworld/backup-tools (deploy/backup/Dockerfile).

METRIC_DIR="${METRIC_DIR:-/var/lib/node_exporter/textfile}"
EW_BACKUP_ENV="${EW_BACKUP_ENV:-unknown}"

_json_escape() { local s=${1//\/\\}; s=${s//\"/\\\"}; s=${s//$'\n'/\n}; s=${s//$'\r'/}; s=${s//$'\t'/\t}; printf '%s' "$s"; }

# log_json LEVEL MESSAGE [key=value...]
log_json() {
  local level=$1 msg=$2; shift 2
  local out kv
  out="{\"ts\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"level\":\"$level\",\"script\":\"$(basename "$0")\",\"env\":\"$(_json_escape "$EW_BACKUP_ENV")\",\"msg\":\"$(_json_escape "$msg")\""
  for kv in "$@"; do out+=",\"$(_json_escape "${kv%%=*}")\":\"$(_json_escape "${kv#*=}")\""; done
  printf '%s}\n' "$out" >&2
}

die() { log_json error "$*"; exit 1; }

# audit ACTION [key=value...] — the record of a restore/deploy action (actor, env, purpose…) on stdout (F130-AC-15).
audit() {
  local action=$1; shift
  local out kv
  out="{\"ts\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"audit\":true,\"action\":\"$(_json_escape "$action")\",\"actor\":\"$(_json_escape "${EW_ACTOR:-${GITHUB_ACTOR:-${USER:-unknown}}}")\",\"env\":\"$(_json_escape "$EW_BACKUP_ENV")\""
  for kv in "$@"; do out+=",\"$(_json_escape "${kv%%=*}")\":\"$(_json_escape "${kv#*=}")\""; done
  printf '%s}\n' "$out"
}

# load_secret VAR FILE — exports the trimmed content of FILE as VAR; missing/empty file stops naming VAR only.
load_secret() {
  local var=$1 file=$2 val
  [ -s "$file" ] || die "$var: secret file missing or empty"
  val=$(cat "$file")
  val="${val%"${val##*[![:space:]]}"}"
  [ -n "$val" ] || die "$var: secret file missing or empty"
  export "$var=$val"
}

# metrics_write TARGET TYPE STATUS START END [extra metric lines...]
# STATUS 1 = success, 0 = failure. Every series carries target and type, so the files of different jobs never clash.
metrics_write() {
  local target=$1 type=$2 status=$3 start=$4 end=$5; shift 5
  local file="$METRIC_DIR/eduworld_backup_${target}_${type}.prom" labels="target=\"$target\",type=\"$type\""
  mkdir -p "$METRIC_DIR"
  local prev_success=""
  if [ "$status" != 1 ] && [ -f "$file" ]; then
    prev_success=$(grep -E '^eduworld_backup_last_success_timestamp' "$file" || true)
  fi
  {
    echo "# HELP eduworld_backup_last_run_timestamp Unix time of the last run of the backup job (F130-AC-13)."
    echo "# TYPE eduworld_backup_last_run_timestamp gauge"
    echo "eduworld_backup_last_run_timestamp{$labels} $end"
    echo "# HELP eduworld_backup_last_status 1 when the last run succeeded, 0 when it failed."
    echo "# TYPE eduworld_backup_last_status gauge"
    echo "eduworld_backup_last_status{$labels} $status"
    echo "# HELP eduworld_backup_duration_seconds Duration of the last run."
    echo "# TYPE eduworld_backup_duration_seconds gauge"
    echo "eduworld_backup_duration_seconds{$labels} $((end - start))"
    echo "# HELP eduworld_backup_last_success_timestamp Unix time of the last successful run (BackupMissing, MONITORING §5)."
    echo "# TYPE eduworld_backup_last_success_timestamp gauge"
    if [ "$status" = 1 ]; then
      echo "eduworld_backup_last_success_timestamp{$labels} $end"
    elif [ -n "$prev_success" ]; then
      echo "$prev_success"
    fi
    local line
    for line in "$@"; do echo "$line"; done
  } > "$file.tmp.$$"
  mv -f "$file.tmp.$$" "$file"
}

# rclone_s3_remote NAME ENDPOINT REGION ACCESS_KEY_VAR SECRET_KEY_VAR — configures an S3 remote through the
# environment (standard S3 API, SigV4, path-style: works with SeaweedFS and any managed S3, ADR-0093).
rclone_s3_remote() {
  local n; n=$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')
  export "RCLONE_CONFIG_${n}_TYPE=s3" "RCLONE_CONFIG_${n}_PROVIDER=Other" "RCLONE_CONFIG_${n}_ENDPOINT=$2" \
    "RCLONE_CONFIG_${n}_REGION=$3" "RCLONE_CONFIG_${n}_FORCE_PATH_STYLE=true" "RCLONE_CONFIG_${n}_ENV_AUTH=false" \
    "RCLONE_CONFIG_${n}_NO_CHECK_BUCKET=true"
  export "RCLONE_CONFIG_${n}_ACCESS_KEY_ID=${!4}" "RCLONE_CONFIG_${n}_SECRET_ACCESS_KEY=${!5}"
}

# rclone_crypt_remote NAME UNDERLYING_REMOTE:PATH PASSWORD_VAR — client-side encryption at rest (BACKUP_RECOVERY §3.3, §6).
rclone_crypt_remote() {
  local n; n=$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')
  local obscured; obscured=$(printf '%s' "${!3}" | rclone obscure -)
  export "RCLONE_CONFIG_${n}_TYPE=crypt" "RCLONE_CONFIG_${n}_REMOTE=$2" "RCLONE_CONFIG_${n}_PASSWORD=$obscured" \
    "RCLONE_CONFIG_${n}_FILENAME_ENCRYPTION=off" "RCLONE_CONFIG_${n}_DIRECTORY_NAME_ENCRYPTION=false"
}

# load_repo_write_credentials — S3 write credential + cipher of the backup repo (pgBackRest reads PGBACKREST_*).
load_repo_write_credentials() {
  load_secret PGBACKREST_REPO1_S3_KEY "${EW_SECRETS_DIR:-/run/secrets}/backup_repo_write_access_key"
  load_secret PGBACKREST_REPO1_S3_KEY_SECRET "${EW_SECRETS_DIR:-/run/secrets}/backup_repo_write_secret_key"
  load_secret PGBACKREST_REPO1_CIPHER_PASS "${EW_SECRETS_DIR:-/run/secrets}/backup_repo_cipher_pass"
}
