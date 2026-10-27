import { QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';

import { createQueryClient } from '@/Core/Api/QueryClient';
import { envelope, META_BODY, READY_BODY, stubFetch } from '@/Core/Testing/Fetch';
import { useMeta } from '@/Modules/System/Hooks/useMeta';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={createQueryClient({ retry: false })}>{children}</QueryClientProvider>
);

describe('useMeta', () => {
  it('loads the version, the commit and whether setup is pending', async () => {
    stubFetch(() => envelope(META_BODY));
    const { result } = renderHook(() => useMeta(), { wrapper });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual({ Version: 'v0.1.0', Commit: '2e39188', SetupRequired: false });
  });

  it('exposes a typed ApiError when the server is down', async () => {
    stubFetch(() => {
      throw new TypeError('Failed to fetch');
    });
    const { result } = renderHook(() => useMeta(), { wrapper });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error?.Code).toBe('NETWORK');
  });
});

describe('useReadiness', () => {
  it('loads the checks', async () => {
    stubFetch(() => envelope(READY_BODY));
    const { result } = renderHook(() => useReadiness(), { wrapper });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.Checks).toHaveLength(2);
  });

  it('keeps the failing checks when the server answers 503', async () => {
    const body = {
      Success: false,
      Error: { Code: 'SERVICE_UNAVAILABLE', Message: 'x' },
      Data: { Status: 'unavailable', Checks: [{ Name: 'database', Ok: false }] },
    };
    stubFetch(() => envelope(body, 503));
    const { result } = renderHook(() => useReadiness(), { wrapper });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.Status).toBe('unavailable');
    expect(result.current.data?.Checks[0]).toEqual({ Name: 'database', Ok: false });
  });

  it('stays idle until it is enabled', () => {
    const { calls } = stubFetch(() => envelope(READY_BODY));
    const { result } = renderHook(() => useReadiness(false), { wrapper });
    expect(result.current.fetchStatus).toBe('idle');
    expect(calls).toHaveLength(0);
  });
});
