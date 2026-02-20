import { describe, it, expect, beforeEach, vi, type Mock } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';

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

beforeEach(() => {
  vi.clearAllMocks();
  // Default: onChange returns a no-op unsubscribe
  mockOnChange.mockReturnValue(() => {});
});

// ─── useRep ──────────────────────────────────────────────────────────────────

describe('useRep', () => {
  it('returns the value from get() on initial render', () => {
    mockGet.mockReturnValue('https://api.example.com');
    const { result } = renderHook(() => useRep('API_URL'));
    expect(result.current).toBe('https://api.example.com');
  });

  it('returns undefined when get() returns undefined', () => {
    mockGet.mockReturnValue(undefined);
    const { result } = renderHook(() => useRep('MISSING'));
    expect(result.current).toBeUndefined();
  });

  it('returns the defaultValue when provided and get() returns undefined', () => {
    mockGet.mockImplementation((_key: string, def?: string) => def);
    const { result } = renderHook(() => useRep('MISSING', 'fallback'));
    expect(result.current).toBe('fallback');
  });

  it('subscribes to onChange on mount', () => {
    mockGet.mockReturnValue('v1');
    renderHook(() => useRep('API_URL'));
    expect(mockOnChange).toHaveBeenCalledWith('API_URL', expect.any(Function));
  });

  it('updates value when onChange callback fires', async () => {
    mockGet.mockReturnValue('v1');

    let capturedCallback: ((newVal: string) => void) | undefined;
    mockOnChange.mockImplementation((_key: string, cb: (v: string) => void) => {
      capturedCallback = cb;
      return () => {};
    });

    const { result } = renderHook(() => useRep('API_URL'));
    expect(result.current).toBe('v1');

    act(() => {
      capturedCallback?.('v2');
    });

    expect(result.current).toBe('v2');
  });

  it('calls unsubscribe on unmount', () => {
    const unsubscribe = vi.fn();
    mockGet.mockReturnValue('v1');
    mockOnChange.mockReturnValue(unsubscribe);

    const { unmount } = renderHook(() => useRep('API_URL'));
    expect(unsubscribe).not.toHaveBeenCalled();
    unmount();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it('calls get() with the key on each render', () => {
    mockGet.mockReturnValue('val');
    renderHook(() => useRep('MY_KEY'));
    expect(mockGet).toHaveBeenCalledWith('MY_KEY', undefined);
  });
});

// ─── useRepSecure ─────────────────────────────────────────────────────────────

describe('useRepSecure', () => {
  it('starts with loading=true and value=null', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {})); // never resolves
    const { result } = renderHook(() => useRepSecure('ANALYTICS_KEY'));
    expect(result.current.loading).toBe(true);
    expect(result.current.value).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('sets value and loading=false on successful resolution', async () => {
    mockGetSecure.mockResolvedValue('secret-value');
    const { result } = renderHook(() => useRepSecure('ANALYTICS_KEY'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.value).toBe('secret-value');
    expect(result.current.error).toBeNull();
  });

  it('sets error and loading=false on rejection', async () => {
    const err = new Error('Session key fetch failed');
    mockGetSecure.mockRejectedValue(err);
    const { result } = renderHook(() => useRepSecure('ANALYTICS_KEY'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.value).toBeNull();
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('Session key fetch failed');
  });

  it('wraps non-Error rejections in an Error', async () => {
    mockGetSecure.mockRejectedValue('plain string error');
    const { result } = renderHook(() => useRepSecure('KEY'));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe('plain string error');
  });

  it('calls getSecure with the correct key', () => {
    mockGetSecure.mockReturnValue(new Promise(() => {}));
    renderHook(() => useRepSecure('MY_KEY'));
    expect(mockGetSecure).toHaveBeenCalledWith('MY_KEY');
  });

  it('re-fetches when key changes', async () => {
    mockGetSecure.mockResolvedValue('value');
    const { rerender } = renderHook(({ key }: { key: string }) => useRepSecure(key), {
      initialProps: { key: 'KEY_A' },
    });

    await waitFor(() => expect(mockGetSecure).toHaveBeenCalledWith('KEY_A'));

    rerender({ key: 'KEY_B' });
    await waitFor(() => expect(mockGetSecure).toHaveBeenCalledWith('KEY_B'));
  });
});
