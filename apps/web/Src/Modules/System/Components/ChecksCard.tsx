import { Card } from '@/Core/Ui/Card';
import { Icon } from '@/Core/Ui/Icon';
import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import type { TCheckName, THealthStatus } from '@/Modules/System/Types/SystemTypes';

export interface IChecksCardProps {
  health: THealthStatus;
  onOpenCheck: (name: TCheckName) => void;
}

export const ChecksCard = ({ health, onOpenCheck }: IChecksCardProps) => {
  const t = useT();
  const overall = health.Status === 'ok' ? 'live' : 'failed';

  return (
    <Card
      title={t('status.checks.title')}
      icon="health"
      end={
        <StatusDot status={overall}>{t(overall === 'live' ? 'status.state.live' : 'status.state.failed')}</StatusDot>
      }
    >
      {health.Checks.length === 0 ? (
        <p className="m-0 text-caption text-tertiary">{t('status.checks.empty')}</p>
      ) : (
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {health.Checks.map((check) => (
            <li key={check.Name}>
              <button
                type="button"
                onClick={() => {
                  onOpenCheck(check.Name);
                }}
                className={
                  'flex w-full cursor-pointer items-center justify-between gap-3 rounded-md border-0 bg-row px-3 py-2.5 ' +
                  'text-start text-body-sm text-primary hover:bg-surface-raised'
                }
              >
                <span>{t(`status.check.${check.Name}`)}</span>
                <span className="flex items-center gap-2">
                  <StatusDot status={check.Ok ? 'live' : 'failed'}>
                    {t(check.Ok ? 'status.state.live' : 'status.state.failed')}
                  </StatusDot>
                  <Icon name="next" size={16} className="text-muted" />
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
};
