import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import {
  listSessions,
  revokeOtherSessions,
  revokeSession,
  type TSessionView,
} from '@/Modules/Session/Api/SessionApi';

export const SESSIONS_QUERY_KEY = ['sessions'] as const;

export const useSessions = (): UseQueryResult<TSessionView[], ApiError> =>
  useQuery<TSessionView[], ApiError>({ queryKey: SESSIONS_QUERY_KEY, queryFn: listSessions });

export const useRevokeSession = () => {
  const client = useQueryClient();
  return useMutation<boolean, ApiError, string>({
    mutationFn: revokeSession,
    onSettled: () => client.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY }),
  });
};

export const useRevokeOtherSessions = () => {
  const client = useQueryClient();
  return useMutation<number, ApiError>({
    mutationFn: revokeOtherSessions,
    onSettled: () => client.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY }),
  });
};
