import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { AuditBody } from '@/Modules/Audit/Components/AuditBody';
import { AuditFooter } from '@/Modules/Audit/Components/AuditFooter';

export const AuditPage = () => {
  const t = useT();
  const navigate = useNavigate();

  return (
    <Page title={t('audit.title')} lead={t('audit.subtitle')} action={<AuditFooter />}>
      <AuditBody
        onOpenEvent={(eventId) => {
          void navigate({ to: '/audit/$eventId', params: { eventId } });
        }}
      />
    </Page>
  );
};
