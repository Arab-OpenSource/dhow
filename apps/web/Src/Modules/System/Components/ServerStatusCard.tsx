import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, type TCardState } from '@/Core/Ui/CardStates';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { CheckBadge } from '@/Modules/System/Components/CheckBadge';
import { StatusSummary } from '@/Modules/System/Components/StatusSummary';
import { useMeta } from '@/Modules/System/Hooks/useMeta';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';
import { CHECK_ICONS, type TCheckName } from '@/Modules/System/Types/SystemTypes';

export interface IServerStatusCardProps {
  onOpenCheck: (name: TCheckName) => void;
  onOpenStatus: () => void;
  className?: string;
}

export const ServerStatusCard = ({ onOpenCheck, onOpenStatus, className }: IServerStatusCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const meta = useMeta();
  const readiness = useReadiness();
  const failure = readiness.error ?? meta.error;
  const checks = readiness.data?.Checks ?? [];
  let state: TCardState = 'ready';
  if (failure) state = 'error';
  else if (meta.data === undefined || readiness.data === undefined) state = 'loading';
  else if (checks.length === 0) state = 'empty';

  return (
    <Card
      title={t('status.title')}
      icon="shield"
      className={className}
      end={
        <Button size="sm" variant="ghost" onClick={onOpenStatus}>
          {t('home.status.open')}
        </Button>
      }
    >
      <CardStates
        state={state}
        loadingLabel={t('status.loading')}
        errorTitle={t('home.status.error')}
        errorText={failure ? errorText(failure) : undefined}
        retryLabel={t('jobs.retry')}
        onRetry={() => {
          void meta.refetch();
          void readiness.refetch();
        }}
        emptyIcon="server"
        emptyText={t('home.status.empty')}
      >
        {meta.data && readiness.data ? (
          <>
            <div className="px-card pt-1 pb-3">
              <StatusSummary meta={meta.data} health={readiness.data} />
            </div>
            <Rows label={t('status.checks.title')}>
              {checks.map((check) => (
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
              <Row
                icon="info"
                title={t('status.build.version')}
                end={
                  <bdi dir="ltr" className="font-mono text-code text-primary">
                    {meta.data.Version}
                  </bdi>
                }
              />
            </Rows>
          </>
        ) : null}
      </CardStates>
    </Card>
  );
};
