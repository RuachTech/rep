# REP — Runtime Environment Protocol

**Build once. Configure anywhere. Securely.**

REP is an open specification and reference implementation for injecting environment variables into browser-hosted applications at runtime — with security classification, integrity verification, and zero build-tool coupling.

---

## The Problem

Every modern frontend framework (React, Vue, Svelte, Angular, Solid, etc.) resolves environment variables at **build time** via static string replacement. The resulting bundle is plain JS/HTML/CSS — there is no `process` object, no runtime, no server. The browser has no concept of environment variables.

This creates a fundamental contradiction:

> **Containers are designed to be environment-agnostic artifacts, but frontend builds are environment-specific artifacts.**

The consequences are severe:

- **One image per environment.** You build `app:staging`, `app:prod`, `app:dev` — each with baked-in config. This defeats Docker's entire value proposition.
- **Broken CI/CD promotion.** The image that passed your tests is a different binary than what goes to prod.
- **Config changes require redeployment.** Changed an API URL? Rebuild and redeploy, even though zero application code changed.
- **No security model.** Every existing workaround treats all env vars as flat, unclassified, plaintext strings dumped into `window.__ENV__`.

## Existing Solutions and Why They Fall Short

| Approach | Limitation |
|---|---|
| `envsubst` / `sed` on JS bundles at container start | Fragile string replacement on minified code. Mutates container filesystem. |
| Fetch `/config.json` at app startup | Network dependency, loading delay, race conditions. |
| `window.__ENV__` via shell script | No standard, no security model, requires Node.js or bash in prod container. |
| Build-tool plugins (`import-meta-env`, `vite-plugin-runtime-env`) | Couples your build pipeline. Doesn't address security. Framework-specific. |

**None of these have a security model. None verify integrity. None classify variables by sensitivity. None are standardised.**

## What REP Does Differently

REP is not another env injection hack. It is a **protocol specification** with a **security-first design** that operates entirely at the infrastructure layer — no build tool plugins, no framework coupling, no application code changes beyond swapping `import.meta.env.X` for `rep.get('X')`.

### Core Differentiators

| Capability | Existing Tools | REP |
|---|---|---|
| Security classification (public / sensitive / server-only) | ❌ | ✅ |
| Encrypted sensitive variables | ❌ | ✅ |
| Framework agnostic | Partial | ✅ |
| Build tool dependency | Required | None |
| Integrity verification (HMAC + SRI) | ❌ | ✅ |
| Automatic secret leak detection | ❌ | ✅ |
| Hot config reload (SSE) | ❌ | ✅ |
| Standalone binary (no Node.js / bash) | ❌ | ✅ |
| Formal specification | ❌ | ✅ |

## Project Structure

```
rep/
├── README.md                    # This file
├── spec/
│   ├── REP-RFC-0001.md          # The protocol specification
│   ├── SECURITY-MODEL.md        # Detailed security analysis and threat model
│   └── INTEGRATION-GUIDE.md     # Framework integration patterns
├── schema/
│   ├── rep-manifest.schema.json # JSON Schema for .rep.yaml manifest
│   └── rep-payload.schema.json  # JSON Schema for injected payload
├── gateway/                     # Go implementation of REP gateway
├── sdk/                         # @rep-protocol/sdk (TypeScript)
├── cli/                         # @rep-protocol/cli (TypeScript)
├── adapters/                    # Framework adapters (React, Vue, Svelte)
├── codemod/                     # @rep-protocol/codemod (migration tool)
└── examples/
    ├── nginx-basic/             # Minimal nginx + REP gateway
    ├── caddy-basic/             # Minimal caddy + REP gateway
    └── kubernetes/              # K8s deployment with ConfigMap hot reload
```

## Development Setup

This project uses **pnpm workspaces** for managing the monorepo. All TypeScript packages (SDK, CLI, adapters, codemod) are managed as workspace packages with efficient dependency hoisting.

### Prerequisites

- **Node.js** >= 20.0.0
- **pnpm** >= 9.0.0 (install via `npm install -g pnpm`)
- **Go** >= 1.24.5 (for gateway development)

### Installation

```bash
# Clone the repository
git clone https://github.com/ruachtech/rep.git
cd rep

# Install all workspace dependencies
pnpm install

# Build all packages
pnpm run build

# Run tests across all packages
pnpm run test
```

### Workspace Commands

```bash
# Build a specific package
pnpm --filter @rep-protocol/sdk run build
pnpm --filter @rep-protocol/cli run build

# Run SDK in watch mode
pnpm run dev:sdk

# Run CLI in watch mode
pnpm run dev:cli

# Run tests for a specific package
pnpm --filter @rep-protocol/sdk run test

# Clean all build artifacts
pnpm run clean
```

### Package Dependencies

The workspace is configured so that:
- All adapters (`@rep-protocol/react`, `@rep-protocol/vue`, `@rep-protocol/svelte`) depend on `@rep-protocol/sdk` via `workspace:*` protocol
- Shared dev dependencies (`typescript`, `vitest`, `tsup`) are hoisted to the root
- Each package maintains its own production dependencies

**Note:** This project uses pnpm exclusively. `package-lock.json` and `yarn.lock` files are ignored. Always use `pnpm install` instead of `npm install`.

## Quick Start (Conceptual)

```dockerfile
# Your existing multi-stage frontend Dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY . .
RUN npm ci && npm run build

# Add REP gateway — single binary, ~3MB
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY --from=ghcr.io/ruachtech/rep/gateway:latest /usr/local/bin/rep-gateway /usr/local/bin/rep-gateway

ENTRYPOINT ["rep-gateway"]
CMD ["--upstream", "nginx", "--port", "8080"]
```

```yaml
# docker-compose.yml — same image, different config
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

```typescript
// In your application — framework agnostic
import { rep } from '@rep-protocol/sdk';

const apiUrl = rep.get('API_URL');           // Synchronous, immediate
const flags  = rep.get('FEATURE_FLAGS');     // No async, no loading state
const key    = await rep.getSecure('ANALYTICS_KEY');  // Decrypted on demand

rep.onChange('FEATURE_FLAGS', (newVal) => {   // Hot reload
  // React to config change without page reload
});
```

## Specification Status

| Document | Status | Version |
|---|---|---|
| [REP-RFC-0001](spec/REP-RFC-0001.md) | **Active** | 0.1.0 |
| [Security Model](spec/SECURITY-MODEL.md) | **Active** | 0.1.0 |
| [Integration Guide](spec/INTEGRATION-GUIDE.md) | **Active** | 0.1.0 |

## Releases & Versioning

All npm packages (`@rep-protocol/sdk`, `@rep-protocol/cli`, `@rep-protocol/codemod`, and the framework adapters) share a single version number and are released together using [release-please](https://github.com/googleapis/release-please).

- Versions are determined automatically from **conventional commits** on `main`
- `fix: ...` → patch bump, `feat: ...` → minor bump, `feat!: ...` → major bump
- On push to `main`, a Release PR is created/updated with version bumps and changelogs
- Merging the Release PR triggers npm publishing for all packages
- The Go gateway is versioned independently via `gateway/VERSION` and released through GoReleaser

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) for details on:

- Development setup and workflow
- Commit message conventions (conventional commits required)
- How the automated release process works
- Code style guidelines for Go and TypeScript

## License

This specification is released under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Reference implementations are released under the [Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).

---

**REP is a proposal by [Ruach Tech](https://github.com/ruachtech).** Built to solve a problem the entire frontend ecosystem has accepted for too long.
