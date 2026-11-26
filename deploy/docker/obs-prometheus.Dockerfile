# eduworld/obs-prometheus (F131 T014): Prometheus with the reviewed configuration baked in — prometheus.swarm.yml and
# every rule file of deploy/observability/prometheus/rules (immutable, versioned with OPS_VERSION; the rules are tested
# by promtool in CI before the image is built). Targets of the blackbox probes come per environment as Docker configs.
# Build: docker build -f deploy/docker/obs-prometheus.Dockerfile .
ARG PROM_IMAGE=prom/prometheus:v3.2.1
FROM ${PROM_IMAGE}
ARG VERSION=dev
LABEL org.opencontainers.image.title="eduworld/obs-prometheus" org.opencontainers.image.version="${VERSION}"
COPY deploy/observability/prometheus/prometheus.swarm.yml /etc/prometheus/prometheus.yml
COPY deploy/observability/prometheus/rules/ /etc/prometheus/rules/
