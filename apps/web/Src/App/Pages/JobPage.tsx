import { useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { JobBody } from '@/Modules/Jobs/Components/JobBody';
import { WhyWaitingCard } from '@/Modules/Runners/Components/WhyWaitingCard';

export const JobPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { jobId } = useParams({ strict: false });
  if (!isUuid(jobId)) return <NotFoundPage />;

  return (
    <Page title={t('jobs.detail.title')} lead={t('jobs.detail.subtitle')}>
      <JobBody
        key={jobId}
        jobId={jobId}
        waitingFor={(job) => (
          <WhyWaitingCard
            needs={{ Labels: job.Labels, Isolation: job.Isolation }}
            onAddRunner={() => {
              void navigate({ to: '/runners/tokens' });
            }}
            onOpenRunner={(runnerId) => {
              void navigate({ to: '/runners/$runnerId', params: { runnerId } });
            }}
          />
        )}
      />
    </Page>
  );
};
