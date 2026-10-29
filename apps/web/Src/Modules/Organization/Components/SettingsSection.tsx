import { type TOrganization, type TRole } from '@/Core/Auth/AuthTypes';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import type { TIconName } from '@/Core/Ui/Icons';
import { useT } from '@/I18n/useT';

export const SecondFactorSummary = ({ organization }: { organization: TOrganization }) => {
  const t = useT();
  const time = useRelativeTime();
  const rule = organization.RequireSecondFactor;

  return (
    <Card title={t('organization.secondFactor.title')} icon="shield" padded>
      <div className="grid gap-1.5">
        <p className="m-0 text-body text-primary">{t(`organization.secondFactor.rule.${rule}`)}</p>
        <p className="m-0 text-body-sm text-secondary">{t(`organization.secondFactor.about.${rule}`)}</p>
        {organization.RequireSecondFactorSince && rule !== 'off' ? (
          <p className="m-0 text-body-sm text-tertiary">
            {t('organization.secondFactor.since', { date: time.full(organization.RequireSecondFactorSince) })}
          </p>
        ) : null}
        <p className="m-0 text-body-sm text-tertiary">{t('orgSettings.security.ownerOnly')}</p>
      </div>
    </Card>
  );
};

interface IDangerItemProps {
  title: string;
  about: string;
  action: string;
  icon: TIconName;
  onClick: () => void;
}

const DangerItem = ({ title, about, action, icon, onClick }: IDangerItemProps) => (
  <li className="grid min-w-0 gap-3 border-t border-line px-card py-4 first:border-t-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
    <span className="grid min-w-0 gap-1">
      <span className="text-label">{title}</span>
      <small className="text-body-sm text-secondary">{about}</small>
    </span>
    <Button variant="danger" iconStart={icon} className="justify-self-start" onClick={onClick}>
      {action}
    </Button>
  </li>
);

export interface IDangerZoneProps {
  organization: TOrganization;
  role: TRole;
  onTransfer: () => void;
  onLeave: () => void;
}

export const DangerZone = ({ organization, role, onTransfer, onLeave }: IDangerZoneProps) => {
  const t = useT();
  return (
    <div className="min-w-0 rounded-md border border-bad-line bg-surface">
      <ul aria-label={t('orgSettings.group.danger')} className="m-0 grid list-none p-0">
        {role === 'owner' ? (
          <DangerItem
            title={t('organization.transfer.title')}
            about={t('organization.transfer.about')}
            action={t('organization.transfer.open')}
            icon="swap"
            onClick={onTransfer}
          />
        ) : null}
        <DangerItem
          title={t('organization.leave.title')}
          about={t('organization.leave.about')}
          action={t('organization.leave.action', { name: organization.Name })}
          icon="out"
          onClick={onLeave}
        />
      </ul>
    </div>
  );
};
