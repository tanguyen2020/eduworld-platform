# eduworld/obs-alertmanager (F131 T025): Alertmanager with the routing file (placeholders rendered at start by
# alertmanager-entrypoint.sh from obs.env), templates and entrypoint baked in. Receiver URLs and the SMTP password come
# from Docker secret files only. Build: docker build -f deploy/docker/obs-alertmanager.Dockerfile .
ARG AM_IMAGE=prom/alertmanager:v0.28.1
FROM ${AM_IMAGE}
ARG VERSION=dev
LABEL org.opencontainers.image.title="eduworld/obs-alertmanager" org.opencontainers.image.version="${VERSION}"
COPY deploy/observability/alertmanager/alertmanager.yml /etc/alertmanager/alertmanager.tpl.yml
COPY deploy/observability/alertmanager/templates/ /etc/alertmanager/templates/
COPY deploy/swarm/config/alertmanager-entrypoint.sh /usr/local/bin/ew-entrypoint.sh
ENTRYPOINT ["/bin/sh", "/usr/local/bin/ew-entrypoint.sh"]
