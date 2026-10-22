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

FROM debian:bookworm-slim
# FFmpeg (with Chromaprint for intro detection, zimg for HDR tone mapping, VAAPI/NVENC for graphics
# cards), certificates for TMDB/TheTVDB, curl for the health check, tzdata for local times.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl tzdata \
 && rm -rf /var/lib/apt/lists/*
COPY --from=server /out/ams /usr/local/bin/ams
# /config holds the database, artwork, backups and certificates; mount your media read-only
# (e.g. at /media) and add libraries in the app.
ENV AMS_DATA=/config \
    AMS_LISTEN=0.0.0.0:8099
VOLUME /config
EXPOSE 8099
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s CMD curl -fsS http://127.0.0.1:8099/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/ams", "-config", "none"]
