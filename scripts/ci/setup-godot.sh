#!/usr/bin/env bash
# Installs the pinned Godot engine for CI (F060 T042, GODOT_ARCHITECTURE §11, ADR-0088): official GitHub release
# assets only, verified against SHA-512 values pinned below (taken from the release's SHA512-SUMS.txt).
# Usage: bash scripts/ci/setup-godot.sh linux [--templates]  (CI runs Godot on Linux only; no Windows job, ADR-0101)
# Exports GODOT_BIN (and installs export templates with --templates) through $GITHUB_ENV when available.
set -euo pipefail

cd "$(dirname "$0")/../.."
version="$(sed -n 's/^GODOT_FULL_VERSION[[:space:]]*:=[[:space:]]*//p' tools/versions.mk)"
pinned_minor="$(tr -d '[:space:]' <src/game/.godot-version)"
if [[ "${version%.*}" != "$pinned_minor" ]]; then
  echo "setup-godot: tools/versions.mk GODOT_FULL_VERSION=$version does not match src/game/.godot-version=$pinned_minor" >&2
  exit 1
fi

declare -A SHA512=(
  ["Godot_v4.7.2-stable_linux.x86_64.zip"]="9aa00f7a605200940bce3027a567b782f49bd8e940dd06ae9e987bd65aee1b1467edd56ed84fcdcbdd44354bf613bdbb4e5d2913e925850368e150c59ed54c65"
  ["Godot_v4.7.2-stable_export_templates.tpz"]="ca4d71c4d7b81dfc15d1a98baa07534aa95b03fdda78a0075b06672e1648d2e5f40980c9adc28d23e1b92e732ee7bf3461997aa804af74ec2fcd7a93ccb84079"
)
base="https://github.com/godotengine/godot/releases/download/${version}-stable"
dest="${RUNNER_TEMP:-/tmp}/godot-${version}"
mkdir -p "$dest"

fetch() { # fetch <asset> → $dest/<asset>, verified
  local asset="$1" want="${SHA512[$1]:-}"
  if [[ -z "$want" ]]; then
    echo "setup-godot: no pinned SHA-512 for $asset (update scripts/ci/setup-godot.sh with the version)" >&2
    exit 1
  fi
  if [[ ! -f "$dest/$asset" ]]; then
    curl -fsSL --retry 3 -o "$dest/$asset" "$base/$asset"
  fi
  echo "$want  $dest/$asset" | sha512sum -c - >/dev/null
}

case "${1:-}" in
  linux)
    asset="Godot_v${version}-stable_linux.x86_64.zip"
    fetch "$asset"
    unzip -o -q "$dest/$asset" -d "$dest"
    bin="$dest/Godot_v${version}-stable_linux.x86_64"
    chmod +x "$bin"
    templates_dir="$HOME/.local/share/godot/export_templates/${version}.stable"
    ;;
  *)
    echo "usage: setup-godot.sh linux [--templates]" >&2
    exit 2
    ;;
esac

if [[ "${2:-}" == "--templates" ]]; then
  tpz="Godot_v${version}-stable_export_templates.tpz"
  fetch "$tpz"
  mkdir -p "$templates_dir"
  unzip -o -q "$dest/$tpz" -d "$dest/tpl"
  cp -r "$dest/tpl/templates/." "$templates_dir/"
fi

"$bin" --headless --version
if [[ -n "${GITHUB_ENV:-}" ]]; then
  echo "GODOT_BIN=$bin" >>"$GITHUB_ENV"
fi
echo "setup-godot: GODOT_BIN=$bin"
