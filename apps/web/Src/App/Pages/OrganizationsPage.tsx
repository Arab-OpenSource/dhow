import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { useSlide } from '@/App/Navigation/useSlide';
import { useT } from '@/I18n/useT';
import { OrganizationsBody } from '@/Modules/Organization/Components/OrganizationsBody';

export const OrganizationsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();

  return (
    <Page title={t('organizations.title')} lead={t('organizations.subtitle')}>
      <OrganizationsBody
        onSwitched={() => {
          void navigate({ to: '/' });
        }}
        onSettings={() => {
          void navigate({ to: '/org/settings' });
        }}
        onCreate={() => {
          open('organization-new');
        }}
      />
    </Page>
  );
};
