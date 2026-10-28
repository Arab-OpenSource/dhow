import { useMutation, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import type { TTeamChange } from '@/Modules/Organization/Api/OrganizationApi';
import { teamKey, useRefreshOrganization } from '@/Modules/Organization/Hooks/useOrganization';

export const useTeamChange = <TArgs>(organizationId: string, teamId: string, run: (args: TArgs) => Promise<TTeamChange>) => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  return useMutation<TTeamChange, ApiError, TArgs & { done: string }>({
    mutationFn: (args) => run(args),
    onSuccess: (change, args) => {
      client.setQueryData(teamKey(organizationId, teamId), change.Team);
      Push({
        Id: `team-${teamId}-${Date.now()}`,
        Accent: 'live',
        Title: args.done,
        Detail: change.Revoked > 0 ? t('team.revoked', { count: change.Revoked }) : undefined,
      });
      void refresh();
    },
    onError: (error) => {
      Push({ Id: `team-${teamId}-failed`, Accent: 'failed', Title: errorText(error) });
    },
  });
};
