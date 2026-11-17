#!/usr/bin/env bash
# Install pinned Go tools into ./bin (F001 T002). Versions come from tools/versions.mk.
set -euo pipefail
cd "$(dirname "$0")/../.."

ver() { sed -n "s/^$1[[:space:]]*:=[[:space:]]*//p" tools/versions.mk; }

export GOBIN="$PWD/bin"
mkdir -p "$GOBIN"

go install "entgo.io/ent/cmd/ent@$(ver ENT_VERSION)"
go install "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(ver OAPI_CODEGEN_VERSION)"
go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(ver GOLANGCI_LINT_VERSION)"
go install "github.com/fe3dback/go-arch-lint@$(ver GO_ARCH_LINT_VERSION)"
go install "golang.org/x/vuln/cmd/govulncheck@$(ver GOVULNCHECK_VERSION)"

docker pull -q "$(ver ATLAS_IMAGE)" >/dev/null
echo "tools installed in $GOBIN; atlas image $(ver ATLAS_IMAGE) pulled"
