import { create } from 'zustand';

interface ISetupTokenState {
  Token: string | null;
  Rejection: string | null;
  Set: (token: string) => void;
  Reject: (message: string) => void;
  Clear: () => void;
}

export const useSetupTokenStore = create<ISetupTokenState>((set) => ({
  Token: null,
  Rejection: null,
  Set: (token) => {
    set({ Token: token, Rejection: null });
  },
  Reject: (message) => {
    set({ Token: null, Rejection: message });
  },
  Clear: () => {
    set({ Token: null, Rejection: null });
  },
}));
