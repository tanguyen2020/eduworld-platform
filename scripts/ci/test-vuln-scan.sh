#!/usr/bin/env bash
# F001 T033 / F001-AC-14 — prove govulncheck fails on a module calling a known-vulnerable symbol.
# The fixture module is generated in a temp dir so the repository never carries a vulnerable go.mod
# (it would make security.yml red and add a second go.mod to the repo).
set -euo pipefail
cd "$(dirname "$0")/../.."
GOVULN="golang.org/x/vuln/cmd/govulncheck@$(sed -n 's/^GOVULNCHECK_VERSION[[:space:]]*:=[[:space:]]*//p' tools/versions.mk)"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat > "$work/go.mod" <<'EOF'
module example.com/vulnfixture

go 1.24

require golang.org/x/text v0.3.7
EOF
cat > "$work/main.go" <<'EOF'
package main

import (
	"fmt"

	"golang.org/x/text/language"
)

// GO-2022-1059: ParseAcceptLanguage before x/text v0.3.8 can be made to consume excessive CPU.
func main() { fmt.Println(language.ParseAcceptLanguage("en-US,en;q=0.9")) }
EOF
(cd "$work" && GOFLAGS=-mod=mod go mod tidy >/dev/null 2>&1)

set +e
(cd "$work" && go run "$GOVULN" ./... >"$work/out.txt" 2>&1)
code=$?
set -e
if [ "$code" -eq 0 ]; then
  echo "govulncheck: vulnerable fixture was NOT reported" >&2; cat "$work/out.txt" >&2; exit 1
fi
grep -q "GO-2022-1059" "$work/out.txt" || { echo "govulncheck: unexpected output" >&2; cat "$work/out.txt" >&2; exit 1; }
echo "govulncheck: ok (GO-2022-1059 reported, exit $code)"
