# syntax=docker/dockerfile:1.7

# ─── frontend build ──────────────────────────────────────────────────────────
FROM node:22-alpine AS frontend

RUN corepack enable

WORKDIR /src/web

# Install deps first — cached until package files change, not on every source edit.
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile --ignore-scripts

# Copy source and build. Source changes reuse the install cache layer above.
COPY web/ ./
WORKDIR /src
RUN mkdir -p internal/transport/routes/web/dist && \
    cd web && pnpm build

# ─── backend build ───────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine AS build

WORKDIR /src

# Tools used in the build stage:
#   curl  — fetch vendored third-party assets (Scalar bundle)
#   make  — drives the vendor target
#   bash  — Makefile sets SHELL := /bin/bash (Alpine ships ash by default)
RUN apk add --no-cache curl make bash

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

# Overwrite placeholder with the real frontend build
COPY --from=frontend /src/internal/transport/routes/web/dist/ \
    internal/transport/routes/web/dist/

# Fetch vendored assets (Scalar API reference bundle) into the embed path.
RUN make vendor

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/sarga \
        ./cmd/sarga

# ─── runtime stage ───────────────────────────────────────────────────────────
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

RUN mkdir -p /data && chown -R 65532:65532 /data && chmod 0700 /data
WORKDIR /
USER 65532:65532

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/sarga", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/sarga"]
CMD ["serve"]
