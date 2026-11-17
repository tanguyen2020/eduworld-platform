#!/usr/bin/env bash
# F001 T031 — verify workflow path filters and the per-module matrix against the expectation table.
set -euo pipefail
cd "$(dirname "$0")/../.."
node --test scripts/ci/path-filters.test.mjs
