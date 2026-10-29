import { create } from 'zustand';

interface ICrumbState {
  Titles: Readonly<Record<string, string>>;
  Publish: (id: string, title: string) => void;
  Withdraw: (id: string) => void;
}

export const useCrumbStore = create<ICrumbState>((set) => ({
  Titles: {},
  Publish: (id, title) => {
    set((state) => (state.Titles[id] === title ? state : { Titles: { ...state.Titles, [id]: title } }));
  },
  Withdraw: (id) => {
    set((state) => {
      if (!Object.hasOwn(state.Titles, id)) return state;
      return { Titles: Object.fromEntries(Object.entries(state.Titles).filter(([key]) => key !== id)) };
    });
  },
}));
