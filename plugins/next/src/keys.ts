import { generateKeys, type Keys } from './crypto.js';

/**
 * Module-scoped singleton for ephemeral cryptographic keys.
 *
 * In Next.js dev, the server is a long-running Node.js process, so keys
 * persist across requests within the same server lifecycle. New keys are
 * generated on server restart — matching the gateway's behavior.
 *
 * RepScript and the session-key route handler both import this module,
 * ensuring they share the same keys (encryption key used for the blob
 * matches the key returned by /rep/session-key).
 */
let _keys: Keys | null = null;

export function getOrCreateKeys(): Keys {
  if (!_keys) {
    _keys = generateKeys();
  }
  return _keys;
}

/** @internal For testing only. */
export function _resetKeys(): void {
  _keys = null;
}
