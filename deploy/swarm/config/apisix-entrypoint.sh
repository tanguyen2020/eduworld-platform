#!/bin/sh
# Reads the gateway secrets from Docker secret files into the process environment of APISIX (never into the service
# spec: `docker service inspect` shows only the file paths) and starts APISIX (F130-AC-08).
set -eu
read_secret() { [ -s "$2" ] || { echo "apisix: $1: secret file missing or empty" >&2; exit 1; }; export "$1=$(cat "$2")"; }
read_secret APISIX_ETCD_PASSWORD /run/secrets/apisix_etcd_password
read_secret APISIX_ADMIN_KEY /run/secrets/apisix_admin_key
read_secret APISIX_VIEWER_KEY /run/secrets/apisix_viewer_key
if [ -e /run/secrets/apisix_login_key_secret ]; then read_secret EW_APISIX_LOGIN_KEY_SECRET /run/secrets/apisix_login_key_secret; fi
if [ -e /run/secrets/apisix_valkey_password ]; then read_secret EW_APISIX_VALKEY_PASSWORD /run/secrets/apisix_valkey_password; fi
exec /docker-entrypoint.sh docker-start
