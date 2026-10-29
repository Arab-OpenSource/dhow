import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { OrganizationGate } from '@/App/Pages/OrganizationGate';
import { useSlide } from '@/App/Navigation/useSlide';
import { useT } from '@/I18n/useT';
import { SettingsSection } from '@/Modules/Organization/Components/SettingsSection';

export const OrgSettingsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();

  return (
    <Page title={t('shell.nav.orgSettings')} lead={t('page.orgSettings.lead')} nav={<OrgNav />}>
      <OrganizationGate>
        {(organization, membership) => (
          <SettingsSection
            organization={organization}
            role={membership.Role}
            onTransfer={() => {
              open('organization-transfer');
            }}
            onTokens={() => {
              void navigate({ to: '/org/api-keys' });
            }}
            onLeft={() => {
              void navigate({ to: '/' });
            }}
          />
        )}
      </OrganizationGate>
    </Page>
  );
};
