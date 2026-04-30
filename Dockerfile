# ── Stage 1: build Beacon SPA ─────────────────────────────────────────────────
FROM node:22-alpine AS beacon-builder

WORKDIR /src/beacon
COPY beacon/package.json beacon/package-lock.json ./
RUN npm ci --silent
COPY beacon/ ./
RUN npm run build

# ── Stage 2: build korva-vault ────────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY . .

# Embed the Beacon dist built in stage 1.
COPY --from=beacon-builder /src/beacon/dist ./vault/internal/ui/dist

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN go work sync && \
    CGO_ENABLED=0 GOOS=linux go build \
      -tags embedui \
      -ldflags="-s -w \
        -X github.com/alcandev/korva/internal/version.Version=${VERSION} \
        -X github.com/alcandev/korva/internal/version.Commit=${COMMIT} \
        -X github.com/alcandev/korva/internal/version.Date=${BUILD_DATE}" \
      -o /bin/korva-vault \
      ./vault/cmd/korva-vault/

# ── Stage 3: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S korva && adduser -S korva -G korva

COPY --from=go-builder /bin/korva-vault /usr/local/bin/korva-vault

# Data directory — mount a named volume here in production.
RUN mkdir -p /data && chown korva:korva /data

USER korva
VOLUME ["/data"]
EXPOSE 7437

# Defaults — all overridable via env vars in Coolify / docker-compose.
ENV KORVA_VAULT_DB=/data/vault.db \
    KORVA_VAULT_PORT=7437 \
    KORVA_VAULT_HOST=0.0.0.0 \
    KORVA_VAULT_MODE=http

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:7437/healthz | grep -q '"status":"ok"' || exit 1

ENTRYPOINT ["korva-vault"]
