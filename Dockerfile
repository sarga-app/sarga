# syntax=docker/dockerfile:1.7

# ─── build stage ────────────────────────────────────────────────────────────
FROM golang:1.26.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/sarga \
        ./cmd/sarga

# ─── runtime stage ──────────────────────────────────────────────────────────
FROM alpine:3.23.4

# Runtime deps:
#   ffmpeg              → transcoding (Phase 4)
#   chromaprint         → AcoustID fingerprinting via fpcalc (Phase 2)
#   ca-certificates     → outbound HTTPS (MusicBrainz, AcoustID, lrclib, scrobblers, OIDC)
#   tzdata              → per-user timezone for scrobbles + listening events
RUN apk add --no-cache \
        ffmpeg \
        chromaprint \
        ca-certificates \
        tzdata

COPY --from=build /out/sarga /usr/local/bin/sarga

USER 65532:65532

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/sarga", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/sarga"]
CMD ["serve"]
