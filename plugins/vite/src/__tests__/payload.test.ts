import { describe, it, expect } from 'vitest';
import { createHash, createHmac, createDecipheriv } from 'node:crypto';
import { buildPayload } from '../payload.js';
import { generateKeys, canonicalize, goEscapeJSON } from '../crypto.js';
import type { ClassifiedVars } from '../env.js';

describe('buildPayload', () => {
  it('produces valid JSON with correct structure', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { API_URL: 'https://api.example.com', FEATURE_FLAGS: 'dark-mode' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const parsed = JSON.parse(result.json);

    expect(parsed.public).toBeDefined();
    expect(parsed._meta).toBeDefined();
    expect(parsed._meta.version).toBe('0.1.0');
    expect(parsed._meta.integrity).toMatch(/^hmac-sha256:/);
    expect(parsed._meta.ttl).toBe(0);
    expect(parsed.sensitive).toBeUndefined();
    expect(parsed._meta.key_endpoint).toBeUndefined();
  });

  it('includes sensitive blob and key_endpoint when sensitive vars exist', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { API: 'url' },
      sensitive: { SECRET: 'value' },
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const parsed = JSON.parse(result.json);

    expect(parsed.sensitive).toBeDefined();
    expect(typeof parsed.sensitive).toBe('string');
    expect(parsed._meta.key_endpoint).toBe('/rep/session-key');
  });

  it('includes hot_reload endpoint when enabled', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { API: 'url' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0', hotReload: true });
    const parsed = JSON.parse(result.json);
    expect(parsed._meta.hot_reload).toBe('/rep/changes');
  });

  it('SRI hash matches SHA-256 of JSON bytes', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { KEY: 'value' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const expectedSRI = 'sha256-' + createHash('sha256').update(Buffer.from(result.json)).digest('base64');
    expect(result.sri).toBe(expectedSRI);
  });

  it('script tag has correct attributes', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { A: '1' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    expect(result.scriptTag).toContain('id="__rep__"');
    expect(result.scriptTag).toContain('type="application/json"');
    expect(result.scriptTag).toContain('data-rep-version="0.1.0"');
    expect(result.scriptTag).toContain(`data-rep-integrity="${result.sri}"`);
    expect(result.scriptTag).toContain(result.json);
  });

  it('public keys are sorted alphabetically in JSON', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { Z_KEY: 'z', A_KEY: 'a', M_KEY: 'm' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const publicStart = result.json.indexOf('"public":{') + '"public":'.length;
    const publicEnd = result.json.indexOf('}', publicStart) + 1;
    const publicStr = result.json.slice(publicStart, publicEnd);

    // Keys must appear in alphabetical order
    const aIdx = publicStr.indexOf('"A_KEY"');
    const mIdx = publicStr.indexOf('"M_KEY"');
    const zIdx = publicStr.indexOf('"Z_KEY"');
    expect(aIdx).toBeLessThan(mIdx);
    expect(mIdx).toBeLessThan(zIdx);
  });

  it('applies Go HTML escaping to values containing < > &', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { HTML: '<b>test</b>&more' },
      sensitive: {},
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    expect(result.json).toContain('\\u003c');
    expect(result.json).toContain('\\u003e');
    expect(result.json).toContain('\\u0026');
    expect(result.json).not.toMatch(/<b>/);
  });

  it('encrypted sensitive vars can be decrypted', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { API: 'url' },
      sensitive: { TOKEN: 'my-secret-token' },
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const parsed = JSON.parse(result.json);

    // Decrypt the sensitive blob
    const data = Buffer.from(parsed.sensitive, 'base64');
    const nonce = data.subarray(0, 12);
    const authTag = data.subarray(data.length - 16);
    const ciphertext = data.subarray(12, data.length - 16);

    const decipher = createDecipheriv('aes-256-gcm', keys.encryptionKey, nonce);
    decipher.setAuthTag(authTag);
    decipher.setAAD(Buffer.from(parsed._meta.integrity));

    const plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
    const decrypted = JSON.parse(plaintext.toString());

    expect(decrypted.TOKEN).toBe('my-secret-token');
  });

  it('integrity is computed over public vars with empty sensitive blob', () => {
    const keys = generateKeys();
    const classified: ClassifiedVars = {
      public: { A: '1' },
      sensitive: { SECRET: 'val' },
      server: {},
    };

    const result = buildPayload(classified, keys, { version: '0.1.0' });
    const parsed = JSON.parse(result.json);

    // Recompute: integrity = HMAC(canonicalize(public) + "|" + "")
    const canonical = canonicalize(classified.public);
    const message = canonical + '|';
    const expected = 'hmac-sha256:' + createHmac('sha256', keys.hmacSecret).update(message).digest('base64');

    expect(parsed._meta.integrity).toBe(expected);
  });
});
