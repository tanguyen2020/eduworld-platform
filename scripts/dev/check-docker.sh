#!/usr/bin/env bash
# Edge case of F001: refuse to start the stack when Docker has less memory than LOCAL_DEVELOPMENT §1
# requires (≥ 8 GB), instead of starting part of it silently. Docker Desktop "8 GB" reports ~7.6 GiB.
set -euo pipefail
MIN_BYTES=$((7500 * 1024 * 1024))
mem=$(docker info --format '{{.MemTotal}}' 2>/dev/null || true)
if [ -z "$mem" ]; then
  echo "Docker is not running (docker info failed). Start Docker Desktop/Engine first." >&2
  exit 1
fi
if [ "$mem" -lt "$MIN_BYTES" ]; then
  echo "Docker has $((mem / 1024 / 1024)) MiB; EduWorld needs ≥ 8 GB (LOCAL_DEVELOPMENT §1). Increase Docker memory." >&2
  exit 1
fi
