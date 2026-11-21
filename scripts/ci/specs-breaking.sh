#!/usr/bin/env bash
# F001 T038 — `make specs-breaking`: oasdiff breaking-change check of every public and internal OpenAPI file against a
# base ref (default origin/main). The logic lives in specs-breaking.mjs: specs normalized for the oasdiff 3.0 loader,
# intentional breaking changes accepted only through the explicit allowlist specs/breaking-allowlist.yaml (each entry
# cites its ADR — ADR-0095 mục 7); every other breaking change fails.
set -euo pipefail
cd "$(dirname "$0")/../.."
[ -d scripts/node_modules/js-yaml ] || npm ci --prefix scripts --no-audit --no-fund >/dev/null
node scripts/ci/specs-breaking.mjs "${1:-origin/main}"
