import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { Button } from '@/Core/Ui/Button';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { useT } from '@/I18n/useT';

export const NotFoundPage = () => {
  const t = useT();
  const { goWorkspace } = useShellNavigation();

  return (
    <EmptyState
      icon="layers"
      text={t('notFound.text')}
      action={<Button variant="primary" onClick={goWorkspace}>{t('notFound.action')}</Button>}
    />
  );
};
