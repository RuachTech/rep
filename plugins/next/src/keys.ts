import { generateKeys, type Keys } from './crypto.js';

/**
 * Process-wide singleton for ephemeral cryptographic keys.
 *
 * Uses globalThis so that the singleton survives even when bundlers (tsup,
 * Turbopack, webpack) duplicate the module across CJS entry points. This
 * is the standard Next.js singleton pattern (same approach Prisma uses).
 *
 * In Next.js dev, the server is a long-running Node.js process, so keys
 * persist across requests within the same server lifecycle. New keys are
 * generated on server restart — matching the gateway's behavior.
 *
 * RepScript and the session-key route handler both call getOrCreateKeys(),
 * ensuring they share the same keys (encryption key used for the blob
 * matches the key returned by /rep/session-key).
 */

const GLOBAL_KEY = Symbol.for('__rep_next_keys__');

function getGlobal(): { keys: Keys | null } {
  const g = globalThis as unknown as Record<symbol, { keys: Keys | null } | undefined>;
  if (!g[GLOBAL_KEY]) {
    g[GLOBAL_KEY] = { keys: null };
  }
  return g[GLOBAL_KEY]!;
}

export function getOrCreateKeys(): Keys {
  const store = getGlobal();
  if (!store.keys) {
    store.keys = generateKeys();
  }
  return store.keys;
}

/** @internal For testing only. */
export function _resetKeys(): void {
  const store = getGlobal();
  store.keys = null;
}
