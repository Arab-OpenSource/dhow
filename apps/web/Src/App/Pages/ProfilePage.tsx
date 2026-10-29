import { Page } from '@/App/Layout/Page';
import { AccountNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useMe } from '@/Core/Auth/useMe';
import { useSignOut } from '@/Core/Auth/useSignOut';
import { Button } from '@/Core/Ui/Button';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { ProfileCard } from '@/Modules/Session/Components/AccountBody';
import { TokensCard } from '@/Modules/Tokens/Components/TokensCard';

export const ProfilePage = () => {
  const t = useT();
  const me = useMe();
  const { open } = useSlide();
  const { SignOut, Pending } = useSignOut();

  return (
    <Page
      title={t('shell.nav.profile')}
      lead={t('page.profile.lead')}
      action={
        <Button variant="danger" iconStart="out" disabled={Pending} onClick={() => void SignOut()}>
          {t('account.signOut')}
        </Button>
      }
      nav={<AccountNav />}
    >
      {me.data ? <ProfileCard me={me.data} /> : <Skeleton label={t('ui.loading')} lines={4} />}
      <TokensCard
        onNew={() => {
          open('account-token-new');
        }}
      />
    </Page>
  );
};
