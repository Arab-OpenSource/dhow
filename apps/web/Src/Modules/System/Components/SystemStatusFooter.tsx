import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { useMeta } from '@/Modules/System/Hooks/useMeta';
import { useReadiness } from '@/Modules/System/Hooks/useReadiness';

export const SystemStatusFooter = () => {
  const t = useT();
  const meta = useMeta();
  const readiness = useReadiness();
  const busy = meta.isFetching || readiness.isFetching;

  return (
    <>
      <span className="flex-1" />
      <Button
        variant="primary"
        disabled={busy}
        onClick={() => {
          void meta.refetch();
          void readiness.refetch();
        }}
      >
        {busy ? t('status.refreshing') : t('status.refresh')}
      </Button>
    </>
  );
};
