# REP — Runtime Environment Protocol

**Build once. Configure anywhere. Securely.**

REP is an open protocol for injecting environment variables into browser apps **at container runtime** — not build time. It gives you security classification, encryption, integrity verification, and hot reload, with zero build-tool coupling.

[Documentation](https://rep-protocol.dev)

## Quick Start

### 1. Install the SDK

```bash
npm install @rep-protocol/sdk
```

### 2. Replace your env var calls

```diff
- const apiUrl = import.meta.env.VITE_API_URL;
+ import { rep } from '@rep-protocol/sdk';
+ const apiUrl = rep.get('API_URL');  // synchronous, no loading state
```

### 3. Add the gateway to your Dockerfile

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY . .
RUN npm ci && npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY --from=ghcr.io/ruachtech/rep/gateway:latest /usr/local/bin/rep-gateway /usr/local/bin/rep-gateway

ENTRYPOINT ["rep-gateway"]
CMD ["--upstream", "nginx", "--port", "8080"]
```

### 4. Pass config at runtime

```bash
docker run -p 8080:8080 \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,beta \
  -e REP_SENSITIVE_ANALYTICS_KEY=UA-12345-1 \
  myapp:latest
```

Same image, different config per environment. Done.

---

## Packages

All packages are published to npm under the `@rep-protocol` scope.

| Package | Install | Description |
|---|---|---|
| [`@rep-protocol/sdk`](https://www.npmjs.com/package/@rep-protocol/sdk) | `npm i @rep-protocol/sdk` | Core SDK — zero dependencies, ~1.5KB gzipped |
| [`@rep-protocol/react`](https://www.npmjs.com/package/@rep-protocol/react) | `npm i @rep-protocol/react` | React hooks — `useRep()`, `useRepSecure()` |
| [`@rep-protocol/vue`](https://www.npmjs.com/package/@rep-protocol/vue) | `npm i @rep-protocol/vue` | Vue composables — `useRep()`, `useRepSecure()` |
| [`@rep-protocol/svelte`](https://www.npmjs.com/package/@rep-protocol/svelte) | `npm i @rep-protocol/svelte` | Svelte stores — `repStore()`, `repSecureStore()` |
| [`@rep-protocol/cli`](https://www.npmjs.com/package/@rep-protocol/cli) | `npm i -D @rep-protocol/cli` | CLI — `rep dev`, `rep lint`, `rep validate`, `rep typegen` |
| [`@rep-protocol/codemod`](https://www.npmjs.com/package/@rep-protocol/codemod) | `npx @rep-protocol/codemod` | Auto-migrate from Vite, CRA, or Next.js env patterns |

---

## SDK Usage

### Core SDK (any framework)

```typescript
import { rep } from '@rep-protocol/sdk';

// PUBLIC vars — synchronous, no async, no loading state
const apiUrl = rep.get('API_URL');
const flags  = rep.get('FEATURE_FLAGS');

// SENSITIVE vars — encrypted, decrypted on demand
const key = await rep.getSecure('ANALYTICS_KEY');

// Hot reload — react to config changes without page refresh
rep.onChange('FEATURE_FLAGS', (newValue, oldValue) => {
  console.log(`Flags changed: ${oldValue} → ${newValue}`);
});
```

### React

```bash
npm install @rep-protocol/sdk @rep-protocol/react
```

```tsx
import { useRep, useRepSecure } from '@rep-protocol/react';

function App() {
  const apiUrl = useRep('API_URL');
  const flags  = useRep('FEATURE_FLAGS', 'defaults');
  const { value: analyticsKey, loading } = useRepSecure('ANALYTICS_KEY');

  // Auto re-renders on hot reload config changes
  return <div>API: {apiUrl}</div>;
}
```

### Vue

```bash
npm install @rep-protocol/sdk @rep-protocol/vue
```

```vue
<script setup>
import { useRep, useRepSecure } from '@rep-protocol/vue';

const apiUrl = useRep('API_URL');
const analyticsKey = useRepSecure('ANALYTICS_KEY');
</script>

<template>
  <div>API: {{ apiUrl }}</div>
</template>
```

### Svelte

```bash
npm install @rep-protocol/sdk @rep-protocol/svelte
```

```svelte
<script>
  import { repStore, repSecureStore } from '@rep-protocol/svelte';

  const apiUrl = repStore('API_URL');
  const analyticsKey = repSecureStore('ANALYTICS_KEY');
</script>

<div>API: {$apiUrl}</div>
```

---

## How Variables Work

REP uses a prefix convention to classify variables into security tiers:

| Prefix | Tier | Behaviour |
|---|---|---|
| `REP_PUBLIC_*` | PUBLIC | Plaintext in page source. Synchronous access via `rep.get()`. |
| `REP_SENSITIVE_*` | SENSITIVE | AES-256-GCM encrypted. Async access via `rep.getSecure()`. |
| `REP_SERVER_*` | SERVER | **Never sent to the browser.** Gateway-only. |

Prefixes are stripped in code: `REP_PUBLIC_API_URL` becomes `rep.get('API_URL')`.

---

## Migration

Migrate an existing project automatically with the codemod:

```bash
# Vite project
npx @rep-protocol/codemod --framework vite --src ./src

# Create React App
npx @rep-protocol/codemod --framework cra --src ./src

# Next.js
npx @rep-protocol/codemod --framework next --src ./src
```

This transforms `import.meta.env.VITE_*` / `process.env.REACT_APP_*` / `process.env.NEXT_PUBLIC_*` calls into `rep.get()` calls and adds the SDK import.

---

## CLI

The CLI bundles a platform-specific gateway binary and provides development tools:

```bash
npm install -D @rep-protocol/cli

# Local dev server (wraps the gateway)
npx rep dev --env-file .env

# Scan your built bundle for leaked secrets
npx rep lint ./dist

# Validate your .rep.yaml manifest
npx rep validate

# Generate TypeScript types from your manifest
npx rep typegen
```

---

## Deployment

### Docker Compose — same image, per-environment config

```yaml
services:
  frontend-staging:
    image: myapp:latest
    environment:
      REP_PUBLIC_API_URL: "https://api.staging.example.com"
      REP_PUBLIC_FEATURE_FLAGS: "dark-mode,beta-checkout"
      REP_SENSITIVE_ANALYTICS_KEY: "UA-XXXXX-staging"
      REP_SERVER_INTERNAL_SECRET: "never-reaches-browser"

  frontend-prod:
    image: myapp:latest  # SAME IMAGE
    environment:
      REP_PUBLIC_API_URL: "https://api.example.com"
      REP_PUBLIC_FEATURE_FLAGS: "dark-mode"
      REP_SENSITIVE_ANALYTICS_KEY: "UA-XXXXX-prod"
      REP_SERVER_INTERNAL_SECRET: "also-never-reaches-browser"
```

### Minimal `FROM scratch` container

The gateway is a static Go binary with zero dependencies. You can run it in a scratch container:

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY . .
RUN npm ci && npm run build

FROM scratch
COPY --from=ghcr.io/ruachtech/rep/gateway:latest /usr/local/bin/rep-gateway /rep-gateway
COPY --from=build /app/dist /static
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/rep-gateway", "--mode", "embedded", "--static-dir", "/static"]
```

### Gateway modes

- **Proxy mode** (default): Reverse proxy to an upstream (nginx, caddy). Injects config into proxied HTML responses.
- **Embedded mode**: Serves static files directly. No upstream needed. Enables `FROM scratch` containers.

---

## The Problem REP Solves

Every frontend framework resolves environment variables at **build time** via static string replacement. The resulting bundle is plain JS/HTML/CSS — there is no `process` object, no runtime. The browser has no concept of environment variables.

This means:
- **One image per environment.** You build `app:staging`, `app:prod`, `app:dev` — each with baked-in config.
- **Broken CI/CD promotion.** The image that passed your tests is a different binary than what goes to prod.
- **Config changes require redeployment.** Changed an API URL? Rebuild and redeploy.
- **No security model.** Every workaround dumps all env vars as plaintext into `window.__ENV__`.

### Why existing solutions fall short

| Approach | Limitation |
|---|---|
| `envsubst` / `sed` on JS bundles | Fragile string replacement on minified code. Mutates container filesystem. |
| Fetch `/config.json` at startup | Network dependency, loading delay, race conditions. |
| `window.__ENV__` via shell script | No standard, no security model, requires Node.js or bash in prod container. |
| Build-tool plugins | Couples your build pipeline. Framework-specific. No security. |

---

## Security Model

REP has a security-first design:

- **AES-256-GCM encryption** for sensitive variables
- **HMAC-SHA256 integrity verification** + SRI hashing on every payload
- **Automatic secret detection** — Shannon entropy analysis + known key format matching (AWS keys, JWTs, GitHub tokens, etc.)
- **Single-use session keys** with 30-second TTL and rate limiting
- **Ephemeral keys** generated at startup, never persisted to disk
- **`--strict` mode** turns secret detection warnings into hard failures

Full threat model: [spec/SECURITY-MODEL.md](spec/SECURITY-MODEL.md)

---

## Specification

REP is a formal, open specification — not just a tool.

| Document | Status |
|---|---|
| [REP-RFC-0001](spec/REP-RFC-0001.md) | Active |
| [Security Model](spec/SECURITY-MODEL.md) | Active |
| [Integration Guide](spec/INTEGRATION-GUIDE.md) | Active |

---

## Contributing

We welcome contributions. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, commit conventions, and the release process.

### Development setup

```bash
git clone https://github.com/ruachtech/rep.git
cd rep
pnpm install     # requires pnpm >= 9.0.0
pnpm -r build    # build all packages
pnpm -r test     # test all packages

# Gateway (requires Go >= 1.24.5)
cd gateway && make test
```

## License

Specification: [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Code: [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0).

---

**REP is a proposal by [Ruach Tech](https://github.com/ruachtech).** Built to solve a problem the entire frontend ecosystem has accepted for too long.
