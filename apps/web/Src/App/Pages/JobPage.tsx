import { useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { JobBody } from '@/Modules/Jobs/Components/JobBody';

export const JobPage = () => {
  const t = useT();
  const { jobId } = useParams({ strict: false });
  if (!isUuid(jobId)) return <NotFoundPage />;

  return (
    <Page title={t('jobs.detail.title')} lead={t('jobs.detail.subtitle')}>
      <JobBody key={jobId} jobId={jobId} />
    </Page>
  );
};
