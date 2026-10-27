import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { useT } from '@/I18n/useT';

export const WorkspacePage = () => {
  const t = useT();

  return (
    <div className="grid gap-4">
      <h1 className="m-0 text-title-xl">{t('workspace.title')}</h1>
      <Card>
        <EmptyState icon="project" text={t('workspace.empty.text')} />
      </Card>
    </div>
  );
};
