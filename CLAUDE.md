# CLAUDE.md — Agent context for REP (Runtime Environment Protocol)

## Project Identity

**Name:** REP — Runtime Environment Protocol
**Organisation:** Ruach Tech (`github.com/ruachtech`)
**Author:** Olamide Adebayo
**License:** Spec documents under CC BY 4.0, code under Apache 2.0
**Status:** Pre-release — all packages implemented, CI/CD in place, pending first public release

---

## What This Project Is

REP is an open specification and reference implementation for injecting environment variables into browser-hosted applications **at container runtime** rather than at build time. It solves the fundamental contradiction that containers should be environment-agnostic artifacts, but frontend builds bake in environment-specific config via static string replacement (`process.env.*`, `import.meta.env.*`).

REP introduces:
1. A **three-tier security classification** (PUBLIC / SENSITIVE / SERVER) via naming convention
2. **AES-256-GCM encryption** for sensitive variables
3. **HMAC-SHA256 integrity verification** + SRI hashing on every payload
4. **Automatic secret detection guardrails** (Shannon entropy, known key format matching)
5. **Hot config reload** via Server-Sent Events (optional)
6. A **lightweight Go gateway binary** (~3–5MB, zero dependencies, `FROM scratch` compatible)
7. A **zero-dependency TypeScript SDK** (~1.5KB gzipped) with synchronous access for public vars

---

## File Structure

```
rep/
├── .github/workflows/
│   ├── gateway.yml                    # Go CI (vet, test, build)
│   ├── sdk.yml                        # TypeScript packages CI
│   └── release-sdk.yml                # Release workflow (npm + GoReleaser + Docker)
│
├── package.json                       # Monorepo root (pnpm 9.0.0, private)
├── pnpm-workspace.yaml                # Workspace: sdk, cli, adapters/*, codemod, examples/*
├── pnpm-lock.yaml
├── release-please-config.json         # Release-please config for all packages
├── .release-please-manifest.json      # Per-package version tracker
├── .gitignore
├── CONTRIBUTING.md
├── README.md
├── LICENSE
│
├── spec/
│   ├── REP-RFC-0001.md                # Core protocol specification (14 sections)
│   ├── SECURITY-MODEL.md              # Threat model, 7 threat analyses
│   └── INTEGRATION-GUIDE.md           # Framework patterns, CI/CD, K8s, migration
│
├── schema/
│   ├── rep-payload.schema.json
│   └── rep-manifest.schema.json
│
├── gateway/                           # Go reference implementation (zero deps)
│   ├── .goreleaser.yml                # Multi-platform release config
│   ├── Dockerfile                     # Multi-stage, FROM scratch final
│   ├── Makefile
│   ├── VERSION                        # "0.1.0"
│   ├── go.mod                         # Go 1.24.5, zero external deps
│   ├── cmd/rep-gateway/
│   │   └── main.go                    # Entrypoint: flags, signals, graceful shutdown
│   ├── internal/
│   │   ├── config/
│   │   │   ├── config.go              # CLI flag + env var parsing (REP_GATEWAY_*)
│   │   │   ├── classify.go            # Reads REP_* vars → PUBLIC/SENSITIVE/SERVER
│   │   │   ├── envfile.go             # .env file parsing
│   │   │   └── *_test.go
│   │   ├── crypto/
│   │   │   ├── crypto.go              # AES-256-GCM, HMAC-SHA256, SRI hash
│   │   │   ├── session_key.go         # /rep/session-key: rate limiting, single-use, CORS
│   │   │   └── *_test.go
│   │   ├── guardrails/
│   │   │   ├── guardrails.go          # Secret detection: entropy, known formats
│   │   │   └── guardrails_test.go
│   │   ├── health/
│   │   │   ├── health.go              # /rep/health endpoint
│   │   │   └── health_test.go
│   │   ├── hotreload/
│   │   │   ├── hotreload.go           # /rep/changes SSE hub
│   │   │   └── hotreload_test.go
│   │   ├── inject/
│   │   │   ├── inject.go              # HTML injection middleware (mutex-protected, compression-aware)
│   │   │   └── inject_test.go
│   │   ├── manifest/
│   │   │   ├── manifest.go            # Hand-rolled YAML subset parser (zero deps)
│   │   │   └── manifest_test.go
│   │   └── server/
│   │       ├── server.go              # Orchestrator: startup, proxy/embedded modes, reload
│   │       └── server_test.go
│   ├── pkg/payload/
│   │   ├── payload.go                 # Payload builder: JSON + <script> tag
│   │   └── payload_test.go
│   └── testdata/static/
│       └── index.html
│
├── sdk/                               # @rep-protocol/sdk (zero runtime deps)
│   ├── package.json                   # v0.1.2
│   ├── src/
│   │   ├── index.ts                   # get(), getSecure(), onChange(), verify(), meta()
│   │   └── __tests__/index.test.ts    # 24 tests
│   └── vitest.config.ts
│
├── cli/                               # @rep-protocol/cli
│   ├── package.json                   # v0.1.1
│   ├── bin/rep.js                     # Executable entry
│   ├── scripts/postinstall.js         # Copies gateway binary per OS
│   └── src/
│       ├── commands/
│       │   ├── dev.ts                 # Dev server (wraps gateway)
│       │   ├── lint.ts                # Bundle secret scanning
│       │   ├── typegen.ts             # TypeScript type generation
│       │   └── validate.ts            # Manifest validation
│       └── utils/
│           ├── guardrails.ts
│           ├── manifest.ts
│           └── __tests__/
│
├── adapters/
│   ├── react/                         # @rep-protocol/react — useRep(), useRepSecure()
│   ├── vue/                           # @rep-protocol/vue — useRep() composable
│   └── svelte/                        # @rep-protocol/svelte — repStore()
│
├── codemod/                           # @rep-protocol/codemod
│   └── src/transforms/               # CRA, Next.js, Vite transforms
│
└── examples/
    ├── .rep.yaml                      # Example manifest
    └── todo-react/                    # Full React todo app with REP gateway
```

---

## Architecture

### How REP Works (High-Level Flow)

```
Container boot:
  1. Gateway reads all REP_* environment variables (+ optional .env file)
  2. Classifies into PUBLIC / SENSITIVE / SERVER tiers (by prefix)
  3. Runs guardrails (entropy scan, known format detection) on PUBLIC vars
  4. Generates ephemeral AES-256 key + HMAC-256 secret (in-memory only)
  5. Encrypts SENSITIVE vars → base64 blob
  6. Computes HMAC integrity token
  7. Pre-renders <script id="__rep__" type="application/json"> tag

Request flow:
  Client → [REP Gateway :8080] → [Upstream :80 (nginx/caddy)]

  For HTML responses (Content-Type: text/html):
    Gateway intercepts response, injects <script> before </head>

  For all other responses:
    Passed through unmodified
```

### Variable Classification (Prefix Convention)

| Prefix | Tier | Behaviour |
|---|---|---|
| `REP_PUBLIC_*` | PUBLIC | Plaintext JSON in page source. Synchronous access via `rep.get()`. |
| `REP_SENSITIVE_*` | SENSITIVE | AES-256-GCM encrypted blob. Decrypted via session key. `await rep.getSecure()`. |
| `REP_SERVER_*` | SERVER | **Never sent to client.** Gateway-only. |
| `REP_GATEWAY_*` | (config) | Gateway configuration, not app variables. Ignored by classifier. |

Prefixes are stripped in the payload: `REP_PUBLIC_API_URL` → `"API_URL"` in the JSON.

### Gateway Modes

- **Proxy mode (default):** Reverse proxy to upstream (nginx, caddy, etc.). Injects into proxied HTML.
- **Embedded mode:** Serves static files directly. No upstream needed. Enables `FROM scratch` containers.

### Endpoints

| Path | Method | Description |
|---|---|---|
| `/rep/health` | GET | Health check — variable counts, guardrail status, uptime |
| `/rep/session-key` | GET | Short-lived AES decryption key (30s TTL, single-use, rate-limited, CORS-checked) |
| `/rep/changes` | GET (SSE) | Hot reload event stream (if `--hot-reload` enabled) |
| `/*` | * | All other requests proxied/served with HTML injection |

### Payload Wire Format

Injected into HTML as `<script id="__rep__" type="application/json">`:

```json
{
  "public": {
    "API_URL": "https://api.example.com",
    "FEATURE_FLAGS": "dark-mode,beta"
  },
  "sensitive": "<base64 AES-256-GCM blob: [12B nonce][ciphertext][16B auth tag]>",
  "_meta": {
    "version": "0.1.0",
    "injected_at": "2026-02-18T14:30:00.000Z",
    "integrity": "hmac-sha256:<base64 signature>",
    "key_endpoint": "/rep/session-key",
    "hot_reload": "/rep/changes",
    "ttl": 0
  }
}
```

The `<script>` tag also carries `data-rep-integrity="sha256-<base64>"` for SRI verification.

### Security Model (Summary)

- **PUBLIC vars are visible in page source.** By design. Don't put secrets here.
- **SENSITIVE vars are encrypted at rest in HTML.** Requires a session key endpoint call to decrypt. Session keys are single-use, 30s TTL, rate-limited, origin-validated.
- **SERVER vars never leave the gateway process.** Only tier suitable for true secrets.
- **Integrity token detects transit tampering** (CDN compromise, MITM). Does NOT authenticate the source.
- **Guardrails detect misclassified secrets** at boot: Shannon entropy > 4.5, known formats (AKIA*, eyJ*, ghp_*, sk_live_*, sk-*, xoxb-*, -----BEGIN, etc.).
- **`--strict` mode** makes guardrail warnings into hard failures.

Full threat analysis in `spec/SECURITY-MODEL.md`.

---

## Technical Decisions & Rationale

| Decision | Rationale |
|---|---|
| **Go for the gateway** | Static compilation (CGO_ENABLED=0), zero runtime deps, ~3MB binary, `FROM scratch` compatible. No Node.js or bash needed in prod. |
| **Zero external Go dependencies** | Minimises supply chain risk. Only stdlib + crypto. Manifest parsing uses a hand-rolled YAML subset parser (~250 lines) to maintain this constraint. |
| **`pkg/payload` imports from `internal/`** | Valid Go — `internal/` rule only restricts imports from outside the parent directory tree. Both live under `gateway/`. |
| **`inject.go` strips `Accept-Encoding`** | Upstreams always respond with identity encoding, avoiding decompress/recompress. Gzip fallback via `compress/gzip` (stdlib) for non-compliant upstreams. Brotli unsupported (no stdlib, zero-dep constraint) — logged and passed through uninjected. |
| **`type="application/json"` on script tag** | Browser does NOT execute it. Inert data. No CSP conflicts. |
| **Synchronous `get()`, async `getSecure()`** | Public vars available instantly (no loading states). Sensitive vars accept one network call. |
| **HMAC integrity computed over canonicalised JSON** | Deterministic (sorted keys, no whitespace). Verifiable independently. |
| **Ephemeral keys (generated at startup, never stored)** | Key compromise requires gateway process compromise. No key storage = no key theft from disk. |
| **Session keys are single-use** | Prevents replay. Rate limiting prevents brute force. |
| **Prefix-based classification** | Forces explicit security decision per variable. No ambiguity. |
| **Hot reload via SSE (not WebSocket)** | Simpler, auto-reconnects, works through most proxies, sufficient for one-directional config push. |
| **pnpm monorepo** | Single lockfile, workspace linking, strict dependency resolution. All TS packages in one repo. |
| **release-please** | Conventional-commit-driven releases, independent versioning per package, automated changelogs. |

---

## Remaining Work

### Robustness
- [ ] **Handle chunked transfer encoding** — the recorder buffers the entire response. Consider streaming for large non-HTML responses (pass through without buffering).

---

## Code Conventions

### Go (Gateway)

- **Standard library only.** No third-party dependencies. If you need something, implement it.
- **Package names are single words.** `config`, `crypto`, `inject`, not `env_config` or `html_inject`.
- **`internal/` for implementation, `pkg/` for public API.** Only `pkg/payload` is importable by external Go code.
- **Structured logging via `log/slog`.** All security events use specific event names: `rep.guardrail.warning`, `rep.session_key.issued`, `rep.session_key.rejected`, `rep.session_key.rate_limited`, `rep.config.changed`, `rep.inject.html`.
- **Error wrapping with `fmt.Errorf("context: %w", err)`.** Always add context to errors.
- **No `init()` functions except where strictly necessary.** The gateway's lifecycle is explicit.

### Go (Testing)

- **All tests use stdlib only** (`testing`, `net/http/httptest`). No testify or third-party test frameworks.
- **Use `t.Setenv()` for env var tests.** Auto-cleans on test completion. Do NOT use `os.Setenv`/`os.Unsetenv` directly — it breaks `t.Setenv` cleanup.
- **Server integration tests build the mux directly** rather than going through `server.New()` to avoid env var pollution. See `server_test.go:buildTestMux()`.
- **Run with `-race` flag.** The inject middleware has concurrent access patterns that must be validated.
- **`clearREPEnv()` helper in `classify_test.go`** removes stale REP_* vars from the process environment for clean test isolation.

### TypeScript (SDK)

- **Zero runtime dependencies.** The `package.json` only has devDependencies (tsup, typescript, vitest, jsdom).
- **Module-scoped state with underscore prefix.** `_payload`, `_available`, `_tampered`.
- **Synchronous init, lazy async.** SDK reads the DOM synchronously on import. SSE connects lazily on first `onChange()` call.
- **Named export + default namespace.** Both `import { get } from '@rep-protocol/sdk'` and `import { rep } from '@rep-protocol/sdk'` work.

### TypeScript (Testing)

- **Vitest + jsdom** across all TS packages.
- **`vi.resetModules()` before each test** in SDK tests. The SDK's `_init()` runs on module load, so each test must reset module cache and use dynamic `import('../index')` to get a fresh instance.
- **DOM cleanup in `beforeEach`.** Clear `document.head` and `document.body` before each test.
- **Mock `EventSource`** for hot reload tests. jsdom doesn't provide `EventSource`.
- **Mock `fetch`** for `getSecure()` tests.

---

## Build & Run Commands

```bash
# Monorepo (from root)
pnpm install                    # Install all workspace dependencies
pnpm -r build                   # Build all TS packages
pnpm -r test                    # Test all TS packages

# Gateway (from gateway/)
make build                      # Build for current platform → bin/rep-gateway
make build-linux                # Cross-compile for Linux amd64
make docker                     # Build Docker image
make test                       # Run all tests
make run-example                # Run locally with example env vars
go test -race ./...             # Run all tests with race detector (recommended)
go test -race -count=1 ./...    # Same, bypassing cache

# SDK (from sdk/)
pnpm build                      # Build CJS + ESM + types → dist/
pnpm test                       # Run vitest (24 tests, jsdom environment)

# CLI (from cli/)
pnpm build && pnpm test

# Adapters (from adapters/react/, adapters/vue/, adapters/svelte/)
pnpm build && pnpm test
```

---

## Environment Variables (Application)

```bash
# Application variables (injected into HTML)
REP_PUBLIC_API_URL="https://api.example.com"           # → rep.get('API_URL')
REP_PUBLIC_FEATURE_FLAGS="dark-mode,beta"               # → rep.get('FEATURE_FLAGS')
REP_SENSITIVE_ANALYTICS_KEY="UA-12345-1"                # → await rep.getSecure('ANALYTICS_KEY')
REP_SERVER_DB_PASSWORD="never-reaches-browser"          # Gateway-only

# Gateway configuration (NOT injected into HTML)
REP_GATEWAY_MODE=proxy
REP_GATEWAY_PORT=8080
REP_GATEWAY_UPSTREAM=localhost:80
REP_GATEWAY_STRICT=true
REP_GATEWAY_HOT_RELOAD=true
REP_GATEWAY_LOG_FORMAT=json
REP_GATEWAY_ALLOWED_ORIGINS=https://app.example.com
```

---

## Non-Obvious Design Choices to Preserve

1. **`type="application/json"` on the script tag is critical.** It prevents the browser from executing the tag. It's inert data. Do NOT change this to `type="text/javascript"`.

2. **The SDK's `get()` MUST remain synchronous.** No promises, no async, no lazy loading. This is a core design requirement (§R4). If `get()` becomes async, every consuming component needs loading states, and the DX advantage over `fetch('/config.json')` vanishes.

3. **The gateway generates NEW ephemeral keys on every restart.** This is intentional. It means a gateway restart invalidates all previously issued session keys and re-encrypts the sensitive blob.

4. **The HMAC secret is never transmitted.** It exists only in the gateway's memory. The SDK cannot verify the HMAC — it can only verify the SRI hash (content matches the `data-rep-integrity` attribute).

5. **Prefix stripping creates a flat namespace.** `REP_PUBLIC_API_URL` and `REP_SENSITIVE_API_URL` would both become `API_URL` in the payload — which is why the gateway MUST reject this collision at startup. This is enforced in `classify.go`.

6. **Hot reload SSE connects lazily.** The SDK does NOT establish an SSE connection on import. It only connects when `onChange()` or `onAnyChange()` is first called.

---

## Key Spec References

| Topic | Location |
|---|---|
| Variable classification rules | REP-RFC-0001.md §3 |
| Secret detection guardrails | REP-RFC-0001.md §3.3 |
| Gateway startup sequence | REP-RFC-0001.md §4.2 |
| HTML injection rules | REP-RFC-0001.md §4.3 |
| Session key endpoint | REP-RFC-0001.md §4.4 |
| Health check endpoint | REP-RFC-0001.md §4.5 |
| Hot reload SSE | REP-RFC-0001.md §4.6 |
| Client SDK API | REP-RFC-0001.md §5.2 |
| SDK init (must be sync) | REP-RFC-0001.md §5.3 |
| Manifest schema | REP-RFC-0001.md §6 |
| Gateway CLI flags | REP-RFC-0001.md §7 |
| Payload JSON schema | REP-RFC-0001.md §8.1 |
| Encrypted blob format | REP-RFC-0001.md §8.2 |
| HMAC integrity | REP-RFC-0001.md §8.3 |
| Deployment patterns | REP-RFC-0001.md §9 |
| Threat analyses | SECURITY-MODEL.md §2 |
| CSP recommendations | SECURITY-MODEL.md §4.2 |
| Framework integration | INTEGRATION-GUIDE.md §2 |
| CI/CD patterns | INTEGRATION-GUIDE.md §3 |
