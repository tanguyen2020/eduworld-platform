# core-api image (F001 T039/T047, DEPLOYMENT §3.1): static binary on distroless, non-root UID 10001,
# no shell. Version/commit/build time are injected into pkg/buildinfo via -ldflags.
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
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Binary=core-api \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Version=${VERSION} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Commit=${COMMIT} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/core-api ./cmd/core-api

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/core-api" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=build /out/core-api /app/core-api
# Runtime inputs read through EW_OPENAPI_SPEC (request validation, core-api) and EW_PORTAL_MENU_CATALOG (portal
# workspaces and feature gate) — OQ F010-2.
COPY specs/openapi/eduworld-api.v1.yaml /app/specs/openapi/eduworld-api.v1.yaml
COPY specs/portal/menu-catalog.yaml /app/specs/portal/menu-catalog.yaml
# distroless:nonroot defaults to WORKDIR /home/nonroot (mode 700, uid 65532); uid 10001 cannot enter it and
# runtimes that chdir after dropping privileges (Kiri, some containerd setups) refuse to start. /app is readable.
WORKDIR /app
USER 10001:10001
EXPOSE 8100 9090
ENTRYPOINT ["/app/core-api"]
