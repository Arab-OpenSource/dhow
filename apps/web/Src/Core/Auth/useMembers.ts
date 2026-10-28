import { keepPreviousData, useQuery, useQueryClient, type QueryClient, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { ApiClient, requestPage, type IPage } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TMember = components['schemas']['Member'];

export const MEMBERS_QUERY_KEY = ['members'] as const;

interface IMembersSeed {
  Data: IPage<TMember>;
  UpdatedAt: number;
}

export const fetchMembers = (organizationId: string, page: number, limit: number): Promise<IPage<TMember>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/members', { params: { path: { id: organizationId }, query: { Page: page, Limit: limit } } }),
  );

export const coveringPage = (cached: IPage<TMember>, page: number, limit: number): IPage<TMember> | undefined => {
  const from = (cached.Meta.Page - 1) * cached.Meta.Limit;
  const start = (page - 1) * limit;
  const end = Math.min(start + limit, cached.Meta.Total);
  if (start < from || end > from + cached.Items.length || (start >= end && start > 0)) return undefined;
  return { Items: cached.Items.slice(start - from, end - from), Meta: { Page: page, Limit: limit, Total: cached.Meta.Total } };
};

const seedOf = (client: QueryClient, organizationId: string | undefined, page: number, limit: number): IMembersSeed | undefined => {
  if (organizationId === undefined) return undefined;
  for (const query of client.getQueryCache().findAll({ queryKey: [...MEMBERS_QUERY_KEY, organizationId] })) {
    const cached = query.state.data as IPage<TMember> | undefined;
    const Data = cached === undefined || query.state.isInvalidated ? undefined : coveringPage(cached, page, limit);
    if (Data !== undefined) return { Data, UpdatedAt: query.state.dataUpdatedAt };
  }
  return undefined;
};

export const useMembers = (
  organizationId: string | undefined,
  page: number,
  limit: number,
): UseQueryResult<IPage<TMember>, ApiError> => {
  const client = useQueryClient();
  return useQuery<IPage<TMember>, ApiError>({
    queryKey: [...MEMBERS_QUERY_KEY, organizationId, page, limit],
    queryFn: () => fetchMembers(organizationId ?? '', page, limit),
    enabled: organizationId !== undefined,
    placeholderData: keepPreviousData,
    initialData: () => seedOf(client, organizationId, page, limit)?.Data,
    initialDataUpdatedAt: () => seedOf(client, organizationId, page, limit)?.UpdatedAt,
  });
};
