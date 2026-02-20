# @rep-protocol/sdk

Client SDK for the [Runtime Environment Protocol](https://github.com/ruach-tech/rep).

Zero-dependency, framework-agnostic, ~1.5KB gzipped. Reads environment variables injected by the REP gateway at runtime — no build tool plugins, no framework coupling.

## Install

```bash
npm install @rep-protocol/sdk
```

## Usage

```typescript
import { rep } from '@rep-protocol/sdk';

// PUBLIC tier — synchronous, immediate, no network call.
const apiUrl = rep.get('API_URL');
const flags = rep.get('FEATURE_FLAGS', '').split(',');

// SENSITIVE tier — async, fetches session key, decrypts.
const analyticsKey = await rep.getSecure('ANALYTICS_KEY');

// All public vars as a frozen object.
const all = rep.getAll();

// Verify payload integrity.
if (!rep.verify()) {
  console.error('Config may have been tampered with!');
}

// Hot reload (if gateway has --hot-reload enabled).
const unsub = rep.onChange('FEATURE_FLAGS', (newVal, oldVal) => {
  console.log(`Flags changed: ${oldVal} → ${newVal}`);
});

// Cleanup.
unsub();
```

## API

| Method | Returns | Description |
|---|---|---|
| `rep.get(key, default?)` | `string \| undefined` | Synchronous. PUBLIC tier variable. |
| `rep.getSecure(key)` | `Promise<string>` | Async. SENSITIVE tier variable (decrypts via session key). |
| `rep.getAll()` | `Record<string, string>` | All PUBLIC vars as a frozen object. |
| `rep.verify()` | `boolean` | Check payload integrity. |
| `rep.meta()` | `REPMeta \| null` | Payload metadata (version, counts, status). |
| `rep.onChange(key, cb)` | `() => void` | Subscribe to hot reload changes. Returns unsubscribe fn. |
| `rep.onAnyChange(cb)` | `() => void` | Subscribe to all changes. Returns unsubscribe fn. |

## Development Mode

Without the REP gateway, `rep.get()` returns `undefined`. Use defaults:

```typescript
const apiUrl = rep.get('API_URL', 'http://localhost:3000');
```

Or mock the payload in your `index.html` during development:

```html
<script id="__rep__" type="application/json">
{"public":{"API_URL":"http://localhost:3000"},"_meta":{"version":"0.1.0","injected_at":"2026-01-01T00:00:00Z","integrity":"hmac-sha256:dev","ttl":0}}
</script>
```

## Specification

Implements [REP-RFC-0001 §5 — Client SDK Specification](https://github.com/ruach-tech/rep/blob/main/spec/REP-RFC-0001.md#5-client-sdk-specification).

## License

Apache 2.0
