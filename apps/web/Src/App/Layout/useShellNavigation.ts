import { useNavigate, useSearch } from '@tanstack/react-router';

import type { TPanelId, TPanelParam } from '@/App/Panels';

export interface IShellNavigation {
  panel: TPanelParam | undefined;
  openPanel: (id: TPanelId) => void;
  closePanel: () => void;
  discardPanel: () => void;
  goWorkspace: () => void;
}

export const useShellNavigation = (): IShellNavigation => {
  const navigate = useNavigate();
  const { panel } = useSearch({ strict: false });

  return {
    panel,
    openPanel: (id) => {
      void navigate({ to: '/', search: (previous) => ({ ...previous, panel: id }) });
    },
    closePanel: () => {
      void navigate({ to: '/', search: (previous) => ({ ...previous, panel: undefined }) });
    },
    discardPanel: () => {
      void navigate({ to: '/', search: (previous) => ({ ...previous, panel: undefined }), replace: true });
    },
    goWorkspace: () => {
      void navigate({ to: '/', search: (previous) => ({ ...previous, panel: undefined }) });
    },
  };
};
