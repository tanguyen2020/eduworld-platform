# eduworld/swarm-exporter (F131 T016, MONITORING §1): exports Swarm service/task/node metrics read through the Docker
# API allowlist proxy (GET /services, /tasks, /nodes only). Static binary, distroless non-root, no shell
# (scripts/ci/check-image.sh). HEALTHCHECK in the stack file: ["CMD", "/app/swarm-exporter", "healthcheck"].
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download
COPY src/backend/tools/swarm-exporter/ tools/swarm-exporter/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/swarm-exporter ./tools/swarm-exporter/cmd/swarm-exporter

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/swarm-exporter" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=build /out/swarm-exporter /app/swarm-exporter
USER 65532:65532
EXPOSE 9090
ENTRYPOINT ["/app/swarm-exporter"]
CMD ["run"]
