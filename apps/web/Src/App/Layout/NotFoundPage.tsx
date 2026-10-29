import { useNavigate } from '@tanstack/react-router';

import { Button } from '@/Core/Ui/Button';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { useT } from '@/I18n/useT';

export const NotFoundPage = () => {
  const t = useT();
  const navigate = useNavigate();

  return (
    <EmptyState
      icon="layers"
      text={t('notFound.text')}
      action={<Button variant="primary" onClick={() => void navigate({ to: '/' })}>{t('notFound.action')}</Button>}
    />
  );
};
