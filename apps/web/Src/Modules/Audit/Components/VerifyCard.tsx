import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { Summary } from '@/Core/Ui/Summary';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useT } from '@/I18n/useT';
import type { TVerifyState } from '@/Modules/Audit/Hooks/useVerifyChain';

export interface IVerifyCardProps {
  state: TVerifyState;
  onRetry: () => void;
}

export const VerifyCard = ({ state, onRetry }: IVerifyCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();

  if (state.Status === 'running') return <Skeleton label={t('audit.verify.running')} lines={2} />;
  if (state.Status === 'error') {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('audit.verify.error')}>
          {errorText(state.Error)}
        </InlineAlert>
        <Button size="sm" onClick={onRetry}>
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (state.Status === 'done' && state.Result.Valid) {
    return (
      <Summary
        tone="ok"
        icon="shield"
        title={t('audit.verify.intact.title')}
        sub={t('audit.verify.intact.sub', { count: state.Result.Checked })}
      />
    );
  }
  if (state.Status === 'done') {
    return (
      <Summary
        tone="bad"
        icon="alert"
        title={t('audit.verify.broken.title')}
        sub={
          <>
            {t('audit.verify.broken.sub', { count: state.Result.Checked })}
            {state.Result.FirstBrokenId ? (
              <>
                {' '}
                <bdi dir="ltr" className="font-mono">
                  {state.Result.FirstBrokenId}
                </bdi>
              </>
            ) : null}
          </>
        }
      />
    );
  }
  return <InlineAlert accent="building" title={t('audit.verify.idle.title')}>{t('audit.verify.idle.sub')}</InlineAlert>;
};
