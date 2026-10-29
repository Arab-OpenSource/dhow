import { useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { AuditEventBody } from '@/Modules/Audit/Components/AuditEventBody';

export const AuditEventPage = () => {
  const t = useT();
  const { eventId } = useParams({ strict: false });
  if (!isUuid(eventId)) return <NotFoundPage />;

  return (
    <Page title={t('audit.detail.title')} lead={t('audit.detail.subtitle')}>
      <AuditEventBody key={eventId} eventId={eventId} />
    </Page>
  );
};
