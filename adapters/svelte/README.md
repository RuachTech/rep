# @rep-protocol/svelte

Svelte stores for the [Runtime Environment Protocol](https://github.com/ruachtech/rep).

Provides `repStore()` and `repSecureStore()` which wrap the REP SDK into Svelte-native readable stores. Subscribes to hot-reload updates automatically; the SSE connection is closed when all subscribers unsubscribe.

## Install

```bash
npm install @rep-protocol/svelte @rep-protocol/sdk
```

## Usage

```svelte
<script lang="ts">
  import { repStore, repSecureStore } from '@rep-protocol/svelte';

  // PUBLIC tier — synchronous initial value, updates on hot reload.
  const apiUrl = repStore('API_URL', 'http://localhost:3000');
  const flags = repStore('FEATURE_FLAGS', '');

  // SENSITIVE tier — starts as null, resolves after session key fetch.
  const analyticsKey = repSecureStore('ANALYTICS_KEY');
</script>

<p>API: {$apiUrl}</p>
<p>Analytics: {$analyticsKey ?? 'loading…'}</p>
```

## API

### `repStore(key, defaultValue?)`

Reads a **PUBLIC** tier variable as a Svelte `Readable<string | undefined>`.

```typescript
const value: Readable<string | undefined> = repStore('API_URL');
const value: Readable<string | undefined> = repStore('API_URL', 'fallback');
```

- Synchronous initial value — set immediately from the REP payload.
- Returns `defaultValue` (or `undefined`) if the variable is not present.
- Automatically updates when the variable changes via hot reload.
- **Lazy:** the SSE subscription is established only when there is at least one subscriber. The connection is closed when all subscribers unsubscribe.

### `repSecureStore(key)`

Reads a **SENSITIVE** tier variable as a Svelte `Readable<string | null>`.

```typescript
const value: Readable<string | null> = repSecureStore('ANALYTICS_KEY');
```

- Starts as `null`.
- Resolves to the decrypted value once the session key fetch completes.
- The decrypted value is cached by the SDK for the page lifetime.
- Errors are swallowed silently (the SDK logs them); the store stays `null`.

## Hot Reload

`repStore` subscribes to the REP gateway's SSE stream lazily (on first `subscribe()` call). The store updates automatically when config changes. The SSE connection is closed when there are no remaining subscribers.

`repSecureStore` does not support hot reload — changes to sensitive variables require a page reload.

## Requirements

- Svelte ≥ 4 or Svelte 5
- `@rep-protocol/sdk` as a peer dependency

## Development Mode

Without the REP gateway, `repStore()` returns a store with value `undefined`. Use `defaultValue` for local development:

```typescript
const apiUrl = repStore('API_URL', 'http://localhost:3000');
```

Or inject a mock payload in your `index.html`:

```html
<script id="__rep__" type="application/json">
{"public":{"API_URL":"http://localhost:3000"},"_meta":{"version":"0.1.0","injected_at":"2026-01-01T00:00:00Z","integrity":"hmac-sha256:dev","ttl":0}}
</script>
```

## Specification

Implements [REP-RFC-0001 §5.5 — Framework Adapters](https://github.com/ruachtech/rep/blob/main/spec/REP-RFC-0001.md).

## License

Apache 2.0
