#!/usr/bin/env bash
# F001 T034 / F001-AC-15 — image hardening: non-root UID ≥ 10001, no shell, version label, not tagged latest.
# Usage: scripts/ci/check-image.sh <image:tag>
set -euo pipefail
img="${1:?usage: check-image.sh <image:tag>}"
fail() { echo "check-image: $img: $*" >&2; exit 1; }

case "$img" in *:latest|*[!:]) [[ "$img" == *:* ]] || fail "image has no explicit tag"; esac
[[ "$img" == *:latest ]] && fail "tag 'latest' is not allowed"

user=$(docker image inspect "$img" --format '{{.Config.User}}')
uid="${user%%:*}"
[[ "$uid" =~ ^[0-9]+$ ]] || fail "User must be numeric UID (got '$user')"
[ "$uid" -ge 10001 ] || fail "UID $uid < 10001"

version=$(docker image inspect "$img" --format '{{index .Config.Labels "org.opencontainers.image.version"}}')
[ -n "$version" ] && [ "$version" != "<no value>" ] || fail "missing org.opencontainers.image.version label"

for sh in /bin/sh /bin/bash /busybox/sh; do
  if docker run --rm --entrypoint "$sh" "$img" -c true >/dev/null 2>&1; then fail "shell $sh present"; fi
done
echo "check-image: $img ok (uid=$uid, version=$version, no shell)"
