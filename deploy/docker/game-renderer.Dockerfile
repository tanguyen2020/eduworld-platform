# game-renderer image (F078 US5, FR-CMS-017, research R2): the Go entrypoint src/backend/modules/game/cmd/renderer plus
# Godot and the minimal Godot project of src/game/tools/thumbnail_renderer. It pulls jobs from the JetStream work queue
# GAME_RENDER_JOBS, renders thumbnails/turntables of 3D game assets and uploads them to ew-media-public.
#
# Rendering: `--headless` uses Godot's dummy rendering server (empty images), so Godot runs under Xvfb with the
# Compatibility renderer over OpenGL 3.3 (Mesa llvmpipe, no GPU needed): /usr/local/bin/godot-render wraps
# `xvfb-run ... godot --rendering-driver opengl3 --audio-driver Dummy` and is the default EW_RENDERER_GODOT.
#
# Godot: official release asset of the pinned version (tools/versions.mk GODOT_FULL_VERSION, src/game/.godot-version),
# verified against the SHA-512 pinned in scripts/ci/setup-godot.sh (release SHA512-SUMS.txt, ADR-0088). Bump both
# build args together with those files.
#
#   docker build -f deploy/docker/game-renderer.Dockerfile -t eduworld/game-renderer:local .
ARG GODOT_VERSION=4.7.2
ARG GODOT_SHA512=9aa00f7a605200940bce3027a567b782f49bd8e940dd06ae9e987bd65aee1b1467edd56ed84fcdcbdd44354bf613bdbb4e5d2913e925850368e150c59ed54c65

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY src/backend/go.mod src/backend/go.sum ./
RUN go mod download
COPY src/backend/pkg/ pkg/
COPY src/backend/modules/ modules/
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Binary=game-renderer \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Version=${VERSION} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Commit=${COMMIT} \
        -X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/game-renderer ./modules/game/cmd/renderer

# Godot download, verified before use.
FROM debian:bookworm-slim AS godot
ARG GODOT_VERSION
ARG GODOT_SHA512
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip \
 && rm -rf /var/lib/apt/lists/*
RUN set -eu; \
    asset="Godot_v${GODOT_VERSION}-stable_linux.x86_64.zip"; \
    curl -fsSL --retry 3 -o "/tmp/${asset}" \
      "https://github.com/godotengine/godot/releases/download/${GODOT_VERSION}-stable/${asset}"; \
    echo "${GODOT_SHA512}  /tmp/${asset}" | sha512sum -c -; \
    unzip -q "/tmp/${asset}" -d /tmp/godot; \
    install -m 0755 "/tmp/godot/Godot_v${GODOT_VERSION}-stable_linux.x86_64" /out-godot

# Runtime: Debian slim with Xvfb, Mesa (llvmpipe) and the X11 libraries Godot loads.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates tzdata xvfb xauth libgl1 libgl1-mesa-dri libegl1 libglx-mesa0 \
      libx11-6 libxcursor1 libxinerama1 libxrandr2 libxi6 libxext6 libxrender1 libxkbcommon0 libfontconfig1 \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd -g 10001 app && useradd -u 10001 -g app -m -d /home/app -s /usr/sbin/nologin app
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="eduworld/game-renderer" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/tanguyen2020/eduworld-platform"
COPY --from=godot /out-godot /usr/local/bin/godot
# xvfb-run's default screen is 8-bit; GLX needs a 24-bit visual. -a picks a free display per job.
RUN printf '%s\n' '#!/bin/sh' \
      'exec xvfb-run -a -s "-screen 0 1024x768x24 -nolisten tcp" /usr/local/bin/godot --rendering-driver opengl3 --audio-driver Dummy "$@"' \
      > /usr/local/bin/godot-render \
 && chmod 0755 /usr/local/bin/godot-render
# Minimal Godot project (renderer_project.godot explains why the full src/game project is not used): the renderer
# scripts and the generated 3D art tokens/Environments at their game paths, so res:// paths match src/game.
COPY src/game/tools/thumbnail_renderer/renderer_project.godot /opt/eduworld/game/project.godot
COPY src/game/tools/thumbnail_renderer/render_thumbnails.gd src/game/tools/thumbnail_renderer/render_plan.gd \
     /opt/eduworld/game/tools/thumbnail_renderer/
COPY src/game/resources/art/generated/art3d_tokens.gd /opt/eduworld/game/resources/art/generated/
COPY src/game/resources/art/generated/env_*.tres /opt/eduworld/game/resources/art/generated/
# Import once at build time (global class cache: RenderPlan, Art3dTokens) so the project files stay read-only at run
# time; --headless is fine for an import. Only the engine cache folder .godot/ is left writable for the app user.
RUN HOME=/tmp/godot-import /usr/local/bin/godot --headless --path /opt/eduworld/game --import \
 && rm -rf /tmp/godot-import \
 && chown -R 10001:10001 /opt/eduworld/game/.godot
COPY --from=build /out/game-renderer /app/game-renderer
ENV HOME=/home/app \
    EW_RENDERER_GODOT=/usr/local/bin/godot-render \
    EW_RENDERER_PROJECT=/opt/eduworld/game \
    EW_RENDERER_WORKDIR=/tmp/renderer \
    EW_ADMIN_ADDR=:9090
USER 10001:10001
EXPOSE 9090
HEALTHCHECK --interval=10s --timeout=3s --retries=6 CMD ["/app/game-renderer", "healthcheck"]
ENTRYPOINT ["/app/game-renderer"]
