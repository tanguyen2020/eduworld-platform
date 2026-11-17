#!/usr/bin/env bash
# F001 T038 — `make specs-breaking`: oasdiff breaking-change check of every public and internal OpenAPI
# file against a base ref (default origin/main). New files are skipped (nothing to break).
set -euo pipefail
cd "$(dirname "$0")/../.."
base="${1:-origin/main}"
IMG="tufin/oasdiff:v1.11.7"
root=$(pwd -W 2>/dev/null || pwd)
work=$(mktemp -d)
trap 'rm -f "$work"/*.yaml; rmdir "$work"' EXIT
wdir=$( (cd "$work" && (pwd -W 2>/dev/null || pwd)) )

status=0
for f in specs/openapi/eduworld-api.v1.yaml specs/openapi/modules/*.yaml; do
  name=$(basename "$f")
  if ! git show "$base:$f" > "$work/base-$name" 2>/dev/null; then
    rm -f "$work/base-$name"
    # spec moved: compare with the pre-ADR-0074 path if it exists
    old="specs/openapi/services/$name"
    git show "$base:$old" > "$work/base-$name" 2>/dev/null || { rm -f "$work/base-$name"; echo "skip (new): $f"; continue; }
  fi
  if ! MSYS_NO_PATHCONV=1 docker run --rm -v "$wdir:/base:ro" -v "$root/specs:/specs:ro" "$IMG" \
      breaking "/base/base-$name" "/${f}" --fail-on ERR >/dev/null 2>"$work/err"; then
    echo "BREAKING: $f" >&2; status=1
  else
    echo "ok: $f"
  fi
done
exit $status
