import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useDelayedFlag } from '@/Core/Hooks/useDelayedFlag';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { BuildInfoCard } from '@/Modules/System/Components/BuildInfoCard';
import { ChecksCard } from '@/Modules/System/Components/ChecksCard';
import { useMeta } from '@/Modules/System/Hooks/useMeta';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';
import type { TCheckName } from '@/Modules/System/Types/SystemTypes';

const SKELETON_DELAY_MS = 300;

export interface ISystemStatusBodyProps {
  onOpenCheck: (name: TCheckName) => void;
}

export const SystemStatusBody = ({ onOpenCheck }: ISystemStatusBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const meta = useMeta();
  const readiness = useReadiness();
  const failure = readiness.error ?? meta.error;
  const loading = !failure && (meta.isPending || readiness.isPending);
  const showSkeleton = useDelayedFlag(loading, SKELETON_DELAY_MS);

  const Retry = () => {
    void meta.refetch();
    void readiness.refetch();
  };

  return (
    <>
      {showSkeleton ? <Skeleton label={t('status.loading')} lines={5} /> : null}
      {failure ? (
        <div className="grid justify-items-start gap-3">
          <InlineAlert accent="failed" title={t('status.error.title')}>
            {errorText(failure)}
          </InlineAlert>
          <Button size="sm" onClick={Retry}>
            {t('status.retry')}
          </Button>
        </div>
      ) : null}
      {meta.data && readiness.data && !failure ? (
        <>
          <BuildInfoCard meta={meta.data} />
          <ChecksCard health={readiness.data} onOpenCheck={onOpenCheck} />
        </>
      ) : null}
    </>
  );
};
