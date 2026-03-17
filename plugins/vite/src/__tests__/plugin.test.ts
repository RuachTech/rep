import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { repPlugin } from '../index.js';

const testDir = join(tmpdir(), 'rep-vite-plugin-test-' + process.pid);

beforeEach(() => {
  mkdirSync(testDir, { recursive: true });
  // Clear REP_ vars
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('REP_') && !key.startsWith('REP_GATEWAY_')) {
      delete process.env[key];
    }
  }
});

afterEach(() => {
  rmSync(testDir, { recursive: true, force: true });
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('REP_') && !key.startsWith('REP_GATEWAY_')) {
      delete process.env[key];
    }
  }
});

describe('repPlugin', () => {
  it('returns a plugin with correct name', () => {
    const plugin = repPlugin();
    expect(plugin.name).toBe('rep-protocol');
  });

  it('has config hook that detects build mode', () => {
    const plugin = repPlugin();
    // Simulate build command
    (plugin.config as Function)({}, { command: 'build' });

    // In build mode, transformIndexHtml should return html unchanged
    const handler = typeof plugin.transformIndexHtml === 'object'
      ? (plugin.transformIndexHtml as any).handler
      : plugin.transformIndexHtml;

    const html = '<html><head></head><body></body></html>';
    const result = handler(html);
    expect(result).toBe(html);
  });

  it('configureServer sets up middleware and payload', () => {
    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API_URL=https://api.test.com\n');

    const plugin = repPlugin({ env: '.env.local' });

    const middlewares: Function[] = [];
    const watchedPaths: string[] = [];
    const mockServer = {
      config: {
        root: testDir,
        logger: {
          info: () => {},
          warn: () => {},
          error: () => {},
        },
      },
      watcher: {
        add: (p: string) => watchedPaths.push(p),
        on: () => {},
      },
      ws: { send: () => {} },
      middlewares: {
        use: (fn: Function) => middlewares.push(fn),
      },
    };

    (plugin.configureServer as Function)(mockServer);

    // Should have registered session-key middleware
    expect(middlewares).toHaveLength(1);

    // Should watch the env file
    expect(watchedPaths).toContain(join(testDir, '.env.local'));
  });

  it('session-key middleware returns encryption key', () => {
    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_SENSITIVE_TOKEN=secret\n');

    const plugin = repPlugin({ env: '.env.local' });

    let sessionKeyHandler: Function | undefined;
    const mockServer = {
      config: {
        root: testDir,
        logger: { info: () => {}, warn: () => {}, error: () => {} },
      },
      watcher: { add: () => {}, on: () => {} },
      ws: { send: () => {} },
      middlewares: {
        use: (fn: Function) => { sessionKeyHandler = fn; },
      },
    };

    (plugin.configureServer as Function)(mockServer);

    // Simulate GET /rep/session-key
    const headers: Record<string, string> = {};
    let body = '';
    const req = { url: '/rep/session-key', method: 'GET' };
    const res = {
      setHeader: (k: string, v: string) => { headers[k] = v; },
      end: (b: string) => { body = b; },
    };

    sessionKeyHandler!(req, res, () => {});

    const parsed = JSON.parse(body);
    expect(parsed.key).toBeDefined();
    expect(typeof parsed.key).toBe('string');
    expect(parsed.expires_at).toBeDefined();
    expect(headers['Cache-Control']).toBe('no-store');
    expect(headers['Content-Type']).toBe('application/json');
  });

  it('session-key middleware calls next() for other paths', () => {
    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, '');

    const plugin = repPlugin({ env: '.env.local' });

    let handler: Function | undefined;
    const mockServer = {
      config: {
        root: testDir,
        logger: { info: () => {}, warn: () => {}, error: () => {} },
      },
      watcher: { add: () => {}, on: () => {} },
      ws: { send: () => {} },
      middlewares: {
        use: (fn: Function) => { handler = fn; },
      },
    };

    (plugin.configureServer as Function)(mockServer);

    let nextCalled = false;
    handler!({ url: '/other', method: 'GET' }, {}, () => { nextCalled = true; });
    expect(nextCalled).toBe(true);
  });

  it('transformIndexHtml injects script tag in dev mode', () => {
    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API_URL=https://api.test.com\n');

    const plugin = repPlugin({ env: '.env.local' });

    // Simulate dev command
    (plugin.config as Function)({}, { command: 'serve' });

    // Setup server to initialize state
    const mockServer = {
      config: {
        root: testDir,
        logger: { info: () => {}, warn: () => {}, error: () => {} },
      },
      watcher: { add: () => {}, on: () => {} },
      ws: { send: () => {} },
      middlewares: { use: () => {} },
    };
    (plugin.configureServer as Function)(mockServer);

    const handler = typeof plugin.transformIndexHtml === 'object'
      ? (plugin.transformIndexHtml as any).handler
      : plugin.transformIndexHtml;

    const result = handler('<html><head></head><body></body></html>');

    // Should return HtmlTagDescriptor array
    expect(Array.isArray(result)).toBe(true);
    expect(result).toHaveLength(1);
    expect(result[0].tag).toBe('script');
    expect(result[0].attrs.id).toBe('__rep__');
    expect(result[0].attrs.type).toBe('application/json');
    expect(result[0].attrs['data-rep-integrity']).toMatch(/^sha256-/);
    expect(result[0].children).toContain('"API_URL"');
    expect(result[0].injectTo).toBe('head');
  });

  it('strict mode throws on guardrail warnings', () => {
    const envPath = join(testDir, '.env.local');
    writeFileSync(envPath, 'REP_PUBLIC_API_KEY=sk_live_abcdef1234567890abcdef\n');

    const plugin = repPlugin({ env: '.env.local', strict: true });

    const mockServer = {
      config: {
        root: testDir,
        logger: { info: () => {}, warn: () => {}, error: () => {} },
      },
      watcher: { add: () => {}, on: () => {} },
      ws: { send: () => {} },
      middlewares: { use: () => {} },
    };

    expect(() => (plugin.configureServer as Function)(mockServer)).toThrow(/guardrail/i);
  });
});
