import { create } from 'zustand';

interface IInviteTokenState {
  Token: string | null;
  Set: (token: string) => void;
  Clear: () => void;
}

export const useInviteTokenStore = create<IInviteTokenState>((set) => ({
  Token: null,
  Set: (token) => {
    set({ Token: token });
  },
  Clear: () => {
    set({ Token: null });
  },
}));
