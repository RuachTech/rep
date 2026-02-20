/**
 * @rep-protocol/svelte — Svelte stores for the Runtime Environment Protocol.
 *
 * Provides `repStore()` and `repSecureStore()` which wrap the SDK into
 * Svelte-native readable stores. Subscribers receive hot-reload updates
 * automatically; the SSE connection is closed when all subscribers unsub.
 *
 * @see https://github.com/ruachtech/rep — REP-RFC-0001 §5.5
 * @license Apache-2.0
 * @version 0.1.0
 */

import { readable } from 'svelte/store';
import type { Readable } from 'svelte/store';
import { get, getSecure, onChange } from '@rep-protocol/sdk';

/**
 * A readable store for a PUBLIC tier variable. Synchronous initial value.
 *
 * The store updates whenever the variable changes via hot reload.
 * The underlying SSE connection is cleaned up when all subscribers unsubscribe.
 *
 * @param key          Variable name (without `REP_PUBLIC_` prefix).
 * @param defaultValue Fallback value if the variable is not present.
 *
 * @example
 *   const apiUrl = repStore('API_URL', 'https://api.example.com');
 *   $: console.log($apiUrl);
 */
export function repStore(
  key: string,
  defaultValue?: string,
): Readable<string | undefined> {
  return readable<string | undefined>(get(key, defaultValue as string), (set) => {
    const unsubscribe = onChange(key, (newValue) => {
      set(newValue !== '' ? newValue : defaultValue);
    });
    return unsubscribe;
  });
}

/**
 * A readable store for a SENSITIVE tier variable. Starts as `null`, resolves async.
 *
 * Fetches the session key on first subscription and decrypts the value.
 * The decrypted value is cached by the SDK for the page lifetime.
 *
 * @param key Variable name (without `REP_SENSITIVE_` prefix).
 *
 * @example
 *   const analyticsKey = repSecureStore('ANALYTICS_KEY');
 *   $: console.log($analyticsKey); // null initially, then the decrypted value
 */
export function repSecureStore(key: string): Readable<string | null> {
  return readable<string | null>(null, (set) => {
    getSecure(key)
      .then((v) => set(v))
      .catch(() => {
        // Errors are logged by the SDK; store remains null
      });

    // No event-based updates for sensitive vars (re-encryption requires page reload)
    return () => {};
  });
}
