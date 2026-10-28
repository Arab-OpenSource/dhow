import { create } from 'zustand';

export type TOrganizationTab = 'members' | 'teams' | 'invitations' | 'settings';

interface IOrganizationTabState {
  Tab: TOrganizationTab;
  SetTab: (tab: TOrganizationTab) => void;
}

export const useOrganizationTabStore = create<IOrganizationTabState>((set) => ({
  Tab: 'members',
  SetTab: (tab) => {
    set({ Tab: tab });
  },
}));
