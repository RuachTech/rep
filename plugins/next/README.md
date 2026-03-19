# @rep-protocol/next

Next.js plugin for the [Runtime Environment Protocol (REP)](https://github.com/RuachTech/rep). Injects REP environment variables during development without needing the Go gateway.

In production, this plugin does nothing — the REP gateway handles variable injection.

## Install

```bash
pnpm add @rep-protocol/next
# or
npm install @rep-protocol/next
```

Peer dependencies: `next >= 14`, `react >= 18`.

## Setup

### 1. Add `<RepScript />` to your root layout

`RepScript` is a React Server Component that injects a `<script id="__rep__">` tag containing your REP variables. It only renders in development; in production (`NODE_ENV=production`) it returns `null`.

```tsx
// app/layout.tsx
import { RepScript } from "@rep-protocol/next";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        <RepScript />
      </head>
      <body>{children}</body>
    </html>
  );
}
```

#### Props

| Prop     | Type      | Default        | Description                                      |
| -------- | --------- | -------------- | ------------------------------------------------ |
| `env`    | `string`  | `".env.local"` | Path to env file, relative to project root.      |
| `strict` | `boolean` | `false`        | Promote guardrail warnings to errors.            |

### 2. Add the session-key API route

The `@rep-protocol/sdk` decrypts `SENSITIVE` tier variables client-side using a key fetched from `/rep/session-key`. This plugin provides a route handler for that endpoint.

```ts
// app/api/rep/session-key/route.ts
export { GET } from "@rep-protocol/next/session-key";
```

The handler returns the decryption key in development and a `404` in production (the gateway serves this endpoint in prod with rate limiting and single-use tokens).

**Static export (`output: "export"`):** Add `force-static` so the export build doesn't reject the route:

```ts
// app/api/rep/session-key/route.ts
export const dynamic = "force-static";
export { GET } from "@rep-protocol/next/session-key";
```

### 3. Wire up the rewrite (for `next dev`)

The SDK fetches `/rep/session-key` (no `/api` prefix), so add a rewrite in `next.config.js`:

```js
// next.config.js
const nextConfig = {
  rewrites: async () => [
    { source: "/rep/session-key", destination: "/api/rep/session-key" },
  ],
};

module.exports = nextConfig;
```

> With `output: "export"`, Next.js warns that rewrites don't apply to exported builds — this is expected. The rewrite only needs to work during `next dev`; in production the gateway serves `/rep/session-key` directly.

### 4. Configure your `.env.local`

```env
# PUBLIC tier — injected as plaintext in the <script> tag
REP_PUBLIC_SUPABASE_URL=https://your-project.supabase.co
REP_PUBLIC_API_URL=http://localhost:8080

# SENSITIVE tier — AES-256-GCM encrypted in the <script> tag
REP_SENSITIVE_SUPABASE_ANON_KEY=your_supabase_anon_key
```

Variables are classified by prefix:

| Prefix            | Tier        | Client access            |
| ----------------- | ----------- | ------------------------ |
| `REP_PUBLIC_`     | public      | Plaintext in HTML        |
| `REP_SENSITIVE_`  | sensitive   | Encrypted, decrypted via session key |
| `REP_SERVER_`     | server      | Never reaches the client |

## How it works

1. **`RepScript`** reads `.env.local` at render time, classifies variables by prefix, encrypts sensitive vars with AES-256-GCM, and outputs a `<script type="application/json">` tag.
2. **`@rep-protocol/sdk`** (`rep.get()` / `rep.getSecure()`) reads that script tag on the client to access variables.
3. **Session key route** serves the ephemeral decryption key so the SDK can decrypt sensitive vars in the browser.
4. **Guardrails** scan `PUBLIC` values for patterns that look like secrets (known prefixes like `ghp_`, `sk_live_`, high Shannon entropy, long opaque strings) and warn at dev time.

Both `RepScript` and the session-key route share the same ephemeral keys via a process-wide singleton (`globalThis`), so the encryption key always matches the decryption key within a dev server lifecycle.

## Security

- `SERVER` tier variables never leave the server process.
- `SENSITIVE` vars are AES-256-GCM encrypted with an ephemeral key regenerated on each server restart.
- JSON payloads are Go-escaped (`<` `>` `&` → `\u003c` `\u003e` `\u0026`) to prevent `</script>` injection.
- The dev session-key endpoint has no rate limiting or single-use semantics — production deployments must use the REP gateway.

## Troubleshooting

### `OperationError: The operation failed for an operation-specific reason`

Key mismatch between `RepScript` and the session-key route. This happens if the bundler creates separate module instances for each entry point. The fix (applied in v0.1.12+) uses `globalThis` with `Symbol.for` to ensure a single key store across all bundles. Update to the latest version.

### `REPError: SENSITIVE variable "X" not found in payload`

The variable isn't set in your `.env.local`. If it's optional, catch the error:

```ts
const getOptionalKey = (): Promise<string> =>
  rep.getSecure("OPTIONAL_KEY").catch(() => "");
```

### `404` on `/rep/session-key` during development

Either the API route or the rewrite is missing. Ensure both are set up (steps 2 and 3 above).

## License

Apache-2.0
