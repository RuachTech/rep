# Using REP Without a Manifest

The `.rep.yaml` manifest is entirely optional. The gateway works with nothing
but environment variables — the naming convention **is** the configuration.

This guide shows the minimum viable setup: no manifest file, no CLI tooling,
no configuration step. Just rename your vars, add the SDK, and run the gateway.

---

## How it works

The gateway classifies every `REP_*` variable it finds in the environment by
prefix. Nothing to declare, nothing to register:

| Env var prefix | Tier | In the browser |
|---|---|---|
| `REP_PUBLIC_*` | PUBLIC | Plaintext in `<script id="__rep__">`. Read with `rep.get()`. |
| `REP_SENSITIVE_*` | SENSITIVE | AES-256-GCM encrypted. Decrypt with `await rep.getSecure()`. |
| `REP_SERVER_*` | SERVER | Never leaves the gateway process. |
| `REP_GATEWAY_*` | Config | Gateway settings. Not injected into the app. |

That's the entire protocol. No file needed to make it work.

---

## Step 1 — Rename your env vars

If you're migrating from Vite or Create React App, rename your existing vars:

```bash
# Before (Vite)            → After (REP)
VITE_API_URL               → REP_PUBLIC_API_URL
VITE_FEATURE_FLAGS         → REP_PUBLIC_FEATURE_FLAGS

# Before (CRA)             → After (REP)
REACT_APP_API_URL          → REP_PUBLIC_API_URL

# Anything that should stay encrypted
REACT_APP_ANALYTICS_KEY    → REP_SENSITIVE_ANALYTICS_KEY

# Anything that should never reach the browser
DB_PASSWORD                → REP_SERVER_DB_PASSWORD
```

**Nothing changes in your build.** The Vite/CRA references will break (that's
the point — you'll update them in Step 2).

---

## Step 2 — Install the SDK and update your reads

```bash
npm install @rep-protocol/sdk
```

Replace every `import.meta.env.VITE_X` / `process.env.REACT_APP_X` call:

```ts
import { rep } from '@rep-protocol/sdk';

// Was: import.meta.env.VITE_API_URL
const apiUrl = rep.get('API_URL');

// Was: import.meta.env.VITE_API_URL ?? 'http://localhost:3001'
const apiUrl = rep.get('API_URL', 'http://localhost:3001');

// Was: process.env.REACT_APP_ANALYTICS_KEY  (sensitive — encrypted)
const key = await rep.getSecure('ANALYTICS_KEY');
```

`rep.get()` is synchronous — the SDK reads the payload from the DOM on import,
before your first component renders. No loading state, no Suspense.

---

## Step 3 — Build your app

Nothing changes in your build command. The gateway injects config at request
time, after the build:

```bash
# Vite
vite build

# Create React App
react-scripts build

# Next.js (static export)
next build && next export
```

The output `dist/` (or `build/`, `out/`) is environment-agnostic. The same
artifact goes to every environment.

---

## Step 4 — Run the gateway

Download the latest binary for your platform from
[GitHub Releases](https://github.com/ruachtech/rep/releases), then:

```bash
# Embedded mode: gateway serves your static files directly.
REP_PUBLIC_API_URL=https://api.example.com \
REP_PUBLIC_FEATURE_FLAGS=dark-mode,new-checkout \
REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
./rep-gateway --mode embedded --static-dir ./dist
```

Or with Docker (no binary download needed):

```bash
docker run --rm -p 8080:8080 \
  -e REP_PUBLIC_API_URL=https://api.example.com \
  -e REP_PUBLIC_FEATURE_FLAGS=dark-mode,new-checkout \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_live_abc123 \
  -v "$(pwd)/dist:/static:ro" \
  ghcr.io/ruachtech/rep/gateway:latest \
  --mode embedded --static-dir /static
```

Open `http://localhost:8080` — done. The gateway injected your vars into every
HTML response without modifying the build artifact.

---

## Changing config without rebuilding

Stop the gateway. Restart with different values. The app picks them up
immediately — no `npm run build`, no CI pipeline, no new image:

```bash
# Switch to production config
REP_PUBLIC_API_URL=https://api.prod.example.com \
REP_PUBLIC_FEATURE_FLAGS=dark-mode \
REP_SENSITIVE_ANALYTICS_KEY=ak_live_prod_xyz \
./rep-gateway --mode embedded --static-dir ./dist
```

Same `dist/` folder. Different runtime config. This is the core proposition.

---

## Proxy mode (existing nginx / Caddy upstream)

If you already have a static file server, run the gateway in front of it
instead of replacing it:

```bash
# Proxy mode: gateway injects into HTML responses from the upstream.
REP_PUBLIC_API_URL=https://api.example.com \
./rep-gateway --mode proxy --upstream localhost:80
```

The gateway intercepts `text/html` responses, injects the `<script>` tag, and
passes everything else through unmodified.

---

## When to add a `.rep.yaml` manifest

The manifest is additive. You can start without it and add it when you need:

- **Startup validation** — fail fast if a required variable is missing
- **Type constraints** — enforce `url`, `number`, `enum`, `csv` shapes
- **TypeScript types** — `rep typegen` generates `get()` / `getSecure()`
  overloads keyed to your actual variable names
- **Documentation** — a single source of truth for every variable the app uses

See [`examples/.rep.yaml`](./.rep.yaml) for a full annotated example, or the
[todo-react example](./todo-react/README.md) for a complete app that uses one.
