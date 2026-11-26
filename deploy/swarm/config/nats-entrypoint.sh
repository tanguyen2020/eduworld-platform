#!/bin/sh
# NATS node entrypoint (F130): the route password is read from its Docker secret into the process environment only.
set -eu
[ -s /run/secrets/nats_cluster_password ] || { echo "nats: nats_cluster_password missing or empty" >&2; exit 1; }
NATS_CLUSTER_PASSWORD="$(cat /run/secrets/nats_cluster_password)"
export NATS_CLUSTER_PASSWORD
exec nats-server --config /etc/nats/nats.conf
