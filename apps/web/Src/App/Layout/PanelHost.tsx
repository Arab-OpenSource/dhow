import { NotificationsPanel } from '@/App/Layout/NotificationsPanel';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';

export const PanelHost = () => {
  const { panel, closePanel } = useShellNavigation();

  return <NotificationsPanel open={panel === 'notifications'} onClose={closePanel} />;
};
