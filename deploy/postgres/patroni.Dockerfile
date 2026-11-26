# eduworld/postgres-patroni (F130 T014): PostgreSQL 17 + Patroni + pgBackRest + PgBouncer for the data stack of
# staging/production (DEPLOYMENT §7, BACKUP_RECOVERY §3). One image for three Swarm roles:
#   pg-N          → patroni (PostgreSQL managed by Patroni, archive_command = pgBackRest archive-push)
#   pgbackrest-N  → pgbackrest-server (TLS server next to pg-N, research R2)
#   pgbouncer     → pgbouncer
# Same PostgreSQL and pgBackRest versions as eduworld/backup-tools (deploy/backup/Dockerfile): a repo written by one
# is read by the other. Build: docker build -f deploy/postgres/patroni.Dockerfile .
ARG PG_IMAGE=postgres:17.12-trixie
FROM ${PG_IMAGE}
ARG VERSION=dev
ARG COMMIT=unknown
# Versions pinned to the pgdg/Debian packages of the base image (checked by tests/ops/integration).
RUN set -eux; apt-get update; \
    apt-get install -y --no-install-recommends pgbackrest patroni python3-etcd pgbouncer ca-certificates; \
    rm -rf /var/lib/apt/lists/*; \
    install -d -o postgres -g postgres -m 0750 /var/lib/pgbackrest /var/spool/pgbackrest /var/log/pgbackrest /etc/pgbackrest /etc/patroni; \
    install -d -o postgres -g postgres -m 0755 /var/run/postgresql
COPY deploy/postgres/bin/ /usr/local/bin/
RUN chmod 0755 /usr/local/bin/patroni-start /usr/local/bin/pg-healthcheck /usr/local/bin/pgbackrest-server /usr/local/bin/ew-secrets.sh
LABEL org.opencontainers.image.title="eduworld/postgres-patroni" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
USER postgres
ENTRYPOINT []
CMD ["patroni-start"]
