# Build context must be the repo root (go.work spans multiple modules).
# docker build -f forge/licensing-server/Dockerfile \
#              --build-arg VERSION=v1.0.0 \
#              -t ghcr.io/alcandev/korva-licensing:latest .

# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN go work sync && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /bin/korva-licensing \
      github.com/alcandev/korva/forge/licensing-server

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S korva && adduser -S korva -G korva

COPY --from=builder /bin/korva-licensing /usr/local/bin/korva-licensing

# Data directory — mount a persistent volume here in production
RUN mkdir -p /data && chown korva:korva /data

USER korva
VOLUME ["/data"]
EXPOSE 7440

# Required at runtime — never bake secrets into the image:
#   KORVA_LICENSING_ADMIN_SECRET   — bearer token for /v1/issue
#   KORVA_LICENSING_PRIVATE_KEY_PEM or KORVA_LICENSING_PRIVATE_KEY_FILE
ENV KORVA_LICENSING_PORT=7440 \
    KORVA_LICENSING_DB=/data/licensing.db

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:7440/v1/health | grep -q '"ok"' || exit 1

ENTRYPOINT ["korva-licensing"]
