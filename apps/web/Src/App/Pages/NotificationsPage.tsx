import { Page } from '@/App/Layout/Page';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { useT } from '@/I18n/useT';

export const NotificationsPage = () => {
  const t = useT();

  return (
    <Page title={t('notifications.title')} lead={t('page.notifications.lead')}>
      <Card>
        <EmptyState icon="bell" text={t('notifications.empty')} />
      </Card>
    </Page>
  );
};
