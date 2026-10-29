import { keepPreviousData, useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { IPage } from '@/Core/Api/Client';
import {
  getRunner,
  listRegistrationTokens,
  listRunners,
  registrationTokensKey,
  revokeRegistrationToken,
  revokeRunner,
  runnerKey,
  runnerListKey,
  RUNNERS_SUMMARY_LIMIT,
  updateRunner,
  type TRegistrationToken,
  type TRunner,
  type TUpdateRunner,
} from '@/Modules/Runners/Api/RunnersApi';

export const useRunners = (organizationId: string, page: number): UseQueryResult<IPage<TRunner>, ApiError> =>
  useQuery<IPage<TRunner>, ApiError>({
    queryKey: [...runnerListKey(organizationId), page],
    queryFn: () => listRunners(organizationId, page),
    placeholderData: keepPreviousData,
  });

export const useRunnerSummary = (organizationId: string): UseQueryResult<IPage<TRunner>, ApiError> =>
  useQuery<IPage<TRunner>, ApiError>({
    queryKey: [...runnerListKey(organizationId), 'summary'],
    queryFn: () => listRunners(organizationId, 1, RUNNERS_SUMMARY_LIMIT),
  });

export const useRunner = (organizationId: string, runnerId: string): UseQueryResult<TRunner, ApiError> => {
  const client = useQueryClient();
  return useQuery<TRunner, ApiError>({
    queryKey: runnerKey(organizationId, runnerId),
    queryFn: () => getRunner(organizationId, runnerId),
    initialData: () =>
      client
        .getQueriesData<IPage<TRunner>>({ queryKey: runnerListKey(organizationId) })
        .flatMap(([, page]) => page?.Items ?? [])
        .find((runner) => runner.Id === runnerId),
    initialDataUpdatedAt: 0,
  });
};

export const useRegistrationTokens = (
  organizationId: string,
  page: number,
): UseQueryResult<IPage<TRegistrationToken>, ApiError> =>
  useQuery<IPage<TRegistrationToken>, ApiError>({
    queryKey: [...registrationTokensKey(organizationId), page],
    queryFn: () => listRegistrationTokens(organizationId, page),
    placeholderData: keepPreviousData,
  });

const useRunnerSaved = (organizationId: string) => {
  const client = useQueryClient();
  return (runner: TRunner) => {
    client.setQueryData(runnerKey(organizationId, runner.Id), runner);
    void client.invalidateQueries({ queryKey: runnerListKey(organizationId) });
  };
};

export const useUpdateRunner = (organizationId: string, runnerId: string): UseMutationResult<TRunner, ApiError, TUpdateRunner> => {
  const saved = useRunnerSaved(organizationId);
  return useMutation<TRunner, ApiError, TUpdateRunner>({
    mutationFn: (body) => updateRunner(organizationId, runnerId, body),
    onSuccess: saved,
  });
};

export const useRevokeRunner = (organizationId: string, runnerId: string) => {
  const saved = useRunnerSaved(organizationId);
  return useMutation<TRunner, ApiError>({
    mutationFn: () => revokeRunner(organizationId, runnerId),
    onSuccess: saved,
  });
};

export const useRevokeRegistrationToken = (organizationId: string): UseMutationResult<TRegistrationToken, ApiError, string> => {
  const client = useQueryClient();
  return useMutation<TRegistrationToken, ApiError, string>({
    mutationFn: (tokenId) => revokeRegistrationToken(organizationId, tokenId),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: registrationTokensKey(organizationId) });
    },
  });
};
