# @rep-protocol/vue

Vue composables for the [Runtime Environment Protocol](https://github.com/ruachtech/rep).

Provides `useRep()` and `useRepSecure()` composables that expose REP environment variables as reactive refs. Hot-reload updates are reflected automatically; subscriptions are cleaned up on unmount.

## Install

```bash
npm install @rep-protocol/vue @rep-protocol/sdk
```

## Usage

```vue
<script setup lang="ts">
import { useRep, useRepSecure } from '@rep-protocol/vue';

// PUBLIC tier — synchronous, immediate, reactive.
const apiUrl = useRep('API_URL', 'http://localhost:3000');
const flags = useRep('FEATURE_FLAGS', '');

// SENSITIVE tier — async, resolves after session key fetch.
const analyticsKey = useRepSecure('ANALYTICS_KEY');
</script>

<template>
  <div>
    <p>API: {{ apiUrl }}</p>
    <p>Analytics: {{ analyticsKey ?? 'loading…' }}</p>
  </div>
</template>
```

## API

### `useRep(key, defaultValue?)`

Reads a **PUBLIC** tier variable as a reactive `Ref`. Synchronous initial value.

```typescript
const value: Ref<string | undefined> = useRep('API_URL');
const value: Ref<string | undefined> = useRep('API_URL', 'fallback');
```

- Returns a `Ref` that is set immediately to the value from the injected REP payload.
- Returns `defaultValue` (or `undefined`) if the variable is not present.
- Automatically updates when the variable changes via hot reload.
- Unsubscribes from hot reload in `onUnmounted` — must be called inside `setup()`.

### `useRepSecure(key)`

Reads a **SENSITIVE** tier variable as a reactive `Ref`. Resolves asynchronously.

```typescript
const analyticsKey: Ref<string | null> = useRepSecure('ANALYTICS_KEY');
```

- Starts as `null`.
- Sets the ref value once the session key fetch and decryption complete.
- The decrypted value is cached by the SDK for the page lifetime.
- Errors are swallowed silently (the SDK logs them); the ref stays `null`.

## Hot Reload

`useRep` subscribes to the REP gateway's SSE stream on mount and updates the ref value automatically when config changes. The subscription is cleaned up when the component unmounts.

`useRepSecure` does not subscribe to hot reload — sensitive variable changes require a page reload to re-derive a new session key.

## Requirements

- Vue ≥ 3.0
- `@rep-protocol/sdk` as a peer dependency
- Composables must be called from a component's `setup()` function (for `onUnmounted` cleanup)

## Development Mode

Without the REP gateway, `useRep()` returns a ref with value `undefined`. Use `defaultValue` for local development:

```typescript
const apiUrl = useRep('API_URL', 'http://localhost:3000');
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
