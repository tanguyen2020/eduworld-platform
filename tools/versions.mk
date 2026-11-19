# Pinned tool versions (F001 T002). One place to bump; `make tools` installs Go tools into ./bin.
# Atlas runs from its official image (the go-installable CLI is outdated), pinned below.
# Kept out of go.mod on purpose: the single backend go.mod (ADR-0074) must not pull tool
# dependencies into govulncheck/SBOM scope of the shipped binaries.

ENT_VERSION            := v0.14.6
OAPI_CODEGEN_VERSION   := v2.8.2
GOLANGCI_LINT_VERSION  := v2.15.0
GO_ARCH_LINT_VERSION   := v1.19.1
GOVULNCHECK_VERSION    := v1.9.0

ATLAS_IMAGE            := arigaio/atlas:0.38.0
POSTGRES_IMAGE         := postgres:17

# Game client (F060, ADR-0014): Godot minor pinned in src/game/.godot-version; full version for CI downloads.
GODOT_FULL_VERSION     := 4.7.2
GDTOOLKIT_VERSION      := 4.5.0
