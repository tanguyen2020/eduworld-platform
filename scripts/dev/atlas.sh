#!/usr/bin/env bash
# Run the pinned Atlas CLI from its image (used by cmd/migrate locally: --atlas="bash scripts/dev/atlas.sh").
# Rewrites the host migration dir and the localhost DSN so they work inside the compose network.
set -euo pipefail
cd "$(dirname "$0")/../.."
img=$(sed -n 's/^ATLAS_IMAGE[[:space:]]*:=[[:space:]]*//p' tools/versions.mk)
root=$(pwd -W 2>/dev/null || pwd)

args=()
for a in "$@"; do
  case "$a" in
    file://*/modules/*) a="file:///modules/${a##*/modules/}" ;;
    *@localhost:5432/*) a="${a/@localhost:5432/@postgres:5432}" ;;
  esac
  args+=("$a")
done
MSYS_NO_PATHCONV=1 exec docker run --rm --network eduworld_default -v "$root/src/backend/modules:/modules" "$img" "${args[@]}"
