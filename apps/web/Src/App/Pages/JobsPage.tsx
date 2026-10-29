import { useNavigate, useSearch } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { JobsBody } from '@/Modules/Jobs/Components/JobsBody';

export const JobsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { state } = useSearch({ strict: false });
  const filter = state ?? null;

  return (
    <Page title={t('jobs.title')} lead={t('jobs.subtitle')}>
      <JobsBody
        key={filter ?? 'all'}
        filter={filter}
        onFilterChange={(next) => {
          void navigate({ to: '/jobs', search: next === null ? {} : { state: next }, replace: true });
        }}
        onOpenJob={(jobId) => {
          void navigate({ to: '/jobs/$jobId', params: { jobId } });
        }}
      />
    </Page>
  );
};
