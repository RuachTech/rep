import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest';

/**
 * Helper to inject a <script id="__rep__"> element into the DOM.
 */
function injectPayload(
  payload: Record<string, unknown>,
  integrity?: string
) {
  const el = document.createElement('script');
  el.id = '__rep__';
  el.type = 'application/json';
  el.textContent = JSON.stringify(payload);
  if (integrity) {
    el.setAttribute('data-rep-integrity', integrity);
  }
  document.head.appendChild(el);
}

function makePayload(
  publicVars: Record<string, string> = {},
  opts: { sensitive?: string; hotReload?: string; keyEndpoint?: string } = {}
) {
  return {
    public: publicVars,
    sensitive: opts.sensitive || undefined,
    _meta: {
      version: '0.1.0',
      injected_at: '2026-02-20T12:00:00.000Z',
      integrity: 'hmac-sha256:dGVzdA==',
      key_endpoint: opts.keyEndpoint || undefined,
      hot_reload: opts.hotReload || undefined,
      ttl: 0,
    },
  };
}

beforeEach(() => {
  document.head.innerHTML = '';
  document.body.innerHTML = '';
  vi.resetModules();
  vi.restoreAllMocks();
});

// ─── get() ──────────────────────────────────────────────────────────────────

describe('get()', () => {
  it('returns a public variable value', async () => {
    injectPayload(makePayload({ API_URL: 'https://api.example.com' }));
    const { get } = await import('../index');
    expect(get('API_URL')).toBe('https://api.example.com');
  });

  it('returns undefined for a missing key', async () => {
    injectPayload(makePayload({ API_URL: 'https://api.example.com' }));
    const { get } = await import('../index');
    expect(get('NONEXISTENT')).toBeUndefined();
  });

  it('returns defaultValue for a missing key', async () => {
    injectPayload(makePayload({ API_URL: 'https://api.example.com' }));
    const { get } = await import('../index');
    expect(get('NONEXISTENT', 'fallback')).toBe('fallback');
  });

  it('returns undefined when no payload exists', async () => {
    // No __rep__ element injected.
    const { get } = await import('../index');
    expect(get('ANYTHING')).toBeUndefined();
  });

  it('returns defaultValue when no payload exists', async () => {
    const { get } = await import('../index');
    expect(get('ANYTHING', 'default')).toBe('default');
  });

  it('is synchronous (does not return a Promise)', async () => {
    injectPayload(makePayload({ KEY: 'value' }));
    const { get } = await import('../index');
    const result = get('KEY');
    expect(result).not.toBeInstanceOf(Promise);
    expect(typeof result).toBe('string');
  });
});

// ─── getAll() ───────────────────────────────────────────────────────────────

describe('getAll()', () => {
  it('returns all public variables', async () => {
    injectPayload(makePayload({ A: '1', B: '2', C: '3' }));
    const { getAll } = await import('../index');
    const all = getAll();
    expect(all).toEqual({ A: '1', B: '2', C: '3' });
  });

  it('returns a frozen object', async () => {
    injectPayload(makePayload({ A: '1' }));
    const { getAll } = await import('../index');
    const all = getAll();
    expect(Object.isFrozen(all)).toBe(true);
  });

  it('returns empty frozen object when no payload', async () => {
    const { getAll } = await import('../index');
    const all = getAll();
    expect(Object.keys(all)).toHaveLength(0);
    expect(Object.isFrozen(all)).toBe(true);
  });
});

// ─── verify() ───────────────────────────────────────────────────────────────

describe('verify()', () => {
  it('returns true for valid payload', async () => {
    injectPayload(makePayload({ KEY: 'value' }));
    const { verify } = await import('../index');
    expect(verify()).toBe(true);
  });

  it('returns false when no payload exists', async () => {
    const { verify } = await import('../index');
    expect(verify()).toBe(false);
  });
});

// ─── meta() ─────────────────────────────────────────────────────────────────

describe('meta()', () => {
  it('returns correct metadata shape', async () => {
    injectPayload(
      makePayload({ A: '1', B: '2' }, { sensitive: 'blob', hotReload: '/rep/changes' })
    );
    const { meta } = await import('../index');
    const m = meta();

    expect(m).not.toBeNull();
    expect(m!.version).toBe('0.1.0');
    expect(m!.injectedAt).toBeInstanceOf(Date);
    expect(m!.publicCount).toBe(2);
    expect(m!.sensitiveAvailable).toBe(true);
    expect(m!.hotReloadAvailable).toBe(true);
    expect(typeof m!.integrityValid).toBe('boolean');
  });

  it('returns null when no payload', async () => {
    const { meta } = await import('../index');
    expect(meta()).toBeNull();
  });

  it('reports sensitiveAvailable=false when no sensitive blob', async () => {
    injectPayload(makePayload({ A: '1' }));
    const { meta } = await import('../index');
    expect(meta()!.sensitiveAvailable).toBe(false);
  });

  it('reports hotReloadAvailable=false when no hot_reload endpoint', async () => {
    injectPayload(makePayload({ A: '1' }));
    const { meta } = await import('../index');
    expect(meta()!.hotReloadAvailable).toBe(false);
  });
});

// ─── getSecure() ────────────────────────────────────────────────────────────

describe('getSecure()', () => {
  it('throws when no payload', async () => {
    const { getSecure } = await import('../index');
    await expect(getSecure('KEY')).rejects.toThrow('not available');
  });

  it('throws when no sensitive data', async () => {
    injectPayload(makePayload({ PUBLIC: 'val' }));
    const { getSecure } = await import('../index');
    await expect(getSecure('KEY')).rejects.toThrow('No SENSITIVE');
  });

  it('throws on fetch failure', async () => {
    injectPayload(
      makePayload({}, { sensitive: 'dGVzdA==', keyEndpoint: '/rep/session-key' })
    );
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
    });
    vi.stubGlobal('fetch', fetchMock);

    const { getSecure } = await import('../index');
    await expect(getSecure('KEY')).rejects.toThrow('500');
  });
});

// ─── onChange() ──────────────────────────────────────────────────────────────

describe('onChange()', () => {
  it('returns no-op when hot reload is not available', async () => {
    injectPayload(makePayload({ KEY: 'val' }));
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const { onChange } = await import('../index');
    const unsub = onChange('KEY', () => {});

    expect(typeof unsub).toBe('function');
    expect(warnSpy).toHaveBeenCalled();
    unsub(); // Should not throw.
  });

  it('establishes EventSource on first call when hot reload is available', async () => {
    const mockES = {
      addEventListener: vi.fn(),
      close: vi.fn(),
      set onerror(_fn: unknown) {},
    };
    vi.stubGlobal('EventSource', vi.fn(() => mockES));

    injectPayload(makePayload({ KEY: 'val' }, { hotReload: '/rep/changes' }));
    const { onChange } = await import('../index');

    onChange('KEY', () => {});

    expect(EventSource).toHaveBeenCalledWith('/rep/changes');
    expect(mockES.addEventListener).toHaveBeenCalledWith(
      'rep:config:update',
      expect.any(Function)
    );
    expect(mockES.addEventListener).toHaveBeenCalledWith(
      'rep:config:delete',
      expect.any(Function)
    );
  });

  it('unsubscribe closes EventSource when no listeners remain', async () => {
    const mockES = {
      addEventListener: vi.fn(),
      close: vi.fn(),
      set onerror(_fn: unknown) {},
    };
    vi.stubGlobal('EventSource', vi.fn(() => mockES));

    injectPayload(makePayload({ KEY: 'val' }, { hotReload: '/rep/changes' }));
    const { onChange } = await import('../index');

    const unsub = onChange('KEY', () => {});
    unsub();

    expect(mockES.close).toHaveBeenCalled();
  });
});

// ─── onAnyChange() ──────────────────────────────────────────────────────────

describe('onAnyChange()', () => {
  it('returns no-op when hot reload is not available', async () => {
    injectPayload(makePayload({ KEY: 'val' }));
    vi.spyOn(console, 'warn').mockImplementation(() => {});

    const { onAnyChange } = await import('../index');
    const unsub = onAnyChange(() => {});

    expect(typeof unsub).toBe('function');
    unsub();
  });
});

// ─── No network call on import ──────────────────────────────────────────────

describe('module import', () => {
  it('does not make any fetch calls during import', async () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);

    injectPayload(
      makePayload({ KEY: 'val' }, { sensitive: 'blob', keyEndpoint: '/rep/session-key' })
    );
    await import('../index');

    expect(fetchSpy).not.toHaveBeenCalled();
  });
});

// ─── Default export ─────────────────────────────────────────────────────────

describe('exports', () => {
  it('provides named exports and default namespace', async () => {
    injectPayload(makePayload({ KEY: 'val' }));
    const mod = await import('../index');

    expect(typeof mod.get).toBe('function');
    expect(typeof mod.getSecure).toBe('function');
    expect(typeof mod.getAll).toBe('function');
    expect(typeof mod.verify).toBe('function');
    expect(typeof mod.meta).toBe('function');
    expect(typeof mod.onChange).toBe('function');
    expect(typeof mod.onAnyChange).toBe('function');
    expect(typeof mod.rep).toBe('object');
    expect(mod.rep.get).toBe(mod.get);
    expect(mod.default.get).toBe(mod.get);
  });
});
