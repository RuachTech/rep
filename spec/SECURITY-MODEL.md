# REP Security Model

```
Document:  REP Security Model
Version:   0.1.0
Status:    Active
Authors:   Olamide Olayinka (Ruach Tech)
Created:   2026-02-18
```

---

## 1. Threat Model Overview

This document provides an honest, rigorous security analysis of the Runtime Environment Protocol. We explicitly acknowledge what REP protects against, what it does not, and where residual risks remain. **REP does not claim to make browser-side configuration "secure" in an absolute sense.** It claims to make it *significantly more secure than the status quo* while making security trade-offs explicit and auditable.

### 1.1 Trust Boundaries

```
┌─────────────────────────────────────────────────────────────────┐
│                    TRUSTED ZONE                                  │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │  Orchestrator │    │ REP Gateway  │    │ Static File      │   │
│  │  (K8s, ECS)  │───▶│  (Go binary) │───▶│ Server (nginx)   │   │
│  └──────────────┘    └──────┬───────┘    └──────────────────┘   │
│                             │                                    │
│         REP_SERVER_* vars   │  REP_PUBLIC_* + REP_SENSITIVE_*   │
│         NEVER cross this    │  vars are injected here            │
│         boundary            │                                    │
├─────────────────────────────┼────────────────────────────────────┤
│                    TRANSIT ZONE                                   │
│                             │                                    │
│                        HTTPS/TLS                                 │
│                             │                                    │
├─────────────────────────────┼────────────────────────────────────┤
│                    UNTRUSTED ZONE                                 │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Browser                                │   │
│  │                                                           │   │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │   │
│  │  │ Application │  │ REP SDK      │  │ Browser        │  │   │
│  │  │ Code        │  │ (@rep/sdk)   │  │ Extensions     │  │   │
│  │  └─────────────┘  └──────────────┘  └────────────────┘  │   │
│  │                                                           │   │
│  │  ┌─────────────────────────────────────────────────────┐  │   │
│  │  │ DevTools — EVERYTHING here is visible to the user   │  │   │
│  │  └─────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 Fundamental Axiom

> **Anything that reaches the browser is ultimately accessible to the user and to any code running in the browser context.**

REP does not and cannot change this axiom. What REP does is:

1. **Prevent accidental exposure** through classification and guardrails.
2. **Raise the bar for casual extraction** through encryption of sensitive values.
3. **Make intentional access auditable** through session key logging and rate limiting.
4. **Eliminate unnecessary exposure** through the SERVER tier.
5. **Detect tampering** through integrity verification.

---

## 2. Threat Analysis

### 2.1 THREAT: Accidental Secret Exposure (Severity: HIGH)

**Description:** A developer prefixes a database password or API secret key with `REP_PUBLIC_` instead of `REP_SENSITIVE_` or `REP_SERVER_`, causing it to appear in plaintext in the page source.

**Likelihood without REP:** Very high. With `window.__ENV__` or `process.env`, there is no classification at all. Every variable is public. Developers routinely commit `.env` files and expose secrets in frontend bundles — this is one of the most common security incidents in web development.

**REP Mitigations:**

| Control | Description |
|---|---|
| Naming convention | The `REP_PUBLIC_` / `REP_SENSITIVE_` / `REP_SERVER_` prefixes force developers to make an explicit classification decision for every variable. |
| Guardrail scanning | At startup, the gateway scans `REP_PUBLIC_*` values for secret-like patterns (high entropy, known key formats). |
| `--strict` mode | In strict mode, guardrail warnings become hard failures — the gateway refuses to start. Recommended for production. |
| Manifest validation | If a `.rep.yaml` manifest is present, the gateway validates that each variable matches its declared tier. |

**Residual Risk:** The guardrails use heuristics, not certainty. A low-entropy secret (e.g., a short password) may not trigger detection. **Mitigation:** Use `--strict` in production and review `.rep.yaml` manifests in code review.

---

### 2.2 THREAT: XSS Exfiltration of Public Variables (Severity: MEDIUM)

**Description:** An attacker achieves Cross-Site Scripting (XSS) in the application and reads public configuration values (e.g., API URLs, feature flags).

**Likelihood:** Depends on the application's XSS posture. If XSS exists, this is trivial.

**REP Mitigations:**

This is largely **out of scope** for REP. PUBLIC tier variables are, by definition, visible to any code running in the browser context — including XSS payloads. This is the same as any other global variable, DOM element, or network response.

However, REP helps indirectly:

| Control | Description |
|---|---|
| Classification forces thought | By requiring developers to classify variables, REP forces them to consider whether each value *should* be client-visible at all. |
| SERVER tier | Variables that should never reach the client can be moved to `REP_SERVER_*`, removing them from the attack surface entirely. |
| Minimal surface | REP injects a single, predictable `<script>` block rather than scattering globals across `window`, `localStorage`, cookies, etc. This makes CSP policy authoring easier. |

**Residual Risk:** XSS can read anything the application can read. REP does not change this. **Mitigation:** Standard XSS prevention (CSP, input sanitisation, framework auto-escaping).

---

### 2.3 THREAT: XSS Exfiltration of Sensitive Variables (Severity: MEDIUM-HIGH)

**Description:** An attacker with XSS calls `rep.getSecure('KEY')` to decrypt and exfiltrate sensitive values.

**Likelihood:** Requires active XSS exploitation. The attacker must understand the REP SDK API.

**REP Mitigations:**

| Control | Description |
|---|---|
| Encrypted at rest | Sensitive values are AES-256-GCM encrypted in the HTML. A passive observer (View Source, DOM scraper, browser extension scanning the page) cannot read them. |
| Session key required | Decryption requires fetching a key from `/rep/session-key`. This is an observable network request. |
| Rate limiting | The session key endpoint is rate-limited (default: 10 requests/minute/IP). An automated exfiltration script will be throttled. |
| Single-use keys | Each session key can only be used once. The gateway tracks issued keys. |
| Key TTL | Session keys expire after 30 seconds. A stored key cannot be used later. |
| Origin validation | The session key endpoint validates `Origin` and `Referer` headers. Cross-origin XSS payloads loaded from attacker domains will fail. |
| Audit logging | Every session key issuance is logged with timestamp, client IP, and origin. Anomalous patterns are detectable. |

**Residual Risk:** A sophisticated, same-origin XSS attack can call `getSecure()` and exfiltrate the result within the rate limit. **REP's encryption does not prevent this.** It raises the bar from "view page source" to "achieve XSS, understand the SDK, make a network call within rate limits, exfiltrate before key expires."

**Honest Assessment:** For truly secret values, use `REP_SERVER_*` and keep them off the client. The SENSITIVE tier is designed for values that are **not public** but **not catastrophic if exposed** — analytics keys, client-side OAuth identifiers, non-critical API tokens.

---

### 2.4 THREAT: Man-in-the-Middle Payload Tampering (Severity: HIGH)

**Description:** An attacker intercepts the HTML response in transit and modifies the REP payload — e.g., changing `API_URL` to point to a phishing server.

**Likelihood:** Low if HTTPS is properly configured. Higher in corporate proxy environments, compromised CDNs, or misconfigured TLS.

**REP Mitigations:**

| Control | Description |
|---|---|
| HMAC integrity | The payload includes an HMAC-SHA256 signature. The SDK verifies this on load and logs a console error if verification fails. |
| SRI attribute | The `data-rep-integrity` attribute contains a SHA-256 hash of the JSON content, compatible with Subresource Integrity verification. |
| SDK tamper flag | If integrity verification fails, the SDK sets an internal `_tampered` flag. Applications can check `rep.verify()` and refuse to start. |

**Limitation:** The HMAC secret is known only to the gateway. The SDK cannot independently verify the HMAC — it can only verify the SRI hash (that the JSON content matches the hash in the attribute). If an attacker controls both the JSON content AND the `data-rep-integrity` attribute, they can forge a valid-looking payload.

**Residual Risk:** Integrity verification protects against partial tampering (modifying values without updating the hash). It does NOT protect against an attacker who rewrites the entire `<script>` block. **Mitigation:** HTTPS, CSP `script-src` directives, and CDN integrity features.

---

### 2.5 THREAT: Environment Variable Leakage via Gateway (Severity: CRITICAL)

**Description:** A vulnerability in the REP gateway binary itself leaks environment variables from the SERVER tier or from the host system.

**Likelihood:** Low (the gateway is intentionally minimal), but the impact would be severe.

**REP Mitigations:**

| Control | Description |
|---|---|
| Prefix filtering | The gateway only reads `REP_*` variables. `PATH`, `HOME`, `DATABASE_URL`, etc. are never accessed. |
| No shell execution | The gateway is a compiled Go binary. It does not invoke shell commands, scripts, or subprocesses. |
| No filesystem writes | The gateway does not write to the filesystem (except optional logs). |
| Minimal dependencies | The reference implementation uses only the Go standard library plus `crypto/aes` and `crypto/hmac`. No third-party dependencies. |
| Read-only container | The gateway is compatible with read-only root filesystem Docker containers. |
| Static binary | Compiled as a fully static binary (`CGO_ENABLED=0`). No shared library loading. |
| `FROM scratch` compatible | Can run in a `FROM scratch` container with zero OS, zero shell, zero utilities. |

**Residual Risk:** A bug in the Go standard library's HTTP server or crypto packages. **Mitigation:** Automated dependency scanning, minimal surface area, frequent rebuilds from upstream Go releases.

---

### 2.6 THREAT: Denial of Service via Session Key Endpoint (Severity: LOW)

**Description:** An attacker floods the `/rep/session-key` endpoint to exhaust gateway resources.

**REP Mitigations:**

| Control | Description |
|---|---|
| Rate limiting | 10 requests/minute/IP by default (configurable). |
| Minimal computation | Key issuance involves generating a random key and storing it in an in-memory map. No database, no I/O. |
| Key expiry cleanup | Expired keys are garbage-collected periodically. Memory usage is bounded. |
| Separate health port | The health check can run on a separate port, unaffected by session key traffic. |

---

### 2.7 THREAT: Supply Chain Attack on SDK (Severity: HIGH)

**Description:** An attacker compromises the `@rep-protocol/sdk` npm package and publishes a malicious version that exfiltrates configuration.

**REP Mitigations:**

| Control | Description |
|---|---|
| Minimal surface | The SDK is under 2KB. It is trivially auditable. |
| Zero dependencies | The SDK has no `node_modules`. No transitive supply chain risk. |
| Lockfile pinning | Standard npm/yarn lockfile practices apply. |
| Provenance | Published with npm provenance attestations. |
| Self-hostable | The SDK can be vendored (copied into your codebase) instead of installed from npm. |

**Residual Risk:** Same as any npm package. **Mitigation:** Vendor the SDK, audit it (it's small enough to read in 5 minutes), pin versions.

---

## 3. Security Classification Guidance

### 3.1 Decision Tree

```
Is this value a secret that should NEVER reach a browser?
├─ YES → REP_SERVER_*
│        Examples: database credentials, internal service tokens,
│        encryption master keys, admin API keys
│
└─ NO → Would exposure via "View Source" cause harm?
         ├─ YES → REP_SENSITIVE_*
         │        Examples: analytics tracking IDs, client-side OAuth client IDs,
         │        non-critical API keys with low-privilege scopes,
         │        third-party service identifiers
         │
         └─ NO → REP_PUBLIC_*
                  Examples: API base URLs, feature flags, app version,
                  environment name ("production", "staging"),
                  publicly known configuration
```

### 3.2 Common Misclassifications

| Variable | Wrong Classification | Correct Classification | Why |
|---|---|---|---|
| Database connection string | `REP_PUBLIC_` | `REP_SERVER_` | Never needed client-side |
| JWT signing secret | `REP_SENSITIVE_` | `REP_SERVER_` | Must never reach browser |
| Stripe publishable key | `REP_SENSITIVE_` | `REP_PUBLIC_` | It's designed to be public (that's what "publishable" means) |
| OAuth client secret | `REP_PUBLIC_` | `REP_SERVER_` | Client secrets should use backend-for-frontend pattern |
| OAuth client ID | `REP_SENSITIVE_` | `REP_PUBLIC_` | Client IDs are public by design in OAuth |
| API URL | `REP_SENSITIVE_` | `REP_PUBLIC_` | URLs are visible in network tab regardless |
| Analytics write key | `REP_PUBLIC_` | `REP_SENSITIVE_` | Abuse could pollute analytics data |
| Feature flag list | `REP_SENSITIVE_` | `REP_PUBLIC_` | Feature flags are visible via UI behaviour anyway |
| Internal microservice URL | `REP_PUBLIC_` | `REP_SERVER_` | Reveals internal architecture |

---

## 4. Hardening Recommendations

### 4.1 Gateway Hardening

```yaml
# Recommended production configuration
REP_GATEWAY_STRICT: "true"                    # Fail on guardrail warnings
REP_GATEWAY_ALLOWED_ORIGINS: "https://app.example.com"  # Strict CORS
REP_GATEWAY_LOG_LEVEL: "info"                 # Log all session key issuance
REP_GATEWAY_LOG_FORMAT: "json"                # Structured for SIEM ingestion
```

```dockerfile
# Minimal container
FROM scratch
COPY --from=build /static /static
COPY rep-gateway /rep-gateway
USER 65534:65534                               # nonroot
EXPOSE 8080
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--strict"]
```

### 4.2 CSP Configuration

REP's injected `<script type="application/json">` block is NOT executed by the browser and does NOT conflict with CSP `script-src` directives. However, the REP SDK itself must be allowed to execute.

Recommended CSP:

```
Content-Security-Policy:
  default-src 'self';
  script-src 'self';
  connect-src 'self';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data:;
  font-src 'self';
  object-src 'none';
  base-uri 'self';
  form-action 'self';
  frame-ancestors 'none';
```

Note: `connect-src 'self'` is required for the session key endpoint (`/rep/session-key`) and hot reload SSE (`/rep/changes`).

### 4.3 Monitoring and Alerting

The gateway emits structured log events for security-relevant operations:

| Event | Level | Fields | Alert Recommendation |
|---|---|---|---|
| `rep.guardrail.warning` | WARN | `variable_name`, `detection_type` | Alert immediately in production |
| `rep.session_key.issued` | INFO | `client_ip`, `origin`, `key_id` | Monitor for anomalous volume |
| `rep.session_key.rejected` | WARN | `client_ip`, `reason` (expired, reuse, rate_limit, origin_mismatch) | Alert on sustained rejections |
| `rep.session_key.rate_limited` | WARN | `client_ip`, `requests_in_window` | Alert on repeated rate limiting |
| `rep.integrity.payload_generated` | INFO | `public_count`, `sensitive_count`, `hash` | Audit trail |
| `rep.config.changed` | INFO | `key`, `tier`, `action` (update, delete) | Hot reload audit trail |

---

## 5. Comparison with Alternative Security Approaches

### 5.1 Backend-for-Frontend (BFF) Pattern

The BFF pattern routes all API calls through a backend proxy, keeping secrets server-side. REP's `SERVER` tier achieves a similar goal for configuration — variables that should never reach the client stay in the gateway process.

REP is **complementary** to BFF, not a replacement. Use BFF for API authentication. Use REP for configuration injection.

### 5.2 Vault / Secret Manager Integration

Tools like HashiCorp Vault, AWS Secrets Manager, and GCP Secret Manager store and rotate secrets securely. REP's gateway can source its environment variables from these tools (via Kubernetes secrets, init containers, or sidecar injectors). REP does not replace secret managers — it consumes their output and classifies it for frontend delivery.

### 5.3 Feature Flag Services (LaunchDarkly, Split, Flagsmith)

Feature flag services provide sophisticated targeting, gradual rollouts, and A/B testing. REP's `REP_PUBLIC_FEATURE_FLAGS` is a simpler mechanism for basic feature flags. For complex use cases, use a dedicated service. For simple on/off flags that vary by environment, REP may be sufficient.

---

## 6. Known Limitations

1. **The browser is an untrusted environment.** REP cannot prevent a determined, technically sophisticated user from reading any value that reaches their browser. It can only make this harder and more auditable.

2. **SENSITIVE tier is not a vault.** It protects against passive observation, not active exploitation. Do not use it for values where exposure would be catastrophic.

3. **Integrity verification is one-directional.** The SDK can detect that the payload doesn't match its hash, but it cannot verify that the payload came from a legitimate gateway (no mutual authentication).

4. **Session keys add latency.** The first `getSecure()` call incurs a network round-trip. Subsequent calls use the cached decrypted values.

5. **Hot reload has eventual consistency.** There is a window between when the environment variable changes and when all connected clients receive the update.

---

*End of REP Security Model*
