import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { getOverview, type TOverview } from '@/Modules/Overview/Api/OverviewApi';

export const OVERVIEW_REFRESH_MS = 15_000;
export const OVERVIEW_QUERY_KEY = ['overview'] as const;

export const overviewKey = (organizationId: string | undefined) => [...OVERVIEW_QUERY_KEY, organizationId] as const;

export const useOverview = (organizationId: string | undefined): UseQueryResult<TOverview, ApiError> =>
  useQuery<TOverview, ApiError>({
    queryKey: overviewKey(organizationId),
    queryFn: () => getOverview(organizationId ?? ''),
    enabled: organizationId !== undefined,
    refetchInterval: OVERVIEW_REFRESH_MS,
  });
