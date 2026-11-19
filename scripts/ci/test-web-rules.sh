#!/usr/bin/env bash
# F050 T035 — web.yml path filter and jobs (F050-AC-21).
set -euo pipefail
cd "$(dirname "$0")/../.."
node --test scripts/ci/web-rules.test.mjs
