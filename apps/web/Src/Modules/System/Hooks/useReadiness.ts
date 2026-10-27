import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { getReadiness } from '@/Modules/System/Api/SystemApi';
import type { THealthStatus } from '@/Modules/System/Types/SystemTypes';

export const READINESS_QUERY_KEY = ['system', 'readiness'] as const;

export const useReadiness = (enabled = true): UseQueryResult<THealthStatus, ApiError> =>
  useQuery<THealthStatus, ApiError>({
    queryKey: READINESS_QUERY_KEY,
    queryFn: getReadiness,
    enabled,
  });
