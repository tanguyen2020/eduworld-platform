#!/usr/bin/env bash
# Daily rescan of the images running on an environment (F132-AC-19, SECURITY_ARCHITECTURE §15): list the running
# digests (running-images.sh), scan each with Trivy (CRITICAL/HIGH with a fix), open/update one issue per critical CVE
# with the due date critical_vuln_fix (vuln-issues.mjs). Exit 1 when a critical CVE with a fix runs (the scheduled
# run turns red until the image is rebuilt); the release gate itself is the trivy step of release.yml.
# Usage: bash scripts/security/rescan.sh --env=staging|production [--out=DIR] [--dry-run]
# Env: DOCKER_HOST/DOCKER_TLS_VERIFY/DOCKER_CERT_PATH (manager), TRIVY_USERNAME/TRIVY_PASSWORD (registry, read only),
#      GH_TOKEN (issues: write) unless --dry-run.
set -euo pipefail
cd "$(dirname "$0")/../.."
TRIVY_IMAGE=${TRIVY_IMAGE:-aquasec/trivy:0.64.1}
ENV_NAME="" OUT="tmp/security/rescan" DRY=""
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV_NAME="${arg#*=}" ;;
    --out=*) OUT="${arg#*=}" ;;
    --dry-run) DRY="--dry-run" ;;
    *) echo "rescan: unknown argument $arg" >&2; exit 2 ;;
  esac
done
case "$ENV_NAME" in staging|production) ;; *) echo "rescan: --env=staging|production" >&2; exit 2 ;; esac
mkdir -p "$OUT"
images=$(bash scripts/security/running-images.sh --env="$ENV_NAME")
n=0
while read -r ref; do
  [ -n "$ref" ] || continue
  n=$((n + 1))
  # The scanner runs locally on the deploy runner (DOCKER_HOST is only for the manager query above).
  DOCKER_HOST="" DOCKER_TLS_VERIFY="" docker run --rm \
    -e TRIVY_USERNAME -e TRIVY_PASSWORD \
    -v "$PWD/$OUT:/out" -v ew-trivy-cache:/root/.cache/trivy \
    "$TRIVY_IMAGE" image --quiet --format json --severity CRITICAL,HIGH --ignore-unfixed \
    --output "/out/trivy-$n.json" "$ref"
done <<< "$images"
echo "rescan: $n running image(s) of $ENV_NAME scanned"
node scripts/security/vuln-issues.mjs --env="$ENV_NAME" ${DRY:+"$DRY"} "$OUT"/trivy-*.json | tee "$OUT/summary.txt"
if grep -q '^vuln-issues: 0 critical' "$OUT/summary.txt"; then
  exit 0
fi
exit 1
