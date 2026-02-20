import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest';
import { get as readStore } from 'svelte/store';

// Mock @rep-protocol/sdk before importing the adapter
vi.mock('@rep-protocol/sdk', () => ({
  get: vi.fn(),
  getSecure: vi.fn(),
  onChange: vi.fn(),
}));

import { get, getSecure, onChange } from '@rep-protocol/sdk';
import { repStore, repSecureStore } from '../index';

const mockGet = get as Mock;
const mockGetSecure = getSecure as Mock;
const mockOnChange = onChange as Mock;

beforeEach(() => {
  vi.clearAllMocks();
  mockOnChange.mockReturnValue(() => {});
});

// ─── repStore ─────────────────────────────────────────────────────────────────

describe('repStore', () => {
  it('initial store value comes from get()', () => {
    mockGet.mockReturnValue('https://api.example.com');
    const store = repStore('API_URL');
    expect(readStore(store)).toBe('https://api.example.com');
  });

  it('initial value is undefined when get() returns undefined', () => {
    mockGet.mockReturnValue(undefined);
    const store = repStore('MISSING');
    expect(readStore(store)).toBeUndefined();
  });

  it('uses defaultValue as initial value when get() returns undefined', () => {
    mockGet.mockImplementation((_key: string, def?: string) => def);
    const store = repStore('MISSING', 'fallback');
    expect(readStore(store)).toBe('fallback');
  });

  it('subscribes to onChange when first subscriber attaches', () => {
    mockGet.mockReturnValue('v1');
    const store = repStore('API_URL');

    // onChange is called lazily — only when subscribe is called
    expect(mockOnChange).not.toHaveBeenCalled();

    const unsub = store.subscribe(() => {});
    expect(mockOnChange).toHaveBeenCalledWith('API_URL', expect.any(Function));
    unsub();
  });

  it('updates store value when onChange callback fires', () => {
    mockGet.mockReturnValue('v1');

    let capturedCb: ((newVal: string) => void) | undefined;
    mockOnChange.mockImplementation((_key: string, cb: (v: string) => void) => {
      capturedCb = cb;
      return () => {};
    });

    const store = repStore('API_URL');
    const values: (string | undefined)[] = [];
    const unsub = store.subscribe((v) => values.push(v));

    expect(values).toEqual(['v1']);

    capturedCb?.('v2');
    expect(values).toEqual(['v1', 'v2']);

    unsub();
  });

  it('calls the onChange unsubscribe when last subscriber unsubscribes', () => {
    const unsubscribe = vi.fn();
    mockGet.mockReturnValue('v1');
    mockOnChange.mockReturnValue(unsubscribe);

    const store = repStore('API_URL');
    const unsub = store.subscribe(() => {});

    expect(unsubscribe).not.toHaveBeenCalled();
    unsub();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it('delivers current value synchronously to each subscriber', () => {
    mockGet.mockReturnValue('val');
    const store = repStore('KEY');

    let received: string | undefined;
    store.subscribe((v) => {
      received = v;
    })();

    expect(received).toBe('val');
  });
});

// ─── repSecureStore ───────────────────────────────────────────────────────────

describe('repSecureStore', () => {
  it('starts with null', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {})); // never resolves
    const store = repSecureStore('ANALYTICS_KEY');
    expect(readStore(store)).toBeNull();
  });

  it('sets the store value after getSecure resolves', async () => {
    mockGetSecure.mockResolvedValue('secret-value');
    const store = repSecureStore('ANALYTICS_KEY');

    const values: (string | null)[] = [];
    const unsub = store.subscribe((v) => values.push(v));

    // Wait for the microtask to settle
    await vi.waitFor(() => expect(values).toContain('secret-value'));

    expect(values[0]).toBeNull(); // initial null
    expect(values[values.length - 1]).toBe('secret-value');
    unsub();
  });

  it('stays null when getSecure rejects', async () => {
    mockGetSecure.mockRejectedValue(new Error('Session key failed'));
    const store = repSecureStore('ANALYTICS_KEY');

    const values: (string | null)[] = [];
    const unsub = store.subscribe((v) => values.push(v));

    // Give the rejection time to settle
    await new Promise((r) => setTimeout(r, 10));
    expect(values).toEqual([null]); // never updated beyond initial null
    unsub();
  });

  it('calls getSecure with the correct key on first subscription', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {}));
    const store = repSecureStore('MY_KEY');

    expect(mockGetSecure).not.toHaveBeenCalled(); // lazy

    const unsub = store.subscribe(() => {});
    expect(mockGetSecure).toHaveBeenCalledWith('MY_KEY');
    unsub();
  });

  it('unsubscribe function can be called without error', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {}));
    const store = repSecureStore('KEY');
    const unsub = store.subscribe(() => {});
    expect(() => unsub()).not.toThrow();
  });
});
