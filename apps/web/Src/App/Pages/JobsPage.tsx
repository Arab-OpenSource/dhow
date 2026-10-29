import { useNavigate, useSearch } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { jobsSearchOf, jobStatesOf } from '@/App/Navigation/JobsSearch';
import { useT } from '@/I18n/useT';
import { JobsBody } from '@/Modules/Jobs/Components/JobsBody';

export const JobsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { state } = useSearch({ strict: false });
  const filter = jobStatesOf(state);

  return (
    <Page title={t('jobs.title')} lead={t('jobs.subtitle')}>
      <JobsBody
        key={state ?? 'all'}
        filter={filter}
        onFilterChange={(next) => {
          void navigate({ to: '/jobs', search: jobsSearchOf(next), replace: true });
        }}
        onOpenJob={(jobId) => {
          void navigate({ to: '/jobs/$jobId', params: { jobId } });
        }}
      />
    </Page>
  );
};
