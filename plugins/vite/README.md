# @rep-protocol/vite

Vite plugin for the [Runtime Environment Protocol (REP)](https://github.com/RuachTech/rep). Injects REP environment variables during development without needing the Go gateway.

In production, this plugin does nothing — the REP gateway handles variable injection (and, in `embedded` mode, also serves your built static files).

## Install

```bash
pnpm add -D @rep-protocol/vite
# or
npm install -D @rep-protocol/vite
```

Peer dependency: `vite >= 4`.

## Setup

### 1. Add the plugin to `vite.config.ts`

```ts
// vite.config.ts
import { defineConfig } from 'vite';
import { repPlugin } from '@rep-protocol/vite';

export default defineConfig({
  plugins: [repPlugin()],
});
```

The plugin is dev-only by construction: it injects the `<script id="__rep__">`
payload into `index.html` and serves the `/rep/session-key` endpoint while the
Vite dev server runs. It does nothing during `vite build`.

#### Options

| Option   | Type      | Default        | Description                                 |
| -------- | --------- | -------------- | ------------------------------------------- |
| `env`    | `string`  | `".env.local"` | Path to env file, relative to project root. |
| `strict` | `boolean` | `false`        | Promote guardrail warnings to errors.       |

### 2. Configure your `.env.local`

```env
# PUBLIC tier — injected as plaintext in the <script> tag
REP_PUBLIC_API_URL=http://localhost:8080
REP_PUBLIC_SUPABASE_URL=https://your-project.supabase.co

# SENSITIVE tier — AES-256-GCM encrypted in the <script> tag
REP_SENSITIVE_SUPABASE_ANON_KEY=your_supabase_anon_key
```

Variables are classified by prefix:

| Prefix           | Tier      | Client access                        |
| ---------------- | --------- | ------------------------------------ |
| `REP_PUBLIC_`    | public    | Plaintext in HTML                    |
| `REP_SENSITIVE_` | sensitive | Encrypted, decrypted via session key |
| `REP_SERVER_`    | server    | Never reaches the client             |

### 3. Read variables on the client

Use [`@rep-protocol/sdk`](https://github.com/RuachTech/rep/tree/main/sdk):

```ts
import { rep } from '@rep-protocol/sdk';

const apiUrl = rep.get('API_URL'); // PUBLIC — synchronous
const anonKey = await rep.getSecure('SUPABASE_ANON_KEY'); // SENSITIVE — async
```

## How it works

1. **`transformIndexHtml`** reads `.env.local`, classifies variables by prefix,
   encrypts sensitive vars with AES-256-GCM, and injects a
   `<script id="__rep__" type="application/json">` tag into `<head>`.
2. **`@rep-protocol/sdk`** (`rep.get()` / `rep.getSecure()`) reads that script tag
   on the client to access variables.
3. **The `/rep/session-key` middleware** serves the ephemeral decryption key so
   the SDK can decrypt sensitive vars in the browser. Both the payload and the
   session key use the same ephemeral keys generated when the dev server starts.
4. **Env hot reload** — the plugin watches the env file and triggers a full page
   reload when it changes, so new values take effect without restarting Vite.
5. **Guardrails** scan `PUBLIC` values for patterns that look like secrets (known
   prefixes like `ghp_`, `sk_live_`, high Shannon entropy, long opaque strings)
   and warn at dev time (or throw, with `strict: true`).

## Production

This plugin is for development only. In production, run the REP gateway in front
of (or serving) your built `dist/`:

```sh
# Embedded mode — the gateway serves the static SPA itself, injects the payload,
# and serves /rep/session-key. No separate static server needed.
rep-gateway --mode embedded --static-dir ./dist --port 8080
```

The gateway reads `REP_PUBLIC_*` / `REP_SENSITIVE_*` from the container
environment at request time, so the same build runs against any environment.

## Security

- `SERVER` tier variables never leave the server process.
- `SENSITIVE` vars are AES-256-GCM encrypted with an ephemeral key regenerated on
  each dev-server restart.
- JSON payloads are Go-escaped (`<` `>` `&` → `<` `>` `&`) to
  prevent `</script>` injection.
- The dev session-key endpoint has no rate limiting or single-use semantics —
  production deployments must use the REP gateway.

## Troubleshooting

### `404` / no `#__rep__` script in development

Ensure `repPlugin()` is in your `vite.config.ts` `plugins` array and that your
env file exists at the configured path (default `.env.local`, relative to the
project root).

### `REPError: SENSITIVE variable "X" not found in payload`

The variable isn't set in your `.env.local`. If it's optional, catch the error:

```ts
const getOptionalKey = (): Promise<string> =>
  rep.getSecure('OPTIONAL_KEY').catch(() => '');
```

## License

Apache-2.0
