# eduworld/obs-grafana (F131 T031): Grafana with datasources, dashboard providers and the dashboards of MONITORING §4
# (built by scripts/obs/build-dashboards.mjs, checked by check-dashboards) baked in, read-only in the UI. Login only via
# Keycloak (generic OAuth, client eduworld-ops-tools, platform roles; no MFA — ADR-0094), configured by environment.
# Build: docker build -f deploy/docker/obs-grafana.Dockerfile .
ARG GRAFANA_IMAGE=grafana/grafana:11.5.2
FROM ${GRAFANA_IMAGE}
ARG VERSION=dev
LABEL org.opencontainers.image.title="eduworld/obs-grafana" org.opencontainers.image.version="${VERSION}"
COPY deploy/observability/grafana/provisioning/ /etc/grafana/provisioning/
COPY deploy/observability/grafana/dashboards/ /var/lib/grafana/dashboards/
