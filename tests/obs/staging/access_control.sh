#!/usr/bin/env bash
# Staging scenario F131-AC-07 (T012; quickstart step 7). Needs a real staging + VPN — not run in CI. Grafana accepts
# only Keycloak logins of platform staff (realm role platform_staff, role_attribute_strict; no MFA — ADR-0094): a school
# admin (ad.truong.a) is refused, a platform operator gets in; production Loki queries leave an access log line
# (Grafana router log + Loki query log, both collected into Loki). Credentials come from files outside the repo:
#   EW_GRAFANA_URL, EW_LOKI_URL, EW_TENANT_USER, EW_TENANT_PASSWORD_FILE, EW_PLATFORM_USER, EW_PLATFORM_PASSWORD_FILE
set -euo pipefail
: "${EW_GRAFANA_URL:?}" "${EW_LOKI_URL:?}" "${EW_TENANT_USER:?}" "${EW_TENANT_PASSWORD_FILE:?}" "${EW_PLATFORM_USER:?}" "${EW_PLATFORM_PASSWORD_FILE:?}"
pass=0 fail=0
ok() { pass=$((pass + 1)); echo "ok   $*"; }
ko() { fail=$((fail + 1)); echo "FAIL $*" >&2; }

# login USER PASSWORD_FILE → prints the final HTTP status of Grafana after the Keycloak form (cookie jar per user).
login() {
  local jar form action
  jar=$(mktemp); form=$(mktemp)
  curl -sS -c "$jar" -b "$jar" -L -o "$form" "$EW_GRAFANA_URL/login/generic_oauth"
  action=$(grep -o 'action="[^"]*"' "$form" | head -1 | sed -e 's/^action="//' -e 's/"$//' -e 's/&amp;/\&/g')
  [ -n "$action" ] || { echo "no-form"; rm -f "$jar" "$form"; return; }
  curl -sS -c "$jar" -b "$jar" -L -o /dev/null -w '%{url_effective} %{http_code}' \
    --data-urlencode "username=$1" --data-urlencode "password@$2" "$action"
  echo
  curl -sS -b "$jar" -o /dev/null -w '%{http_code}' "$EW_GRAFANA_URL/api/user"
  rm -f "$jar" "$form"
}

out=$(login "$EW_TENANT_USER" "$EW_TENANT_PASSWORD_FILE")
[[ "$out" == *401 || "$out" == *login* ]] && ok "tenant staff refused by Grafana" || ko "tenant staff got in: $out"
out=$(login "$EW_PLATFORM_USER" "$EW_PLATFORM_PASSWORD_FILE")
[[ "$out" == *200 ]] && ok "platform operator logged in" || ko "platform operator refused: $out"

# Access log of Loki queries: the query just made through Grafana's proxy is in the Grafana router log.
since=$(date -u +%s)
sleep 30
n=$(curl -fsS -G "$EW_LOKI_URL/loki/api/v1/query_range" --data-urlencode 'query={service="grafana"} |= "/api/ds/query"' \
  --data-urlencode "start=$((since - 600))000000000" | jq '[.data.result[].values[]] | length')
[ "$n" -gt 0 ] && ok "Grafana query access log present in Loki ($n lines)" || ko "no Grafana query access log"

echo "access_control: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
