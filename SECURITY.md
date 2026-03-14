# Security Policy

## Supported Versions

Only the latest release of each component receives security fixes. We do not backport patches to older versions.

| Component | Package / Binary | Current Version | Supported |
|---|---|---|---|
| Gateway | `rep-gateway` (Go binary) | 0.1.x | ✅ |
| SDK | `@rep-protocol/sdk` | 0.1.x | ✅ |
| CLI | `@rep-protocol/cli` | 0.1.x | ✅ |
| React adapter | `@rep-protocol/react` | 0.1.x | ✅ |
| Vue adapter | `@rep-protocol/vue` | 0.1.x | ✅ |
| Svelte adapter | `@rep-protocol/svelte` | 0.1.x | ✅ |

> REP is currently pre-1.0. Any version older than the latest patch release in the 0.1.x line is **not** supported.

---

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Use one of these private channels:

### Option 1 — GitHub Private Vulnerability Reporting (preferred)

[Report a vulnerability via GitHub Security Advisories](https://github.com/ruachtech/rep/security/advisories/new)

GitHub keeps the report private until a fix is released. You will receive an acknowledgement within **2 business days** and a resolution update within **14 days**.

### Option 2 — Email

Send a report to **hello@ruac.tech** with the subject line `[REP] Security Vulnerability`.

Include:
- A description of the vulnerability and the affected component(s)
- Steps to reproduce or a proof-of-concept
- The potential impact and attack scenario
- Your suggested severity (Critical / High / Medium / Low)

---

## What to Expect

| Step | Timeline |
|---|---|
| Acknowledgement | Within 2 business days |
| Initial assessment and severity confirmation | Within 5 business days |
| Fix and patch release | Within 14 days for Critical/High; 30 days for Medium/Low |
| Public disclosure (GitHub Security Advisory) | After the fix is released |

We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure). We will credit reporters by name (or handle) in the advisory unless you request otherwise.

---

## Scope

### In scope

- **Gateway binary** — authentication bypass, session key leakage, guardrail bypass, SSRF via proxy mode, path traversal in embedded mode, denial-of-service via malformed requests
- **SDK** — integrity verification bypass, sensitive variable exposure, XSS via injected payload
- **CLI** — secret leakage through `rep lint` false negatives, arbitrary code execution during `rep typegen` or `rep validate`
- **Payload wire format** — HMAC bypass, SRI bypass, encrypted blob weaknesses

### Out of scope

- Vulnerabilities that require physical access to the host machine
- Vulnerabilities in upstream dependencies outside our control (report those to the upstream project directly)
- Issues in example applications that don't reflect a defect in REP itself
- Theoretical attacks without a demonstrated exploit path
- Security trade-offs that are explicitly documented in [`spec/SECURITY-MODEL.md`](spec/SECURITY-MODEL.md) (e.g. PUBLIC vars being visible in page source — this is by design)

---

## Security Design

REP's threat model, trust boundaries, and known residual risks are documented in detail in [`spec/SECURITY-MODEL.md`](spec/SECURITY-MODEL.md). We recommend reading it before reporting an issue — many apparent weaknesses are intentional design trade-offs that are explicitly acknowledged there.

Key properties:
- **PUBLIC variables are visible in HTML source.** This is by design. Do not put secrets in `REP_PUBLIC_*`.
- **SENSITIVE variables are AES-256-GCM encrypted** in the payload and require a single-use, rate-limited session key to decrypt.
- **SERVER variables never leave the gateway process.** They are the only tier suitable for true server-side secrets.
- **Guardrails detect misclassified secrets** at boot using Shannon entropy and known secret format matching.
- **Ephemeral keys** are generated at startup and never written to disk. A gateway restart invalidates all previously issued session keys.
