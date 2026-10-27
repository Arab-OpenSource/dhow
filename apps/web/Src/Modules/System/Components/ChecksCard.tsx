import { Card } from '@/Core/Ui/Card';
import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import type { THealthStatus } from '@/Modules/System/Types/SystemTypes';

export const ChecksCard = ({ health }: { health: THealthStatus }) => {
  const t = useT();
  const overall = health.Status === 'ok' ? 'live' : 'failed';

  return (
    <Card
      title={t('status.checks.title')}
      icon="health"
      end={<StatusDot status={overall}>{t(overall === 'live' ? 'status.state.live' : 'status.state.failed')}</StatusDot>}
    >
      {health.Checks.length === 0 ? (
        <p className="m-0 text-caption text-tertiary">{t('status.checks.empty')}</p>
      ) : (
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {health.Checks.map((check) => (
            <li
              key={check.Name}
              className="flex items-center justify-between gap-3 rounded-md bg-surface-inset px-3 py-2.5 text-body-sm"
            >
              <span>{t(`status.check.${check.Name}`)}</span>
              <StatusDot status={check.Ok ? 'live' : 'failed'}>
                {t(check.Ok ? 'status.state.live' : 'status.state.failed')}
              </StatusDot>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
};
