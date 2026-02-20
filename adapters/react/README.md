# @rep-protocol/react

React hooks for the [Runtime Environment Protocol](https://github.com/ruachtech/rep).

Provides `useRep()` and `useRepSecure()` hooks that read environment variables injected by the REP gateway. Both hooks automatically re-render when config changes via hot reload.

## Install

```bash
npm install @rep-protocol/react @rep-protocol/sdk
```

## Usage

```tsx
import { useRep, useRepSecure } from '@rep-protocol/react';

function App() {
  // PUBLIC tier — synchronous, no loading state.
  const apiUrl = useRep('API_URL', 'http://localhost:3000');
  const flags = useRep('FEATURE_FLAGS', '').split(',');

  // SENSITIVE tier — async, renders null until resolved.
  const { value: analyticsKey, loading, error } = useRepSecure('ANALYTICS_KEY');

  if (loading) return <span>Loading…</span>;
  if (error) return <span>Config error: {error.message}</span>;

  return (
    <div>
      <p>API: {apiUrl}</p>
      <p>Analytics: {analyticsKey}</p>
    </div>
  );
}
```

## API

### `useRep(key, defaultValue?)`

Reads a **PUBLIC** tier variable. Synchronous — no loading state, no Suspense, no network call.

```tsx
const value = useRep('API_URL');              // string | undefined
const value = useRep('API_URL', 'fallback'); // string (never undefined)
```

- Returns the value from the injected REP payload immediately.
- Returns `defaultValue` (or `undefined`) if the variable is not present.
- Automatically re-renders when the variable changes via hot reload.
- Unsubscribes from hot reload on unmount.

### `useRepSecure(key)`

Reads a **SENSITIVE** tier variable. Async — fetches a session key on first call.

```tsx
const { value, loading, error } = useRepSecure('ANALYTICS_KEY');
// value:   string | null  — null until resolved
// loading: boolean        — true while fetching
// error:   Error | null   — set if the session key fetch or decryption fails
```

- Starts with `{ value: null, loading: true, error: null }`.
- Resolves to `{ value: '...', loading: false, error: null }` on success.
- The decrypted value is cached by the SDK for the page lifetime.
- Re-fetches if the `key` prop changes.

## Hot Reload

Both hooks subscribe to the REP gateway's SSE stream (if `--hot-reload` is enabled). When a variable changes, the component re-renders with the new value automatically.

If hot reload is not available, the hooks still work — they just won't update after initial render.

## Requirements

- React ≥ 16.8 (hooks support)
- `@rep-protocol/sdk` as a peer dependency
- REP gateway running and injecting the `<script id="__rep__">` payload

## Development Mode

Without the REP gateway, `useRep()` returns `undefined`. Use `defaultValue` for local development:

```tsx
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
