import { useDelayedFlag } from '@/Core/Hooks/useDelayedFlag';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { SlideOver } from '@/Core/Ui/SlideOver';
import { useT } from '@/I18n/useT';
import { BuildInfoCard } from '@/Modules/System/Components/BuildInfoCard';
import { ChecksCard } from '@/Modules/System/Components/ChecksCard';
import { useMeta } from '@/Modules/System/Hooks/useMeta';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';

const SKELETON_DELAY_MS = 300;

export interface ISystemStatusPanelProps {
  open: boolean;
  onClose: () => void;
}

export const SystemStatusPanel = ({ open, onClose }: ISystemStatusPanelProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const meta = useMeta();
  const readiness = useReadiness(open);
  const failure = readiness.error ?? meta.error;
  const loading = open && !failure && (meta.isPending || readiness.isPending);
  const showSkeleton = useDelayedFlag(loading, SKELETON_DELAY_MS);

  const Refresh = () => {
    void meta.refetch();
    void readiness.refetch();
  };

  return (
    <SlideOver
      open={open}
      onClose={onClose}
      title={t('status.title')}
      subtitle={t('status.subtitle')}
      closeLabel={t('ui.close')}
      footer={
        <Button variant="aurora" disabled={meta.isFetching || readiness.isFetching} onClick={Refresh}>
          {meta.isFetching || readiness.isFetching ? t('status.refreshing') : t('status.refresh')}
        </Button>
      }
    >
      {showSkeleton ? <Skeleton label={t('status.loading')} lines={5} /> : null}
      {failure ? (
        <div className="grid justify-items-start gap-3">
          <InlineAlert accent="failed" title={t('status.error.title')}>
            {errorText(failure)}
          </InlineAlert>
          <Button size="sm" onClick={Refresh}>
            {t('status.retry')}
          </Button>
        </div>
      ) : null}
      {meta.data && readiness.data && !failure ? (
        <>
          <BuildInfoCard meta={meta.data} />
          <ChecksCard health={readiness.data} />
        </>
      ) : null}
    </SlideOver>
  );
};
