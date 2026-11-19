#!/usr/bin/env bash
# F060 T036 — godot.yml path filter and job order (F060-AC-19).
set -euo pipefail
cd "$(dirname "$0")/../.."
node --test scripts/ci/godot-rules.test.mjs
