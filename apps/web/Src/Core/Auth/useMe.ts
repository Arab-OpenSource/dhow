import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { TMe, TMembership } from '@/Core/Auth/AuthTypes';
import { fetchMe, ME_QUERY_KEY } from '@/Core/Auth/MeQuery';

const ME_STALE_MS = 60_000;

export const useMe = (): UseQueryResult<TMe, ApiError> =>
  useQuery<TMe, ApiError>({ queryKey: ME_QUERY_KEY, queryFn: fetchMe, staleTime: ME_STALE_MS });

export const useCurrentMembership = (): TMembership | undefined => useMe().data?.Memberships[0];
