import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import type { TJob, TJobAttemptState } from '@/Modules/Jobs/Api/JobsApi';
import { attemptStatusOf, statusOf } from '@/Modules/Jobs/Types/JobStates';

export const JobStateDot = ({ job }: { job: Pick<TJob, 'State' | 'QueuedReason'> }) => {
  const t = useT();
  return <StatusDot status={statusOf(job)}>{t(`jobs.state.${job.State}`)}</StatusDot>;
};

export const AttemptStateDot = ({ state }: { state: TJobAttemptState }) => {
  const t = useT();
  return <StatusDot status={attemptStatusOf(state)}>{t(`jobs.state.${state}`)}</StatusDot>;
};
