#!/usr/bin/env bash
# F001 T043 — F001-AC-17, F001-AC-21, F001-AC-22, F001-AC-23: drop one violating file at a time into the
# real tree, assert the expected check fails (or passes), and always remove it.
set -euo pipefail
cd "$(dirname "$0")/../../src/backend" # the single Go module (ADR-0092)
MOD=$(sed -n 's/^module //p' go.mod)
created=""
# Remove only the single fixture file and its (then empty) directory — never a recursive delete.
cleanup() {
  if [ -n "$created" ] && [ -f "$created/zz_archfixture.go" ]; then
    rm -f -- "$created/zz_archfixture.go"
    rmdir -- "$created" 2>/dev/null || true
  fi
  created=""
}
trap cleanup EXIT
pass=0; fail=0

fixture() { # dir file-content
  created="$1"; mkdir -p "$1"; printf '%s\n' "$2" > "$1/zz_archfixture.go"
}
expect() { # name want(fail|pass) command…
  local name="$1" want="$2"; shift 2
  if "$@" >/dev/null 2>&1; then got=pass; else got=fail; fi
  if [ "$got" = "$want" ]; then echo "ok   - $name"; pass=$((pass + 1)); else echo "FAIL - $name (want $want, got $got)"; fail=$((fail + 1)); fi
  cleanup
}

expect "baseline: go build ./cmd/... and arch-check pass" pass bash -c "go build ./cmd/... && go run ./tools/archgen --check"

fixture modules/game/internal/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/learning/internal/domain\""
expect "AC-21 game imports learning/internal → go build fails" fail go build ./modules/...

# F134 made every module with a database an allowed caller of every other one (listOutboxEventIds/listProcessedEventIds);
# realtime (no database) stays outside x-allowed-callers of the reconciliation facades (F134-AC-05).
fixture modules/realtime/internal/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/learning/api\""
expect "AC-22, F134-AC-05 realtime (not an allowed caller of learning) imports learning/api → arch-check fails" fail go run ./tools/archgen --check

fixture modules/identity/internal/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/chat/api\""
expect "AC-22 identity (allowed caller of chat) imports chat/api → arch-check passes" pass go run ./tools/archgen --check

fixture modules/game/internal/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/learning/api\""
expect "AC-22 game (allowed caller of learning) imports learning/api → arch-check passes" pass go run ./tools/archgen --check

fixture pkg/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/identity\""
expect "AC-23 pkg/ imports modules/ → arch-check fails" fail go run ./tools/archgen --check

fixture cmd/zz_archfixture "package main
import _ \"$MOD/modules/identity/internal/domain\"
func main() {}"
expect "AC-21 cmd/ imports modules/x/internal → go build fails" fail go build ./cmd/...

fixture cmd/zz_archfixture "package main
import _ \"$MOD/modules/identity/api\"
func main() {}"
expect "AC-23 cmd/ imports modules/x/api (not Register) → arch-check fails" fail go run ./tools/archgen --check

fixture modules/learning/internal/domain/zz_archfixture "package zz_archfixture
import _ \"net/http\""
expect "F002-AC-26 domain imports net/http → arch-check fails" fail go run ./tools/archgen --check

fixture modules/learning/internal/domain/zz_archfixture "package zz_archfixture
import _ \"github.com/jackc/pgx/v5\""
expect "F002-AC-26 domain imports pgx → arch-check fails" fail go run ./tools/archgen --check

fixture modules/learning/internal/application/zz_archfixture "package zz_archfixture
import _ \"$MOD/modules/learning/internal/infrastructure\""
expect "F002-AC-26 application imports infrastructure → arch-check fails" fail go run ./tools/archgen --check

fixture pkg/tenancy/zz_archfixture "package zz_archfixture
import _ \"github.com/jackc/pgx/v5\""
expect "F002-AC-27 pkg/tenancy/... depends on pgx → arch-check fails" fail go run ./tools/archgen --check

fixture modules/learning/internal/application/query/zz_archfixture "package zz_archfixture
import _ \"$MOD/pkg/entx\""
expect "F003-AC-10 query imports pkg/entx → arch-check fails" fail go run ./tools/archgen --check

fixture modules/learning/internal/infrastructure/persistence/read/zz_archfixture "package zz_archfixture
import _ \"$MOD/pkg/outbox\""
expect "F003-AC-10 persistence/read imports pkg/outbox → arch-check fails" fail go run ./tools/archgen --check

echo "test-arch-fixtures: $pass pass, $fail fail"
[ "$fail" -eq 0 ]
