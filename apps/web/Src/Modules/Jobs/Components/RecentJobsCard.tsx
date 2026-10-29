import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Rows } from '@/Core/Ui/Row';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useT } from '@/I18n/useT';
import { JobRow } from '@/Modules/Jobs/Components/JobRow';
import { useRecentJobs } from '@/Modules/Jobs/Hooks/useJobs';
import { useNow } from '@/Modules/Jobs/Hooks/useNow';
import { isFinalState } from '@/Modules/Jobs/Types/JobStates';

export interface IRecentJobsCardProps {
  organizationId: string;
  onOpenJob: (jobId: string) => void;
  onShowAll: () => void;
  className?: string;
}

const NOW_TICK_MS = 15_000;

export const RecentJobsCard = ({ organizationId, onOpenJob, onShowAll, className }: IRecentJobsCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const jobs = useRecentJobs(organizationId);
  const items = jobs.data?.Items ?? [];
  const now = useNow(items.some((job) => !isFinalState(job.State)), NOW_TICK_MS);

  return (
    <Card
      title={t('home.jobs.title')}
      icon="play"
      className={className}
      end={
        items.length > 0 ? (
          <Button size="sm" variant="ghost" onClick={onShowAll}>
            {t('home.jobs.all')}
          </Button>
        ) : null
      }
    >
      <CardStates
        state={cardStateOf(jobs, items.length === 0)}
        loadingLabel={t('jobs.loading')}
        errorTitle={t('jobs.error.title')}
        errorText={jobs.error ? errorText(jobs.error) : undefined}
        retryLabel={t('jobs.retry')}
        onRetry={() => {
          void jobs.refetch();
        }}
        emptyIcon="play"
        emptyText={t('home.jobs.empty')}
        lines={5}
      >
        <Rows label={t('home.jobs.title')}>
          {items.map((job) => (
            <JobRow key={job.Id} job={job} now={now} compact onOpen={onOpenJob} />
          ))}
        </Rows>
      </CardStates>
    </Card>
  );
};
