# TASKS.md — REP (Runtime Environment Protocol)

This file tracks all known implementation gaps between the current codebase and the REP-RFC-0001 specification. Tasks are ordered by priority; engineers should be able to pick up any item cold using only this file, CLAUDE.md, and the spec.

---

## P1 — Correctness Blockers (Nothing Ships Without These)

These gaps mean the current implementation is either broken at runtime, non-conformant with the spec, or structurally unsound.

- [x] **Write Go unit tests for all gateway packages** (§11.1 conformance checklist; CLAUDE.md P1)
  - `internal/config/classify_test.go` — prefix stripping, tier assignment, collision detection (two vars that collide after stripping must cause startup failure per §3.2 rule 4)
  - `internal/crypto/crypto_test.go` — AES-256-GCM encrypt/decrypt roundtrip, HMAC-SHA256 computation, SRI hash, AAD binding (§8.2, §8.3)
  - `internal/guardrails/guardrails_test.go` — entropy threshold (> 4.5 bits/char), each known format pattern (AKIA, eyJ, ghp_, sk_live_, -----BEGIN, etc.) (§3.3)
  - `internal/inject/inject_test.go` — injection before `</head>`, injection after `<head>` when no `</head>` exists, prepend fallback when neither tag exists (§4.3)
  - `internal/health/health_test.go` — response JSON shape matches §4.5 schema exactly
  - `pkg/payload/payload_test.go` — payload construction, `<script>` tag rendering, `data-rep-integrity` attribute presence
  - `internal/server/server_test.go` — integration test via `net/http/httptest` covering proxy and embedded modes
  - Run `go test ./...` to gate the build

- [x] **Write TypeScript SDK tests** (§11.2 conformance checklist; CLAUDE.md P1)
  - `sdk/src/__tests__/index.test.ts` using vitest + JSDOM
  - Cover: payload discovery, `get()` returns correct value synchronously, `get()` with missing key returns `undefined`, `get()` with default returns default, `verify()` returns false on tampered `data-rep-integrity`, `meta()` shape, `getSecure()` throws on missing sensitive blob
  - Confirm no network call is made during module import (§5.3 critical requirement)

- [x] **Add `gateway/VERSION` file** (CLAUDE.md P2; Makefile `cat VERSION` will fail without it)
  - Create `gateway/VERSION` containing `0.1.0`
  - The Dockerfile and Makefile both reference this file; without it `make build` and `make docker` produce broken outputs

- [x] **Create `gateway/testdata/static/index.html`** (CLAUDE.md P2; `make run-example` is broken without it)
  - Minimal valid HTML file with `<head>` and `<body>` tags so `make run-example` can exercise the injection path end-to-end
  - Must include `<script type="module">` importing `@rep-protocol/sdk` to demonstrate the full flow

- [x] **Add mutex to `inject.go` `UpdateScriptTag()`** (CLAUDE.md P3; §4.6 hot reload correctness)
  - `UpdateScriptTag()` mutates `m.scriptTag` with no synchronisation
  - Hot reload (SIGHUP) runs on the signal goroutine; HTTP handlers run on separate goroutines — this is a data race
  - Add `sync.RWMutex`: write-lock in `UpdateScriptTag()`, read-lock in the injection middleware

- [x] **Handle gzip/brotli compressed upstream responses in `inject.go`** (CLAUDE.md P3; §4.3 injection correctness)
  - The `responseRecorder` captures raw bytes; if the upstream returns `Content-Encoding: gzip` or `br`, the byte slice is compressed and string-searching for `</head>` will fail silently, producing a corrupted response
  - Fix: strip `Accept-Encoding` from proxied requests so the upstream always responds with identity encoding, OR decompress before injection and re-compress after
  - Stripping `Accept-Encoding` is simpler and has no correctness risk; compression can be re-applied by the gateway itself

---

## P2 — Structural Issues (Required Before First Real Deployment)

These do not break the build today but will cause problems in integration, CI, or contributor onboarding.

- [x] **Implement manifest loading in the gateway** (§6; CLAUDE.md P2 — currently a no-op)
  - `--manifest` flag is parsed but the file is never read or validated
  - The spec requires gateway validation at startup against the manifest (§6, §4.2 step 3 extended)
  - Constraint: zero external Go dependencies — see CLAUDE.md "Technical Decisions" for the four options under consideration (minimal hand-rolled parser, single vendored file, `gopkg.in/yaml.v3` exception, or JSON manifest). Decision must be made before implementation starts.
  - Validation must cover: required variables present in env, pattern constraints (§6.2), type validation (url, number, boolean, csv, json, enum), and `strict_guardrails` setting
  - Log a structured error and exit non-zero on validation failure

- [x] **Implement `file_watch` and `poll` hot reload modes** (§4.6; CLAUDE.md P3 — only `signal` is wired)
  - `file_watch` mode: use `os.ReadDir` polling or `fsnotify`-equivalent via syscall (`inotify` on Linux, `kqueue` on Darwin) — no external deps; alternatively, a tight `time.Ticker` loop stat-checking the `--watch-path` file's mtime is acceptable for v0.1
  - `poll` mode: `time.Ticker` loop re-reading env vars (or re-reading a flat file at `--watch-path`) every `--poll-interval`
  - On change detection, both modes must trigger the same re-classification → re-encryption → SSE broadcast path that SIGHUP currently uses
  - The `hot_reload_mode: file_watch` setting in `.rep.yaml` (§6.1) must be respected when the manifest is loaded

---

## P3 — Security Hardening (Required Before Any Production Use)

These items are called out explicitly in the spec or security model as requirements, not suggestions.

- [ ] **Replace raw AES key in `/rep/session-key` response with a derived key** (§4.4; CLAUDE.md P3)
  - Current: `session_key.go` sends the actual 256-bit AES encryption key to the browser
  - Spec §4.4 implies the session key is used only for decryption of the sensitive blob on the client; sending the raw encryption key means a key interception gives permanent access to any future blobs until gateway restart
  - Fix: use HKDF (RFC 5869) to derive a per-session, single-use decryption key from the master encryption key + a random per-session salt; include the salt in the response; the client uses the derived key to decrypt
  - The `nonce` field already in the response schema (§4.4) is the appropriate place to carry the per-session salt
  - This requires updating the SDK's `getSecure()` to perform the HKDF derivation step using Web Crypto `deriveBits` before calling `decrypt`

- [ ] **Verify SRI integrity check in SDK uses Web Crypto correctly** (§5.3 step 4; §11.2 item 2)
  - The SDK reads `data-rep-integrity="sha256-{base64}"` and must verify it against the raw JSON text content of the script tag using `SubtleCrypto.digest('SHA-256', ...)`
  - Confirm the hash is computed over the exact bytes of the JSON string as it appears in the DOM (no re-serialisation), matching how the gateway computes it
  - If this diverges, `verify()` will always return false or always return true incorrectly

- [ ] **Ensure `REP_GATEWAY_*` vars are excluded from all payload tiers** (§3.1; §3.2 rule 2)
  - Gateway config vars share the `REP_` prefix but must never appear in PUBLIC, SENSITIVE, or SERVER classification output
  - Add an explicit test case in `classify_test.go` confirming `REP_GATEWAY_PORT`, `REP_GATEWAY_MODE`, etc. produce zero classified variables

---

## P4 — Developer Experience (Required for Ecosystem Adoption)

- [ ] **Build the `@rep-protocol/cli` package** (§5.4; CLAUDE.md P4 — `cli/` directory is empty)
  - Required subcommands per spec and CLAUDE.md:
    - `rep validate --manifest .rep.yaml` — parse and validate the manifest against `schema/rep-manifest.schema.json` (§6)
    - `rep typegen --manifest .rep.yaml --output src/rep.d.ts` — generate TypeScript overloads for `get()` and `getSecure()` keyed to declared variable names (§5.4)
    - `rep lint --dir ./dist` — scan built JS bundles for strings matching guardrail patterns (entropy, known formats) to catch secrets accidentally baked into the build (§3.3)
    - `rep dev --env .env.local --port 8080 --proxy http://localhost:5173` — local dev server wrapping the gateway binary for DX parity with `vite dev`
  - Package name: `@rep-protocol/cli`; ship as a Node.js CLI with a `bin` entry in `package.json`; no runtime deps outside Node stdlib

- [ ] **Implement `@rep-protocol/codemod`** (§10.2; CLAUDE.md P4)
  - Transform `import.meta.env.VITE_X` → `rep.get('X')` and `process.env.REACT_APP_X` → `rep.get('X')`
  - Use jscodeshift or ts-morph; output must preserve formatting and add the SDK import if not already present
  - Support `--framework vite`, `--framework cra`, `--framework next` flags
  - Must be idempotent (running twice produces the same result as running once)

- [ ] **Build `@rep-protocol/react` adapter** (§5.5 non-normative; CLAUDE.md P4)
  - `useRep(key: string): string | undefined` — synchronous, no Suspense, no loading state
  - `useRepSecure(key: string): { value: string | null; loading: boolean; error: Error | null }` — async, uses `useEffect` + `useState`
  - Both hooks must subscribe to `onChange` and re-render on hot reload updates
  - Zero runtime deps beyond React peer dep

- [ ] **Build `@rep-protocol/vue` adapter** (§5.5 non-normative; CLAUDE.md P4)
  - `useRep(key: string): Ref<string | undefined>` — reactive ref, synchronous initial value
  - `useRepSecure(key: string): Ref<string | null>` — async, resolves after session key fetch
  - Must clean up `onChange` subscription in `onUnmounted`

- [ ] **Build `@rep-protocol/svelte` adapter** (§5.5 non-normative; CLAUDE.md P4)
  - `repStore(key: string): Readable<string | undefined>` — Svelte readable store wrapping `get()` and `onChange()`
  - `repSecureStore(key: string): Readable<string | null>` — async store wrapping `getSecure()`

---

## P5 — Publishing and Distribution (Required for Public Release)

- [ ] **Set up GitHub Actions CI** (CLAUDE.md P5)
  - Go workflow: `go vet ./...`, `go test ./...`, `go build ./...` on push and PR; target Go 1.22
  - SDK workflow: `npm ci`, `npm run build`, `npm test` on push and PR; target Node 20 LTS
  - Docker workflow: `docker build` smoke test on push to `main`
  - All three workflows must pass before merge is allowed (branch protection rule)

- [ ] **Add GoReleaser config** (CLAUDE.md P5)
  - `.goreleaser.yml` at repo root
  - Cross-compile targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
  - Archive format: `.tar.gz` with binary + LICENSE + README
  - Checksum file: `checksums.txt` (SHA-256)
  - GitHub release trigger: tag matching `gateway/v*`

- [ ] **Add Docker multi-arch build and GHCR publish** (CLAUDE.md P5; §9.1, §9.2 deployment patterns reference `ghcr.io/rep-protocol/gateway`)
  - Use `docker buildx` with `--platform linux/amd64,linux/arm64`
  - Push to `ghcr.io/ruach-tech/rep/gateway:{version}` and `:latest` on release tag
  - The Dockerfile must produce a working image from the current source before this task is started

- [ ] **Add npm provenance attestations for SDK publish** (CLAUDE.md P5)
  - Use `npm publish --provenance` in the CI release workflow
  - Requires the workflow to have `id-token: write` permission for OIDC signing
  - Set `"provenance": true` in `package.json` under `publishConfig`

- [ ] **Tag and publish `@rep-protocol/sdk` v0.1.0 to npm** (CLAUDE.md P5)
  - Prerequisite: SDK tests pass, build output is verified, provenance workflow is in place
  - Publish as public scoped package under the `@rep-protocol` organisation
  - Include `dist/`, `src/`, `README.md`, `LICENSE` in the published package; exclude `node_modules/`, `*.test.ts`
