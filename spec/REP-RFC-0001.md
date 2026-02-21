# REP-RFC-0001: Runtime Environment Protocol

```
Title:    Runtime Environment Protocol (REP)
Version:  0.1.0
Status:   Draft
Authors:  Olamide Adebayo (Ruach Tech)
Created:  2026-02-18
Updated:  2026-02-21
License:  CC BY 4.0
```

---

## Abstract

This document specifies the **Runtime Environment Protocol (REP)**, a standardised method for injecting environment variables into browser-hosted applications at container startup rather than at build time. REP introduces a three-tier security classification system, cryptographic integrity verification, and an optional hot-reload mechanism — addressing a gap that has persisted in the frontend ecosystem since the adoption of single-page application architectures.

REP operates at the infrastructure layer. It requires no build-tool plugins, no framework-specific adapters, and no changes to the application's build process. It is designed to be adopted incrementally with minimal friction.

---

## 1. Introduction

### 1.1 Background

Modern frontend applications are compiled into static assets (HTML, CSS, JavaScript) by build tools such as Webpack, Vite, Rollup, esbuild, and Turbopack. During this compilation, references to environment variables (e.g., `process.env.API_URL`, `import.meta.env.VITE_API_URL`) are resolved via **static string replacement** — the variable reference is literally replaced with its value in the output JavaScript.

This means the resulting bundle is **environment-specific**. A bundle built with `API_URL=https://api.staging.example.com` cannot be reused in production without rebuilding. This directly violates:

- **The Twelve-Factor App methodology** (Factor III: "Store config in the environment")
- **OCI container best practices** ("Build once, ship anywhere")
- **CI/CD promotion models** (The tested artifact should be the deployed artifact)

### 1.2 Problem Statement

The browser has no concept of environment variables. The `process` object does not exist in browser JavaScript. There is no standard mechanism for a web application to receive configuration from its hosting environment at runtime.

Existing workarounds are:

1. **Ad-hoc and non-standard** — each team builds its own injection mechanism
2. **Insecure** — all variables are treated as public, plaintext, and unclassified
3. **Fragile** — relying on shell scripts, string replacement on minified code, or network fetches
4. **Framework-coupled** — most solutions are specific to React, Vite, or a particular build tool

### 1.3 Design Goals

REP is designed to satisfy the following requirements:

| ID | Requirement |
|---|---|
| R1 | **Build-tool agnostic.** Must work with any frontend framework or bundler without build-time plugins. |
| R2 | **Security-classified.** Must distinguish between public, sensitive, and server-only variables. |
| R3 | **Integrity-verified.** The injected configuration must be verifiable by the client SDK. |
| R4 | **Synchronously accessible.** Public variables must be available immediately on page load without async operations. |
| R5 | **Zero application dependencies.** The client SDK must have zero runtime dependencies. |
| R6 | **Minimal footprint.** The gateway binary must be statically compilable and under 5MB. The client SDK must be under 2KB gzipped. |
| R7 | **Container-native.** Must integrate with Docker, Kubernetes, and OCI-compliant runtimes via standard environment variables. |
| R8 | **Incrementally adoptable.** Must not require an all-or-nothing migration. |
| R9 | **Observable.** Must emit structured logs and metrics for config injection events. |
| R10 | **Hot-reloadable (optional).** Must support live config updates without page reload. |

---

## 2. Terminology

| Term | Definition |
|---|---|
| **Gateway** | A lightweight process that reads environment variables, classifies them, and injects them into HTML responses. |
| **Payload** | The JSON object injected into the HTML document containing classified environment variables. |
| **Client SDK** | A JavaScript library that reads the injected payload and exposes it to application code. |
| **Variable Classification** | The security tier assigned to a variable based on its prefix: `PUBLIC`, `SENSITIVE`, or `SERVER`. |
| **Manifest** | An optional YAML file (`.rep.yaml`) that declares expected variables, types, defaults, and constraints. |
| **Integrity Token** | An HMAC-SHA256 signature over the payload, enabling client-side tamper detection. |
| **Session Key** | A short-lived, single-use decryption key issued by the gateway for accessing `SENSITIVE` variables. |

---

## 3. Variable Classification

### 3.1 Classification Tiers

REP classifies environment variables into three tiers based on their prefix:

| Prefix | Tier | Behaviour |
|---|---|---|
| `REP_PUBLIC_*` | **Public** | Injected as plaintext JSON. Visible in page source. Accessible synchronously via `rep.get()`. |
| `REP_SENSITIVE_*` | **Sensitive** | Injected as an encrypted blob. Decrypted client-side via a short-lived session key. Accessible via `await rep.getSecure()`. |
| `REP_SERVER_*` | **Server-only** | Never transmitted to the client under any circumstances. Available only to the gateway process for server-side logic (e.g., proxy routing, header injection). |

Variables without a `REP_` prefix are **ignored** by the gateway. This is intentional — it prevents accidental exposure of system-level variables (e.g., `PATH`, `HOME`, database credentials).

### 3.2 Classification Rules

1. Classification is determined **exclusively** by prefix. There is no override mechanism.
2. The gateway MUST NOT inject any variable that does not begin with `REP_`.
3. The gateway MUST strip the classification prefix before injection. `REP_PUBLIC_API_URL` becomes `API_URL` in the payload.
4. Variable names after prefix stripping MUST be unique across all tiers. If `REP_PUBLIC_API_URL` and `REP_SENSITIVE_API_URL` both exist, the gateway MUST refuse to start and log an error.

### 3.3 Automatic Secret Detection (Guardrails)

At startup, the gateway MUST scan all `REP_PUBLIC_*` values for patterns that indicate they may be misclassified secrets. Detection heuristics include:

| Pattern | Description |
|---|---|
| High Shannon entropy (> 4.5 bits/char) | Random-looking strings typical of API keys and tokens |
| Known key formats | AWS access keys (`AKIA...`), JWT tokens (`eyJ...`), GitHub tokens (`ghp_...`, `gho_...`), Stripe keys (`sk_live_...`, `pk_live_...`), private keys (`-----BEGIN`) |
| Length anomalies | Strings > 64 characters that appear to be encoded secrets |

When a potential misclassification is detected:

- The gateway MUST log a **WARNING** with the variable name (but NOT the value).
- If the `--strict` flag is set, the gateway MUST refuse to start.
- The gateway MUST NOT silently upgrade the classification. The developer must explicitly fix the prefix.

---

## 4. Gateway Specification

### 4.1 Architecture

The REP gateway is a standalone binary that operates in one of two modes:

**Mode A: Reverse Proxy (recommended)**

```
Client → [REP Gateway :8080] → [Nginx/Caddy/static server :80]
```

The gateway proxies all requests to the upstream static file server. For requests that return HTML documents (detected via `Content-Type: text/html`), the gateway intercepts the response and injects the REP payload before the closing `</head>` tag.

**Mode B: Embedded**

```
Client → [REP Gateway :8080] (serves static files directly)
```

The gateway serves static files directly from a configured directory, eliminating the need for a separate web server. This mode uses an embedded file server with HTTP/2, compression, and caching headers.

### 4.2 Startup Sequence

On process start, the gateway MUST perform the following steps in order:

```
1.  IF --env-file specified → READ and parse the .env file, merging
    variables into the process environment (existing env vars take precedence)
2.  READ all environment variables matching REP_* prefix
3.  CLASSIFY each variable into PUBLIC, SENSITIVE, or SERVER tier
4.  VALIDATE uniqueness of variable names after prefix stripping
5.  IF --manifest specified → LOAD the .rep.yaml manifest and validate
    all declared variables against the environment (required vars present,
    types match, patterns match). On validation failure → EXIT with error.
6.  RUN secret detection guardrails on PUBLIC tier variables
7.  IF --strict AND guardrails triggered → EXIT with error
8.  GENERATE ephemeral master key, derive AES-256 encryption key via
    HKDF-SHA256 (see §8.2), and generate HMAC-SHA256 secret
9.  ENCRYPT all SENSITIVE tier values using AES-256-GCM
10. COMPUTE HMAC-SHA256 integrity token over the PUBLIC + SENSITIVE payload
11. CONSTRUCT the REP payload JSON object
12. REGISTER HTTP handlers for:
    - HTML injection (all text/html responses)
    - Session key endpoint (/rep/session-key)
    - Health check endpoint (/rep/health)
    - Hot reload SSE endpoint (/rep/changes) [if enabled]
13. START accepting connections
14. LOG startup summary: variable counts per tier, any guardrail warnings
```

### 4.3 HTML Injection

When the gateway intercepts an HTML response, it MUST inject a `<script>` block immediately before the closing `</head>` tag. If no `</head>` tag exists, the gateway MUST inject immediately after the opening `<head>` tag. If neither exists, the gateway MUST prepend the injection to the response body.

The injected block MUST have the following structure:

```html
<script id="__rep__" type="application/json" data-rep-version="0.1.0"
        data-rep-integrity="sha256-{base64_hash}">
{
  "public": {
    "API_URL": "https://api.example.com",
    "FEATURE_FLAGS": "dark-mode,new-checkout",
    "APP_VERSION": "2.4.1"
  },
  "sensitive": "{base64_encoded_encrypted_blob}",
  "_meta": {
    "version": "0.1.0",
    "injected_at": "2026-02-18T14:30:00.000Z",
    "integrity": "hmac-sha256:{base64_signature}",
    "key_endpoint": "/rep/session-key",
    "hot_reload": "/rep/changes",
    "ttl": 0
  }
}
</script>
```

**Critical implementation notes:**

- `type="application/json"` ensures the browser does NOT execute the script block. It is inert data.
- The `data-rep-integrity` attribute contains a Subresource Integrity (SRI) compatible hash of the JSON content.
- The `id="__rep__"` attribute provides a stable, predictable selector for the client SDK.
- The gateway MUST NOT modify any other part of the HTML response.
- The gateway MUST NOT cache the injected HTML if `REP_SENSITIVE_*` variables are present (the encrypted blob may rotate).

### 4.4 Session Key Endpoint

**Endpoint:** `GET /rep/session-key`

This endpoint issues short-lived decryption keys for `SENSITIVE` tier variables.

**Request requirements:**
- MUST include an `Origin` header matching the configured allowed origins (if origins are configured). If no allowed origins are configured, same-origin requests (empty or absent `Origin` header) are permitted.

**Response:**
```json
{
  "key": "{base64_encoded_derived_aes_key}",
  "expires_at": "2026-02-18T14:30:30.000Z",
  "nonce": "{base64_encoded_nonce}"
}
```

- **`key`**: The HKDF-derived AES-256 encryption key (see §8.2), base64-encoded. This is the derived key, not the master key material.
- **`expires_at`**: RFC 3339 timestamp indicating when this key issuance expires.
- **`nonce`**: A 16-byte cryptographically random value, base64-encoded, generated fresh per request. Ensures each response is unique even within the same key's TTL window.

**Security constraints:**
- Keys MUST expire within 30 seconds of issuance.
- The gateway MUST track issued keys internally for audit and expiry purposes.
- The endpoint MUST be rate-limited to 10 requests per minute per client IP.
- The endpoint MUST NOT be cacheable (`Cache-Control: no-store, no-cache, must-revalidate`).
- CORS policy MUST restrict to configured origins only.

### 4.5 Health Check Endpoint

**Endpoint:** `GET /rep/health`

Returns gateway health status including variable counts and guardrail status.

```json
{
  "status": "healthy",
  "version": "0.1.0",
  "variables": {
    "public": 3,
    "sensitive": 1,
    "server": 2
  },
  "guardrails": {
    "warnings": 0,
    "blocked": 0
  },
  "uptime_seconds": 3421
}
```

### 4.6 Hot Reload Endpoint (Optional)

**Endpoint:** `GET /rep/changes`

Server-Sent Events (SSE) stream that pushes configuration deltas when environment variables change.

**Event format:**
```
event: rep:config:update
data: {"key": "FEATURE_FLAGS", "tier": "public", "value": "dark-mode,new-checkout,ai-assist"}
id: 1708267830000

event: rep:config:delete
data: {"key": "DEPRECATED_FLAG", "tier": "public"}
id: 1708267831000
```

The gateway detects changes via:
1. **File watch mode:** Watches a mounted ConfigMap / secrets volume for file changes.
2. **Signal mode:** Re-reads environment on receipt of `SIGHUP`.
3. **Poll mode:** Periodically re-reads a specified config source (e.g., every N seconds).

**Important:** Hot reload is an OPTIONAL feature. Implementations MAY omit it. The client SDK MUST gracefully handle the absence of the SSE endpoint.

---

## 5. Client SDK Specification

### 5.1 Overview

The REP client SDK is a zero-dependency JavaScript library that reads the injected payload and exposes it to application code. It is designed to be:

- **Synchronous for public variables** — available the instant the SDK is imported.
- **Async only for sensitive variables** — requiring a single network call.
- **Framework agnostic** — no React hooks, no Vue composables, no framework-specific bindings in the core SDK.
- **Tiny** — under 2KB gzipped.

### 5.2 Core API

```typescript
// --- Core ---

/**
 * Retrieve a PUBLIC tier variable.
 * Returns undefined if the variable does not exist.
 * Synchronous — no network call, no promise.
 */
function get(key: string): string | undefined;

/**
 * Retrieve a PUBLIC tier variable with a fallback default.
 * Synchronous.
 */
function get(key: string, defaultValue: string): string;

/**
 * Retrieve a SENSITIVE tier variable.
 * Fetches a session key from the gateway, decrypts the value, and returns it.
 * The decrypted value is cached in memory for the lifetime of the page.
 * Throws REPError if the session key endpoint is unreachable or the key has expired.
 */
function getSecure(key: string): Promise<string>;

/**
 * Retrieve all PUBLIC tier variables as a frozen object.
 * Synchronous.
 */
function getAll(): Readonly<Record<string, string>>;

/**
 * Check whether the REP payload is present and its integrity is valid.
 * Returns false if the payload is missing, malformed, or tampered with.
 * Synchronous.
 */
function verify(): boolean;


// --- Hot Reload (optional) ---

/**
 * Register a callback for when a specific variable changes.
 * Returns an unsubscribe function.
 * If hot reload is not available, logs a warning and returns a no-op unsubscribe.
 */
function onChange(key: string, callback: (newValue: string, oldValue: string | undefined) => void): () => void;

/**
 * Register a callback for any variable change.
 */
function onAnyChange(callback: (key: string, newValue: string, oldValue: string | undefined) => void): () => void;


// --- Diagnostics ---

/**
 * Returns metadata about the current REP payload.
 */
function meta(): {
  version: string;
  injectedAt: Date;
  integrityValid: boolean;
  publicCount: number;
  sensitiveAvailable: boolean;
  hotReloadAvailable: boolean;
} | null;
```

### 5.3 Initialisation Behaviour

On import, the SDK MUST:

1. Locate the `<script id="__rep__">` element in the DOM.
2. If not found, set an internal `_available` flag to `false` and return. All `get()` calls will return `undefined`.
3. Parse the JSON content.
4. Verify the `data-rep-integrity` attribute against the parsed content using the SHA-256 hash from the Web Crypto API.
5. If integrity check fails, log a console error (`[REP] Integrity check failed — payload may have been tampered with`) and set `_tampered` flag.
6. Freeze the `public` object to prevent mutation.
7. If `_meta.hot_reload` is present, establish an SSE connection (lazy — only when `onChange` is first called).

**Critical:** Steps 1–6 MUST be synchronous. The SDK MUST NOT make any network calls during initialisation. This ensures `rep.get()` is available immediately.

### 5.4 Type Generation

REP supports optional build-time type generation from the manifest file (`.rep.yaml`):

```bash
npx @rep-protocol/cli typegen --manifest .rep.yaml --output src/rep.d.ts
```

Generates:

```typescript
declare module '@rep-protocol/sdk' {
  export function get(key: 'API_URL'): string;
  export function get(key: 'FEATURE_FLAGS'): string;
  export function get(key: 'APP_VERSION'): string;
  export function get(key: string): string | undefined;
  export function getSecure(key: 'ANALYTICS_KEY'): Promise<string>;
  // ... rest of API
}
```

This is a **development-time convenience only**. It does not affect runtime behaviour.

### 5.5 Framework Adapters (Non-Normative)

The following adapters are recommended but NOT part of the core specification:

**React:**
```typescript
import { useRep, useRepSecure } from '@rep-protocol/react';

function App() {
  const apiUrl = useRep('API_URL');
  const analyticsKey = useRepSecure('ANALYTICS_KEY');

  // useRep returns string | undefined (synchronous, no suspense)
  // useRepSecure returns { value: string | null, loading: boolean, error: Error | null }
}
```

**Vue:**
```typescript
import { useRep } from '@rep-protocol/vue';

const apiUrl = useRep('API_URL');          // Ref<string | undefined>
const key = await useRepSecure('KEY');     // Ref<string | null>
```

---

## 6. Manifest File (Optional)

The `.rep.yaml` manifest file declares the expected configuration schema. It is used for:

- **Type generation** (Section 5.4)
- **Gateway validation** at startup
- **Documentation** of required and optional variables

### 6.1 Schema

```yaml
# .rep.yaml
version: "0.1.0"

variables:
  API_URL:
    tier: public
    type: url
    required: true
    description: "Base URL for the backend API"
    example: "https://api.example.com"

  FEATURE_FLAGS:
    tier: public
    type: csv       # Comma-separated values
    required: false
    default: ""
    description: "Comma-separated list of enabled feature flags"

  ANALYTICS_KEY:
    tier: sensitive
    type: string
    required: true
    description: "Analytics tracking identifier"
    pattern: "^UA-\\d+-\\d+$"

  INTERNAL_ROUTING_KEY:
    tier: server
    type: string
    required: true
    description: "Used by gateway for upstream routing decisions"

settings:
  strict_guardrails: true
  hot_reload: true
  hot_reload_mode: "file_watch"      # file_watch | signal | poll
  hot_reload_poll_interval: "30s"    # Only for poll mode
  session_key_ttl: "30s"
  session_key_max_rate: 10           # Per minute per IP
  allowed_origins:
    - "https://app.example.com"
    - "https://staging.app.example.com"
```

### 6.2 Supported Types

| Type | Validation |
|---|---|
| `string` | Any string value |
| `url` | Must be a valid URL (RFC 3986) |
| `number` | Must parse as a finite number |
| `boolean` | Must be `"true"` or `"false"` (case-insensitive) |
| `csv` | Comma-separated string (no validation of individual items) |
| `json` | Must be valid JSON |
| `enum` | Must match one of the values in the `values` array |

---

## 7. Gateway Configuration

The gateway is configured via command-line flags, environment variables, or a manifest file. Command-line flags take precedence over environment variables, which take precedence over the manifest.

### 7.1 Command-Line Flags

```
rep-gateway [flags]

Flags:
  --mode            Operating mode: "proxy" or "embedded" (default: "proxy")
  --upstream        Upstream server address (proxy mode only, default: "localhost:80")
  --port            Listen port (default: 8080)
  --static-dir      Static file directory (embedded mode only, default: "/usr/share/nginx/html")
  --manifest        Path to .rep.yaml manifest file (optional)
  --env-file        Path to .env file to load variables from (optional, env vars take precedence)
  --strict          Exit on guardrail warnings (default: false)
  --hot-reload      Enable hot reload SSE endpoint (default: false)
  --hot-reload-mode Hot reload detection mode: file_watch, signal, poll (default: signal)
  --watch-path      Path to watch for config changes (file_watch mode)
  --poll-interval   Poll interval for config changes (poll mode, default: 30s)
  --log-format      Log format: "json" or "text" (default: "json")
  --log-level       Log level: "debug", "info", "warn", "error" (default: "info")
  --allowed-origins Comma-separated list of allowed CORS origins for /rep/session-key
  --tls-cert        Path to TLS certificate (optional, for direct TLS termination)
  --tls-key         Path to TLS private key (optional)
  --health-port     Separate port for health check endpoint (optional, for K8s probes)
```

### 7.2 Environment Variable Configuration

Every flag can also be set via environment variable with the `REP_GATEWAY_` prefix:

```bash
REP_GATEWAY_MODE=proxy
REP_GATEWAY_UPSTREAM=localhost:80
REP_GATEWAY_PORT=8080
REP_GATEWAY_STRICT=true
REP_GATEWAY_HOT_RELOAD=true
REP_GATEWAY_LOG_FORMAT=json
REP_GATEWAY_ALLOWED_ORIGINS=https://app.example.com,https://staging.app.example.com
```

---

## 8. Wire Format

### 8.1 Payload JSON Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "REP Payload",
  "type": "object",
  "required": ["public", "_meta"],
  "properties": {
    "public": {
      "type": "object",
      "additionalProperties": { "type": "string" },
      "description": "Public tier variables as key-value string pairs."
    },
    "sensitive": {
      "type": "string",
      "description": "Base64-encoded AES-256-GCM encrypted blob of sensitive tier variables. Present only if SENSITIVE tier variables exist."
    },
    "_meta": {
      "type": "object",
      "required": ["version", "injected_at", "integrity"],
      "properties": {
        "version": {
          "type": "string",
          "pattern": "^\\d+\\.\\d+\\.\\d+$",
          "description": "REP protocol version."
        },
        "injected_at": {
          "type": "string",
          "format": "date-time",
          "description": "ISO 8601 timestamp of payload injection."
        },
        "integrity": {
          "type": "string",
          "pattern": "^hmac-sha256:.+$",
          "description": "HMAC-SHA256 signature of the public + sensitive fields."
        },
        "key_endpoint": {
          "type": "string",
          "description": "Relative URL for the session key endpoint. Present only if SENSITIVE tier variables exist."
        },
        "hot_reload": {
          "type": "string",
          "description": "Relative URL for the hot reload SSE endpoint. Present only if hot reload is enabled."
        },
        "ttl": {
          "type": "integer",
          "minimum": 0,
          "description": "Seconds until the payload should be considered stale. 0 means no expiry."
        }
      }
    }
  }
}
```

### 8.2 Encrypted Blob Format

The `sensitive` field contains a base64-encoded binary blob with the following structure:

```
┌──────────────┬───────────────┬────────────────────┐
│  Nonce (12B) │  Ciphertext   │  Auth Tag (16B)    │
└──────────────┴───────────────┴────────────────────┘
```

- **Algorithm:** AES-256-GCM
- **Key:** Derived via HKDF-SHA256 (RFC 5869) at gateway startup, rotated on restart. See §8.2.1.
- **Nonce:** 12-byte random, generated per encryption operation.
- **Plaintext format:** JSON object `{"KEY": "value", ...}` containing all SENSITIVE tier variables.
- **Associated data (AAD):** The `_meta.integrity` value, binding the encrypted blob to the integrity token.

#### 8.2.1 Key Derivation

The AES-256 encryption key MUST be derived using HKDF-SHA256 (RFC 5869), not used directly from the random number generator. The derivation process is:

1. **Generate master key:** 32 bytes from a cryptographic RNG. This is the HKDF Input Key Material (IKM).
2. **Generate startup salt:** 32 bytes from a cryptographic RNG. Mixed into HKDF-Extract for domain separation and per-restart uniqueness.
3. **Extract:** `PRK = HMAC-SHA256(key=startupSalt, message=masterKey)`
4. **Expand:** `EncryptionKey = HMAC-SHA256(key=PRK, message="rep-blob-encryption-v1" || 0x01)`
5. **Discard master key and salt.** Neither value is stored or returned — only the 32-byte derived key is retained in memory.

The `info` string `"rep-blob-encryption-v1"` provides domain separation, allowing future key hierarchy expansion (e.g., deriving additional keys with different `info` strings) without protocol changes.

The HMAC secret used for integrity computation (§8.3) is generated independently — it is NOT derived from the same master key.

### 8.3 Integrity Computation

The HMAC-SHA256 integrity token is computed as follows:

```
message = canonicalize(payload.public) + "|" + payload.sensitive
integrity = HMAC-SHA256(key=gateway_startup_secret, message)
```

Where `canonicalize()` produces a deterministic JSON representation (sorted keys, no whitespace).

The `gateway_startup_secret` is a 256-bit random value generated at process start. It is NOT stored or transmitted — it exists only in the gateway's memory. The integrity token allows the client SDK to verify that the payload has not been modified in transit (e.g., by a compromised CDN or browser extension), but it does NOT provide authentication (anyone who can see the payload can recompute the hash if they know the secret). The security model does NOT rely on the integrity token alone — see the [Security Model](SECURITY-MODEL.md) for the full threat analysis.

---

## 9. Deployment Patterns

### 9.1 Docker (Reverse Proxy Mode)

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY . .
RUN npm ci && npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY --from=ghcr.io/rep-protocol/gateway:latest /usr/local/bin/rep-gateway /usr/local/bin/rep-gateway
COPY nginx.conf /etc/nginx/nginx.conf

EXPOSE 8080
ENTRYPOINT ["rep-gateway", "--upstream", "127.0.0.1:80", "--port", "8080"]
```

### 9.2 Docker (Embedded Mode)

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY . .
RUN npm ci && npm run build

FROM scratch
COPY --from=build /app/dist /static
COPY --from=ghcr.io/rep-protocol/gateway:latest /usr/local/bin/rep-gateway /rep-gateway

EXPOSE 8080
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static", "--port", "8080"]
```

Note: Embedded mode produces a `FROM scratch` image — the absolute minimum attack surface.

### 9.3 Kubernetes with ConfigMap Hot Reload

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: myapp:latest
          args:
            - "--hot-reload"
            - "--hot-reload-mode=file_watch"
            - "--watch-path=/config"
          env:
            - name: REP_PUBLIC_APP_VERSION
              value: "2.4.1"
          envFrom:
            - configMapRef:
                name: frontend-config
          volumeMounts:
            - name: config-volume
              mountPath: /config
              readOnly: true
      volumes:
        - name: config-volume
          configMap:
            name: frontend-dynamic-config
```

### 9.4 Sidecar Pattern (Advanced)

For environments where modifying the application container is undesirable:

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: frontend
          image: nginx:alpine
          ports:
            - containerPort: 80

        - name: rep-gateway
          image: ghcr.io/rep-protocol/gateway:latest
          args: ["--upstream", "localhost:80", "--port", "8080"]
          ports:
            - containerPort: 8080
          envFrom:
            - configMapRef:
                name: frontend-config
```

---

## 10. Migration Path

### 10.1 Incremental Adoption

REP is designed for gradual adoption:

**Step 1: Add the gateway.** Modify your Dockerfile to include the REP gateway binary. Set environment variables with `REP_PUBLIC_` prefixes. At this stage, both `import.meta.env.VITE_API_URL` (build-time) and `rep.get('API_URL')` (runtime) will work.

**Step 2: Install the SDK.** Add `@rep-protocol/sdk` to your application. Replace `import.meta.env.VITE_X` references with `rep.get('X')` one module at a time.

**Step 3: Remove build-time variables.** Once all references are migrated, remove `VITE_*` / `REACT_APP_*` from your build process. Your build is now environment-agnostic.

**Step 4 (optional): Add manifest.** Create `.rep.yaml` for type generation and gateway validation.

### 10.2 Codemod

A codemod tool is provided to automate Step 2:

```bash
npx @rep-protocol/codemod --framework vite --src ./src
```

This transforms:
```typescript
// Before
const url = import.meta.env.VITE_API_URL;

// After
import { rep } from '@rep-protocol/sdk';
const url = rep.get('API_URL');
```

---

## 11. Conformance

An implementation is **REP-conformant** if it satisfies the following:

### 11.1 Gateway Conformance (MUST)

1. Reads only `REP_*` prefixed environment variables.
2. Classifies variables into exactly three tiers based on prefix.
3. Strips the classification prefix from variable names in the payload.
4. Rejects startup if variable names collide after prefix stripping.
5. Runs secret detection guardrails on PUBLIC tier variables.
6. Injects a `<script id="__rep__" type="application/json">` block into HTML responses.
7. Computes and includes an HMAC-SHA256 integrity token.
8. Encrypts SENSITIVE tier variables using AES-256-GCM.
9. Issues single-use, time-limited session keys.
10. Never transmits SERVER tier variables to the client.

### 11.2 Client SDK Conformance (MUST)

1. Reads from `<script id="__rep__">` synchronously on import.
2. Verifies payload integrity on initialisation.
3. Exposes `get()` as a synchronous function.
4. Exposes `getSecure()` as an async function.
5. Does not make any network calls during initialisation.
6. Freezes the public variable object to prevent mutation.

### 11.3 Optional Features (MAY)

1. Hot reload via SSE.
2. Manifest validation.
3. Type generation.
4. Framework-specific adapters.
5. Codemod tooling.

---

## 12. Security Considerations

A comprehensive security analysis is provided in [SECURITY-MODEL.md](SECURITY-MODEL.md). Key points:

1. **PUBLIC tier variables are visible in page source.** This is inherent and by design. Do not classify secrets as PUBLIC.
2. **SENSITIVE tier encryption raises the bar but is not impenetrable.** A sophisticated attacker with XSS can still call `getSecure()`. The session key mechanism adds friction, rate limiting, and auditability.
3. **SERVER tier variables never leave the gateway process.** This is the only tier suitable for true secrets.
4. **Integrity verification detects tampering but does not prevent it.** If an attacker controls the gateway, they control the payload. Integrity protects against CDN/transit modification only.
5. **CSP headers should be configured** to restrict script execution alongside REP deployment.

---

## 13. IANA Considerations

This document has no IANA actions. The `__rep__` identifier and `/rep/*` endpoint paths are conventional and do not require registration.

---

## 14. References

- [The Twelve-Factor App - Factor III: Config](https://12factor.net/config)
- [OCI Image Specification](https://github.com/opencontainers/image-spec)
- [RFC 5869 - HMAC-based Extract-and-Expand Key Derivation Function (HKDF)](https://tools.ietf.org/html/rfc5869)
- [RFC 7519 - JSON Web Token (JWT)](https://tools.ietf.org/html/rfc7519)
- [Web Crypto API - SubtleCrypto](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto)
- [Subresource Integrity (SRI)](https://developer.mozilla.org/en-US/docs/Web/Security/Subresource_Integrity)
- [Server-Sent Events Specification](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [Content Security Policy Level 3](https://www.w3.org/TR/CSP3/)

---

## Appendix A: Comparison with Existing Solutions

| Feature | `envsubst` scripts | `runtime-env-cra` | `@import-meta-env` | `react-env` | **REP** |
|---|---|---|---|---|---|
| Framework agnostic | ✅ | ❌ (CRA only) | ⚠️ (Vite/Webpack) | ❌ (React) | ✅ |
| No build tool plugin | ✅ | ❌ | ❌ | ❌ | ✅ |
| Security classification | ❌ | ❌ | ❌ | ❌ | ✅ |
| Encrypted sensitive vars | ❌ | ❌ | ❌ | ❌ | ✅ |
| Integrity verification | ❌ | ❌ | ❌ | ❌ | ✅ |
| Secret leak detection | ❌ | ❌ | ❌ | ❌ | ✅ |
| Hot reload | ❌ | ❌ | ❌ | ❌ | ✅ |
| Synchronous access | ✅ | ✅ | ✅ | ✅ | ✅ |
| No Node.js in prod image | ⚠️ (needs bash) | ❌ | ❌ | ❌ | ✅ |
| Formal specification | ❌ | ❌ | ❌ | ❌ | ✅ |
| `FROM scratch` compatible | ❌ | ❌ | ❌ | ❌ | ✅ |

---

## Appendix B: FAQ

**Q: Why not just use a service mesh / API gateway?**
A: Service meshes (Istio, Linkerd) operate at the network layer (L4/L7) and manage service-to-service communication. They have no concept of injecting configuration into HTML responses. REP operates at the application delivery layer — it is complementary, not competing.

**Q: Why not use SSR frameworks like Next.js that support runtime env vars?**
A: SSR solves this for frameworks that support it, but (a) not all applications need or want SSR, (b) it couples your solution to a specific framework, and (c) many organisations have existing SPAs they cannot migrate. REP works with any SPA regardless of framework.

**Q: Isn't the SENSITIVE tier just security through obscurity?**
A: Partially — see the [Security Model](SECURITY-MODEL.md) for an honest assessment. The SENSITIVE tier is not a vault. It raises the bar against casual exposure (View Source, browser extensions, automated scrapers) while making intentional access auditable. For true secrets, use the SERVER tier and keep them off the client entirely.

**Q: What about CDN-hosted SPAs (Cloudflare Pages, Vercel, Netlify)?**
A: REP requires a compute layer (the gateway) between the CDN and the client. For CDN-only deployments, you can run the gateway as an edge function or serverless function. A Cloudflare Workers adapter is planned.

---

*End of REP-RFC-0001*
