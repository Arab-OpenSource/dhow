import type { TMe } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';

export interface IAccountPartProps {
  me: TMe;
}

export const SecuritySummary = ({ me }: IAccountPartProps) => {
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

export const ProfileCard = ({ me }: IAccountPartProps) => {
  const t = useT();
  const membership = useCurrentMembership();

  return (
    <Card>
      <div className="flex min-w-0 items-center gap-4 border-b border-line p-card">
        <Avatar name={me.User.Name} size="lg" />
        <dl className="m-0 grid min-w-0 gap-0.5">
          <dt className="sr-only">{t('account.profile.name')}</dt>
          <dd className="m-0 truncate text-title-section">
            <bdi>{me.User.Name}</bdi>
          </dd>
          <dt className="sr-only">{t('account.profile.email')}</dt>
          <dd className="m-0 truncate text-body-sm text-secondary">
            <bdi dir="ltr">{me.User.Email}</bdi>
          </dd>
        </dl>
      </div>
      <Rows label={t('account.profile.title')}>
        <Row title={t(`language.${me.User.Locale}`)} sub={t('account.profile.language')} />
        {membership ? (
          <Row
            title={<bdi>{membership.OrganizationName}</bdi>}
            sub={t('account.profile.organization')}
            end={<Badge tone="mute">{t(`role.${membership.Role}`)}</Badge>}
          />
        ) : null}
      </Rows>
      <p className="m-0 border-t border-line px-card py-3 text-body-sm text-tertiary">{t('account.profile.readOnly')}</p>
    </Card>
  );
};

export const PasswordCard = ({ onChange }: { onChange: () => void }) => {
  const t = useT();
  return (
    <Card title={t('account.password.title')} icon="key" padded>
      <div className="grid justify-items-start gap-3 sm:flex sm:items-center">
        <p className="m-0 flex-1 text-body-sm text-secondary">{t('account.password.about')}</p>
        <Button size="sm" iconStart="key" onClick={onChange}>
          {t('account.password.change')}
        </Button>
      </div>
    </Card>
  );
};
