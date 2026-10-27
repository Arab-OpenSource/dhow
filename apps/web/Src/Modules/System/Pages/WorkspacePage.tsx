import { useNavigate } from '@tanstack/react-router';

import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { useT } from '@/I18n/useT';

export const WorkspacePage = () => {
  const t = useT();
  const navigate = useNavigate();

  const OpenStatus = () => {
    void navigate({ to: '/', search: (previous) => ({ ...previous, panel: 'system-status' }) });
  };

  return (
    <div className="grid gap-4">
      <h1 className="m-0 text-title-xl">{t('workspace.title')}</h1>
      <Card>
        <EmptyState
          icon="project"
          text={t('workspace.empty.text')}
          action={<Button onClick={OpenStatus}>{t('workspace.empty.action')}</Button>}
        />
      </Card>
    </div>
  );
};
