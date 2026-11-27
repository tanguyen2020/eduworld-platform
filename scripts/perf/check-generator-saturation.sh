#!/usr/bin/env bash
# Load-generator saturation (F132 edge case, perf_generator_cpu_max): a run whose load generator went above the CPU
# limit measured the generator, not the target, and is marked invalid by render-report.mjs. Two sources:
#   start <dir> [interval_s]   sample /proc/stat of THIS runner in the background (pid in <dir>/.sampler.pid)
#   stop <dir>                 stop the sampler and write <dir>/generator.json {source, samples, max_cpu}
#   prom <url> <start> <end> <dir>
#                              max CPU of every runner VM scraped as job="loadgen" (node_exporter) over the window
#   cpu                        print one CPU busy ratio sample (1 s) and exit (self-test)
# max_cpu is a ratio 0..1 (busy = 1 - (idle + iowait) / total).
set -euo pipefail

read_stat() { # prints "busy total" of the aggregated cpu line
  local _cpu user nice system idle iowait irq softirq steal rest
  read -r _cpu user nice system idle iowait irq softirq steal rest < /proc/stat
  : "${rest:=}"
  local total=$((user + nice + system + idle + ${iowait:-0} + ${irq:-0} + ${softirq:-0} + ${steal:-0}))
  echo "$((total - idle - ${iowait:-0})) ${total}"
}

ratio() { # busy1 total1 busy2 total2 → ratio with 3 decimals
  awk -v b1="$1" -v t1="$2" -v b2="$3" -v t2="$4" 'BEGIN { d = t2 - t1; if (d <= 0) { print "0.000" } else { printf "%.3f\n", (b2 - b1) / d } }'
}

sample_loop() { # interval out
  local interval="$1" out="$2" prev cur
  prev=$(read_stat)
  while true; do
    sleep "$interval"
    cur=$(read_stat)
    # shellcheck disable=SC2086 # word splitting of "busy total" is intended
    ratio $prev $cur >> "$out"
    prev="$cur"
  done
}

write_json() { # dir source samples_file
  local dir="$1" source="$2" file="$3"
  awk -v src="$source" 'BEGIN { max = -1; n = 0; s = "" }
    NF { v = $1 + 0; if (v > max) max = v; s = s (n ? "," : "") v; n++ }
    END { if (n == 0) printf "{\"source\":\"%s\",\"samples\":[],\"max_cpu\":null}\n", src;
          else printf "{\"source\":\"%s\",\"samples\":[%s],\"max_cpu\":%s}\n", src, s, max }' "$file" > "$dir/generator.json"
  cat "$dir/generator.json"
}

cmd="${1:-}"
case "$cmd" in
  cpu)
    a=$(read_stat); sleep 1; b=$(read_stat)
    # shellcheck disable=SC2086
    ratio $a $b
    ;;
  start)
    dir="${2:?dir}"; interval="${3:-5}"
    mkdir -p "$dir"
    : > "$dir/.cpu-samples"
    sample_loop "$interval" "$dir/.cpu-samples" &
    echo $! > "$dir/.sampler.pid"
    ;;
  stop)
    dir="${2:?dir}"
    if [ -f "$dir/.sampler.pid" ]; then
      kill "$(cat "$dir/.sampler.pid")" 2>/dev/null || true
      rm -f "$dir/.sampler.pid"
    fi
    write_json "$dir" "proc_stat" "$dir/.cpu-samples"
    ;;
  prom)
    url="${2:?prometheus url}"; start="${3:?start}"; end="${4:?end}"; dir="${5:?dir}"
    mkdir -p "$dir"
    q='max(1 - sum by (instance) (rate(node_cpu_seconds_total{job="loadgen",mode=~"idle|iowait"}[1m])) / count by (instance) (node_cpu_seconds_total{job="loadgen",mode="idle"}))'
    curl -fsS --get "${url%/}/api/v1/query_range" --data-urlencode "query=${q}" \
      --data-urlencode "start=${start}" --data-urlencode "end=${end}" --data-urlencode "step=15" \
      | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const r=JSON.parse(s).data.result;
          const v=r.flatMap(x=>x.values.map(p=>Number(p[1]))).filter(n=>!Number.isNaN(n));process.stdout.write(v.map(n=>n.toFixed(3)).join("\n")+"\n")})' \
      > "$dir/.cpu-samples"
    write_json "$dir" "prometheus:job=loadgen" "$dir/.cpu-samples"
    ;;
  *)
    echo "usage: $0 cpu | start <dir> [interval_s] | stop <dir> | prom <url> <start> <end> <dir>" >&2
    exit 2
    ;;
esac
