import { describe, it, expect } from 'vitest';
import { transformVite } from '../transforms/vite';
import { transformCRA } from '../transforms/cra';
import { transformNext } from '../transforms/next';

const FILE = 'test.tsx';

// ─── transformVite ────────────────────────────────────────────────────────────

describe('transformVite', () => {
  it('transforms dot-notation VITE_ access', () => {
    const src = `const url = import.meta.env.VITE_API_URL;`;
    const out = transformVite(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
    expect(out).not.toContain('import.meta.env.VITE_API_URL');
  });

  it('transforms bracket-notation VITE_ access', () => {
    const src = `const url = import.meta.env['VITE_API_URL'];`;
    const out = transformVite(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
  });

  it('strips prefix correctly — VITE_API_URL becomes API_URL', () => {
    const src = `const x = import.meta.env.VITE_MY_FEATURE;`;
    const out = transformVite(src, FILE)!;
    expect(out).toContain(`rep.get('MY_FEATURE')`);
  });

  it('does not transform non-VITE_ env vars', () => {
    const src = `const mode = import.meta.env.MODE;`;
    const out = transformVite(src, FILE);
    expect(out).toBeNull();
  });

  it('does not transform import.meta.env.DEV or PROD', () => {
    const src = `const dev = import.meta.env.DEV; const prod = import.meta.env.PROD;`;
    const out = transformVite(src, FILE);
    expect(out).toBeNull();
  });

  it('adds SDK import when absent', () => {
    const src = `const x = import.meta.env.VITE_KEY;`;
    const out = transformVite(src, FILE)!;
    expect(out).toContain(`from '@rep-protocol/sdk'`);
    expect(out).toContain('rep');
  });

  it('does not duplicate SDK import when already present', () => {
    const src = [
      `import { rep } from '@rep-protocol/sdk';`,
      `const x = import.meta.env.VITE_KEY;`,
    ].join('\n');
    const out = transformVite(src, FILE)!;
    const count = (out.match(/@rep-protocol\/sdk/g) ?? []).length;
    expect(count).toBe(1);
  });

  it('is idempotent — returns null on second run', () => {
    const src = `const x = import.meta.env.VITE_KEY;`;
    const first = transformVite(src, FILE)!;
    const second = transformVite(first, FILE);
    expect(second).toBeNull();
  });

  it('transforms multiple VITE_ vars in one file', () => {
    const src = [
      `const a = import.meta.env.VITE_API_URL;`,
      `const b = import.meta.env.VITE_APP_NAME;`,
    ].join('\n');
    const out = transformVite(src, FILE)!;
    expect(out).toContain(`rep.get('API_URL')`);
    expect(out).toContain(`rep.get('APP_NAME')`);
  });

  it('handles TypeScript files (.ts extension)', () => {
    const src = `const x: string = import.meta.env.VITE_KEY ?? '';`;
    const out = transformVite(src, 'config.ts');
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('KEY')`);
  });
});

// ─── transformCRA ─────────────────────────────────────────────────────────────

describe('transformCRA', () => {
  it('transforms REACT_APP_ access', () => {
    const src = `const url = process.env.REACT_APP_API_URL;`;
    const out = transformCRA(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
    expect(out).not.toContain('process.env.REACT_APP_API_URL');
  });

  it('transforms bracket-notation REACT_APP_ access', () => {
    const src = `const url = process.env['REACT_APP_API_URL'];`;
    const out = transformCRA(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
  });

  it('does not transform non-REACT_APP_ env vars', () => {
    const src = `const env = process.env.NODE_ENV;`;
    const out = transformCRA(src, FILE);
    expect(out).toBeNull();
  });

  it('adds SDK import when absent', () => {
    const src = `const x = process.env.REACT_APP_KEY;`;
    const out = transformCRA(src, FILE)!;
    expect(out).toContain(`from '@rep-protocol/sdk'`);
  });

  it('is idempotent — returns null on second run', () => {
    const src = `const x = process.env.REACT_APP_KEY;`;
    const first = transformCRA(src, FILE)!;
    const second = transformCRA(first, FILE);
    expect(second).toBeNull();
  });
});

// ─── transformNext ────────────────────────────────────────────────────────────

describe('transformNext', () => {
  it('transforms NEXT_PUBLIC_ access', () => {
    const src = `const url = process.env.NEXT_PUBLIC_API_URL;`;
    const out = transformNext(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
    expect(out).not.toContain('process.env.NEXT_PUBLIC_API_URL');
  });

  it('transforms bracket-notation NEXT_PUBLIC_ access', () => {
    const src = `const url = process.env['NEXT_PUBLIC_API_URL'];`;
    const out = transformNext(src, FILE);
    expect(out).not.toBeNull();
    expect(out).toContain(`rep.get('API_URL')`);
  });

  it('does not transform non-NEXT_PUBLIC_ env vars', () => {
    const src = `const env = process.env.NODE_ENV;`;
    const out = transformNext(src, FILE);
    expect(out).toBeNull();
  });

  it('adds SDK import when absent', () => {
    const src = `const x = process.env.NEXT_PUBLIC_KEY;`;
    const out = transformNext(src, FILE)!;
    expect(out).toContain(`from '@rep-protocol/sdk'`);
  });

  it('is idempotent — returns null on second run', () => {
    const src = `const x = process.env.NEXT_PUBLIC_KEY;`;
    const first = transformNext(src, FILE)!;
    const second = transformNext(first, FILE);
    expect(second).toBeNull();
  });

  it('does not transform REACT_APP_ vars (wrong framework)', () => {
    const src = `const x = process.env.REACT_APP_KEY;`;
    const out = transformNext(src, FILE);
    expect(out).toBeNull();
  });
});
