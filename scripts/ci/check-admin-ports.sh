#!/usr/bin/env bash
# F001 T019 / F001-AC-06 — admin ports never routed through APISIX nor published outside 9100/9106/9200.
set -euo pipefail
cd "$(dirname "$0")/../.."
node scripts/ci/check-admin-ports.mjs "$@"
