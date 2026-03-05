# REP Examples

Runnable examples covering the main REP deployment patterns. Each example is self-contained — pick the one closest to your stack.

## Overview

| Example | Stack | Gateway mode | Container | Best for |
|---|---|---|---|---|
| [`todo-react/`](#todo-react) | React + Vite | Embedded | `FROM scratch` | Full-featured React reference app |
| [`simple-html/`](#simple-html) | Plain HTML | Embedded | `FROM scratch` | No framework, no build step |
| [`nextjs-proxy/`](#nextjs-proxy) | Next.js 15 (SSR) | Proxy | Node.js | Next.js with server-side rendering |
| [`nextjs-csr-embedded/`](#nextjs-csr-embedded) | Next.js 15 (CSR) | Embedded | `FROM scratch` | Static Next.js + Kubernetes ConfigMap |

---

## todo-react

**[`examples/todo-react/`](todo-react/)**

The primary reference example. A complete React todo application demonstrating the full REP feature set:

- `useRep()` for synchronous public variable access
- `useRepSecure()` for encrypted sensitive variables
- Hot reload — config changes re-render components without a page refresh
- `FROM scratch` Docker image (~8MB) using the REP gateway in embedded mode

Run it:
```bash
cd examples/todo-react
docker build -t rep-todo .
docker run -p 8080:8080 \
  -e REP_PUBLIC_APP_TITLE="My Todo App" \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_ENV_NAME=development \
  -e REP_PUBLIC_MAX_TODOS=10 \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_dev_abc123 \
  rep-todo
```

---

## simple-html

**[`examples/simple-html/`](simple-html/)**

A single `index.html` file — no bundler, no build step, no framework. The REP SDK is loaded directly from [esm.sh](https://esm.sh) in a `<script type="module">` block.

Key points:
- `rep.get()` is synchronous: no loading state in your markup
- `rep.getSecure()` decrypts sensitive vars in the browser via the Web Crypto API
- Integrity verification via `rep.verify()` — detects if the payload was tampered with in transit (CDN modification, MITM proxy)
- 2-stage Dockerfile: Alpine (gateway download) → `FROM scratch`

Run it:
```bash
cd examples/simple-html
docker build -t rep-simple-html .
docker run -p 8080:8080 \
  -e REP_PUBLIC_APP_TITLE="My App" \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_ENV_NAME=production \
  -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,beta \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
  rep-simple-html
```

---

## nextjs-proxy

**[`examples/nextjs-proxy/`](nextjs-proxy/)**

Next.js 15 (App Router, SSR) behind the REP gateway in **proxy mode**. The gateway intercepts every HTML response from the Next.js server and injects the `__rep__` payload before the browser receives it.

Key points:
- REP SDK access is in `'use client'` components only — server components run on the server where there is no DOM
- `docker-compose.yml` runs gateway + Next.js as two services; only the gateway is exposed externally
- Config changes only require restarting the gateway container, not the Next.js app
- Integrity verification in `EnvDisplay` — surfaces `meta().integrityValid` to confirm the payload was not modified in transit

Run it:
```bash
cd examples/nextjs-proxy
docker compose up
# open http://localhost:8080
```

---

## nextjs-csr-embedded

**[`examples/nextjs-csr-embedded/`](nextjs-csr-embedded/)**

Next.js 15 with `output: 'export'` (fully static, CSR). The REP gateway serves the static export in embedded mode — no Node.js server in production.

Key points:
- `FROM scratch` final image (~8MB: gateway binary + static assets)
- Full Kubernetes manifests in `k8s/`:
  - Standard `envFrom` ConfigMap + Secret pattern
  - Volume-mounted ConfigMap with `--hot-reload --hot-reload-mode=file_watch` — update `FEATURE_FLAGS` in the ConfigMap and browsers reflect the change within ~60 seconds, no pod restart required
- `FeatureFlags` component demonstrates `useRep()` hot-reload subscriptions
- Integrity check in `ConfigPanel` — warns if the payload has been modified in transit

Run it:
```bash
cd examples/nextjs-csr-embedded
docker build -t rep-nextjs-csr .
docker run -p 8080:8080 \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_ENV_NAME=production \
  -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,new-checkout \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
  rep-nextjs-csr
```

---

## What integrity verification detects

All examples that show `rep.verify()` / `meta().integrityValid` are checking a **SHA-256 SRI hash** of the injected payload. The gateway embeds this hash as `data-rep-integrity="sha256-<base64>"` on the `<script id="__rep__">` tag. The SDK recomputes the hash in the browser using the Web Crypto API and compares.

A failed check means the payload JSON was modified after the gateway injected it — for example, by a CDN caching a tampered response or a misconfigured reverse proxy altering the response body.

It does **not** prove the payload came from your gateway (there is no source authentication). It proves the content you received matches what was originally injected.

For the full threat model see [`spec/SECURITY-MODEL.md`](../spec/SECURITY-MODEL.md).

---

## No-manifest guide

[`examples/no-manifest.md`](no-manifest.md) — REP works with zero configuration files. The prefix convention alone is the protocol. Read this if you want to get started without creating a `.rep.yaml`.
