import { create } from 'zustand';

interface IPaletteState {
  Open: boolean;
  SetOpen: (open: boolean) => void;
  Toggle: () => void;
}

export const usePaletteStore = create<IPaletteState>((set) => ({
  Open: false,
  SetOpen: (open) => {
    set({ Open: open });
  },
  Toggle: () => {
    set((state) => ({ Open: !state.Open }));
  },
}));
