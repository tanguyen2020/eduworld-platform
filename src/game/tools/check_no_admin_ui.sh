#!/usr/bin/env bash
# F060-AC-11 (AGENTS R32, ADR-0047, TC-UI-010): the game project carries nothing of the Admin/Ops web UI —
# no Ant Design / ops-ui library, no admin token or web theme file, no reference to src/cms/frontend or
# design/admin-tokens, no web stylesheet or React component.
# Usage: bash src/game/tools/check_no_admin_ui.sh [root=src/game]   → exit 1 on any finding.
set -euo pipefail

root="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
found=0
report() { echo "check_no_admin_ui: $1" >&2; found=1; }

# 1) Folders or files named after the Admin UI stack.
while IFS= read -r -d '' p; do
  report "Admin UI artefact: ${p#"$root"/}"
done < <(find "$root" \( -path "$root/.godot" -o -path "$root/addons/gdUnit4" -o -path "$root/reports" \) -prune -o \( -iname 'antd' -o -iname 'ant-design*' -o -iname '@ant-design' -o -iname 'ops-ui' -o -iname 'admin-tokens' -o -iname 'tokens.css' -o -iname 'antd-theme*' \) -print0)

# 2) Web front-end files have no place in the Godot project (the GdUnit4 HTML report template is test tooling).
while IFS= read -r -d '' p; do
  report "web front-end file: ${p#"$root"/}"
done < <(find "$root" \( -path "$root/.godot" -o -path "$root/addons/gdUnit4" -o -path "$root/reports" \) -prune -o -type f \( -name '*.css' -o -name '*.scss' -o -name '*.tsx' -o -name '*.jsx' -o -name 'package.json' \) -print0)

# 3) References to the web portal or the admin token set from game files.
tmp="$(mktemp)"
if grep -R -n -I -E --exclude-dir=.godot --exclude-dir=gdUnit4 --exclude-dir=reports --exclude='check_no_admin_ui*' \
  -e 'src/cms/frontend' -e 'design/admin-tokens' -e '@ant-design' -e 'antd' -e 'ops-ui' "$root" >"$tmp" 2>/dev/null; then
  while IFS= read -r line; do report "reference: ${line#"$root"/}"; done <"$tmp"
fi
rm -f "$tmp"

if [[ $found -ne 0 ]]; then
  exit 1
fi
echo "check_no_admin_ui: OK ($root)"
