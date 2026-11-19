#!/usr/bin/env bash
# F005 T028 — design.yml path filter and workflow_call wiring to godot.yml / web.yml (F005-AC-15).
set -euo pipefail
cd "$(dirname "$0")/../.."
node --test scripts/ci/design-rules.test.mjs
