import { useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { RunnerBody } from '@/Modules/Runners/Components/RunnerBody';

export const RunnerPage = () => {
  const t = useT();
  const { runnerId } = useParams({ strict: false });
  if (!isUuid(runnerId)) return <NotFoundPage />;

  return (
    <Page title={t('runners.detail.title')} lead={t('runners.detail.subtitle')}>
      <RunnerBody key={runnerId} runnerId={runnerId} />
    </Page>
  );
};
