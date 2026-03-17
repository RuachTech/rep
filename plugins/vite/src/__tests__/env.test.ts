import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { readEnvFile, classifyVariables, readAndClassify } from '../env.js';

const testDir = join(tmpdir(), 'rep-vite-env-test-' + process.pid);

beforeEach(() => {
  mkdirSync(testDir, { recursive: true });
});

afterEach(() => {
  rmSync(testDir, { recursive: true, force: true });
});

describe('readEnvFile', () => {
  it('parses key=value pairs', () => {
    const path = join(testDir, '.env');
    writeFileSync(path, 'REP_PUBLIC_API_URL=https://api.example.com\nREP_SENSITIVE_KEY=secret\n');
    const result = readEnvFile(path);
    expect(result).toEqual({
      REP_PUBLIC_API_URL: 'https://api.example.com',
      REP_SENSITIVE_KEY: 'secret',
    });
  });

  it('returns empty object for missing file', () => {
    expect(readEnvFile(join(testDir, 'nonexistent'))).toEqual({});
  });

  it('handles quoted values', () => {
    const path = join(testDir, '.env');
    writeFileSync(path, 'KEY="quoted value"\nKEY2=\'single quoted\'\n');
    const result = readEnvFile(path);
    expect(result.KEY).toBe('quoted value');
    expect(result.KEY2).toBe('single quoted');
  });

  it('skips comments and empty lines', () => {
    const path = join(testDir, '.env');
    writeFileSync(path, '# comment\n\nKEY=value\n');
    const result = readEnvFile(path);
    expect(result).toEqual({ KEY: 'value' });
  });
});

describe('classifyVariables', () => {
  it('classifies PUBLIC, SENSITIVE, SERVER tiers', () => {
    const result = classifyVariables({
      REP_PUBLIC_API_URL: 'https://api.example.com',
      REP_SENSITIVE_TOKEN: 'secret',
      REP_SERVER_DB_PASS: 'hidden',
      UNRELATED: 'ignored',
    });

    expect(result.public).toEqual({ API_URL: 'https://api.example.com' });
    expect(result.sensitive).toEqual({ TOKEN: 'secret' });
    expect(result.server).toEqual({ DB_PASS: 'hidden' });
  });

  it('strips REP_<TIER>_ prefix', () => {
    const result = classifyVariables({
      REP_PUBLIC_MY_VAR: 'value',
    });
    expect(result.public).toEqual({ MY_VAR: 'value' });
  });

  it('ignores REP_GATEWAY_* variables', () => {
    const result = classifyVariables({
      REP_GATEWAY_PORT: '8080',
      REP_PUBLIC_API: 'url',
    });
    expect(result.public).toEqual({ API: 'url' });
  });

  it('ignores non-REP variables', () => {
    const result = classifyVariables({
      HOME: '/home/user',
      PATH: '/usr/bin',
    });
    expect(result.public).toEqual({});
    expect(result.sensitive).toEqual({});
    expect(result.server).toEqual({});
  });

  it('rejects name collisions across tiers', () => {
    expect(() =>
      classifyVariables({
        REP_PUBLIC_API_URL: 'https://api.example.com',
        REP_SENSITIVE_API_URL: 'secret-url',
      }),
    ).toThrow(/collision/i);
  });

  it('ignores bare REP_ prefix without tier', () => {
    const result = classifyVariables({
      REP_SOMETHING: 'value',
    });
    expect(result.public).toEqual({});
    expect(result.sensitive).toEqual({});
    expect(result.server).toEqual({});
  });
});

describe('readAndClassify', () => {
  const savedEnv: Record<string, string | undefined> = {};

  beforeEach(() => {
    // Save and clear REP_ vars from process.env
    for (const key of Object.keys(process.env)) {
      if (key.startsWith('REP_')) {
        savedEnv[key] = process.env[key];
        delete process.env[key];
      }
    }
  });

  afterEach(() => {
    // Clean up any REP_ vars we set
    for (const key of Object.keys(process.env)) {
      if (key.startsWith('REP_')) {
        delete process.env[key];
      }
    }
    // Restore original REP_ vars
    for (const [key, value] of Object.entries(savedEnv)) {
      if (value !== undefined) {
        process.env[key] = value;
      }
    }
  });

  it('reads from env file', () => {
    const path = join(testDir, '.env');
    writeFileSync(path, 'REP_PUBLIC_FROM_FILE=file-value\n');
    const result = readAndClassify(path);
    expect(result.public.FROM_FILE).toBe('file-value');
  });

  it('process.env overrides env file', () => {
    const path = join(testDir, '.env');
    writeFileSync(path, 'REP_PUBLIC_API=from-file\n');
    process.env.REP_PUBLIC_API = 'from-env';
    const result = readAndClassify(path);
    expect(result.public.API).toBe('from-env');
  });
});
