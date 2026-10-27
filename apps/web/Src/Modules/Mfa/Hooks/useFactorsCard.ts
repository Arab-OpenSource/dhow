import { useState } from 'react';

import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import { disableTotp, regenerateRecoveryCodes, type TFactorMethod, type TMfaChange } from '@/Modules/Mfa/Api/MfaApi';
import { useFactorAction, type IFactorAction } from '@/Modules/Mfa/Hooks/useFactorAction';
import { useRefreshMfa } from '@/Modules/Mfa/Hooks/useMfa';

export type TFactorsOpen = 'disable' | 'regenerate' | null;

export interface IFactorsCard {
  Open: TFactorsOpen;
  Codes: string[] | null;
  Disable: IFactorAction<TMfaChange>;
  Regenerate: IFactorAction<TMfaChange>;
  SetOpen: (open: TFactorsOpen) => void;
  Close: () => void;
  ConfirmDisable: () => void;
  ConfirmRegenerate: () => void;
  DoneCodes: () => void;
}

export const useFactorsCard = (methods: readonly TFactorMethod[]): IFactorsCard => {
  const t = useT();
  const refresh = useRefreshMfa();
  const Push = useToastStore((state) => state.Push);
  const [Open, SetOpen] = useState<TFactorsOpen>(null);
  const [Codes, setCodes] = useState<string[] | null>(null);
  const Disable = useFactorAction(methods, async (factor) => disableTotp(await factor()));
  const Regenerate = useFactorAction(methods, async (factor) => regenerateRecoveryCodes(await factor()));

  const Close = () => {
    SetOpen(null);
    Disable.Reset();
    Regenerate.Reset();
  };

  return {
    Open,
    Codes,
    Disable,
    Regenerate,
    SetOpen,
    Close,
    ConfirmDisable: () => {
      void Disable.Run().then(async (change) => {
        if (change === null) return;
        Close();
        await refresh();
        Push({ Id: `totp-off-${Date.now()}`, Accent: 'live', Title: t('mfa.totp.disabled'), Detail: t('mfa.revoked') });
      });
    },
    ConfirmRegenerate: () => {
      void Regenerate.Run().then(async (change) => {
        if (change === null) return;
        Close();
        setCodes(change.RecoveryCodes ?? []);
        await refresh();
      });
    },
    DoneCodes: () => {
      setCodes(null);
    },
  };
};
