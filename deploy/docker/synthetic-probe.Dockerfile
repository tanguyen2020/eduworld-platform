# eduworld/synthetic-probe (F131 T038, F131-AC-19/AC-20, research R4): k6 + supercronic running the probes of
# tests/synthetic/k6 every 5 minutes (deploy/observability/synthetic/probe.crontab). Results are node_exporter textfiles
# (eduworld_synthetic_probe_*) in /textfile; credentials only from Docker secret files. Non-root (k6 user 12345).
# Build: docker build -f deploy/docker/synthetic-probe.Dockerfile .
ARG GO_IMAGE=golang:1.27-alpine
ARG K6_IMAGE=grafana/k6:0.57.0

# supercronic built from its tagged module (integrity checked by the Go checksum database), as in eduworld/backup-tools.
FROM ${GO_IMAGE} AS supercronic
ARG SUPERCRONIC_VERSION=v0.2.33
RUN CGO_ENABLED=0 GOBIN=/out go install -trimpath -ldflags "-s -w" github.com/aptible/supercronic@${SUPERCRONIC_VERSION}

FROM ${K6_IMAGE}
ARG VERSION=dev
LABEL org.opencontainers.image.title="eduworld/synthetic-probe" org.opencontainers.image.version="${VERSION}"
COPY --from=supercronic /out/supercronic /usr/local/bin/supercronic
COPY tests/synthetic/k6/lib /probes/lib
COPY tests/synthetic/k6/client_config.js tests/synthetic/k6/student_login.js /probes/
COPY deploy/observability/synthetic/probe.crontab /etc/eduworld/probe.crontab
USER 12345
ENTRYPOINT ["supercronic", "-json", "/etc/eduworld/probe.crontab"]
