import { Page } from '@/App/Layout/Page';
import { AccountNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useMe } from '@/Core/Auth/useMe';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { MfaSection } from '@/Modules/Mfa/Components/MfaSection';
import { SecuritySummary } from '@/Modules/Session/Components/AccountBody';
import { IdentitiesCard } from '@/Modules/Session/Components/IdentitiesCard';
import { SessionsCard } from '@/Modules/Session/Components/SessionsCard';

export const SecurityPage = () => {
  const t = useT();
  const me = useMe();
  const { open } = useSlide();

  return (
    <Page
      title={t('shell.nav.security')}
      lead={t('page.security.lead')}
      action={
        <Button
          variant="primary"
          iconStart="key"
          onClick={() => {
            open('account-password');
          }}
        >
          {t('account.password.change')}
        </Button>
      }
      nav={<AccountNav />}
    >
      {me.data ? <SecuritySummary me={me.data} /> : null}
      <MfaSection
        onSetUpTotp={() => {
          open('account-totp');
        }}
      />
      <IdentitiesCard />
      <SessionsCard />
    </Page>
  );
};
