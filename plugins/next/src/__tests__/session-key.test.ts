import { describe, it, expect, beforeEach, vi } from 'vitest';

beforeEach(() => {
  vi.resetModules();
});

describe('GET /rep/session-key', () => {
  it('returns 404 in production', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'production';

    try {
      const { GET } = await import('../session-key.js');
      const response = GET();

      expect(response.status).toBe(404);
      const body = await response.json();
      expect(body.error).toContain('disabled in production');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
    }
  });

  it('returns encryption key in development', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    try {
      const { GET } = await import('../session-key.js');
      const response = GET();

      expect(response.status).toBe(200);

      const body = await response.json();
      expect(body.key).toBeDefined();
      expect(typeof body.key).toBe('string');
      expect(body.expires_at).toBeDefined();

      // Key should be valid base64 of 32 bytes
      const keyBuf = Buffer.from(body.key, 'base64');
      expect(keyBuf).toHaveLength(32);

      // Verify headers
      expect(response.headers.get('Cache-Control')).toBe('no-store');
      expect(response.headers.get('Content-Type')).toBe('application/json');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
    }
  });

  it('returns the same key across multiple calls (singleton)', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    try {
      const { GET } = await import('../session-key.js');
      const body1 = await GET().json();
      const body2 = await GET().json();

      expect(body1.key).toBe(body2.key);
    } finally {
      process.env.NODE_ENV = origNodeEnv;
    }
  });

  it('returns key that can decrypt RepScript sensitive blob', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    // Use dynamic imports so they share the same singleton keys
    const { writeFileSync, mkdirSync, rmSync } = await import('node:fs');
    const { join } = await import('node:path');
    const { tmpdir } = await import('node:os');
    const { createDecipheriv } = await import('node:crypto');

    const testDir = join(tmpdir(), 'rep-next-sk-decrypt-' + process.pid);
    mkdirSync(testDir, { recursive: true });

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API=url\nREP_SENSITIVE_SECRET=my-secret-value\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      // Import both from same module context so they share keys
      const { RepScript } = await import('../index.js');
      const { GET } = await import('../session-key.js');

      // Get the payload from RepScript
      const element = RepScript({ env: '.env.local' });
      const json = element!.props.dangerouslySetInnerHTML.__html;
      const parsed = JSON.parse(json);

      // Get the key from session-key endpoint
      const response = GET();
      const { key } = await response.json();
      const encKey = Buffer.from(key, 'base64');

      // Decrypt the sensitive blob
      const data = Buffer.from(parsed.sensitive, 'base64');
      const nonce = data.subarray(0, 12);
      const authTag = data.subarray(data.length - 16);
      const ciphertext = data.subarray(12, data.length - 16);

      const decipher = createDecipheriv('aes-256-gcm', encKey, nonce);
      decipher.setAuthTag(authTag);
      decipher.setAAD(Buffer.from(parsed._meta.integrity));

      const plaintext = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
      const decrypted = JSON.parse(plaintext.toString());

      expect(decrypted.SECRET).toBe('my-secret-value');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
      rmSync(testDir, { recursive: true, force: true });
    }
  });
});
