import { create } from 'zustand';

export interface IOidcCallback {
  Provider: string;
  Code: string;
  State: string;
  Error: string;
}

interface IOidcCallbackState {
  Callback: IOidcCallback | null;
  Set: (callback: IOidcCallback) => void;
  Take: () => IOidcCallback | null;
}

export const useOidcCallbackStore = create<IOidcCallbackState>((set, get) => ({
  Callback: null,
  Set: (callback) => {
    set({ Callback: callback });
  },
  Take: () => {
    const { Callback } = get();
    set({ Callback: null });
    return Callback;
  },
}));
