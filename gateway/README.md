# REP Gateway — Reference Implementation

The REP Gateway is a lightweight, statically-compiled Go binary that implements the [Runtime Environment Protocol (REP-RFC-0001)](../spec/REP-RFC-0001.md).

It reads `REP_*` environment variables at startup, classifies them into PUBLIC / SENSITIVE / SERVER tiers, and injects a signed JSON payload into HTML responses — enabling true "build once, deploy anywhere" for containerised frontend applications.

## Features

- **Zero dependencies** — uses only the Go standard library.
- **~3–5MB binary** — statically compiled, `FROM scratch` compatible.
- **Two modes** — reverse proxy (in front of nginx/caddy) or embedded file server.
- **Three-tier security** — PUBLIC (plaintext), SENSITIVE (AES-256-GCM encrypted), SERVER (never sent to client).
- **Integrity verification** — HMAC-SHA256 signature + SRI hash on every payload.
- **Secret detection guardrails** — scans PUBLIC vars for misclassified secrets at startup.
- **Hot reload** — optional SSE endpoint for live config updates.
- **Structured logging** — JSON or text, with security-relevant event types.

## Quick Start

### Build

```bash
# Build for your current platform
make build

# Build for Linux (Docker/K8s target)
make build-linux

# Build Docker image (~5MB)
make docker
```

### Run Locally

```bash
# Embedded mode — serve static files directly
REP_PUBLIC_API_URL="https://api.example.com" \
REP_PUBLIC_FEATURE_FLAGS="dark-mode,beta" \
REP_SENSITIVE_ANALYTICS_KEY="UA-12345-1" \
REP_SERVER_DB_PASSWORD="secret" \
./bin/rep-gateway --mode embedded --static-dir ./dist --log-format text
```

### Docker

```bash
# Build your frontend image with the REP gateway
docker build -t myapp .

# Run with different configs — SAME image, different env vars
docker run -p 8080:8080 \
  -e REP_PUBLIC_API_URL=https://api.staging.example.com \
  -e REP_PUBLIC_ENV_NAME=staging \
  myapp

docker run -p 8080:8080 \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_ENV_NAME=production \
  myapp
```

## Configuration

All configuration is via CLI flags or `REP_GATEWAY_*` environment variables. Flags take precedence.

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--mode` | `REP_GATEWAY_MODE` | `proxy` | `"proxy"` or `"embedded"` |
| `--upstream` | `REP_GATEWAY_UPSTREAM` | `localhost:80` | Upstream address (proxy mode) |
| `--port` | `REP_GATEWAY_PORT` | `8080` | Listen port |
| `--static-dir` | `REP_GATEWAY_STATIC_DIR` | `/usr/share/nginx/html` | Static files dir (embedded mode) |
| `--strict` | `REP_GATEWAY_STRICT` | `false` | Fail on guardrail warnings |
| `--hot-reload` | `REP_GATEWAY_HOT_RELOAD` | `false` | Enable SSE hot reload |
| `--hot-reload-mode` | `REP_GATEWAY_HOT_RELOAD_MODE` | `signal` | `file_watch`, `signal`, or `poll` |
| `--log-format` | `REP_GATEWAY_LOG_FORMAT` | `json` | `json` or `text` |
| `--log-level` | `REP_GATEWAY_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `--allowed-origins` | `REP_GATEWAY_ALLOWED_ORIGINS` | (empty) | CORS origins for session key endpoint |
| `--session-key-ttl` | `REP_GATEWAY_SESSION_KEY_TTL` | `30s` | Session key time-to-live |
| `--session-key-max-rate` | `REP_GATEWAY_SESSION_KEY_MAX_RATE` | `10` | Max session key requests/min/IP |
| `--health-port` | `REP_GATEWAY_HEALTH_PORT` | `0` | Separate health check port (0 = same) |
| `--version` | — | — | Print version and exit |

## Endpoints

| Path | Method | Description |
|---|---|---|
| `/rep/health` | GET | Health check with variable counts and guardrail status |
| `/rep/session-key` | GET | Short-lived decryption key for SENSITIVE tier variables |
| `/rep/changes` | GET (SSE) | Hot reload event stream (if enabled) |
| `/*` | * | Proxied/served with HTML injection |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Container                                                │
│                                                          │
│   ┌────────────────────────────────────────────────┐    │
│   │ REP Gateway (:8080)                             │    │
│   │                                                  │    │
│   │  1. Reads REP_* env vars at boot                │    │
│   │  2. Classifies: PUBLIC / SENSITIVE / SERVER     │    │
│   │  3. Runs guardrails on PUBLIC values            │    │
│   │  4. Generates AES-256 key + HMAC secret         │    │
│   │  5. Encrypts SENSITIVE vars                     │    │
│   │  6. Computes integrity token                    │    │
│   │  7. Injects <script> into HTML responses        │    │
│   │                                                  │    │
│   │  ┌─────────────────────────────────────────┐    │    │
│   │  │ Proxy Mode        │ Embedded Mode        │    │    │
│   │  │ → upstream :80    │ → /static/*          │    │    │
│   │  └─────────────────────────────────────────┘    │    │
│   └────────────────────────────────────────────────┘    │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

## Project Structure

```
gateway/
├── cmd/rep-gateway/
│   └── main.go              # Entrypoint, signal handling
├── internal/
│   ├── config/
│   │   ├── config.go        # Flag/env parsing
│   │   └── classify.go      # REP_* variable classification
│   ├── crypto/
│   │   ├── crypto.go        # AES-256-GCM encryption, HMAC integrity
│   │   └── session_key.go   # /rep/session-key endpoint
│   ├── guardrails/
│   │   └── guardrails.go    # Secret detection heuristics
│   ├── health/
│   │   └── health.go        # /rep/health endpoint
│   ├── hotreload/
│   │   └── hotreload.go     # /rep/changes SSE hub
│   ├── inject/
│   │   └── inject.go        # HTML injection middleware
│   └── server/
│       └── server.go        # Server orchestration, startup sequence
├── pkg/payload/
│   └── payload.go           # Payload builder, JSON serialisation, <script> tag
├── Dockerfile               # Multi-stage, FROM scratch
├── Makefile                  # Build, test, docker targets
├── go.mod                   # Zero external dependencies
└── README.md                # This file
```

## Specification Compliance

This implementation targets **REP-RFC-0001 v0.1.0**. See the [conformance checklist](../spec/REP-RFC-0001.md#11-conformance) for full details.

| Requirement | Status |
|---|---|
| Reads only REP_* prefixed variables | ✅ |
| Three-tier classification | ✅ |
| Prefix stripping | ✅ |
| Name collision detection | ✅ |
| Secret detection guardrails | ✅ |
| HTML injection via `<script id="__rep__">` | ✅ |
| HMAC-SHA256 integrity | ✅ |
| AES-256-GCM encryption | ✅ |
| Single-use session keys | ✅ |
| SERVER tier never sent to client | ✅ |
| Hot reload (optional) | ✅ |
| Health check endpoint | ✅ |

## License

Apache 2.0 — see [LICENSE](../LICENSE).
