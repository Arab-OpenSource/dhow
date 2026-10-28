import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { useMe } from '@/Core/Auth/useMe';
import { Banner } from '@/Core/Ui/Banner';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export const AccountBanners = () => {
  const t = useT();
  const me = useMe();
  const { openStack } = useShellNavigation();
  if (me.data?.MfaReenrollRequired !== true) return null;
  return (
    <Banner
      accent="awaiting"
      className="mb-4"
      title={t('mfaReset.banner.title')}
      body={t('mfaReset.banner.body')}
      action={
        <Button
          size="sm"
          variant="primary"
          onClick={() => {
            openStack(['account']);
          }}
        >
          {t('mfaReset.banner.action')}
        </Button>
      }
    />
  );
};
