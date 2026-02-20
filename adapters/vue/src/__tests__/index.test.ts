import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest';
import { createApp, defineComponent, nextTick } from 'vue';
import type { Ref } from 'vue';

// Mock @rep-protocol/sdk before importing the adapter
vi.mock('@rep-protocol/sdk', () => ({
  get: vi.fn(),
  getSecure: vi.fn(),
  onChange: vi.fn(),
}));

import { get, getSecure, onChange } from '@rep-protocol/sdk';
import { useRep, useRepSecure } from '../index';

const mockGet = get as Mock;
const mockGetSecure = getSecure as Mock;
const mockOnChange = onChange as Mock;

/**
 * Run a composable inside a real Vue component's setup() context.
 * Returns the result and a cleanup function that unmounts the app.
 */
function withSetup<T>(composable: () => T): [T, () => void] {
  let result!: T;
  const component = defineComponent({
    setup() {
      result = composable();
      return {};
    },
    template: '<span/>',
  });
  const el = document.createElement('div');
  const app = createApp(component);
  app.config.warnHandler = () => {}; // silence expected warnings in tests
  app.mount(el);
  return [result, () => app.unmount()];
}

beforeEach(() => {
  vi.clearAllMocks();
  mockOnChange.mockReturnValue(() => {});
});

// ─── useRep ──────────────────────────────────────────────────────────────────

describe('useRep', () => {
  it('returns a ref with the initial value from get()', () => {
    mockGet.mockReturnValue('https://api.example.com');
    const [value, cleanup] = withSetup(() => useRep('API_URL'));
    expect(value.value).toBe('https://api.example.com');
    cleanup();
  });

  it('returns a ref with undefined when get() returns undefined', () => {
    mockGet.mockReturnValue(undefined);
    const [value, cleanup] = withSetup(() => useRep('MISSING'));
    expect(value.value).toBeUndefined();
    cleanup();
  });

  it('returns defaultValue when provided and variable is absent', () => {
    mockGet.mockImplementation((_key: string, def?: string) => def);
    const [value, cleanup] = withSetup(() => useRep('MISSING', 'fallback'));
    expect(value.value).toBe('fallback');
    cleanup();
  });

  it('subscribes to onChange with the correct key', () => {
    mockGet.mockReturnValue('v1');
    const [, cleanup] = withSetup(() => useRep('API_URL'));
    expect(mockOnChange).toHaveBeenCalledWith('API_URL', expect.any(Function));
    cleanup();
  });

  it('updates the ref when onChange callback fires', async () => {
    mockGet.mockReturnValue('v1');

    let capturedCb: ((newVal: string) => void) | undefined;
    mockOnChange.mockImplementation((_key: string, cb: (v: string) => void) => {
      capturedCb = cb;
      return () => {};
    });

    const [value, cleanup] = withSetup(() => useRep('API_URL'));
    expect(value.value).toBe('v1');

    capturedCb?.('v2');
    await nextTick();
    expect(value.value).toBe('v2');
    cleanup();
  });

  it('calls the onChange unsubscribe when component unmounts', () => {
    const unsubscribe = vi.fn();
    mockGet.mockReturnValue('v1');
    mockOnChange.mockReturnValue(unsubscribe);

    const [, cleanup] = withSetup(() => useRep('API_URL'));
    expect(unsubscribe).not.toHaveBeenCalled();
    cleanup(); // triggers onUnmounted
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it('calls get() with the key', () => {
    mockGet.mockReturnValue('val');
    const [, cleanup] = withSetup(() => useRep('MY_KEY'));
    expect(mockGet).toHaveBeenCalledWith('MY_KEY', undefined);
    cleanup();
  });
});

// ─── useRepSecure ─────────────────────────────────────────────────────────────

describe('useRepSecure', () => {
  it('starts with null', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {})); // never resolves
    const [value, cleanup] = withSetup(() => useRepSecure('ANALYTICS_KEY'));
    expect(value.value).toBeNull();
    cleanup();
  });

  it('sets the ref value after getSecure resolves', async () => {
    mockGetSecure.mockResolvedValue('secret-value');
    const [value, cleanup] = withSetup(() => useRepSecure('ANALYTICS_KEY'));

    await vi.waitFor(() => expect(value.value).toBe('secret-value'));
    cleanup();
  });

  it('leaves value as null when getSecure rejects', async () => {
    mockGetSecure.mockRejectedValue(new Error('Session key failed'));
    const [value, cleanup] = withSetup(() => useRepSecure('ANALYTICS_KEY'));

    // Give the rejected promise time to settle; value stays null
    await nextTick();
    await nextTick();
    expect(value.value).toBeNull();
    cleanup();
  });

  it('calls getSecure with the correct key', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {}));
    const [, cleanup] = withSetup(() => useRepSecure('MY_SENSITIVE_KEY'));
    expect(mockGetSecure).toHaveBeenCalledWith('MY_SENSITIVE_KEY');
    cleanup();
  });
});
