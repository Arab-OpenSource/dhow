import { useState } from 'react';

import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import { removePasskey, renamePasskey, type TAddPasskeyResult, type TFactorMethod } from '@/Modules/Mfa/Api/MfaApi';
import { useFactorAction, type IFactorAction } from '@/Modules/Mfa/Hooks/useFactorAction';
import { useRefreshMfa } from '@/Modules/Mfa/Hooks/useMfa';

export interface IPasskeysCard {
  Removing: string | null;
  Adding: boolean;
  Codes: string[] | null;
  Notice: string | null;
  Remove: IFactorAction<unknown>;
  AskRemove: (id: string) => void;
  CancelRemove: () => void;
  ConfirmRemove: () => void;
  SetAdding: (adding: boolean) => void;
  Added: (result: TAddPasskeyResult) => void;
  Rename: (id: string, name: string) => Promise<boolean>;
  DoneCodes: () => void;
}

export const usePasskeysCard = (methods: readonly TFactorMethod[]): IPasskeysCard => {
  const t = useT();
  const errorText = useApiErrorText();
  const refresh = useRefreshMfa();
  const Push = useToastStore((state) => state.Push);
  const [Removing, setRemoving] = useState<string | null>(null);
  const [Adding, SetAdding] = useState(false);
  const [Codes, setCodes] = useState<string[] | null>(null);
  const [Notice, setNotice] = useState<string | null>(null);
  const Remove = useFactorAction<unknown>(methods, async (factor) => removePasskey(Removing ?? '', await factor()));

  return {
    Removing,
    Adding,
    Codes,
    Notice,
    Remove,
    AskRemove: (id) => {
      Remove.Reset();
      setRemoving(id);
    },
    CancelRemove: () => {
      setRemoving(null);
    },
    ConfirmRemove: () => {
      void Remove.Run().then(async (result) => {
        if (result === null) return;
        setRemoving(null);
        Remove.Reset();
        await refresh();
        Push({ Id: `passkey-removed-${Date.now()}`, Accent: 'live', Title: t('mfa.passkeys.removed'), Detail: t('mfa.revoked') });
      });
    },
    SetAdding,
    Added: (result) => {
      SetAdding(false);
      if (result.RecoveryCodes) setCodes(result.RecoveryCodes);
      void refresh();
      Push({ Id: `passkey-added-${Date.now()}`, Accent: 'live', Title: t('mfa.passkeys.added'), Detail: t('mfa.revoked') });
    },
    Rename: async (id, name) => {
      setNotice(null);
      try {
        await renamePasskey(id, name.trim());
        await refresh();
        Push({ Id: `passkey-renamed-${Date.now()}`, Accent: 'live', Title: t('mfa.passkeys.renamed') });
        return true;
      } catch (error) {
        setNotice(errorText(error));
        return false;
      }
    },
    DoneCodes: () => {
      setCodes(null);
    },
  };
};
