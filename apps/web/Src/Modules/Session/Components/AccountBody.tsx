import type { ReactNode } from 'react';

import type { TMe } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Badge } from '@/Core/Ui/Badge';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';
import { IdentitiesCard } from '@/Modules/Session/Components/IdentitiesCard';
import { SessionsCard } from '@/Modules/Session/Components/SessionsCard';

export interface IAccountBodyProps {
  me: TMe;
  security?: ReactNode;
  tokens?: ReactNode;
}

const SecuritySummary = ({ me }: { me: TMe }) => {
  const t = useT();
  const time = useRelativeTime();
  const status = useMfaStatus();
  const session = t('account.summary.sub', { date: time.full(me.ExpiresAt) });

  if (status.data === undefined) {
    return status.isError ? null : <Skeleton label={t('ui.loading')} lines={1} />;
  }
  if (!status.data.Enabled) {
    return <Summary tone="warn" icon="alert" title={t('account.summary.weak.title')} sub={t('account.summary.weak.sub')} />;
  }
  return <Summary tone="ok" icon="shield" title={t('account.summary.strong.title')} sub={session} />;
};

export const AccountBody = ({ me, security, tokens }: IAccountBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();

  return (
    <>
      <SecuritySummary me={me} />
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
      <IdentitiesCard />
      {tokens}
      <SessionsCard />
    </>
  );
};
