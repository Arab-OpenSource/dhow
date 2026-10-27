import { EmptyState } from '@/Core/Ui/EmptyState';
import { SlideOver } from '@/Core/Ui/SlideOver';
import { useT } from '@/I18n/useT';

export interface IPanelProps {
  open: boolean;
  onClose: () => void;
}

export const NotificationsPanel = ({ open, onClose }: IPanelProps) => {
  const t = useT();

  return (
    <SlideOver open={open} onClose={onClose} title={t('notifications.title')} closeLabel={t('ui.close')}>
      <EmptyState icon="bell" text={t('notifications.empty')} />
    </SlideOver>
  );
};
