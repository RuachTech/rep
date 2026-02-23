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

- **Node.js** >= 20
- **Docker** (for container deployment) — _or_ the REP gateway binary (see below)

---

## Quick start

### 1. Install dependencies

```bash
npm install
```

### 2. Set up environment

```bash
cp .env.example .env.local
# Edit .env.local with your values
```

### 3. Development

The `@rep-protocol/cli` package (installed as a dev dependency) provides the
`rep` command for local development.

**Proxy mode** — run alongside Vite's dev server:

```bash
# Terminal 1: start Vite
npm run dev

# Terminal 2: start the REP gateway (proxies Vite at localhost:5173)
npx rep dev --port 3000 --env .env.local --proxy http://localhost:5173 --hot-reload
```

**Embedded mode** — serve a production build with hot reload:

```bash
npm run build
npx rep dev --port 3000 --env .env.local --static ./dist --hot-reload
```

Or use the shortcut scripts in `package.json`:

```bash
npm run rep:dev     # proxy mode
npm run rep:serve   # embedded mode (build first)
```

### 4. CLI tools

```bash
# Validate manifest
npm run rep:validate

# Generate TypeScript types from manifest
npm run rep:typegen

# Scan build output for leaked secrets
npm run rep:lint
```

---

## Docker

Build context is the **example directory itself** (no monorepo required):

```bash
# Build the image
docker build -t rep-todo .

# Run with development config
docker run --rm -p 8080:8080 \
  -e REP_PUBLIC_APP_TITLE="REP Todo" \
  -e REP_PUBLIC_ENV_NAME=development \
  -e REP_PUBLIC_API_URL=http://localhost:3001 \
  -e REP_PUBLIC_MAX_TODOS=55 \
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

## Config hot reload

The core demonstration of REP: **change runtime config without touching the
build artifact or reloading the page**.

The gateway exposes a Server-Sent Events endpoint at `/rep/changes`. The SDK
connects to it lazily when any `useRep()` hook mounts, and re-renders
subscribed components when a change event arrives.

```bash
npm run rep:serve
```

Open `http://localhost:8080`, then edit `.env.local` — for example, change
`REP_PUBLIC_APP_TITLE=REP Todo` to `REP_PUBLIC_APP_TITLE=My Todos`. The
gateway detects the file change, re-reads it, and pushes updates via SSE.
The browser updates without a page reload.

---

## Environment variables

| Variable | Tier | Type | Default | Description |
|---|---|---|---|---|
| `REP_PUBLIC_APP_TITLE` | PUBLIC | string | `REP Todo` | Page heading |
| `REP_PUBLIC_ENV_NAME` | PUBLIC | enum | _(required)_ | `development` / `staging` / `production` — controls badge colour |
| `REP_PUBLIC_API_URL` | PUBLIC | url | _(required)_ | Backend API base URL |
| `REP_PUBLIC_MAX_TODOS` | PUBLIC | number | `10` | Max todos before the add form locks |
| `REP_SENSITIVE_ANALYTICS_KEY` | SENSITIVE | string | — | Analytics write key — AES-256-GCM encrypted in page source |

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

## Project structure

```
todo-react/
├── .rep.yaml                  # REP manifest — source of truth for variables + types
├── .env.example               # Copy to .env.local; read by rep dev
├── .env.local                 # Your local values (gitignored)
├── Dockerfile                 # Multi-stage build (gateway + React, FROM scratch)
├── README.md
├── package.json
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
