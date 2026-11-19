# migrate image (F001 T025/T039): cmd/migrate + pinned Atlas CLI + every module's migrations.
# Runs as a one-off job per database: `migrate --module=<module>` or `migrate --all`.
FROM arigaio/atlas:0.38.0 AS atlas

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download
COPY src/backend/pkg/ pkg/
COPY src/backend/modules/ modules/
COPY src/backend/cmd/ cmd/
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Binary=migrate \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Version=${VERSION} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Commit=${COMMIT} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/migrate ./cmd/migrate \
 && for m in identity learning game competition chat; do \
      mkdir -p /out/modules/$m && cp -r modules/$m/migrations /out/modules/$m/; \
    done

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/migrate" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=atlas /atlas /app/atlas
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/modules /app/modules
ENV EW_ATLAS_BIN=/app/atlas \
    EW_MIGRATIONS_ROOT=/app
USER 10001:10001
ENTRYPOINT ["/app/migrate"]
