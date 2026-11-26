#!/usr/bin/env bash
# Writes the Docker API client certificate of the deploy runner (env CA, CERT, KEY from GitHub Environment secrets) to
# $DOCKER_CERT_PATH (mode 700, key 600), and APISIX_ADMIN_KEY (when set) to $DOCKER_CERT_PATH/apisix-admin-key, so the
# values never appear on a command line or in the log (F130-AC-08). The workflow removes the directory afterwards.
set -euo pipefail
: "${DOCKER_CERT_PATH:?DOCKER_CERT_PATH required}"
for v in CA CERT KEY; do [ -n "${!v:-}" ] || { echo "write-docker-tls: $v is empty (GitHub Environment secret missing)" >&2; exit 1; }; done
umask 077
mkdir -p "$DOCKER_CERT_PATH"
printf '%s\n' "$CA" > "$DOCKER_CERT_PATH/ca.pem"
printf '%s\n' "$CERT" > "$DOCKER_CERT_PATH/cert.pem"
printf '%s\n' "$KEY" > "$DOCKER_CERT_PATH/key.pem"
if [ -n "${APISIX_ADMIN_KEY:-}" ]; then printf '%s' "$APISIX_ADMIN_KEY" > "$DOCKER_CERT_PATH/apisix-admin-key"; fi
echo "write-docker-tls: client certificate written"
