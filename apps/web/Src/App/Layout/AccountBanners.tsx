import { useNavigate } from '@tanstack/react-router';

import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useIsPast } from '@/Core/Hooks/useIsPast';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Banner } from '@/Core/Ui/Banner';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';

export const AccountBanners = () => {
  const t = useT();
  const time = useRelativeTime();
  const me = useMe();
  const membership = useCurrentMembership();
  const navigate = useNavigate();
  const due = membership?.SecondFactorDue;
  const passed = useIsPast(due);
  const HandleOpenAccount = () => {
    void navigate({ to: '/account/security' });
  };

  if (membership && due !== undefined) {
    return (
      <Banner
        accent={passed ? 'failed' : 'awaiting'}
        className="mb-4"
        title={t('secondFactor.banner.title', { organization: membership.OrganizationName })}
        body={passed ? t('secondFactor.banner.after') : t('secondFactor.banner.before', { date: time.full(due) })}
        action={
          <Button size="sm" variant="primary" onClick={HandleOpenAccount}>
            {t('secondFactor.banner.action')}
          </Button>
        }
      />
    );
  }
  if (me.data?.MfaReenrollRequired !== true) return null;
  return (
    <Banner
      accent="awaiting"
      className="mb-4"
      title={t('mfaReset.banner.title')}
      body={t('mfaReset.banner.body')}
      action={
        <Button size="sm" variant="primary" onClick={HandleOpenAccount}>
          {t('mfaReset.banner.action')}
        </Button>
      }
    />
  );
};
