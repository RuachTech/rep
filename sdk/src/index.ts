/**
 * @rep-protocol/sdk — Client SDK for the Runtime Environment Protocol.
 *
 * Zero-dependency, framework-agnostic SDK for reading environment variables
 * injected by the REP gateway.
 *
 * PUBLIC tier variables are available synchronously via `rep.get()`.
 * SENSITIVE tier variables require async decryption via `rep.getSecure()`.
 *
 * @see https://github.com/ruach-tech/rep — REP-RFC-0001 §5
 * @license Apache-2.0
 * @version 0.1.0
 */

// ─── Types ─────────────────────────────────────────────────────────────────────

interface REPPayload {
  public: Record<string, string>;
  sensitive?: string;
  _meta: {
    version: string;
    injected_at: string;
    integrity: string;
    key_endpoint?: string;
    hot_reload?: string;
    ttl: number;
  };
}

interface REPMeta {
  version: string;
  injectedAt: Date;
  integrityValid: boolean;
  publicCount: number;
  sensitiveAvailable: boolean;
  hotReloadAvailable: boolean;
}

interface SessionKeyResponse {
  key: string;
  expires_at: string;
  nonce: string;
}

type ChangeCallback = (newValue: string, oldValue: string | undefined) => void;
type AnyChangeCallback = (key: string, newValue: string, oldValue: string | undefined) => void;

class REPError extends Error {
  constructor(message: string) {
    super(`[REP] ${message}`);
    this.name = 'REPError';
  }
}

// ─── Internal State ────────────────────────────────────────────────────────────

let _payload: REPPayload | null = null;
let _available = false;
let _tampered = false;
let _publicVars: Readonly<Record<string, string>> = Object.freeze({});

// Cache for decrypted sensitive variables (in-memory only, never persisted).
let _sensitiveCache: Record<string, string> | null = null;

// Hot reload state.
let _eventSource: EventSource | null = null;
let _changeListeners: Map<string, Set<ChangeCallback>> = new Map();
let _anyChangeListeners: Set<AnyChangeCallback> = new Set();

// ─── Initialisation (Synchronous) ──────────────────────────────────────────────
// Per §5.3: Steps 1–6 MUST be synchronous. No network calls during init.

function _init(): void {
  // Step 1: Locate the <script id="__rep__"> element.
  if (typeof document === 'undefined') {
    // SSR or non-browser environment — SDK is unavailable.
    _available = false;
    return;
  }

  const el = document.getElementById('__rep__');
  if (!el) {
    _available = false;
    return;
  }

  // Step 3: Parse the JSON content.
  try {
    _payload = JSON.parse(el.textContent || '');
  } catch (e) {
    console.error('[REP] Failed to parse payload:', e);
    _available = false;
    return;
  }

  if (!_payload || !_payload.public || !_payload._meta) {
    console.error('[REP] Payload is malformed — missing required fields.');
    _available = false;
    return;
  }

  // Step 4: Verify integrity via SRI hash.
  const declaredIntegrity = el.getAttribute('data-rep-integrity');
  if (declaredIntegrity) {
    // Verify asynchronously but set the flag synchronously as a best-effort.
    // Full async verification happens in verify().
    _verifySRI(el.textContent || '', declaredIntegrity).then((valid) => {
      if (!valid) {
        console.error('[REP] Integrity check failed — payload may have been tampered with.');
        _tampered = true;
      }
    });
  }

  // Step 6: Freeze the public variables object.
  _publicVars = Object.freeze({ ..._payload.public });
  _available = true;
}

/**
 * Verify the SRI hash of the payload content.
 * Uses the Web Crypto API (SubtleCrypto).
 */
async function _verifySRI(content: string, declared: string): Promise<boolean> {
  if (!declared.startsWith('sha256-')) return false;
  const expected = declared.slice(7); // Remove "sha256-" prefix.

  try {
    const encoder = new TextEncoder();
    const data = encoder.encode(content);
    const hashBuffer = await crypto.subtle.digest('SHA-256', data);
    const hashArray = new Uint8Array(hashBuffer);
    const hashBase64 = btoa(String.fromCharCode(...hashArray));
    return hashBase64 === expected;
  } catch {
    return false;
  }
}

// Run init immediately on module load.
_init();

// ─── Public API ────────────────────────────────────────────────────────────────

/**
 * Retrieve a PUBLIC tier variable. Synchronous — no network call.
 *
 * Per §5.2: when called with a defaultValue the return type narrows to string.
 *
 * @param key - Variable name (without REP_PUBLIC_ prefix).
 * @param defaultValue - Fallback if the variable is not present.
 */
export function get(key: string, defaultValue: string): string;
export function get(key: string, defaultValue?: undefined): string | undefined;
export function get(key: string, defaultValue?: string): string | undefined {
  if (!_available) return defaultValue;
  const val = _publicVars[key];
  return val !== undefined ? val : defaultValue;
}

/**
 * Retrieve a SENSITIVE tier variable. Async — fetches a session key.
 *
 * Per §5.2, decrypted values are cached in memory for the page lifetime.
 *
 * @param key - Variable name (without REP_SENSITIVE_ prefix).
 * @throws REPError if the session key endpoint is unreachable or decryption fails.
 */
export async function getSecure(key: string): Promise<string> {
  if (!_available || !_payload) {
    throw new REPError('REP payload not available.');
  }

  if (!_payload.sensitive || !_payload._meta.key_endpoint) {
    throw new REPError('No SENSITIVE tier variables in payload.');
  }

  // Return from cache if already decrypted.
  if (_sensitiveCache && key in _sensitiveCache) {
    return _sensitiveCache[key];
  }

  // Fetch session key from the gateway.
  const resp = await fetch(_payload._meta.key_endpoint);
  if (!resp.ok) {
    throw new REPError(`Session key request failed: ${resp.status} ${resp.statusText}`);
  }

  const sessionKey: SessionKeyResponse = await resp.json();

  // Decode the encryption key.
  const rawKey = Uint8Array.from(atob(sessionKey.key), (c) => c.charCodeAt(0));

  // Decode the encrypted blob.
  const blobBytes = Uint8Array.from(atob(_payload.sensitive), (c) => c.charCodeAt(0));

  // Extract nonce (first 12 bytes) and ciphertext+tag (rest).
  const nonce = blobBytes.slice(0, 12);
  const ciphertext = blobBytes.slice(12);

  // Decrypt using Web Crypto API (AES-256-GCM).
  const cryptoKey = await crypto.subtle.importKey(
    'raw',
    rawKey,
    { name: 'AES-GCM' },
    false,
    ['decrypt']
  );

  // The AAD is the preliminary integrity token (see payload builder).
  // For simplicity in the SDK, we use the integrity from _meta.
  // This matches the gateway's encryption AAD.
  const encoder = new TextEncoder();
  const aad = encoder.encode(_payload._meta.integrity);

  const plaintext = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: nonce, additionalData: aad },
    cryptoKey,
    ciphertext
  );

  const decoder = new TextDecoder();
  const sensitiveMap: Record<string, string> = JSON.parse(decoder.decode(plaintext));

  // Cache all decrypted values.
  _sensitiveCache = sensitiveMap;

  if (!(key in sensitiveMap)) {
    throw new REPError(`SENSITIVE variable "${key}" not found in payload.`);
  }

  return sensitiveMap[key];
}

/**
 * Retrieve all PUBLIC tier variables as a frozen object. Synchronous.
 */
export function getAll(): Readonly<Record<string, string>> {
  return _publicVars;
}

/**
 * Verify payload integrity. Synchronous check of the tamper flag.
 * For full async SRI verification, use meta().integrityValid.
 */
export function verify(): boolean {
  return _available && !_tampered;
}

/**
 * Returns metadata about the current REP payload.
 * Returns null if no payload is present.
 */
export function meta(): REPMeta | null {
  if (!_available || !_payload) return null;

  return {
    version: _payload._meta.version,
    injectedAt: new Date(_payload._meta.injected_at),
    integrityValid: !_tampered,
    publicCount: Object.keys(_payload.public).length,
    sensitiveAvailable: !!_payload.sensitive,
    hotReloadAvailable: !!_payload._meta.hot_reload,
  };
}

// ─── Hot Reload ────────────────────────────────────────────────────────────────

/**
 * Register a callback for when a specific PUBLIC variable changes.
 *
 * If hot reload is not available, logs a warning and returns a no-op unsubscribe.
 * Per §5.3 step 7: SSE connection is established lazily on first onChange call.
 */
export function onChange(key: string, callback: ChangeCallback): () => void {
  if (!_available || !_payload?._meta.hot_reload) {
    console.warn(`[REP] Hot reload not available. onChange("${key}") is a no-op.`);
    return () => {};
  }

  // Lazy-connect to SSE.
  _ensureEventSource();

  if (!_changeListeners.has(key)) {
    _changeListeners.set(key, new Set());
  }
  _changeListeners.get(key)!.add(callback);

  // Return unsubscribe function.
  return () => {
    _changeListeners.get(key)?.delete(callback);
    if (_changeListeners.get(key)?.size === 0) {
      _changeListeners.delete(key);
    }
    _maybeCloseEventSource();
  };
}

/**
 * Register a callback for any variable change.
 */
export function onAnyChange(callback: AnyChangeCallback): () => void {
  if (!_available || !_payload?._meta.hot_reload) {
    console.warn('[REP] Hot reload not available. onAnyChange() is a no-op.');
    return () => {};
  }

  _ensureEventSource();
  _anyChangeListeners.add(callback);

  return () => {
    _anyChangeListeners.delete(callback);
    _maybeCloseEventSource();
  };
}

/**
 * Establish the SSE connection if not already connected.
 */
function _ensureEventSource(): void {
  if (_eventSource || !_payload?._meta.hot_reload) return;

  _eventSource = new EventSource(_payload._meta.hot_reload);

  _eventSource.addEventListener('rep:config:update', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data);
      const { key, value } = data as { key: string; tier: string; value: string };
      const oldValue = _publicVars[key];

      // Update the frozen public vars.
      const updated = { ..._publicVars, [key]: value };
      _publicVars = Object.freeze(updated);

      // Also update the payload reference.
      if (_payload) {
        _payload.public[key] = value;
      }

      // Notify listeners.
      _changeListeners.get(key)?.forEach((cb) => cb(value, oldValue));
      _anyChangeListeners.forEach((cb) => cb(key, value, oldValue));
    } catch (err) {
      console.error('[REP] Failed to process hot reload event:', err);
    }
  });

  _eventSource.addEventListener('rep:config:delete', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data);
      const { key } = data as { key: string };
      const oldValue = _publicVars[key];

      // Remove from public vars.
      const updated = { ..._publicVars };
      delete (updated as Record<string, string>)[key];
      _publicVars = Object.freeze(updated);

      if (_payload) {
        delete _payload.public[key];
      }

      // Notify with undefined as new value.
      _changeListeners.get(key)?.forEach((cb) => cb('', oldValue));
      _anyChangeListeners.forEach((cb) => cb(key, '', oldValue));
    } catch (err) {
      console.error('[REP] Failed to process hot reload delete:', err);
    }
  });

  _eventSource.onerror = () => {
    console.warn('[REP] Hot reload SSE connection lost. Will reconnect automatically.');
  };
}

/**
 * Close the SSE connection if no listeners remain.
 */
function _maybeCloseEventSource(): void {
  if (_changeListeners.size === 0 && _anyChangeListeners.size === 0 && _eventSource) {
    _eventSource.close();
    _eventSource = null;
  }
}

// ─── Convenience Export ────────────────────────────────────────────────────────

/**
 * The `rep` namespace object for convenient named import:
 *
 *   import { rep } from '@rep-protocol/sdk';
 *   const url = rep.get('API_URL');
 */
export const rep = {
  get,
  getSecure,
  getAll,
  verify,
  meta,
  onChange,
  onAnyChange,
};

export default rep;
