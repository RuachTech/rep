/**
 * @rep-protocol/vue — Vue composables for the Runtime Environment Protocol.
 *
 * Provides reactive `useRep()` and `useRepSecure()` composables that
 * subscribe to hot-reload updates and update refs automatically.
 *
 * Both composables must be called from within a component's `setup()` context
 * so that `onUnmounted` can clean up the hot-reload subscription.
 *
 * @see https://github.com/ruachtech/rep — REP-RFC-0001 §5.5
 * @license Apache-2.0
 * @version 0.1.0
 */

import { ref, onUnmounted } from 'vue';
import type { Ref } from 'vue';
import { get, getSecure, onChange } from '@rep-protocol/sdk';

/**
 * Read a PUBLIC tier variable as a reactive ref. Synchronous initial value.
 *
 * Automatically updates when the variable changes via hot reload.
 * Cleans up the SSE subscription in `onUnmounted`.
 *
 * @param key          Variable name (without `REP_PUBLIC_` prefix).
 * @param defaultValue Fallback value if the variable is not present.
 */
export function useRep(key: string, defaultValue?: string): Ref<string | undefined> {
  const value = ref<string | undefined>(get(key, defaultValue as string));

  const unsubscribe = onChange(key, (newValue) => {
    value.value = newValue !== '' ? newValue : defaultValue;
  });

  onUnmounted(unsubscribe);

  return value;
}

/**
 * Read a SENSITIVE tier variable as a reactive ref. Resolves asynchronously.
 *
 * The ref starts as `null` and is set once the session key fetch completes.
 * The decrypted value is cached for the page lifetime by the SDK.
 *
 * @param key Variable name (without `REP_SENSITIVE_` prefix).
 */
export function useRepSecure(key: string): Ref<string | null> {
  const value = ref<string | null>(null);

  getSecure(key)
    .then((v) => {
      value.value = v;
    })
    .catch(() => {
      // Errors are logged by the SDK; leave value as null
    });

  return value;
}
