import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useCallback } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import type { TOrganization } from '@/Core/Auth/AuthTypes';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { MEMBERS_QUERY_KEY } from '@/Core/Auth/useMembers';
import {
  getOrganization,
  getTeam,
  listInvitations,
  listTeams,
  renameOrganization,
  type TInvitation,
  type TTeam,
  type TTeamDetail,
} from '@/Modules/Organization/Api/OrganizationApi';

export const ALL_MEMBERS = 100;

export const organizationKey = (organizationId: string) => ['organization', organizationId] as const;
export const teamsKey = (organizationId: string) => ['teams', organizationId] as const;
export const teamKey = (organizationId: string, teamId: string) => ['team', organizationId, teamId] as const;
export const invitationsKey = (organizationId: string) => ['invitations', organizationId] as const;

export const useOrganization = (organizationId: string): UseQueryResult<TOrganization, ApiError> =>
  useQuery<TOrganization, ApiError>({
    queryKey: organizationKey(organizationId),
    queryFn: () => getOrganization(organizationId),
  });

export const useTeams = (organizationId: string): UseQueryResult<TTeam[], ApiError> =>
  useQuery<TTeam[], ApiError>({ queryKey: teamsKey(organizationId), queryFn: () => listTeams(organizationId) });

export const useTeam = (organizationId: string, teamId: string): UseQueryResult<TTeamDetail, ApiError> =>
  useQuery<TTeamDetail, ApiError>({
    queryKey: teamKey(organizationId, teamId),
    queryFn: () => getTeam(organizationId, teamId),
  });

export const useInvitations = (organizationId: string, enabled: boolean): UseQueryResult<TInvitation[], ApiError> =>
  useQuery<TInvitation[], ApiError>({
    queryKey: invitationsKey(organizationId),
    queryFn: () => listInvitations(organizationId),
    enabled,
  });

export const useRefreshOrganization = (organizationId: string): ((settledTeamId?: string) => Promise<void>) => {
  const client = useQueryClient();
  return useCallback(
    async (settledTeamId?: string) => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ME_QUERY_KEY }),
        client.invalidateQueries({ queryKey: [...MEMBERS_QUERY_KEY, organizationId] }),
        client.invalidateQueries({ queryKey: teamsKey(organizationId) }),
        client.invalidateQueries({ queryKey: ['team', organizationId], predicate: (query) => query.queryKey[2] !== settledTeamId }),
        client.invalidateQueries({ queryKey: invitationsKey(organizationId) }),
      ]);
    },
    [client, organizationId],
  );
};

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
