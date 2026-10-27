import type { ReactNode } from 'react';

import type { TMe } from '@/Core/Auth/AuthTypes';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';
import { SessionsCard } from '@/Modules/Session/Components/SessionsCard';

export interface IAccountBodyProps {
  me: TMe;
  security?: ReactNode;
}

export const AccountBody = ({ me, security }: IAccountBodyProps) => {
  const t = useT();
  const time = useRelativeTime();
  const membership = me.Memberships[0];

  return (
    <>
      <Summary
        tone="ok"
        icon="shield"
        title={t('account.summary.title')}
        sub={t('account.summary.sub', { date: time.full(me.ExpiresAt) })}
      />
      <Card title={t('account.profile.title')} icon="user">
        <Rows label={t('account.profile.title')}>
          <Row title={me.User.Name} sub={t('account.profile.name')} />
          <Row title={me.User.Email} sub={t('account.profile.email')} mono />
          <Row title={t(`language.${me.User.Locale}`)} sub={t('account.profile.language')} />
          {membership ? (
            <Row
              title={membership.OrganizationName}
              sub={t('account.profile.organization')}
              end={<Badge tone="mute">{t(`role.${membership.Role}`)}</Badge>}
            />
          ) : null}
        </Rows>
      </Card>
      {security}
      <SessionsCard />
    </>
  );
};
