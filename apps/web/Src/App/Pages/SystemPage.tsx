import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { SystemStatusBody } from '@/Modules/System/Components/SystemStatusBody';
import { SystemStatusFooter } from '@/Modules/System/Components/SystemStatusFooter';

export const SystemPage = () => {
  const t = useT();
  const navigate = useNavigate();

  return (
    <Page title={t('status.title')} lead={t('status.subtitle')} action={<SystemStatusFooter />}>
      <SystemStatusBody
        onOpenCheck={(check) => {
          void navigate({ to: '/system/checks/$check', params: { check } });
        }}
      />
    </Page>
  );
};
