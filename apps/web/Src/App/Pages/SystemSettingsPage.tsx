import { Page } from '@/App/Layout/Page';
import { useT } from '@/I18n/useT';
import { SettingsBody } from '@/Modules/Settings/Components/SettingsBody';

export const SystemSettingsPage = () => {
  const t = useT();

  return (
    <Page title={t('shell.nav.systemSettings')} lead={t('settings.lead')}>
      <SettingsBody />
    </Page>
  );
};
