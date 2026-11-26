#!/usr/bin/env bash
# Real syntax + permission check of the datastore access templates in throwaway containers (F130 T053; DEPLOYMENT §8,
# §8b, §13). TEST VALUES only (random passwords generated here, a throwaway nsc operator): nothing reaches a real
# environment.
#   1. Valkey: renders deploy/valkey/users.acl.template, starts valkey/valkey:8 with --aclfile (a test-only admin user
#      is appended to run ACL DRYRUN), checks AUTH of every user and ACL DRYRUN of allowed/denied commands per user.
#   2. NATS: scripts/deploy/nats-accounts.sh --init on a throwaway nsc store (twice: create, then rewrite in place),
#      `nats-server -t` of deploy/swarm/config/nats.conf with the rendered accounts.conf, then a server with the same
#      limits and publish/subscribe/JetStream probes with each user's creds (nats-box).
#   bash scripts/deploy/validate-acl.sh            (needs docker, node; EW_VALIDATE_PREFIX names the containers)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "$here/../.." && pwd)
P="${EW_VALIDATE_PREFIX:-ew-acl-validate}"
VALKEY_IMAGE="${VALKEY_IMAGE:-valkey/valkey:8}"
NATS_IMAGE="${NATS_IMAGE:-nats:2.10-alpine}"
NATS_BOX_IMAGE="${NATS_BOX_IMAGE:-natsio/nats-box:0.16.0}"
export NATS_BOX_IMAGE
work=$(mktemp -d); chmod 700 "$work"
host_path() { if command -v cygpath >/dev/null; then cygpath -w "$1"; else printf '%s' "$1"; fi; }
D() { MSYS_NO_PATHCONV=1 env -u DOCKER_HOST -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH docker "$@"; }
cleanup() {
  D rm -f "$P-valkey" "$P-nats" >/dev/null 2>&1 || true
  D network rm "$P-net" >/dev/null 2>&1 || true
  rm -rf "${work:?}"
}
trap cleanup EXIT
fail=0
ok() { echo "ok   $*"; }
bad() { echo "FAIL $*"; fail=1; }

# ---------------------------------------------------------------- Valkey
pw() { node -e 'process.stdout.write(require("crypto").randomBytes(18).toString("base64url"))'; }
users="core-api:core_valkey_password worker:worker_valkey_password realtime:realtime_valkey_password apisix:apisix_valkey_password replicator:valkey_masterauth sentinel:valkey_sentinel_password exporter:valkey_exporter_password"
json="{"
for u in $users; do
  v=$(pw)
  printf -v "PW_$(printf '%s' "${u%%:*}" | tr '-' '_')" '%s' "$v"
  key=${u#*:}; [ "$key" = valkey_masterauth ] && v="masterauth $v"
  json+="\"$key\":\"$v\","
done
printf '%s' "${json%,}}" > "$work/secrets.json"
mkdir -p "$work/valkey"
node "$REPO_ROOT/scripts/deploy/render-acl.mjs" valkey --secrets "$work/secrets.json" > "$work/valkey/users.acl"
ADMIN=$(pw)
printf 'user ew-validate-admin on >%s ~* &* +@all\n' "$ADMIN" >> "$work/valkey/users.acl"   # test copy only
chmod 755 "$work/valkey"; chmod 644 "$work/valkey/users.acl"
D run -d --name "$P-valkey" -v "$(host_path "$work/valkey"):/acl:ro" "$VALKEY_IMAGE" valkey-server --aclfile /acl/users.acl >/dev/null
for _ in $(seq 1 30); do D exec "$P-valkey" valkey-cli ping 2>/dev/null | grep -q NOAUTH && break; sleep 1; done
vcli() { D exec "$P-valkey" valkey-cli --no-auth-warning "$@" 2>&1; }
admin() { vcli --user ew-validate-admin --pass "$ADMIN" "$@"; }
[ "$(vcli ping)" = "NOAUTH Authentication required." ] && ok "valkey: default user disabled (NOAUTH)" || bad "valkey: unauthenticated PING not refused"
for u in core-api worker realtime apisix replicator sentinel exporter; do
  v="PW_$(printf '%s' "$u" | tr '-' '_')"
  if [ "$(vcli --user "$u" --pass "${!v}" ping)" = PONG ]; then ok "valkey: AUTH $u"; else bad "valkey: AUTH $u failed"; fi
done
allow() { local u=$1; shift; local r; r=$(admin ACL DRYRUN "$u" "$@"); if [ "$r" = OK ]; then ok "valkey: $u may $*"; else bad "valkey: $u should be allowed $* ($r)"; fi; }
deny() { local u=$1; shift; local r; r=$(admin ACL DRYRUN "$u" "$@"); if [ "$r" != OK ]; then ok "valkey: $u denied $*"; else bad "valkey: $u should be denied $*"; fi; }
for u in core-api worker; do
  allow "$u" SET ew:learning:key:q1 v
  allow "$u" GET ew:identity:principal:u1:t1
  allow "$u" HSET ew:competition:mm:ticket:t1 f v
  allow "$u" SET ew:lock:job:party-presence-sweeper w1 NX PX 5000
  allow "$u" SET ew:game:rt:ticket:abc '{}' EX 60
  allow "$u" ZADD ew:rt:rooms:game 0 r1
  allow "$u" PUBLISH ew:rt:user:u1 m
  allow "$u" PUBLISH ew:chat:fanout:c1 m
  allow "$u" SUBSCRIBE ew:chat:sensitive:version
  allow "$u" EVALSHA 0123456789012345678901234567890123456789 1 ew:chat:seq:c1
  allow "$u" SCAN 0 MATCH ew:chat:* COUNT 10
  deny "$u" GET ew:gateway:rt:conn:u1
  deny "$u" SET ew:rl:api:sub:api:r:1 1
  deny "$u" SUBSCRIBE ew:rt:node:n1:in
  deny "$u" KEYS '*'
  deny "$u" FLUSHALL
  deny "$u" FLUSHDB
  deny "$u" CONFIG GET maxmemory
  deny "$u" DEBUG SLEEP 0
  deny "$u" MONITOR
  deny "$u" CLIENT LIST
done
allow realtime HSET ew:gateway:rt:conn:u1 node n1
allow realtime ZADD ew:gateway:rt:nodes 1 n1
allow realtime GETDEL ew:game:rt:ticket:abc
allow realtime GET ew:game:rt:ticket:abc
allow realtime SUBSCRIBE ew:rt:user:u1 ew:rt:room:r1:out ew:chat:fanout:c1 ew:rt:broadcast
allow realtime PUBLISH ew:rt:node:n2:in m
deny realtime SET ew:game:rt:ticket:abc v
deny realtime DEL ew:game:rt:ticket:abc
deny realtime GET ew:learning:key:q1
deny realtime PUBLISH ew:chat:sensitive:version m
deny realtime KEYS '*'
allow apisix INCR ew:rl:api:sub1:api:r:1
allow apisix EXPIRE ew:rl:api:sub1:api:r:1 61
deny apisix GET ew:rl:api:sub1:api:r:1
deny apisix INCR ew:learning:x
allow exporter INFO
allow exporter CONFIG GET maxmemory
deny exporter GET ew:learning:key:q1
deny exporter SET a b
deny exporter CONFIG SET maxmemory 1
deny exporter KEYS '*'
allow sentinel INFO
allow sentinel PUBLISH __sentinel__:hello m
deny sentinel GET ew:learning:key:q1
allow replicator REPLCONF listening-port 6380
deny replicator GET ew:learning:key:q1

# ---------------------------------------------------------------- NATS
mkdir -p "$work/nats/store" "$work/nats/out"
bash "$REPO_ROOT/scripts/deploy/nats-accounts.sh" --store="$work/nats/store" --out="$work/nats/out" --init >/dev/null 2>"$work/nats/log1" \
  && ok "nats: nats-accounts.sh --init (operator, account, 4 users, self-check)" || { bad "nats: nats-accounts.sh --init"; cat "$work/nats/log1"; }
bash "$REPO_ROOT/scripts/deploy/nats-accounts.sh" --store="$work/nats/store" --out="$work/nats/out" >/dev/null 2>"$work/nats/log2" \
  && ok "nats: nats-accounts.sh rerun (permissions rewritten in place, self-check)" || { bad "nats: nats-accounts.sh rerun"; cat "$work/nats/log2"; }
cp "$REPO_ROOT/deploy/swarm/config/nats.conf" "$work/nats/out/nats.conf"
cp "$work/nats/out/nats_accounts.conf" "$work/nats/out/accounts.conf"
mcl=$(sed -n 's/^max_control_line: *//p' "$REPO_ROOT/deploy/swarm/config/nats.conf")
printf 'server_name: probe\nlisten: 0.0.0.0:4222\nmax_payload: 1MB\nmax_control_line: %s\njetstream { store_dir: /tmp/js }\ninclude accounts.conf\n' "${mcl:-4KB}" > "$work/nats/out/probe.conf"
chmod -R a+rX "$work/nats/out"
NOUT=$(host_path "$work/nats/out")
if D run --rm -e NATS_SERVER_NAME=nats-1 -e NATS_CLUSTER_PASSWORD=validate -v "$NOUT:/etc/nats:ro" "$NATS_IMAGE" \
  nats-server -t -c /etc/nats/nats.conf >/dev/null 2>&1; then ok "nats: nats-server -t deploy/swarm/config/nats.conf + rendered accounts.conf"
else bad "nats: nats-server -t rejected deploy/swarm/config/nats.conf"; fi
D network create "$P-net" >/dev/null
D run -d --name "$P-nats" --network "$P-net" -v "$NOUT:/etc/nats:ro" "$NATS_IMAGE" nats-server -c /etc/nats/probe.conf >/dev/null
sleep 2
# Every client is bounded by `timeout` (an allowed `sub` waits for a message that never comes).
nb() {
  local creds=$1; shift
  D run --rm --name "$P-nb-$RANDOM" --network "$P-net" -v "$NOUT:/c:ro" --entrypoint timeout "$NATS_BOX_IMAGE" 6 \
    nats -s "nats://$P-nats:4222" --creds "/c/$creds.creds" --timeout 3s "$@" 2>&1 || true
}
npass() { local c=$1; shift; local r; r=$(nb "$c" "$@"); if grep -qiE 'violation|control line|authorization' <<< "$r"; then bad "nats: ${c%%_*} should be allowed: $* ($(tail -1 <<< "$r"))"; else ok "nats: ${c%%_*} may $*"; fi; }
ndeny() { local c=$1; shift; local r; r=$(nb "$c" "$@"); if grep -qi 'permissions violation' <<< "$r"; then ok "nats: ${c%%_*} denied $*"; else bad "nats: ${c%%_*} should be denied: $* ($(tail -1 <<< "$r"))"; fi; }
npass realtime_nats_creds pub ew.cmd.chat.message.send x
npass realtime_nats_creds pub ew.in.room.r1 x
npass realtime_nats_creds pub ew.rt.presence.t1 x
ndeny realtime_nats_creds pub ew.evt.game.x.y x
ndeny realtime_nats_creds pub ew.chat.t1.msg.c1 x
ndeny realtime_nats_creds sub 'ew.cmd.>' --count 1
npass core_nats_creds sub 'ew.cmd.>' --count 1
npass core_nats_creds pub ew.int.competition.room.r1.join x
ndeny core_nats_creds pub ew.cmd.chat.message.send x
ndeny core_nats_creds pub ew.evt.identity.user.created x
ndeny core_nats_creds pub ew.rt.presence.t1 x
ndeny core_nats_creds pub '$JS.API.STREAM.CREATE.EVT_LEARNING' '{}'
npass worker_nats_creds pub '$JS.API.STREAM.CREATE.EVT_GAME' '{"name":"EVT_GAME","subjects":["ew.evt.game.>"],"storage":"memory"}'
npass worker_nats_creds pub ew.evt.game.quest.completed x
npass worker_nats_creds pub ew.int.competition.battle.open x
ndeny worker_nats_creds pub ew.cmd.game.x x
ndeny worker_nats_creds pub ew.in.room.r1 x
ndeny worker_nats_creds pub ew.evt.realtime.audit.recorded x
ndeny worker_nats_creds sub 'ew.>'
npass ops_nats_creds pub '$JS.API.STREAM.NAMES' '{}'
ndeny ops_nats_creds pub ew.evt.game.x x
exit "$fail"
