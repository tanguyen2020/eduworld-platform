#!/usr/bin/env bash
# Checks a Web export folder (F133 T028; F133-AC-16, the Web part of F133-AC-25; PERFORMANCE_BUDGET §14, ADR-0086):
#   - the EduWorld shell (src/game/web/shell.html) is the page, every exporter placeholder is substituted, and the
#     generated token stylesheet shell_tokens.css sits next to it;
#   - single-threaded export (no COOP/COEP needed): GODOT_THREADS_ENABLED = false and no extensions;
#   - the page never touches browser storage (the Web build keeps the refresh token in RAM only — SecureStorage is
#     MemoryOnly on Web): no localStorage., sessionStorage., indexedDB. or document.cookie in index.html;
#   - index.wasm and index.pck exist; their sizes are printed (the size gate itself is T012, check-build-size).
# Usage: bash src/game/tools/ci/check_web_export.sh <build/web>   → exit 1 on any problem.
set -euo pipefail

dir="${1:-}"
if [[ -z "$dir" || ! -d "$dir" ]]; then
  echo "usage: check_web_export.sh <web export folder>" >&2
  exit 2
fi
page="$dir/index.html"
problems=0
problem() { echo "check_web_export: $*" >&2; problems=$((problems + 1)); }

[[ -f "$page" ]] || { problem "missing index.html"; exit 1; }
grep -q 'EduWorldShell' "$page" || problem "index.html is not the EduWorld shell (html/custom_html_shell)"
if grep -q -E '\$GODOT_[A-Z_]+' "$page"; then problem "unsubstituted exporter placeholder in index.html"; fi
grep -q -E 'const GODOT_THREADS_ENABLED = false;' "$page" || problem "the Web export must be single-threaded (variant/thread_support=false)"
grep -q -E '"gdextensionLibs":\[\]' "$page" || problem "the Web export must not load GDExtension libraries"
if grep -q -E 'localStorage\.|sessionStorage\.|indexedDB\.|document\.cookie' "$page"; then
  problem "index.html uses browser storage (ADR-0086: the Web build keeps the session in RAM only)"
fi
[[ -f "$dir/shell_tokens.css" ]] || problem "missing shell_tokens.css (make godot-export-web copies src/game/web/shell_tokens.css)"
for f in index.wasm index.pck index.js; do
  [[ -f "$dir/$f" ]] || problem "missing $f"
done

if [[ $problems -ne 0 ]]; then
  exit 1
fi
size() { wc -c <"$1" | tr -d ' '; }
echo "check_web_export: OK (index.wasm $(size "$dir/index.wasm") B, index.pck $(size "$dir/index.pck") B, index.js $(size "$dir/index.js") B)"
