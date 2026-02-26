# examples/todo-react/Dockerfile
#
# Standalone build — no monorepo required.
# Build context is the example directory itself:
#
#   docker build -t rep-todo .
#
# Run:
#   docker run --rm -p 8080:8080 \
#     -e REP_PUBLIC_APP_TITLE="REP Todo" \
#     -e REP_PUBLIC_ENV_NAME=development \
#     -e REP_PUBLIC_API_URL=http://localhost:3001 \
#     -e REP_PUBLIC_MAX_TODOS=5 \
#     -e REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
#     rep-todo

# ── Stage 1: Download pre-built REP Gateway binary ────────────────────────────
FROM alpine:3.21 AS gateway

ARG GATEWAY_VERSION=0.1.3
ARG TARGETARCH=amd64

RUN apk add --no-cache curl ca-certificates && \
    ARCHIVE="rep-gateway_${GATEWAY_VERSION}_linux_${TARGETARCH}.tar.gz" && \
    curl -fsSL \
      "https://github.com/RuachTech/rep/releases/download/v${GATEWAY_VERSION}/${ARCHIVE}" \
      -o /tmp/gateway.tar.gz && \
    tar -xzf /tmp/gateway.tar.gz -C /tmp && \
    mv /tmp/rep-gateway /rep-gateway && \
    chmod +x /rep-gateway

# ── Stage 2: Build the React app ───────────────────────────────────────────────
FROM node:20-alpine AS app

WORKDIR /app

# Copy package files and install dependencies from npm registry.
COPY package.json ./
# install on main deps only,
RUN npm install --omit=dev

# Copy source and build.
COPY . .
RUN npm run build

# ── Stage 3: Minimal runtime image (FROM scratch) ──────────────────────────────
FROM scratch

# Gateway binary.
COPY --from=gateway /rep-gateway /rep-gateway

# Built static files.
COPY --from=app /app/dist /static

# CA certificates for HTTPS upstream calls (if proxying).
COPY --from=gateway /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user (nobody).
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static"]
