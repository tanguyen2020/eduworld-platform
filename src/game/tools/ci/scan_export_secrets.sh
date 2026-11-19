#!/usr/bin/env bash
# F061-AC-11 (TC-AUTH-010, ADR-0068, AGENTS R3/R4): the exported game must not contain a client secret, a Keycloak
# token endpoint, `oidc_*` keys or an OAuth callback intent filter. The patterns have one owner,
# src/game/tools/scan_artifact.sh (also used by `make godot-export-*`); this entry point keeps the F061 task path.
# Usage: bash src/game/tools/ci/scan_export_secrets.sh <artifact|dir> [...]
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
exec bash "$here/../scan_artifact.sh" "$@"
