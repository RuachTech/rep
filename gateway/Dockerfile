# REP Gateway — Multi-stage build.
# Produces a minimal, FROM scratch image containing only the static binary.
#
# Build:
#   docker build -t rep-gateway .
#
# Size: ~5MB (static Go binary, no OS, no shell).

# ── Stage 1: Build ──────────────────────────────────────────
FROM golang:1.22-alpine AS build

WORKDIR /src

# Cache dependency downloads (go.mod only, no go.sum since zero deps).
COPY go.mod ./
RUN go mod download

# Copy source and build.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(cat VERSION 2>/dev/null || echo 0.1.0)" \
    -trimpath \
    -o /rep-gateway \
    ./cmd/rep-gateway

# ── Stage 2: Runtime (FROM scratch) ─────────────────────────
FROM scratch

# Copy the static binary.
COPY --from=build /rep-gateway /rep-gateway

# Copy CA certificates for HTTPS upstream support.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Non-root user (UID 65534 = nobody).
USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/rep-gateway"]
