import { keepPreviousData, useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { IPage } from '@/Core/Api/Client';
import { cancelJob, getJob, listJobs, type TJob, type TJobDetail, type TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { isFinalState } from '@/Modules/Jobs/Types/JobStates';

export const JOBS_PAGE_SIZE = 20;
export const WAITING_LIMIT = 100;
export const LIST_REFRESH_MS = 10_000;
export const DETAIL_REFRESH_MS = 5_000;
export const JOBS_QUERY_KEY = ['jobs'] as const;

export const jobKey = (organizationId: string, jobId: string) => [...JOBS_QUERY_KEY, 'detail', organizationId, jobId] as const;

const anyActive = (page: IPage<TJob> | undefined): boolean => page?.Items.some((job) => !isFinalState(job.State)) ?? false;

export const useJobs = (
  organizationId: string,
  page: number,
  state: TJobState | null,
): UseQueryResult<IPage<TJob>, ApiError> =>
  useQuery<IPage<TJob>, ApiError>({
    queryKey: [...JOBS_QUERY_KEY, 'list', organizationId, page, state],
    queryFn: () => listJobs(organizationId, page, JOBS_PAGE_SIZE, state),
    placeholderData: keepPreviousData,
    refetchInterval: (query) => (anyActive(query.state.data) ? LIST_REFRESH_MS : false),
  });

export const useWaitingJobs = (organizationId: string): UseQueryResult<IPage<TJob>, ApiError> =>
  useQuery<IPage<TJob>, ApiError>({
    queryKey: [...JOBS_QUERY_KEY, 'waiting', organizationId],
    queryFn: () => listJobs(organizationId, 1, WAITING_LIMIT, 'queued'),
    refetchInterval: (query) => (anyActive(query.state.data) ? LIST_REFRESH_MS : false),
  });

export const useJob = (organizationId: string, jobId: string): UseQueryResult<TJobDetail, ApiError> =>
  useQuery<TJobDetail, ApiError>({
    queryKey: jobKey(organizationId, jobId),
    queryFn: () => getJob(organizationId, jobId),
    refetchInterval: (query) => {
      const job = query.state.data?.Job;
      return job !== undefined && !isFinalState(job.State) ? DETAIL_REFRESH_MS : false;
    },
  });

export const useCancelJob = (organizationId: string, jobId: string) => {
  const client = useQueryClient();
  return useMutation<TJob, ApiError>({
    mutationFn: () => cancelJob(organizationId, jobId),
    onSuccess: (job) => {
      client.setQueryData<TJobDetail>(jobKey(organizationId, jobId), (detail) => (detail ? { ...detail, Job: job } : detail));
      void client.invalidateQueries({ queryKey: JOBS_QUERY_KEY });
    },
  });
};
