# examples/simple-html/Dockerfile
#
# Zero build-step example — a single HTML file served by the REP gateway
# in embedded mode. The SDK is loaded via esm.sh at runtime.
#
# Build:
#   docker build -t rep-simple-html .
#
# Run:
#   docker run --rm -p 8080:8080 \
#     -e REP_PUBLIC_APP_TITLE="My App" \
#     -e REP_PUBLIC_API_URL=https://api.example.com \
#     -e REP_PUBLIC_ENV_NAME=production \
#     -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,beta \
#     -e REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
#     rep-simple-html

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

# ── Stage 2: Minimal runtime image (FROM scratch) ──────────────────────────────
FROM scratch

# Gateway binary
COPY --from=gateway /rep-gateway /rep-gateway

# Static HTML file — the only "app" artefact
COPY index.html /static/index.html

# CA certificates for esm.sh and /rep/session-key HTTPS calls from the browser
COPY --from=gateway /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user (nobody)
USER 65534:65534

EXPOSE 8080

# Embedded mode: gateway serves /static directly, no upstream needed
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static"]
