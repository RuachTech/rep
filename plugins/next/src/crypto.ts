import { createCipheriv, createHmac, createHash, randomBytes } from 'node:crypto';

export interface Keys {
  encryptionKey: Buffer;
  hmacSecret: Buffer;
}

/**
 * HKDF-SHA256 key derivation (RFC 5869), single-round.
 * Matches gateway/internal/crypto/crypto.go:DeriveKey exactly.
 *
 *   Extract: PRK = HMAC-SHA256(salt, ikm)
 *   Expand:  T(1) = HMAC-SHA256(PRK, info || 0x01)
 */
export function deriveKey(ikm: Buffer, salt: Buffer, info: string, length: number): Buffer {
  if (length > 32) {
    throw new Error('rep: deriveKey length exceeds one HKDF-SHA256 round (max 32)');
  }

  // Extract
  const prk = createHmac('sha256', salt).update(ikm).digest();

  // Expand
  const okm = createHmac('sha256', prk)
    .update(info)
    .update(Buffer.from([0x01]))
    .digest();

  return okm.subarray(0, length);
}

/**
 * Generate ephemeral keys matching Go's GenerateKeys().
 * Master key is HKDF-derived, then discarded.
 */
export function generateKeys(): Keys {
  const masterKey = randomBytes(32);
  const startupSalt = randomBytes(32);
  const encryptionKey = deriveKey(masterKey, startupSalt, 'rep-blob-encryption-v1', 32);
  const hmacSecret = randomBytes(32);

  return { encryptionKey, hmacSecret };
}

/**
 * Produce deterministic JSON with sorted keys, using Go-compatible HTML escaping.
 * Go's json.Marshal escapes <, >, & as \u003c, \u003e, \u0026.
 */
export function canonicalize(m: Record<string, string>): string {
  const sorted = Object.keys(m).sort();
  const obj: Record<string, string> = {};
  for (const k of sorted) {
    obj[k] = m[k];
  }
  return goEscapeJSON(JSON.stringify(obj));
}

/**
 * Apply Go's json.Marshal HTML escaping to a JSON string.
 * Replaces <, >, & with their unicode escape sequences.
 */
export function goEscapeJSON(json: string): string {
  return json
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026');
}

/**
 * HMAC-SHA256 integrity token.
 * message = canonicalize(public) + "|" + sensitiveBlob
 * Returns "hmac-sha256:<base64>"
 */
export function computeIntegrity(
  publicMap: Record<string, string>,
  sensitiveBlob: string,
  hmacKey: Buffer,
): string {
  const canonical = canonicalize(publicMap);
  const message = canonical + '|' + sensitiveBlob;
  const sig = createHmac('sha256', hmacKey).update(message).digest('base64');
  return 'hmac-sha256:' + sig;
}

/**
 * AES-256-GCM encryption matching Go's EncryptSensitive.
 * Output: base64([nonce 12B][ciphertext][authTag 16B])
 * AAD = integrityToken string.
 *
 * The sensitive map is serialized with sorted keys and Go HTML escaping
 * to match Go's json.Marshal output.
 */
export function encryptSensitive(
  sensitiveMap: Record<string, string>,
  key: Buffer,
  integrityToken: string,
): string {
  if (Object.keys(sensitiveMap).length === 0) {
    return '';
  }

  // Serialize with sorted keys + Go escaping to match Go's json.Marshal
  const plaintext = Buffer.from(goEscapeJSON(JSON.stringify(
    Object.fromEntries(Object.keys(sensitiveMap).sort().map(k => [k, sensitiveMap[k]])),
  )));

  const nonce = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  cipher.setAAD(Buffer.from(integrityToken));

  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const authTag = cipher.getAuthTag();

  // Format: [nonce][ciphertext][authTag] — matches Go's gcm.Seal(nonce, nonce, plaintext, aad)
  const blob = Buffer.concat([nonce, encrypted, authTag]);
  return blob.toString('base64');
}

/**
 * SHA-256 SRI hash. Returns "sha256-<base64>".
 */
export function computeSRI(jsonBytes: Buffer): string {
  const hash = createHash('sha256').update(jsonBytes).digest('base64');
  return 'sha256-' + hash;
}
