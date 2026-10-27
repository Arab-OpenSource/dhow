import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useDelayedFlag } from '@/Core/Hooks/useDelayedFlag';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { Summary } from '@/Core/Ui/Summary';
import { useT } from '@/I18n/useT';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';
import type { TCheckName } from '@/Modules/System/Types/SystemTypes';

const SKELETON_DELAY_MS = 300;
const FIX_STEPS = [1, 2, 3] as const;

export interface ICheckDetailBodyProps {
  name: TCheckName;
}

const CheckSummary = ({ ok }: { ok: boolean | undefined }) => {
  const t = useT();

  if (ok === undefined) {
    return <Summary tone="warn" icon="info" title={t('checkDetail.missing.title')} sub={t('checkDetail.missing')} />;
  }
  return ok ? (
    <Summary tone="ok" icon="shield" title={t('checkDetail.ok.title')} sub={t('checkDetail.okNow')} />
  ) : (
    <Summary tone="bad" icon="alert" title={t('checkDetail.fail.title')} sub={t('checkDetail.failNow')} />
  );
};

export const CheckDetailBody = ({ name }: ICheckDetailBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const readiness = useReadiness();
  const showSkeleton = useDelayedFlag(readiness.isPending && !readiness.error, SKELETON_DELAY_MS);
  const check = readiness.data?.Checks.find((entry) => entry.Name === name);

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
      <CheckSummary ok={check?.Ok} />
      <Card title={t('checkDetail.means')} icon="info" padded>
        <p className="m-0 text-body-sm text-secondary">{t(`checkDetail.${name}.means`)}</p>
      </Card>
      <Card title={t('checkDetail.fix')} icon="settings" padded>
        <ol className="m-0 grid list-decimal gap-2 ps-5 text-body-sm text-secondary">
          {FIX_STEPS.map((step) => (
            <li key={step}>{t(`checkDetail.${name}.fix${step}`)}</li>
          ))}
        </ol>
      </Card>
    </>
  );
};
