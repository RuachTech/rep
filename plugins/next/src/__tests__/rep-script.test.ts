import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { createHash, createDecipheriv, createHmac } from 'node:crypto';

const testDir = join(tmpdir(), 'rep-next-repscript-test-' + process.pid);
const savedEnv: Record<string, string | undefined> = {};

beforeEach(() => {
  mkdirSync(testDir, { recursive: true });
  // Save and clear REP_ vars
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('REP_')) {
      savedEnv[key] = process.env[key];
      delete process.env[key];
    }
  }
  // Reset module caches so keys singleton is fresh
  vi.resetModules();
});

afterEach(() => {
  rmSync(testDir, { recursive: true, force: true });
  // Restore original REP_ vars
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('REP_')) {
      delete process.env[key];
    }
  }
  for (const [key, value] of Object.entries(savedEnv)) {
    if (value !== undefined) {
      process.env[key] = value;
    }
  }
});

describe('RepScript', () => {
  it('returns null in production mode', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'production';
    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      expect(result).toBeNull();
    } finally {
      process.env.NODE_ENV = origNodeEnv;
    }
  });

  it('renders script element in development mode', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API_URL=https://api.test.com\n');

    // Stub process.cwd to point to testDir
    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });

      expect(result).not.toBeNull();
      expect(result!.type).toBe('script');
      expect(result!.props.id).toBe('__rep__');
      expect(result!.props.type).toBe('application/json');
      expect(result!.props['data-rep-integrity']).toMatch(/^sha256-/);

      // Verify the JSON payload contains the public var
      const json = result!.props.dangerouslySetInnerHTML.__html;
      const parsed = JSON.parse(json);
      expect(parsed.public.API_URL).toBe('https://api.test.com');
      expect(parsed._meta.integrity).toMatch(/^hmac-sha256:/);
      expect(parsed._meta.ttl).toBe(0);
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('omits sensitive blob and key_endpoint when no sensitive vars', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_APP_NAME=test\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;
      const parsed = JSON.parse(json);

      expect(parsed.sensitive).toBeUndefined();
      expect(parsed._meta.key_endpoint).toBeUndefined();
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('encrypts sensitive vars and includes key_endpoint', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API=url\nREP_SENSITIVE_TOKEN=secret123\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;
      const parsed = JSON.parse(json);

      expect(parsed.sensitive).toBeDefined();
      expect(typeof parsed.sensitive).toBe('string');
      expect(parsed._meta.key_endpoint).toBe('/rep/session-key');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('SRI hash matches SHA-256 of JSON content', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_KEY=value\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;
      const sri = result!.props['data-rep-integrity'];

      const expected = 'sha256-' + createHash('sha256').update(Buffer.from(json)).digest('base64');
      expect(sri).toBe(expected);
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('Go-escapes HTML characters in values to prevent XSS', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_XSS=</script><script>alert(1)</script>\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;

      // Must not contain raw < or > — all escaped to \u003c / \u003e
      expect(json).not.toContain('<');
      expect(json).not.toContain('>');
      expect(json).toContain('\\u003c');
      expect(json).toContain('\\u003e');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('strict mode throws on guardrail warnings', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API_KEY=sk_live_abcdef1234567890abcdef\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      expect(() => RepScript({ env: '.env.local', strict: true })).toThrow(/guardrail/i);
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('does not expose REP_SERVER_ vars in output', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API=url\nREP_SERVER_DB_PASSWORD=supersecret\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;

      expect(json).not.toContain('supersecret');
      expect(json).not.toContain('DB_PASSWORD');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      cwdSpy.mockRestore();
    }
  });

  it('does not expose non-REP env vars in output', async () => {
    const origNodeEnv = process.env.NODE_ENV;
    process.env.NODE_ENV = 'development';
    process.env.SECRET_THING = 'should-not-appear';

    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API=url\nDATABASE_URL=postgres://secret\n');

    const cwdSpy = vi.spyOn(process, 'cwd').mockReturnValue(testDir);

    try {
      const { RepScript } = await import('../index.js');
      const result = RepScript({ env: '.env.local' });
      const json = result!.props.dangerouslySetInnerHTML.__html;

      expect(json).not.toContain('should-not-appear');
      expect(json).not.toContain('SECRET_THING');
      expect(json).not.toContain('postgres://secret');
      expect(json).not.toContain('DATABASE_URL');
    } finally {
      process.env.NODE_ENV = origNodeEnv;
      delete process.env.SECRET_THING;
      cwdSpy.mockRestore();
    }
  });
});
