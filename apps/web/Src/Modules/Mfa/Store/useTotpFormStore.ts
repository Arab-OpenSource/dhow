import { create } from 'zustand';

interface ITotpFormState {
  Busy: boolean;
  Wait: number;
  Ready: boolean;
  Report: (busy: boolean, wait: number, ready: boolean) => void;
  Reset: () => void;
}

export const TOTP_FORM_ID = 'totp-form';

export const useTotpFormStore = create<ITotpFormState>((set) => ({
  Busy: false,
  Wait: 0,
  Ready: false,
  Report: (busy, wait, ready) => {
    set({ Busy: busy, Wait: wait, Ready: ready });
  },
  Reset: () => {
    set({ Busy: false, Wait: 0, Ready: false });
  },
}));
