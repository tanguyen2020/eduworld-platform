# eduworld/backup-tools (F130 T007, BACKUP_RECOVERY §3, §6, §10, research R2/R3): pgBackRest repo host and backup
# job runner of the ops stack. Contains pgBackRest + PostgreSQL 17 client/server binaries (same base as
# eduworld/postgres-patroni, so restore_test.sh can start an isolated scratch cluster), rclone (object storage copies,
# standard S3 API only), the nats CLI, etcdctl, curl (SigV4), jq and supercronic (cron of ops-jobs.crontab). Non-root (UID 10001).
# No Docker API access. Build: docker build -f deploy/backup/Dockerfile .
ARG PG_IMAGE=postgres:17.12-trixie
ARG GO_IMAGE=golang:1.27-alpine

# supercronic and the nats CLI (consumer resets, RB-01/RB-03) built from their tagged modules (integrity checked by the Go checksum database) instead of a downloaded binary.
FROM ${GO_IMAGE} AS supercronic
ARG SUPERCRONIC_VERSION=v0.2.33
ARG NATSCLI_VERSION=v0.2.4
RUN CGO_ENABLED=0 GOBIN=/out go install -trimpath -ldflags "-s -w" github.com/aptible/supercronic@${SUPERCRONIC_VERSION}  && CGO_ENABLED=0 GOBIN=/out go install -trimpath -ldflags "-s -w" github.com/nats-io/natscli/nats@${NATSCLI_VERSION}

FROM ${PG_IMAGE}
ARG VERSION=dev
ARG COMMIT=unknown
RUN set -eux; apt-get update; \
    apt-get install -y --no-install-recommends pgbackrest rclone etcd-client jq curl ca-certificates procps openssl; \
    rm -rf /var/lib/apt/lists/*; \
    groupadd -g 10001 backup-tools; useradd -u 10001 -g 10001 -M -d /home/backup-tools -s /usr/sbin/nologin backup-tools; \
    install -d -o 10001 -g 10001 -m 0750 /home/backup-tools /var/lib/pgbackrest /var/spool/pgbackrest /var/log/pgbackrest /restore \
      /var/lib/node_exporter/textfile /etc/pgbackrest
COPY --from=supercronic /out/supercronic /out/nats /usr/local/bin/
COPY scripts/backup/ /opt/eduworld/backup/
COPY scripts/ops/ /opt/eduworld/ops/
RUN chmod -R a+rX /opt/eduworld && chmod 0755 /opt/eduworld/backup/*.sh /opt/eduworld/ops/*.sh
ENV PATH=/opt/eduworld/backup:/opt/eduworld/ops:/usr/lib/postgresql/17/bin:$PATH \
    HOME=/home/backup-tools \
    METRIC_DIR=/var/lib/node_exporter/textfile
LABEL org.opencontainers.image.title="eduworld/backup-tools" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
USER 10001:10001
ENTRYPOINT []
CMD ["supercronic", "-json", "/etc/eduworld/ops-jobs.crontab"]
