import { create } from 'zustand';

import type { TInvitationIssued } from '@/Modules/Organization/Api/OrganizationApi';

interface IInviteState {
  Busy: boolean;
  Issued: TInvitationIssued | null;
  SetBusy: (busy: boolean) => void;
  SetIssued: (issued: TInvitationIssued | null) => void;
}

export const useInviteStore = create<IInviteState>((set) => ({
  Busy: false,
  Issued: null,
  SetBusy: (busy) => {
    set({ Busy: busy });
  },
  SetIssued: (issued) => {
    set({ Issued: issued, Busy: false });
  },
}));
