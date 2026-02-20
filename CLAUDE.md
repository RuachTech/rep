# CLAUDE.md — Agent context for REP (Runtime Environment Protocol)

## Project Identity

**Name:** REP — Runtime Environment Protocol
**Organisation:** Ruach Tech (`github.com/ruachtech`)
**Author:** Olamide Adebayo
**License:** Spec documents under CC BY 4.0, code under Apache 2.0
**Status:** Draft specification + reference implementation (pre-release, not yet published)

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

The positioning is: **"The missing security and standardisation layer for frontend runtime config"** — the first solution treating this as a security problem rather than just a convenience problem.

---

## Why This Exists — The Problem

Every modern frontend framework resolves environment variables at **build time** via static string replacement. The bundle is then plain JS/HTML/CSS — the browser has no concept of environment variables. This means:

- **One Docker image per environment** — defeats "build once, deploy anywhere"
- **Broken CI/CD promotion** — the tested artifact ≠ the deployed artifact
- **Config changes require rebuilds** — even for a single URL change
- **No security model** — every existing workaround dumps all vars as plaintext into `window.__ENV__`

### Existing Solutions (All Insufficient)

| Tool | Limitation |
|---|---|
| `envsubst` / `sed` on JS bundles | Fragile string replacement on minified code |
| Fetch `/config.json` at init | Network dependency, loading delay, race conditions |
| `window.__ENV__` via shell script | No standard, no security, requires bash in prod container |
| `runtime-env-cra` | CRA-only, no security model |
| `@beam-australia/react-env` | React/Next only, no security model |
| `@import-meta-env/unplugin` | Most sophisticated — but it's a build-tool plugin, not runtime infrastructure. Framework-coupled. No security classification, no encryption, no integrity verification |
| `vite-plugin-runtime-env` | Vite-specific, uses envsubst placeholders |

**What none of them have:** Security classification, encrypted sensitive vars, integrity verification, secret leak detection, hot reload, standalone binary, formal spec. REP has all of these.

### Competitive Research Summary

The strongest existing competitor is `@import-meta-env/unplugin`. It is fundamentally a **build tool plugin** that modifies bundler behaviour. REP is **runtime infrastructure** that doesn't touch the build at all. They are complementary, not competing.

The Parcel GitHub issue #4049 explicitly states: "sensitive environment variables are exposed to the frontend indiscriminately." This is the open wound REP addresses.

---

## File Structure

```
rep/
├── LICENSE                            # CC BY 4.0 (spec) + Apache 2.0 (code)
├── README.md                          # Project overview, quick start, positioning
│
├── spec/                              # Specification documents
│   ├── REP-RFC-0001.md                # The core protocol specification (14 sections)
│   ├── SECURITY-MODEL.md              # Threat model, trust boundaries, 7 threat analyses
│   └── INTEGRATION-GUIDE.md           # Framework patterns, CI/CD, K8s, migration checklist
│
├── schema/                            # Machine-readable schemas
│   ├── rep-payload.schema.json        # JSON Schema for the injected payload
│   └── rep-manifest.schema.json       # JSON Schema for .rep.yaml manifest file
│
├── examples/
│   └── .rep.yaml                      # Example manifest with all three tiers
│
├── gateway/                           # Go reference implementation
│   ├── README.md                      # Gateway-specific docs
│   ├── VERSION                        # Contains "0.1.0" — referenced by Makefile + Dockerfile
│   ├── Dockerfile                     # Multi-stage, FROM scratch final image
│   ├── Makefile                       # build, test, docker, cross-compile targets
│   ├── go.mod                         # Module: github.com/ruachtech/rep/gateway (Go 1.24, zero deps)
│   ├── go.sum                         # Module checksum file
│   ├── cmd/rep-gateway/
│   │   └── main.go                    # Entrypoint: flag parsing, signal handling, graceful shutdown
│   ├── internal/
│   │   ├── config/
│   │   │   ├── config.go              # CLI flag + env var parsing (REP_GATEWAY_* namespace)
│   │   │   ├── config_test.go         # Flag parsing, env overrides, precedence, validation
│   │   │   ├── classify.go            # Core classifier: reads REP_* vars → PUBLIC/SENSITIVE/SERVER
│   │   │   └── classify_test.go       # Prefix stripping, tier assignment, collision detection
│   │   ├── crypto/
│   │   │   ├── crypto.go              # AES-256-GCM encryption, HMAC-SHA256 integrity, SRI hash
│   │   │   ├── crypto_test.go         # Encrypt/decrypt roundtrip, wrong key/AAD, HMAC, SRI, canonicalize
│   │   │   ├── session_key.go         # /rep/session-key endpoint: rate limiting, single-use, CORS
│   │   │   └── session_key_test.go    # Success, CORS, rate limiting, method rejection, IP extraction
│   │   ├── guardrails/
│   │   │   ├── guardrails.go          # Secret detection: entropy, known formats (AWS, JWT, GitHub, Stripe, etc.)
│   │   │   └── guardrails_test.go     # Known formats, entropy, length anomaly, false positive avoidance
│   │   ├── health/
│   │   │   ├── health.go              # /rep/health endpoint: variable counts, guardrail status, uptime
│   │   │   └── health_test.go         # JSON shape, variable counts, uptime, guardrail warnings
│   │   ├── hotreload/
│   │   │   ├── hotreload.go           # /rep/changes SSE hub: broadcasts config deltas to clients
│   │   │   └── hotreload_test.go      # Broadcast, unsubscribe, client count, SSE headers/events
│   │   ├── inject/
│   │   │   ├── inject.go              # HTML injection middleware: mutex-protected, compression-aware
│   │   │   └── inject_test.go         # Injection positions, middleware, concurrent safety, decompression
│   │   └── server/
│   │       ├── server.go              # Server orchestrator: startup sequence, proxy/embedded modes, reload
│   │       └── server_test.go         # Integration: health, injection, session key endpoints
│   ├── pkg/payload/
│   │   ├── payload.go                 # Payload builder: constructs JSON, renders <script> tag
│   │   └── payload_test.go            # Build, script tag format, JSON validity, integrity format
│   └── testdata/
│       └── static/
│           └── index.html             # Minimal HTML test page for make run-example
│
└── sdk/                               # TypeScript client SDK
    ├── README.md                      # SDK-specific docs
    ├── package.json                   # @rep-protocol/sdk, zero runtime deps, tsup build
    ├── tsconfig.json                  # ES2020, strict, DOM lib
    ├── vitest.config.ts               # Vitest config: jsdom environment, globals
    └── src/
        ├── index.ts                   # Full SDK: get(), getSecure(), onChange(), verify(), meta()
        └── __tests__/
            └── index.test.ts          # 24 tests: get, getSecure, verify, meta, onChange, exports
```

---

## Architecture

### How REP Works (High-Level Flow)

```
Container boot:
  1. Gateway reads all REP_* environment variables
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
- **SENSITIVE vars are encrypted at rest in HTML.** Requires a session key endpoint call to decrypt. Session keys are single-use, 30s TTL, rate-limited, origin-validated. Raises the bar from "View Source" to "achieve XSS + make authed network call + exfiltrate within TTL."
- **SERVER vars never leave the gateway process.** Only tier suitable for true secrets.
- **Integrity token detects transit tampering** (CDN compromise, MITM). Does NOT authenticate the source.
- **Guardrails detect misclassified secrets** at boot: Shannon entropy > 4.5, known formats (AKIA*, eyJ*, ghp_*, sk_live_*, sk-*, xoxb-*, -----BEGIN, etc.).
- **`--strict` mode** makes guardrail warnings into hard failures.

Full threat analysis with 7 specific threats, mitigations, and honest residual risks in `spec/SECURITY-MODEL.md`.

---

## Technical Decisions & Rationale

| Decision | Rationale |
|---|---|
| **Go for the gateway** | Static compilation (CGO_ENABLED=0), zero runtime deps, ~3MB binary, `FROM scratch` compatible. No Node.js or bash needed in prod. |
| **Zero external Go dependencies** | Minimises supply chain risk. Only uses stdlib + crypto packages. **Open question:** manifest loading (§6) requires YAML parsing. Options: (a) roll a minimal YAML subset parser in ~200 lines, (b) accept a single vendored file under Apache 2.0/MIT, (c) add `gopkg.in/yaml.v3` as a justified exception, or (d) support JSON as an alternative manifest format. The tradeoff is supply chain purity vs implementation cost. Decision needed before `--manifest` is implemented. |
| **`pkg/payload` imports from `internal/`** | This is valid Go. The `internal/` rule restricts imports from outside the parent directory tree. Since both `pkg/` and `internal/` live under `gateway/`, the import is allowed. No type extraction to `pkg/types/` is needed. |
| **`inject.go` strips `Accept-Encoding`** | The injection middleware removes `Accept-Encoding` from proxied requests so upstreams always respond with identity encoding. This avoids needing to decompress/recompress to inject the `<script>` tag. A gzip fallback via `compress/gzip` (stdlib) handles non-compliant upstreams. Brotli is unsupported (no stdlib support, zero-dep constraint) — logged and passed through uninjected. |
| **`type="application/json"` on script tag** | Browser does NOT execute it. Inert data. No CSP conflicts. |
| **`id="__rep__"` for discovery** | Stable, predictable selector. SDK finds it synchronously. |
| **Synchronous `get()`, async `getSecure()`** | Public vars must be available instantly (no loading states, no Suspense). Sensitive vars accept one network call. |
| **HMAC integrity computed over canonicalised JSON** | Deterministic (sorted keys, no whitespace). Verifiable independently. |
| **Ephemeral keys (generated at startup, never stored)** | Key compromise requires gateway process compromise. No key storage = no key theft from disk. |
| **Session keys are single-use** | Prevents replay. Rate limiting prevents brute force. |
| **Prefix-based classification** | Forces developers to make an explicit security decision per variable. No ambiguity. |
| **SPA fallback in embedded mode** | Paths without extensions serve `index.html`. Standard SPA routing support. |
| **Hot reload via SSE (not WebSocket)** | SSE is simpler, auto-reconnects, works through most proxies, sufficient for one-directional config push. |

---

## Current State & What Needs Doing

### Completed ✅

- [x] Full RFC specification (REP-RFC-0001.md) — 14 sections covering all aspects
- [x] Security model document — 7 threat analyses with mitigations and residual risks
- [x] Integration guide — React, Vue, Svelte, Angular, vanilla JS + CI/CD + K8s patterns
- [x] JSON schemas for payload and manifest
- [x] Example `.rep.yaml` manifest
- [x] Go gateway source code — all packages, compiles with zero deps
- [x] TypeScript SDK source — full API per spec
- [x] Dockerfile (multi-stage, FROM scratch)
- [x] Makefile with build/test/docker/cross-compile targets
- [x] Go unit tests for all 8 packages (config, crypto, guardrails, health, hotreload, inject, server, payload) — all pass with `-race`
- [x] TypeScript SDK tests — 24 tests via vitest + jsdom (get, getSecure, verify, meta, onChange, exports)
- [x] `gateway/VERSION` file (`0.1.0`)
- [x] `gateway/testdata/static/index.html` for `make run-example`
- [x] `go.sum` generated via `go mod tidy`
- [x] `inject.go` — `sync.RWMutex` added to `Middleware` for concurrent safety during hot reload
- [x] `inject.go` — compressed upstream handling: strips `Accept-Encoding` from proxied requests + gzip decompression fallback via stdlib
- [x] `getSecure()` error handling verified — throws `REPError` for missing payload, missing sensitive blob, and fetch failures (covered by SDK tests)

### Important Findings (Resolved)

- **`pkg/payload` importing `internal/config` is VALID.** Go's `internal/` rule allows imports from any package under the same parent directory. Both `pkg/` and `internal/` are under `gateway/`, so no refactoring needed.
- **Gateway compiles as-is with zero dependencies.** No compilation issues found — `orderedMap`, `responseRecorder`, and all types compile correctly.

### Needs Doing 🔲

#### Priority 2: Structural Issues

- [ ] **Add `.gitignore`** — standard Go + Node ignores (bin/, dist/, node_modules/, coverage.*, *.out)

#### Priority 3: Robustness & Edge Cases (items NOT yet addressed)

- [ ] **Handle chunked transfer encoding** — the recorder buffers the entire response. Consider streaming for large non-HTML responses (pass through without buffering).
- [ ] **Session key endpoint: use derived keys, not raw encryption key** — currently `session_key.go` sends the actual AES encryption key to the client. In production, this should use HKDF to derive a per-session key, or use key wrapping (AES-KW).

#### Priority 4: Developer Experience

- [ ] **CLI tool** — `@rep-protocol/cli` for:
  - `rep validate --manifest .rep.yaml` — validate manifest
  - `rep typegen --manifest .rep.yaml --output src/rep.d.ts` — TypeScript type generation
  - `rep lint --dir ./dist` — scan built bundles for leaked secrets
  - `rep dev --env .env.local --port 8080 --proxy http://localhost:5173` — dev server
- [ ] **Codemod** — `@rep-protocol/codemod` to transform `import.meta.env.VITE_X` → `rep.get('X')`
- [ ] **Framework adapters** — separate packages:
  - `@rep-protocol/react` — `useRep()`, `useRepSecure()` hooks
  - `@rep-protocol/vue` — `useRep()` composable
  - `@rep-protocol/svelte` — `repStore()` readable store

#### Priority 5: Publishing & Distribution

- [ ] **GitHub repo setup** — `github.com/ruachtech/rep` with:
  - GitHub Actions CI (Go test + lint, SDK test + build, Docker build)
  - Release workflow (GoReleaser for multi-platform binaries)
  - GHCR publish for Docker image
  - npm publish for SDK
- [ ] **GoReleaser config** — `.goreleaser.yml` for automated cross-platform binary releases
- [ ] **Docker multi-arch builds** — linux/amd64 + linux/arm64
- [ ] **npm provenance attestations** for SDK package

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

- **Vitest + jsdom.** Config in `sdk/vitest.config.ts` sets `environment: 'jsdom'` and `globals: true`.
- **`vi.resetModules()` before each test.** The SDK's `_init()` runs on module load, so each test must reset module cache and use dynamic `import('../index')` to get a fresh SDK instance.
- **DOM cleanup in `beforeEach`.** Clear `document.head` and `document.body` before each test to remove injected `<script>` elements.
- **Mock `EventSource` for hot reload tests.** Use `vi.stubGlobal('EventSource', vi.fn(() => mockES))` since jsdom doesn't provide `EventSource`.
- **Mock `fetch` for `getSecure()` tests.** Use `vi.stubGlobal('fetch', fetchMock)` to test session key fetch failures without a real server.

### Documentation

- **Every Go package has a doc comment** explaining its role and referencing the relevant RFC section.
- **Every exported function/type has a doc comment.**
- **Spec references use §N.N notation.** e.g., "Per REP-RFC-0001 §4.3" or "See §8.2 for blob format."

---

## Key Spec References (Quick Lookup)

| Topic | Location |
|---|---|
| Variable classification rules | REP-RFC-0001.md §3 |
| Secret detection guardrails | REP-RFC-0001.md §3.3 |
| Gateway startup sequence (10 steps) | REP-RFC-0001.md §4.2 |
| HTML injection rules | REP-RFC-0001.md §4.3 |
| Session key endpoint spec | REP-RFC-0001.md §4.4 |
| Health check endpoint spec | REP-RFC-0001.md §4.5 |
| Hot reload SSE spec | REP-RFC-0001.md §4.6 |
| Client SDK API | REP-RFC-0001.md §5.2 |
| SDK init behaviour (must be sync) | REP-RFC-0001.md §5.3 |
| Manifest schema | REP-RFC-0001.md §6 |
| Gateway CLI flags | REP-RFC-0001.md §7 |
| Payload JSON schema | REP-RFC-0001.md §8.1 |
| Encrypted blob format | REP-RFC-0001.md §8.2 |
| HMAC integrity computation | REP-RFC-0001.md §8.3 |
| Deployment patterns (Docker, K8s, sidecar) | REP-RFC-0001.md §9 |
| Migration path | REP-RFC-0001.md §10 |
| Conformance checklist | REP-RFC-0001.md §11 |
| Trust boundary diagram | SECURITY-MODEL.md §1.1 |
| 7 threat analyses | SECURITY-MODEL.md §2 |
| Classification decision tree | SECURITY-MODEL.md §3.1 |
| Common misclassification table | SECURITY-MODEL.md §3.2 |
| CSP recommendations | SECURITY-MODEL.md §4.2 |
| Log event catalogue | SECURITY-MODEL.md §4.3 |
| Framework integration examples | INTEGRATION-GUIDE.md §2 |
| CI/CD patterns | INTEGRATION-GUIDE.md §3 |
| Container patterns | INTEGRATION-GUIDE.md §4 |
| Testing strategies | INTEGRATION-GUIDE.md §5 |
| Migration checklist | INTEGRATION-GUIDE.md §6 |

---

## Build & Run Commands

```bash
# Gateway
cd gateway
make build                  # Build for current platform → bin/rep-gateway
make build-linux            # Cross-compile for Linux amd64
make docker                 # Build Docker image
make test                   # Run all tests
make run-example            # Run locally with example env vars
go test -race ./...         # Run all tests with race detector (recommended)
go test -race -count=1 ./...  # Same, bypassing cache

# SDK
cd sdk
npm install                 # Also installs jsdom (devDep for vitest)
npm run build               # Build CJS + ESM + types → dist/
npm test                    # Run vitest (24 tests, jsdom environment)
npm run test:watch          # Run vitest in watch mode
```

---

## Environment Variables (Application)

The gateway reads these from the container environment:

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

3. **The gateway generates NEW ephemeral keys on every restart.** This is intentional. It means a gateway restart invalidates all previously issued session keys and re-encrypts the sensitive blob. Clients that cached decrypted values will still have them (in-memory), but new `getSecure()` calls will use the new keys.

4. **The HMAC secret is never transmitted.** It exists only in the gateway's memory. The SDK cannot verify the HMAC — it can only verify the SRI hash (content matches the `data-rep-integrity` attribute). This is an honest limitation documented in the security model.

5. **Prefix stripping creates a flat namespace.** `REP_PUBLIC_API_URL` and `REP_SENSITIVE_API_URL` would both become `API_URL` in the payload — which is why the gateway MUST reject this collision at startup. This is enforced in `classify.go`.

6. **Hot reload SSE connects lazily.** The SDK does NOT establish an SSE connection on import. It only connects when `onChange()` or `onAnyChange()` is first called. This avoids unnecessary connections for apps that don't use hot reload.