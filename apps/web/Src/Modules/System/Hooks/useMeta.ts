import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { getMeta } from '@/Modules/System/Api/SystemApi';
import type { TMeta } from '@/Modules/System/Types/SystemTypes';

export const META_QUERY_KEY = ['system', 'meta'] as const;
const RECOVERY_INTERVAL_MS = 10_000;

export const useMeta = (): UseQueryResult<TMeta, ApiError> =>
  useQuery<TMeta, ApiError>({
    queryKey: META_QUERY_KEY,
    queryFn: getMeta,
    refetchInterval: (query) => (query.state.status === 'error' ? RECOVERY_INTERVAL_MS : false),
  });
