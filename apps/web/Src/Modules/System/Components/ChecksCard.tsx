import { Card } from '@/Core/Ui/Card';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { CheckBadge } from '@/Modules/System/Components/CheckBadge';
import { CHECK_ICONS, type TCheckName, type THealthStatus } from '@/Modules/System/Types/SystemTypes';

export interface IChecksCardProps {
  health: THealthStatus;
  onOpenCheck: (name: TCheckName) => void;
}

export const ChecksCard = ({ health, onOpenCheck }: IChecksCardProps) => {
  const t = useT();

  return (
    <Card title={t('status.checks.title')} icon="server" end={<CheckBadge ok={health.Status === 'ok'} />}>
      {health.Checks.length === 0 ? (
        <p className="m-0 px-card pt-1.5 pb-card text-body-sm text-secondary">{t('status.checks.empty')}</p>
      ) : (
        <Rows>
          {health.Checks.map((check) => (
            <Row
              key={check.Name}
              icon={CHECK_ICONS[check.Name]}
              title={t(`status.check.${check.Name}`)}
              end={<CheckBadge ok={check.Ok} />}
              onSelect={() => {
                onOpenCheck(check.Name);
              }}
            />
          ))}
        </Rows>
      )}
    </Card>
  );
};
