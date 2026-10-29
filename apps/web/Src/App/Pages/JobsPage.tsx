import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { JobsBody } from '@/Modules/Jobs/Components/JobsBody';

export const JobsPage = () => {
  const t = useT();
  const navigate = useNavigate();

  return (
    <Page title={t('jobs.title')} lead={t('jobs.subtitle')}>
      <JobsBody
        onOpenJob={(jobId) => {
          void navigate({ to: '/jobs/$jobId', params: { jobId } });
        }}
      />
    </Page>
  );
};
