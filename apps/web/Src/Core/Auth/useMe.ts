import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { TMe, TMembership } from '@/Core/Auth/AuthTypes';
import { fetchMe, ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';

const ME_STALE_MS = 60_000;

export const useMe = (): UseQueryResult<TMe, ApiError> =>
  useQuery<TMe, ApiError>({ queryKey: ME_QUERY_KEY, queryFn: fetchMe, staleTime: ME_STALE_MS });

export const currentMembership = (memberships: readonly TMembership[], currentId: string | null): TMembership | undefined =>
  memberships.find((membership) => membership.OrganizationId === currentId) ?? memberships[0];

export const useCurrentMembership = (): TMembership | undefined => {
  const memberships = useMe().data?.Memberships;
  const currentId = useOrganizationStore((state) => state.CurrentId);
  return memberships === undefined ? undefined : currentMembership(memberships, currentId);
};
