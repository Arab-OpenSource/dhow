import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useDelayedFlag } from '@/Core/Hooks/useDelayedFlag';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { StatusDot } from '@/Core/Ui/StatusDot';
import { useT } from '@/I18n/useT';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';
import type { TCheckName } from '@/Modules/System/Types/SystemTypes';

const SKELETON_DELAY_MS = 300;
const FIX_STEPS = [1, 2, 3] as const;

export interface ICheckDetailBodyProps {
  name: TCheckName;
}

export const CheckDetailBody = ({ name }: ICheckDetailBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const readiness = useReadiness();
  const showSkeleton = useDelayedFlag(readiness.isPending && !readiness.error, SKELETON_DELAY_MS);
  const check = readiness.data?.Checks.find((entry) => entry.Name === name);
  const failing = check !== undefined && !check.Ok;

  if (readiness.error) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('status.error.title')}>
          {errorText(readiness.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void readiness.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }

  if (!readiness.data) {
    return showSkeleton ? <Skeleton label={t('status.loading')} lines={5} /> : null;
  }

  return (
    <>
      <Card title={t('checkDetail.state')} icon={name === 'database' ? 'database' : 'migrations'}>
        {check ? (
          <div className="grid gap-1.5">
            <StatusDot status={check.Ok ? 'live' : 'failed'}>
              {t(check.Ok ? 'status.state.live' : 'status.state.failed')}
            </StatusDot>
            <p className="m-0 text-caption text-tertiary">
              {t(check.Ok ? 'checkDetail.okNow' : 'checkDetail.failNow')}
            </p>
          </div>
        ) : (
          <p className="m-0 text-caption text-tertiary">{t('checkDetail.missing')}</p>
        )}
      </Card>
      <Card title={t('checkDetail.means')} icon="info">
        <p className="m-0 text-body-sm text-secondary">{t(`checkDetail.${name}.means`)}</p>
      </Card>
      <section
        className={
          failing
            ? 'rounded-xl bg-aurora-card p-4 text-on-light'
            : 'relative rounded-lg bg-card-gradient p-4 shadow-card'
        }
      >
        <h3 className="m-0 mb-3 text-title-lg">{t('checkDetail.fix')}</h3>
        <ol className="m-0 grid gap-2 ps-5 text-body-sm">
          {FIX_STEPS.map((step) => (
            <li key={step}>{t(`checkDetail.${name}.fix${step}`)}</li>
          ))}
        </ol>
      </section>
    </>
  );
};
