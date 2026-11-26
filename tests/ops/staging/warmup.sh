#!/usr/bin/env bash
# F130-AC-22 (TC-PERF-017) on staging: with a test window in scale-schedule.yaml (deployed with the ops stack), every
# scheduled service is at `peak` and healthy before the window starts.   WINDOW_START=<epoch of the test window start>
# shellcheck source=tests/ops/staging/lib.sh
. "$(dirname "$0")/lib.sh"
: "${WINDOW_START:?}"
sched="$root/deploy/swarm/staging/scale-schedule.yaml"
now=$(date +%s)
[ "$now" -lt "$WINDOW_START" ] || { echo "WINDOW_START must be in the future" >&2; exit 2; }
wait_s=$(( WINDOW_START - now - 60 ))
[ "$wait_s" -gt 0 ] && sleep "$wait_s"
while read -r name peak; do
  running=$(docker service ls --filter "name=${STACK}_$name" --format '{{.Replicas}}' | head -1)
  check "$name at peak $peak and healthy 1 min before the window ($running)" test "$running" = "$peak/$peak"
done < <(node -e 'const y=require(process.argv[1]);for(const s of y.load(require("fs").readFileSync(process.argv[2],"utf8")).services)console.log(s.name,s.peak)' \
  "$root/scripts/node_modules/js-yaml" "$sched")
finish
