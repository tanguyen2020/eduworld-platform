#!/usr/bin/env bash
# F004 T015 — apply realm eduworld: keycloak-config-cli (realm-eduworld.json, ${env} substitution) then
# scripts/post-apply.mjs; with EW_ENV=local also the dev data (users, organizations). Idempotent.
# Needs the compose stack (Keycloak healthy) and KC_ADMIN_PASSWORD (local: the bootstrap admin password).
set -euo pipefail
cd "$(dirname "$0")/../../.."
compose=(docker compose -f deploy/compose/docker-compose.yml -p eduworld)
if [ -z "${KC_ADMIN_PASSWORD:-}" ]; then
  KC_ADMIN_PASSWORD="$(docker exec eduworld-keycloak-1 printenv KC_BOOTSTRAP_ADMIN_PASSWORD)"
  export KC_ADMIN_PASSWORD
fi
MSYS_NO_PATHCONV=1 KC_IMPORT_FILES=/config/realm-eduworld.json "${compose[@]}" run --rm keycloak-config >/tmp/ew-kc-import.log 2>&1 || {
  grep -E "ERROR|Exception" /tmp/ew-kc-import.log >&2; exit 1; }
grep -q "keycloak-config-cli ran" /tmp/ew-kc-import.log || { echo "keycloak-config-cli did not run" >&2; exit 1; }
if grep -q "ERROR" /tmp/ew-kc-import.log; then grep "ERROR" /tmp/ew-kc-import.log >&2; exit 1; fi
node deploy/keycloak/scripts/post-apply.mjs
if [ "${EW_ENV:-local}" = "local" ]; then
  node deploy/keycloak/scripts/seed-dev.mjs
fi
echo "realm eduworld applied"
