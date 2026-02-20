# REP Todo — React Example

A minimal todo app that demonstrates how to use the **REP gateway** with the
**`@rep-protocol/react`** adapter. Environment variables are injected by the
gateway at container startup — no rebuild required to change config.

## What this shows

| Feature | Where |
|---|---|
| `useRep('APP_TITLE')` — synchronous public var | `App.tsx` header |
| `useRep('ENV_NAME')` — drives env badge colour | `App.tsx` header |
| `useRep('MAX_TODOS')` — limits list length | `TodoList.tsx` |
| `useRepSecure('ANALYTICS_KEY')` — async, encrypted | `RepConfigPanel.tsx` |
| Payload metadata via `meta()` | `RepConfigPanel.tsx` |

Click **Show Config** in the running app to see every injected variable side by
side with its tier (PUBLIC / SENSITIVE) and the raw payload metadata.

---

## Prerequisites

- **Node.js** >= 20 and **pnpm** >= 9
- **Go** >= 1.24.5 (to build the gateway binary) — _or_ **Docker** (skip Go)

---

## Option A — Local (no Docker)

### 1. Install workspace dependencies

```bash
# From the repo root
pnpm install
```

### 2. Build the SDK chain

The example depends on `@rep-protocol/sdk` and `@rep-protocol/react`.
Build them before building the app:

```bash
pnpm --filter @rep-protocol/sdk   run build
pnpm --filter @rep-protocol/react run build
```

### 3. Build the CLI and gateway

```bash
# CLI — provides rep dev / validate / typegen / lint
pnpm --filter @rep-protocol/cli run build

# Gateway binary — the runtime the CLI wraps
cd gateway && make build && cd ..
# Output: gateway/bin/rep-gateway
```

---

### A0 — CLI-first workflow (recommended)

`@rep-protocol/cli` is the intended DX layer. Instead of memorising gateway
flags and manually prefixing every env var, you put values in `.env.local` and
let the CLI handle the rest. The four commands map directly to the development
lifecycle — and they're all wired as `pnpm` scripts in `package.json`.

**Setup — copy the env file:**

```bash
# From examples/todo-react/
cp .env.example .env.local
# Edit .env.local with your values
```

---

**1. `rep validate` — fail fast before touching the gateway**

```bash
pnpm rep:validate
# ✓ Manifest is valid
#   Version: 0.1.0
#   Variables: 5 total
#     - PUBLIC: 4
#     - SENSITIVE: 1
#     - SERVER: 0
```

Checks every variable declared in `.rep.yaml` against the schema: required
fields, type constraints (`url`, `enum`, `number`), and allowed enum values.
Exits non-zero on failure — safe to run as a pre-start hook or CI gate.

---

**2. `rep typegen` — typed `get()` and `getSecure()` overloads**

```bash
pnpm rep:typegen
# ✓ Generated type definitions: src/rep.d.ts
#   PUBLIC variables: 4
#   SENSITIVE variables: 1
```

Writes `src/rep.d.ts` with overloads keyed to your exact variable names:

```ts
// Generated — do not edit. Re-run `pnpm rep:typegen` after manifest changes.
declare module "@rep-protocol/sdk" {
  export function get(key: "APP_TITLE" | "ENV_NAME" | "API_URL" | "MAX_TODOS"): string | undefined;
  export function getSecure(key: "ANALYTICS_KEY"): Promise<string>;
}
```

After this, `useRep('TYPO')` is a **TypeScript compile error**. The manifest
becomes the single source of truth for both the gateway and the type checker.
Re-run whenever you add or rename a variable in `.rep.yaml`.

---

**3. `rep dev` — replaces all the raw gateway commands**

Instead of manually exporting `REP_PUBLIC_*` vars and passing gateway flags,
point the CLI at `.env.local`:

```bash
# Proxy mode — Vite dev server must be running separately at 5173
pnpm rep:dev
# Starting REP development server...
# Loading environment from: .env.local
# Loaded 5 variable(s)
#
# Configuration:
#   Gateway binary: ../../gateway/bin/rep-gateway
#   Mode: proxy  →  Upstream: http://localhost:5173
#   Hot reload: disabled
#
# Gateway listening on :8080
```

```bash
# Embedded mode — serves built dist/ with hot reload
pnpm rep:serve
```

The CLI reads `.env.local`, strips comments, exports every `REP_*` var into
the spawned gateway's environment, and passes the right `--mode`, `--upstream`,
and `--static-dir` flags automatically.

---

**4. `rep lint` — catch secrets before they ship**

```bash
# Run after pnpm build
pnpm rep:lint
# Scanning directory: ./dist
# Found 3 file(s) to scan
#
# ✓ No potential secrets detected
```

Applies the same entropy and known-format detectors as the gateway guardrails
to every `.js`/`.mjs` file in `dist/`. Catches `AKIA*` AWS keys, `sk_live_*`
Stripe keys, `eyJ*` JWT tokens, high-entropy strings — anything that looks
like it should have been injected via REP instead of baked into the bundle.

Add `--strict` to fail CI on any finding:

```bash
pnpm rep:lint -- --strict
```

---

> **A1 / A2 / A3 below show the raw gateway commands** — useful for
> understanding what `rep dev` does under the hood, or if you prefer not to use
> the CLI. Skip ahead to [Option B](#option-b--docker) if you're done.

### A1 — Dev server (fastest iteration, active code changes)

Run Vite's dev server and the gateway simultaneously. The gateway proxies
Vite and injects the REP payload on every request, so you get live REP vars
without a production build step.

**Terminal 1** — Vite dev server:

```bash
pnpm --filter @rep-protocol/example-todo-react run dev
# Vite starts at http://localhost:5173
```

**Terminal 2** — Gateway in proxy mode:

```bash
REP_PUBLIC_APP_TITLE="REP Todo" \
REP_PUBLIC_ENV_NAME=development \
REP_PUBLIC_API_URL=http://localhost:3001 \
REP_PUBLIC_MAX_TODOS=5 \
REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
./gateway/bin/rep-gateway \
  --mode proxy \
  --upstream localhost:5173
```

| URL | What you get |
|---|---|
| `http://localhost:5173` | Vite direct — Vite HMR works, no REP vars injected |
| `http://localhost:8080` | Gateway proxy — REP vars injected, Vite HMR not available |

> **Why two ports?** Vite HMR uses WebSocket. The REP gateway's reverse proxy
> does not upgrade WebSocket connections, so HMR only works when hitting Vite
> directly. In practice: write code at 5173 (instant feedback), verify REP
> integration at 8080.

---

### A2 — Production build (full end-to-end, no live Vite)

Build once, then run the gateway against the built `dist/`. Refresh the
browser manually after each code change.

**Build the app:**

```bash
pnpm --filter @rep-protocol/example-todo-react run build
# Output: examples/todo-react/dist/
```

**Run the gateway:**

```bash
REP_PUBLIC_APP_TITLE="REP Todo" \
REP_PUBLIC_ENV_NAME=development \
REP_PUBLIC_API_URL=http://localhost:3001 \
REP_PUBLIC_MAX_TODOS=5 \
REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
./gateway/bin/rep-gateway \
  --mode embedded \
  --static-dir examples/todo-react/dist
```

Open **http://localhost:8080**.

---

### A3 — Config hot reload (no rebuild, no page reload)

This is the core demonstration of REP: **change runtime config without
touching the build artifact or reloading the page**.

The gateway exposes a Server-Sent Events endpoint at `/rep/changes`. The SDK
connects to it lazily when any `useRep()` hook mounts, and re-renders
subscribed components when a change event arrives.

**Start the gateway with hot reload enabled:**

```bash
REP_PUBLIC_APP_TITLE="REP Todo" \
REP_PUBLIC_ENV_NAME=development \
REP_PUBLIC_API_URL=http://localhost:3001 \
REP_PUBLIC_MAX_TODOS=5 \
REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
./gateway/bin/rep-gateway \
  --mode embedded \
  --static-dir examples/todo-react/dist \
  --hot-reload
```

Open `http://localhost:8080` and click **Show Config**. Observe the SSE
connection in the Network tab (`/rep/changes`, type: `eventsource`).

**Trigger a reload — no rebuild, no page refresh:**

In a second terminal, send `SIGHUP` to the gateway process. It re-reads
`os.Environ()`, rebuilds the payload, and broadcasts diffs over SSE:

```bash
kill -HUP $(pgrep rep-gateway)
```

The connected browser tabs receive the event and `useRep()` hooks re-render
with the updated values — **no page load**.

**Changing values between restarts (no rebuild):**

To see different values, restart the gateway with new env vars — the built
`dist/` is untouched:

```bash
# Ctrl+C the running gateway, then:
REP_PUBLIC_APP_TITLE="My Todos" \
REP_PUBLIC_ENV_NAME=staging \
REP_PUBLIC_MAX_TODOS=3 \
REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
./gateway/bin/rep-gateway \
  --mode embedded \
  --static-dir examples/todo-react/dist \
  --hot-reload
```

The title changes from `REP Todo` to `My Todos`, the badge turns amber
(`staging`), and the add form locks after 3 todos — all from env vars, all
without `npm run build`.

> **Production hot reload:** In Kubernetes or Docker Swarm, env vars can be
> rotated in-flight via ConfigMaps or secrets. The gateway detects the change
> (via `--hot-reload-mode poll` or `file_watch`) and pushes SSE events to all
> open browser sessions simultaneously, with no restart or page reload. See
> the spec (`REP-RFC-0001 §4.6`) for the full SSE wire format.

---

## Option B — Docker

Build context is the **repo root** (required for the pnpm workspace):

```bash
# Build the image
docker build \
  -f examples/todo-react/Dockerfile \
  -t rep-todo \
  .

# Run with development config
docker run --rm -p 8080:8080 \
  -e REP_PUBLIC_APP_TITLE="REP Todo" \
  -e REP_PUBLIC_ENV_NAME=development \
  -e REP_PUBLIC_API_URL=http://localhost:3001 \
  -e REP_PUBLIC_MAX_TODOS=5 \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_demo_abc123 \
  rep-todo
```

Open **http://localhost:8080**.

Switch to staging config — **same image, no rebuild**:

```bash
docker run --rm -p 8080:8080 \
  -e REP_PUBLIC_APP_TITLE="REP Todo (Staging)" \
  -e REP_PUBLIC_ENV_NAME=staging \
  -e REP_PUBLIC_API_URL=https://api.staging.example.com \
  -e REP_PUBLIC_MAX_TODOS=20 \
  -e REP_SENSITIVE_ANALYTICS_KEY=ak_staging_xyz789 \
  rep-todo
```

---

## Environment variables

| Variable | Tier | Type | Default | Description |
|---|---|---|---|---|
| `REP_PUBLIC_APP_TITLE` | PUBLIC | string | `REP Todo` | Page heading |
| `REP_PUBLIC_ENV_NAME` | PUBLIC | enum | _(required)_ | `development` / `staging` / `production` — controls badge colour |
| `REP_PUBLIC_API_URL` | PUBLIC | url | _(required)_ | Backend API base URL |
| `REP_PUBLIC_MAX_TODOS` | PUBLIC | number | `10` | Max todos before the add form locks |
| `REP_SENSITIVE_ANALYTICS_KEY` | SENSITIVE | string | — | Analytics write key — AES-256-GCM encrypted in page source |

Gateway configuration (not injected into the app):

| Variable | Default | Description |
|---|---|---|
| `REP_GATEWAY_PORT` | `8080` | Port the gateway listens on |
| `REP_GATEWAY_MODE` | `embedded` | `embedded` (serve static files) or `proxy` (reverse proxy) |
| `REP_GATEWAY_LOG_FORMAT` | `text` | `text` or `json` |

---

## How it works

```
Browser request → REP Gateway (port 8080)
                       │
                       ├─ Reads all REP_* env vars at startup
                       ├─ Classifies: PUBLIC / SENSITIVE / SERVER
                       ├─ Encrypts SENSITIVE vars (AES-256-GCM)
                       ├─ Computes HMAC integrity token
                       └─ Pre-renders <script id="__rep__"> tag

On each HTML request:
  Gateway serves dist/index.html with the <script> injected before </head>

In the browser:
  @rep-protocol/sdk reads the <script> tag synchronously on module load
  useRep('KEY')        → synchronous read from the public JSON blob
  useRepSecure('KEY')  → fetches /rep/session-key, decrypts AES blob
```

The same `dist/` folder (the build artifact) is served unchanged across
`development`, `staging`, and `production`. Only the gateway's environment
variables differ between deployments — exactly like any other twelve-factor app.

---

## CLI scripts (package.json)

| Script | Command | Description |
|---|---|---|
| `pnpm rep:validate` | `rep validate --manifest .rep.yaml` | Validate manifest schema and constraints |
| `pnpm rep:typegen` | `rep typegen --manifest .rep.yaml --output src/rep.d.ts` | Generate typed SDK overloads |
| `pnpm rep:dev` | `rep dev --env .env.local --proxy http://localhost:5173` | Dev server (proxy mode) |
| `pnpm rep:serve` | `rep dev --env .env.local --static ./dist --hot-reload` | Dev server (embedded, hot reload) |
| `pnpm rep:lint` | `rep lint --dir ./dist` | Scan build output for leaked secrets |

---

## Project structure

```
examples/todo-react/
├── .rep.yaml                  # REP manifest — source of truth for variables + types
├── .env.example               # Copy to .env.local; read by rep dev
├── .env.local                 # Your local values (gitignored)
├── Dockerfile                 # Multi-stage build (gateway + React, FROM scratch)
├── README.md
├── package.json               # Scripts: rep:validate, rep:typegen, rep:dev, rep:serve, rep:lint
├── tsconfig.json
├── vite.config.ts
├── index.html                 # <head> tag required for gateway injection
└── src/
    ├── rep.d.ts               # Generated by rep:typegen — typed get()/getSecure() overloads
    ├── main.tsx
    ├── App.tsx                # useRep() for title, env badge, maxTodos
    ├── types.ts
    └── components/
        ├── TodoList.tsx       # maxTodos limit shown inline
        ├── TodoItem.tsx
        └── RepConfigPanel.tsx # useRepSecure() + meta() visualisation
```
