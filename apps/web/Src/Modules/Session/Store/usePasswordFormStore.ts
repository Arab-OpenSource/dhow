import { create } from 'zustand';

interface IPasswordFormState {
  Busy: boolean;
  Wait: number;
  Report: (busy: boolean, wait: number) => void;
  Reset: () => void;
}

export const usePasswordFormStore = create<IPasswordFormState>((set) => ({
  Busy: false,
  Wait: 0,
  Report: (busy, wait) => {
    set({ Busy: busy, Wait: wait });
  },
  Reset: () => {
    set({ Busy: false, Wait: 0 });
  },
}));
