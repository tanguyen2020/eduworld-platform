# worker image (F001 T039/T047): one image for every profile; the profile is chosen at runtime
# (--profile / EW_WORKER_PROFILE), so buildinfo.Profile stays empty at link time.
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
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Binary=worker \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Version=${VERSION} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Commit=${COMMIT} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/worker ./cmd/worker

# Runtime: Alpine with ffmpeg/ffprobe (media worker of module learning, profile events: Ogg Vorbis/MP3, Ogg Theora,
# H.264 MP4 and HLS — F027, CONTENT_MODEL §7). Images are processed in pure Go (no libvips/cgo).
FROM alpine:3.22
RUN apk add --no-cache ffmpeg ca-certificates tzdata  && addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/worker" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=build /out/worker /app/worker
# Runtime inputs read through EW_OPENAPI_SPEC (request validation, core-api) and EW_PORTAL_MENU_CATALOG (portal
# workspaces and feature gate) — OQ F010-2.
COPY specs/openapi/eduworld-api.v1.yaml /app/specs/openapi/eduworld-api.v1.yaml
COPY specs/portal/menu-catalog.yaml /app/specs/portal/menu-catalog.yaml
USER 10001:10001
EXPOSE 9090
ENTRYPOINT ["/app/worker"]
