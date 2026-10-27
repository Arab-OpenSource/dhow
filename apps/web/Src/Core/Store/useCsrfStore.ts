import { create } from 'zustand';

interface ICsrfState {
  Token: string | null;
  Set: (token: string) => void;
  Clear: () => void;
}

export const useCsrfStore = create<ICsrfState>((set) => ({
  Token: null,
  Set: (token) => {
    set({ Token: token });
  },
  Clear: () => {
    set({ Token: null });
  },
}));
