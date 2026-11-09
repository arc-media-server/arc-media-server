# Arc Media Server in a container: the web app (Node), the server (Go, no C needed: SQLite is pure
# Go), then a small Debian image with FFmpeg for converting and finding intros.
# Build: docker build -t arc .   Run: see docker-compose.yml.

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# Writes to ../internal/webui/dist, which the Go build embeds.
RUN mkdir -p ../internal/webui && npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS server
ARG TARGETOS TARGETARCH
ARG GIT_REVISION=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.revision=$GIT_REVISION" -o /out/ams ./cmd/ams

# Debian trixie (13). (Its own FFmpeg 7.1 kept Dolby Vision when packaging HLS where bookworm's 5.1
# dropped it, owner's test 31 October; jellyfin-ffmpeg 8.1 below keeps it too, with the same check.)
FROM debian:trixie-slim
LABEL org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.source="https://github.com/arc-media-server/arc-media-server"
# FFmpeg: Jellyfin's build (jellyfin-ffmpeg, FFmpeg 8.1 with media-server patches), pinned and
# checked. It brings its own Intel and AMD video drivers (libva, iHD/VPL, Mesa, Vulkan) and adds
# tonemapx, a fast HDR-to-SDR filter. Chromaprint (intro detection) and zimg come with it. i965 stays
# for older Intel graphics. Certificates for TMDB/TheTVDB, curl for the health check, tzdata for
# local times.
ARG JELLYFIN_FFMPEG=8.1.3-2
RUN apt-get update \
 && arch="$(dpkg --print-architecture)" \
 && case "$arch" in \
      amd64) sum=82bd7422530740af738891c54b548eadf9bb572ff875976eddbe231e6f02482b; hw_packages="i965-va-driver" ;; \
      arm64) sum=21e7f67d3fe70b4c757f8c5064f1317e2580a08b3b84460b7a79af0a99c1095f; hw_packages="" ;; \
      *) echo "no jellyfin-ffmpeg for $arch" >&2; exit 1 ;; \
    esac \
 && apt-get install -y --no-install-recommends ca-certificates curl tzdata $hw_packages \
 && curl -fsSL -o /tmp/jellyfin-ffmpeg.deb \
      "https://github.com/jellyfin/jellyfin-ffmpeg/releases/download/v${JELLYFIN_FFMPEG}/jellyfin-ffmpeg8_${JELLYFIN_FFMPEG}-trixie_${arch}.deb" \
 && echo "$sum  /tmp/jellyfin-ffmpeg.deb" | sha256sum -c - \
 && apt-get install -y --no-install-recommends /tmp/jellyfin-ffmpeg.deb \
 && ln -s /usr/lib/jellyfin-ffmpeg/ffmpeg /usr/local/bin/ffmpeg \
 && ln -s /usr/lib/jellyfin-ffmpeg/ffprobe /usr/local/bin/ffprobe \
 && rm -rf /tmp/jellyfin-ffmpeg.deb /var/lib/apt/lists/*
COPY --from=server /out/ams /usr/local/bin/ams
# /config holds the database, artwork, backups and certificates; mount your media read-only
# (e.g. at /media) and add libraries in the app.
ENV AMS_DATA=/config \
    AMS_LISTEN=0.0.0.0:8099
VOLUME /config
EXPOSE 8099
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s CMD curl -fsS http://127.0.0.1:8099/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/ams", "-config", "none"]
