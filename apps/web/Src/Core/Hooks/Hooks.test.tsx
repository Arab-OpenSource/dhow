import { act, fireEvent, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useDelayedFlag } from '@/Core/Hooks/useDelayedFlag';
import { useDocumentDirection } from '@/Core/Hooks/useDocumentDirection';
import { useHotkey } from '@/Core/Hooks/useHotkey';

afterEach(() => {
  vi.useRealTimers();
});

describe('useDelayedFlag', () => {
  it('turns on only after the delay and off as soon as the work ends', () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ active }) => useDelayedFlag(active, 300), {
      initialProps: { active: true },
    });
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe(false);
    act(() => {
      vi.advanceTimersByTime(2);
    });
    expect(result.current).toBe(true);
    rerender({ active: false });
    expect(result.current).toBe(false);
  });

  it('never turns on when the work finishes inside the delay', () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ active }) => useDelayedFlag(active, 300), {
      initialProps: { active: true },
    });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    rerender({ active: false });
    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(result.current).toBe(false);
  });
});

describe('useHotkey', () => {
  it('fires on Ctrl and on Meta, ignores the bare key, and stops after unmount', () => {
    const handler = vi.fn();
    const { unmount } = renderHook(() => {
      useHotkey('k', handler);
    });
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true });
    fireEvent.keyDown(document, { key: 'K', metaKey: true });
    fireEvent.keyDown(document, { key: 'k' });
    expect(handler).toHaveBeenCalledTimes(2);
    unmount();
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true });
    expect(handler).toHaveBeenCalledTimes(2);
  });

  it('prevents the browser default so the browser does not take the shortcut', () => {
    renderHook(() => {
      useHotkey('k', vi.fn());
    });
    expect(fireEvent.keyDown(document, { key: 'k', ctrlKey: true })).toBe(false);
  });
});

describe('useDocumentDirection', () => {
  it('follows the dir attribute on the root element', async () => {
    document.documentElement.dir = 'rtl';
    const { result } = renderHook(() => useDocumentDirection());
    expect(result.current).toBe('rtl');
    await act(async () => {
      document.documentElement.dir = 'ltr';
      await Promise.resolve();
    });
    expect(result.current).toBe('ltr');
  });
});
