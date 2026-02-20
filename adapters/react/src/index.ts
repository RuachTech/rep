/**
 * @rep-protocol/react — React hooks for the Runtime Environment Protocol.
 *
 * Provides synchronous `useRep()` and async `useRepSecure()` hooks that
 * subscribe to hot-reload updates and re-render on config changes.
 *
 * @see https://github.com/ruachtech/rep — REP-RFC-0001 §5.5
 * @license Apache-2.0
 * @version 0.1.0
 */

import { useState, useEffect } from 'react';
import { get, getSecure, onChange } from '@rep-protocol/sdk';

/**
 * Read a PUBLIC tier variable. Synchronous — no loading state, no Suspense.
 *
 * Automatically re-renders when the variable changes via hot reload.
 *
 * @param key          Variable name (without `REP_PUBLIC_` prefix).
 * @param defaultValue Fallback value if the variable is not present.
 */
export function useRep(key: string, defaultValue?: string): string | undefined {
  const [value, setValue] = useState<string | undefined>(() =>
    get(key, defaultValue as string),
  );

  useEffect(() => {
    // Sync to latest value in case it changed between render and effect
    setValue(get(key, defaultValue as string));

    // Subscribe to hot reload updates
    const unsubscribe = onChange(key, (newValue) => {
      setValue(newValue !== '' ? newValue : defaultValue);
    });

    return unsubscribe;
  }, [key, defaultValue]);

  return value;
}

/**
 * Read a SENSITIVE tier variable. Async — fetches a session key on first call.
 *
 * Returns `{ value, loading, error }`. The value is cached for the page lifetime.
 * Automatically re-renders when the variable changes via hot reload.
 *
 * @param key Variable name (without `REP_SENSITIVE_` prefix).
 */
export function useRepSecure(key: string): {
  value: string | null;
  loading: boolean;
  error: Error | null;
} {
  const [value, setValue] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;

    setLoading(true);
    setError(null);

    getSecure(key)
      .then((v) => {
        if (!cancelled) {
          setValue(v);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [key]);

  return { value, loading, error };
}
