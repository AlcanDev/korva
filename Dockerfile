# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY . .

RUN go work sync && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w \
        -X github.com/alcandev/korva/internal/version.Version=${VERSION:-dev} \
        -X github.com/alcandev/korva/internal/version.Commit=${COMMIT:-none} \
        -X github.com/alcandev/korva/internal/version.Date=${BUILD_DATE:-unknown}" \
      -o /bin/korva-vault \
      ./vault/cmd/korva-vault/

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S korva && adduser -S korva -G korva

COPY --from=builder /bin/korva-vault /usr/local/bin/korva-vault

# Data directory (mount a volume here in production)
RUN mkdir -p /data && chown korva:korva /data

USER korva
VOLUME ["/data"]
EXPOSE 7437

ENV KORVA_VAULT_DB=/data/vault.db \
    KORVA_VAULT_PORT=7437 \
    KORVA_VAULT_MODE=http

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:7437/healthz | grep -q '"status":"ok"' || exit 1

ENTRYPOINT ["korva-vault"]
CMD ["--mode=http", "--port=7437", "--db=/data/vault.db"]
