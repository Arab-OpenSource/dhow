import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { TOrganization } from '@/Core/Auth/AuthTypes';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { getOrganization, renameOrganization } from '@/Modules/Identity/Api/IdentityApi';

export const organizationKey = (organizationId: string) => ['organization', organizationId] as const;

export const useOrganization = (organizationId: string): UseQueryResult<TOrganization, ApiError> =>
  useQuery<TOrganization, ApiError>({
    queryKey: organizationKey(organizationId),
    queryFn: () => getOrganization(organizationId),
    staleTime: 0,
  });

export const useRenameOrganization = (organizationId: string) => {
  const client = useQueryClient();
  return useMutation<TOrganization, ApiError, string>({
    mutationFn: (name) => renameOrganization(organizationId, name),
    onSuccess: (organization) => {
      client.setQueryData(organizationKey(organizationId), organization);
      void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
    },
  });
};
