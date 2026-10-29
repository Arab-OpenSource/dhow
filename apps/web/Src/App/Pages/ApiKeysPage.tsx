import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useT } from '@/I18n/useT';
import { OrganizationTokensBody, OrganizationTokensFooter } from '@/Modules/Tokens/Components/OrganizationTokensBody';

export const ApiKeysPage = () => {
  const t = useT();
  const { open } = useSlide();
  const HandleNew = () => {
    open('organization-token-new');
  };

  return (
    <Page title={t('shell.nav.apiKeys')} lead={t('orgTokens.subtitle')} action={<OrganizationTokensFooter onNew={HandleNew} />} nav={<OrgNav />}>
      <OrganizationTokensBody onNew={HandleNew} />
    </Page>
  );
};
