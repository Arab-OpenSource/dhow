import { useState } from 'react';

import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import { useRevokeOtherSessions, useRevokeSession } from '@/Modules/Session/Hooks/useSessions';

export const ALL_OTHERS = 'others';

export interface ISessionActions {
  Confirming: string | null;
  Notice: string | null;
  Busy: boolean;
  Ask: (target: string) => void;
  Cancel: () => void;
  Revoke: (id: string) => void;
  RevokeOthers: () => void;
}

export const useSessionActions = (): ISessionActions => {
  const t = useT();
  const errorText = useApiErrorText();
  const Push = useToastStore((state) => state.Push);
  const revoke = useRevokeSession();
  const revokeOthers = useRevokeOtherSessions();
  const [Confirming, setConfirming] = useState<string | null>(null);
  const [Notice, setNotice] = useState<string | null>(null);

  const finish = (error?: Error) => {
    setConfirming(null);
    setNotice(error ? errorText(error) : null);
  };

  return {
    Confirming,
    Notice,
    Busy: revoke.isPending || revokeOthers.isPending,
    Ask: setConfirming,
    Cancel: () => {
      setConfirming(null);
    },
    Revoke: (id) => {
      setNotice(null);
      revoke.mutate(id, {
        onSuccess: () => {
          finish();
          Push({ Id: `session-revoked-${id}`, Accent: 'live', Title: t('sessions.revoked'), Detail: t('sessions.revoked.detail') });
        },
        onError: finish,
      });
    },
    RevokeOthers: () => {
      setNotice(null);
      revokeOthers.mutate(undefined, {
        onSuccess: (count) => {
          finish();
          Push({
            Id: `sessions-revoked-others-${Date.now()}`,
            Accent: 'live',
            Title: count > 0 ? t('sessions.revokedOthers', { count }) : t('sessions.revokedOthers.none'),
          });
        },
        onError: finish,
      });
    },
  };
};
