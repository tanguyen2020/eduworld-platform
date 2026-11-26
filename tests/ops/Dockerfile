# Test image of tests/ops (F130): the backup-tools image + Node.js, so the unit tests of the bash scripts run on Linux
# with the same bash/coreutils/jq/rclone as production. Never deployed. Built by tests/ops/integration/run.sh.
ARG BASE=ew-f130/backup-tools:test
FROM ${BASE}
USER root
RUN apt-get update && apt-get install -y --no-install-recommends nodejs && rm -rf /var/lib/apt/lists/*
USER 10001:10001
WORKDIR /repo
