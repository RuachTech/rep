# examples/todo-react/Dockerfile
#
# Builds a self-contained image: REP gateway binary + built React app.
# Build context must be the REPO ROOT:
#
#   docker build -f examples/todo-react/Dockerfile -t rep-todo .
#
# Run:
#   docker run --rm -p 8080:8080 \
#     -e REP_PUBLIC_APP_TITLE="REP Todo" \
#     -e REP_PUBLIC_ENV_NAME=development \
#     -e REP_PUBLIC_API_URL=http://localhost:3001 \
#     -e REP_PUBLIC_MAX_TODOS=5 \
#     -e REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
#     rep-todo

# ── Stage 1: Build the REP Gateway ─────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine AS gateway

ARG TARGETOS=linux
ARG TARGETARCH=arm64

WORKDIR /src
COPY gateway/go.mod ./
RUN go mod download
COPY gateway/ .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /rep-gateway \
    ./cmd/rep-gateway

# ── Stage 2: Build the React app ───────────────────────────────────────────────
FROM node:20-alpine AS app

RUN corepack enable && corepack prepare pnpm@9 --activate

WORKDIR /repo

# Copy workspace descriptor and lockfile first (layer cache for pnpm install).
COPY pnpm-workspace.yaml pnpm-lock.yaml ./

# Copy every package.json in the workspace so pnpm can resolve the full graph.
COPY sdk/package.json                    sdk/
COPY cli/package.json                    cli/
COPY cli/bin/rep.js                      cli/bin/
COPY cli/scripts/postinstall.js          cli/scripts/
COPY codemod/package.json                codemod/
COPY adapters/react/package.json         adapters/react/
COPY adapters/vue/package.json           adapters/vue/
COPY adapters/svelte/package.json        adapters/svelte/
COPY examples/todo-react/package.json   examples/todo-react/

RUN pnpm install --frozen-lockfile

# Copy source for the packages we actually need to build.
COPY sdk/src/              sdk/src/
COPY sdk/tsconfig.json     sdk/

COPY adapters/react/src/         adapters/react/src/
COPY adapters/react/tsconfig.json adapters/react/

COPY examples/todo-react/   examples/todo-react/

# Build: sdk → react adapter → app (in dependency order).
RUN pnpm --filter @rep-protocol/sdk   run build && \
    pnpm --filter @rep-protocol/react run build && \
    pnpm --filter @rep-protocol/example-todo-react run build

# ── Stage 3: Minimal runtime image (FROM scratch) ──────────────────────────────
FROM scratch

# Gateway binary.
COPY --from=gateway /rep-gateway /rep-gateway

# Built static files.
COPY --from=app /repo/examples/todo-react/dist /static

# CA certificates for HTTPS upstream calls (if proxying).
COPY --from=gateway /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user (nobody).
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static"]
