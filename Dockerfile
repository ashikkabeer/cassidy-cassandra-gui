# syntax=docker/dockerfile:1

# ── 1. Frontend build ───────────────────────────────────────────────────────
FROM node:22-bookworm-slim AS frontend
WORKDIR /app/web
RUN corepack enable
# Cache deps on the lockfile alone.
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build
# → /app/web/webfs/dist

# ── 2. Backend build (cgo-free static binary, SPA embedded) ──────────────────
FROM golang:1.25-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overlay the freshly built SPA over the committed .gitkeep placeholder so
# //go:embed all:dist picks up real assets.
COPY --from=frontend /app/web/webfs/dist ./web/webfs/dist
# Pre-create a data dir owned by the runtime non-root uid so a fresh named
# volume inherits writable ownership (distroless has no shell to chown at boot).
RUN mkdir -p /out/data
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/cassidy ./cmd/server

# ── 3. Runtime (distroless static, non-root) ─────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend --chown=nonroot:nonroot /out/cassidy /usr/local/bin/cassidy
COPY --from=backend --chown=nonroot:nonroot /out/data /data
ENV CASSIDY_LISTEN_ADDR=:8080 \
    CASSIDY_DATA_DIR=/data
EXPOSE 8080
VOLUME /data
USER nonroot
ENTRYPOINT ["/usr/local/bin/cassidy"]
