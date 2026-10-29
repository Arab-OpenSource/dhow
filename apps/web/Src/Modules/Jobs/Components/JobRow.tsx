import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Row } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TJob } from '@/Modules/Jobs/Api/JobsApi';
import { JobStateDot } from '@/Modules/Jobs/Components/JobStateDot';
import { useJobText } from '@/Modules/Jobs/Hooks/useJobText';
import { isStuck } from '@/Modules/Jobs/Types/JobStates';

export interface IJobRowProps {
  job: TJob;
  now: number;
  onOpen: (jobId: string) => void;
}

const useTiming = (job: TJob, now: number): string => {
  const t = useT();
  const text = useJobText();
  if (job.State === 'queued') return t('jobs.row.waited', { duration: text.duration(job.QueuedAt, now) });
  if (job.State === 'leased' || job.State === 'running') {
    return t('jobs.row.running', { duration: text.duration(job.StartedAt ?? job.QueuedAt, now) });
  }
  if (job.StartedAt !== undefined && job.FinishedAt !== undefined) {
    return t('jobs.row.took', { duration: text.duration(job.StartedAt, job.FinishedAt) });
  }
  return '';
};

export const JobRow = ({ job, now, onOpen }: IJobRowProps) => {
  const t = useT();
  const time = useRelativeTime();
  const text = useJobText();
  const timing = useTiming(job, now);
  const parts = [
    t('jobs.row.created', { time: time.ago(job.CreatedAt) }),
    timing,
    job.AttemptCount > 0 ? t('jobs.row.attempts', { count: job.AttemptCount }) : t('jobs.row.noAttempts'),
  ].filter((part) => part !== '');
  const reason = job.FailureReason === undefined ? null : text.reason(job.FailureReason);

  return (
    <Row
      icon="play"
      title={<bdi>{job.Name}</bdi>}
      sub={
        <>
          <span title={time.full(job.CreatedAt)}>{parts.join(' · ')}</span>
          {reason === null ? null : <span className="block text-secondary">{reason}</span>}
        </>
      }
      end={
        <span className="flex flex-wrap items-center justify-end gap-2">
          {isStuck(job) ? (
            <Badge tone="warn" icon="alert">
              {t('jobs.stuck.badge')}
            </Badge>
          ) : null}
          <JobStateDot job={job} />
        </span>
      }
      onSelect={() => {
        onOpen(job.Id);
      }}
    />
  );
};
