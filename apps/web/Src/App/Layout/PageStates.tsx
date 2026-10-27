import { AuthPage } from '@/Core/Ui/AuthPage';
import { Button } from '@/Core/Ui/Button';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';

export const PageLoading = () => {
  const t = useT();

  return (
    <div className="grid min-h-dvh place-items-center bg-canvas px-6">
      <Skeleton label={t('ui.loading')} lines={4} className="max-w-[360px]" />
    </div>
  );
};

export interface IPageErrorProps {
  busy: boolean;
  onRetry: () => void;
}

export const PageError = ({ busy, onRetry }: IPageErrorProps) => {
  const t = useT();

  return (
    <AuthPage title={t('apiDown.title')} lead={t('apiDown.body')}>
      <Button variant="primary" disabled={busy} onClick={onRetry}>
        {t('apiDown.retry')}
      </Button>
    </AuthPage>
  );
};
