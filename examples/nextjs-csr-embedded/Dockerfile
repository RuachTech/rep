# examples/nextjs-csr-embedded/Dockerfile
#
# Next.js static export served by the REP gateway in embedded mode.
# Final image is FROM scratch — no OS, no shell, no package manager.
#
# Build:
#   docker build -t rep-nextjs-csr .
#
# Run:
#   docker run --rm -p 8080:8080 \
#     -e REP_PUBLIC_API_URL=https://api.example.com \
#     -e REP_PUBLIC_ENV_NAME=production \
#     -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,new-checkout \
#     -e REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
#     rep-nextjs-csr
#
# For Kubernetes deployment, see k8s/ in this directory.

# ── Stage 1: Download pre-built REP Gateway binary ────────────────────────────
FROM alpine:3.21 AS gateway

ARG GATEWAY_VERSION=0.1.3
ARG TARGETARCH=amd64

RUN apk add --no-cache curl ca-certificates && \
    ARCHIVE="rep-gateway_${GATEWAY_VERSION}_linux_${TARGETARCH}.tar.gz" && \
    curl -fsSL \
      "https://github.com/RuachTech/rep/releases/download/gateway/v${GATEWAY_VERSION}/${ARCHIVE}" \
      -o /tmp/gateway.tar.gz && \
    tar -xzf /tmp/gateway.tar.gz -C /tmp && \
    mv /tmp/rep-gateway /rep-gateway && \
    chmod +x /rep-gateway

# ── Stage 2: Build the Next.js static export ──────────────────────────────────
FROM node:22-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

# Build produces a fully static site in `out/` (next.config.ts: output: 'export')
# No env vars are baked in — the build output is environment-agnostic.
RUN npm run build

# ── Stage 3: Minimal runtime image (FROM scratch) ──────────────────────────────
FROM scratch

# Gateway binary
COPY --from=gateway /rep-gateway /rep-gateway

# Static export artefacts
COPY --from=builder /app/out /static

# CA certificates for /rep/session-key HTTPS calls
COPY --from=gateway /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user (nobody)
USER 65534:65534

EXPOSE 8080

# Embedded mode: gateway serves /static directly, no upstream needed
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static"]
