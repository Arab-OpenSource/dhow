import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useCallback } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { MFA_QUERY_KEY } from '@/Core/Auth/Factor';
import { listPasskeys, type TPasskey } from '@/Modules/Mfa/Api/MfaApi';

export const PASSKEYS_QUERY_KEY = ['passkeys'] as const;
const SESSIONS_QUERY_KEY = ['sessions'] as const;

export const usePasskeys = (): UseQueryResult<TPasskey[], ApiError> =>
  useQuery<TPasskey[], ApiError>({ queryKey: PASSKEYS_QUERY_KEY, queryFn: listPasskeys, staleTime: 0 });

export const useRefreshMfa = (): (() => Promise<void>) => {
  const client = useQueryClient();
  return useCallback(async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: MFA_QUERY_KEY }),
      client.invalidateQueries({ queryKey: PASSKEYS_QUERY_KEY }),
      client.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY }),
    ]);
  }, [client]);
};
