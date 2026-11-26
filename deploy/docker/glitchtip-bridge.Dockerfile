# eduworld/glitchtip-bridge (F131 T037, MONITORING §9): GlitchTip alert webhook → one GitHub Issue per new production
# issue (label module:<name> / service:<binary>, source:glitchtip). Static binary, distroless non-root, no shell
# (scripts/ci/check-image.sh). HEALTHCHECK in the stack file: ["CMD", "/app/glitchtip-bridge", "healthcheck"].
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download
# pkg/: redaction (pkg/observability) and Docker secret files (pkg/config) shared with the backend binaries
COPY src/backend/pkg/ pkg/
COPY src/backend/tools/glitchtip-bridge/ tools/glitchtip-bridge/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/glitchtip-bridge ./tools/glitchtip-bridge/cmd/glitchtip-bridge

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/glitchtip-bridge" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=build /out/glitchtip-bridge /app/glitchtip-bridge
USER 65532:65532
EXPOSE 8080 9090
ENTRYPOINT ["/app/glitchtip-bridge"]
CMD ["run"]
