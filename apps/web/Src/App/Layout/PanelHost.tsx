import { NotificationsPanel } from '@/App/Layout/NotificationsPanel';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { SystemStatusPanel } from '@/Modules/System/Components/SystemStatusPanel';

export const PanelHost = () => {
  const { panel, closePanel } = useShellNavigation();

  return (
    <>
      <SystemStatusPanel open={panel === 'system-status'} onClose={closePanel} />
      <NotificationsPanel open={panel === 'notifications'} onClose={closePanel} />
    </>
  );
};
