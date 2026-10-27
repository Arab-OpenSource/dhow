import type { ReactNode } from 'react';

import { useMe } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { AccountBody } from '@/Modules/Session/Components/AccountBody';

export const AccountSubtitle = () => {
  const t = useT();
  const me = useMe();
  const role = me.data?.Memberships[0]?.Role;

  return (
    <>
      <bdi dir="ltr" className="font-mono">
        {me.data?.User.Email}
      </bdi>
      {role ? <Badge tone="mute">{t(`role.${role}`)}</Badge> : null}
    </>
  );
};

export interface IAccountSlideProps {
  security?: ReactNode;
}

export const AccountSlide = ({ security }: IAccountSlideProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const me = useMe();

  if (me.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('account.error.title')}>
          {errorText(me.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void me.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (me.data === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return <AccountBody me={me.data} security={security} />;
};
