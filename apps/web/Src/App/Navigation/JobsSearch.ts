import type { TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { JOB_STATES } from '@/Modules/Jobs/Types/JobStates';

export interface IJobsSearch {
  state?: TJobState;
}

const isJobState = (value: unknown): value is TJobState => JOB_STATES.some((state) => state === value);

export const parseJobsSearch = (search: Record<string, unknown>): IJobsSearch =>
  isJobState(search.state) ? { state: search.state } : {};
