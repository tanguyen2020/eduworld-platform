# eduworld/ops-scheduler (F130 T047): scales Swarm services on the school timetable through the Docker API allowlist
# proxy. Static binary, distroless non-root, no shell (scripts/ci/check-image.sh).
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download
COPY src/backend/tools/ops-scheduler/ tools/ops-scheduler/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ops-scheduler ./tools/ops-scheduler/cmd/ops-scheduler

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/ops-scheduler" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=build /out/ops-scheduler /app/ops-scheduler
USER 65532:65532
EXPOSE 9090
ENTRYPOINT ["/app/ops-scheduler"]
CMD ["run"]
