import { describe, it, expect } from 'vitest';
import { createDecipheriv, createHmac, createHash } from 'node:crypto';
import {
  deriveKey,
  generateKeys,
  canonicalize,
  goEscapeJSON,
  computeIntegrity,
  encryptSensitive,
  computeSRI,
} from '../crypto.js';

describe('deriveKey', () => {
  it('produces deterministic output for same inputs', () => {
    const ikm = Buffer.alloc(32, 0xaa);
    const salt = Buffer.alloc(32, 0xbb);
    const a = deriveKey(ikm, salt, 'rep-blob-encryption-v1', 32);
    const b = deriveKey(ikm, salt, 'rep-blob-encryption-v1', 32);
    expect(a).toEqual(b);
  });

  it('produces different output for different info strings', () => {
    const ikm = Buffer.alloc(32, 0xaa);
    const salt = Buffer.alloc(32, 0xbb);
    const a = deriveKey(ikm, salt, 'rep-blob-encryption-v1', 32);
    const b = deriveKey(ikm, salt, 'different-info', 32);
    expect(a).not.toEqual(b);
  });

  it('throws for length > 32', () => {
    expect(() => deriveKey(Buffer.alloc(32), Buffer.alloc(32), 'test', 33)).toThrow();
  });

  it('matches manual HKDF-SHA256 computation', () => {
    const ikm = Buffer.from('test-master-key-material-here!!!' );
    const salt = Buffer.from('test-salt-value-here-32-bytes!!!' );
    const info = 'rep-blob-encryption-v1';

    // Manual HKDF
    const prk = createHmac('sha256', salt).update(ikm).digest();
    const okm = createHmac('sha256', prk)
      .update(info)
      .update(Buffer.from([0x01]))
      .digest();

    const result = deriveKey(ikm, salt, info, 32);
    expect(result).toEqual(okm);
  });
});

describe('generateKeys', () => {
  it('returns 32-byte keys', () => {
    const keys = generateKeys();
    expect(keys.encryptionKey).toHaveLength(32);
    expect(keys.hmacSecret).toHaveLength(32);
  });

  it('generates different keys on each call', () => {
    const a = generateKeys();
    const b = generateKeys();
    expect(a.encryptionKey).not.toEqual(b.encryptionKey);
    expect(a.hmacSecret).not.toEqual(b.hmacSecret);
  });
});

describe('canonicalize', () => {
  it('sorts keys alphabetically', () => {
    const result = canonicalize({ Z: '1', A: '2', M: '3' });
    expect(result).toBe('{"A":"2","M":"3","Z":"1"}');
  });

  it('produces empty object for empty map', () => {
    expect(canonicalize({})).toBe('{}');
  });

  it('applies Go HTML escaping', () => {
    const result = canonicalize({ KEY: '<script>&test</script>' });
    expect(result).toContain('\\u003c');
    expect(result).toContain('\\u003e');
    expect(result).toContain('\\u0026');
    expect(result).not.toContain('<');
    expect(result).not.toContain('>');
    expect(result).not.toContain('&');
  });
});

describe('goEscapeJSON', () => {
  it('escapes HTML characters like Go json.Marshal', () => {
    expect(goEscapeJSON('"<script>"')).toBe('"\\u003cscript\\u003e"');
    expect(goEscapeJSON('"a&b"')).toBe('"a\\u0026b"');
  });

  it('leaves normal strings unchanged', () => {
    expect(goEscapeJSON('"hello"')).toBe('"hello"');
  });
});

describe('computeIntegrity', () => {
  it('returns hmac-sha256: prefixed string', () => {
    const keys = generateKeys();
    const result = computeIntegrity({ API_URL: 'https://api.example.com' }, '', keys.hmacSecret);
    expect(result).toMatch(/^hmac-sha256:[A-Za-z0-9+/]+=*$/);
  });

  it('is deterministic for same inputs', () => {
    const hmacKey = Buffer.alloc(32, 0xcc);
    const pub = { A: '1', B: '2' };
    const a = computeIntegrity(pub, '', hmacKey);
    const b = computeIntegrity(pub, '', hmacKey);
    expect(a).toBe(b);
  });

  it('differs when public vars change', () => {
    const hmacKey = Buffer.alloc(32, 0xcc);
    const a = computeIntegrity({ A: '1' }, '', hmacKey);
    const b = computeIntegrity({ A: '2' }, '', hmacKey);
    expect(a).not.toBe(b);
  });

  it('matches manual HMAC computation', () => {
    const hmacKey = Buffer.alloc(32, 0xdd);
    const pub = { B: '2', A: '1' };
    const canonical = '{"A":"1","B":"2"}'; // sorted
    const message = canonical + '|';
    const expected = 'hmac-sha256:' + createHmac('sha256', hmacKey).update(message).digest('base64');
    expect(computeIntegrity(pub, '', hmacKey)).toBe(expected);
  });
});

describe('encryptSensitive', () => {
  it('returns empty string for empty map', () => {
    const keys = generateKeys();
    expect(encryptSensitive({}, keys.encryptionKey, 'token')).toBe('');
  });

  it('returns base64-encoded blob', () => {
    const keys = generateKeys();
    const blob = encryptSensitive({ SECRET: 'value' }, keys.encryptionKey, 'token');
    expect(blob).toMatch(/^[A-Za-z0-9+/]+=*$/);
  });

  it('can be decrypted with the same key and AAD', () => {
    const keys = generateKeys();
    const integrity = 'hmac-sha256:test';
    const original = { KEY: 'secret-value', OTHER: 'also-secret' };
    const blob = encryptSensitive(original, keys.encryptionKey, integrity);

    // Decrypt
    const data = Buffer.from(blob, 'base64');
    const nonce = data.subarray(0, 12);
    const authTag = data.subarray(data.length - 16);
    const ciphertext = data.subarray(12, data.length - 16);

    const decipher = createDecipheriv('aes-256-gcm', keys.encryptionKey, nonce);
    decipher.setAuthTag(authTag);
    decipher.setAAD(Buffer.from(integrity));

    const plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
    const decrypted = JSON.parse(plaintext.toString());

    expect(decrypted).toEqual({ KEY: 'secret-value', OTHER: 'also-secret' });
  });

  it('produces different blobs each call (random nonce)', () => {
    const keys = generateKeys();
    const a = encryptSensitive({ K: 'v' }, keys.encryptionKey, 'tok');
    const b = encryptSensitive({ K: 'v' }, keys.encryptionKey, 'tok');
    expect(a).not.toBe(b);
  });
});

describe('computeSRI', () => {
  it('returns sha256- prefixed hash', () => {
    const result = computeSRI(Buffer.from('test'));
    expect(result).toMatch(/^sha256-[A-Za-z0-9+/]+=*$/);
  });

  it('matches manual SHA-256', () => {
    const data = Buffer.from('{"public":{},"_meta":{}}');
    const expected = 'sha256-' + createHash('sha256').update(data).digest('base64');
    expect(computeSRI(data)).toBe(expected);
  });
});
