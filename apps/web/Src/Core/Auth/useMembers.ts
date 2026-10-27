import { keepPreviousData, useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { ApiClient, requestPage, type IPage } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TMember = components['schemas']['Member'];

export const MEMBERS_QUERY_KEY = ['members'] as const;

export const fetchMembers = (organizationId: string, page: number, limit: number): Promise<IPage<TMember>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/members', { params: { path: { id: organizationId }, query: { Page: page, Limit: limit } } }),
  );

export const useMembers = (
  organizationId: string | undefined,
  page: number,
  limit: number,
): UseQueryResult<IPage<TMember>, ApiError> =>
  useQuery<IPage<TMember>, ApiError>({
    queryKey: [...MEMBERS_QUERY_KEY, organizationId, page, limit],
    queryFn: () => fetchMembers(organizationId ?? '', page, limit),
    enabled: organizationId !== undefined,
    placeholderData: keepPreviousData,
  });
