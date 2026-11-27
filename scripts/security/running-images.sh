#!/usr/bin/env bash
# Image references (repo@sha256 digest) of every Swarm service running on an environment (F132-AC-19,
# SECURITY_ARCHITECTURE §15): the daily rescan scans what runs, not what was built. Same source of truth as the
# deploy scripts: `docker service inspect … .Spec.TaskTemplate.ContainerSpec.Image` (Swarm pins the digest at deploy).
# Needs DOCKER_HOST/DOCKER_TLS_VERIFY/DOCKER_CERT_PATH of the manager (scripts/deploy/write-docker-tls.sh).
# Usage: bash scripts/security/running-images.sh [--env=staging|production] > images.txt
set -euo pipefail
DOCKER=${DOCKER:-docker}
ENV_NAME=""
for arg in "$@"; do
  case "$arg" in
    --env=*) ENV_NAME="${arg#*=}" ;;
    *) echo "running-images: unknown argument $arg" >&2; exit 2 ;;
  esac
done
case "$ENV_NAME" in
  staging|production|"") ;;
  *) echo "running-images: --env must be staging or production" >&2; exit 2 ;;
esac
ids=$($DOCKER service ls -q)
if [ -z "$ids" ]; then
  echo "running-images: no Swarm service found${ENV_NAME:+ on $ENV_NAME}" >&2
  exit 1
fi
# shellcheck disable=SC2086 # ids is a whitespace list of service ids
$DOCKER service inspect --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}' $ids |
  awk 'NF' |
  while read -r ref; do
    case "$ref" in
      *@sha256:*) printf '%s\n' "$ref" ;;
      *) echo "running-images: $ref has no digest (not deployed by digest)" >&2; printf '%s\n' "$ref" ;;
    esac
  done | sort -u
