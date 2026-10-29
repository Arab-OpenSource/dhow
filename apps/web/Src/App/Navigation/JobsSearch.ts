import type { TJobState } from '@/Modules/Jobs/Api/JobsApi';
import { JOB_STATES } from '@/Modules/Jobs/Types/JobStates';

export interface IJobsSearch {
  state?: string;
}

const SEPARATOR = ',';

export const jobStatesOf = (value: unknown): TJobState[] => {
  if (typeof value !== 'string') return [];
  const wanted = new Set(value.split(SEPARATOR).map((part) => part.trim()));
  return JOB_STATES.filter((state) => wanted.has(state));
};

export const jobsSearchOf = (states: readonly TJobState[]): IJobsSearch => {
  const known = jobStatesOf(states.join(SEPARATOR));
  return known.length === 0 ? {} : { state: known.join(SEPARATOR) };
};

export const parseJobsSearch = (search: Record<string, unknown>): IJobsSearch => jobsSearchOf(jobStatesOf(search.state));
