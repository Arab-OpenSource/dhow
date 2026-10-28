import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useToastStore } from '@/Core/Store/useToastStore';
import { useT } from '@/I18n/useT';
import { listMyTokens, revokeMyToken, TOKENS_QUERY_KEY, type TApiToken } from '@/Modules/Tokens/Api/TokensApi';

export const useMyTokens = (): UseQueryResult<TApiToken[], ApiError> =>
  useQuery<TApiToken[], ApiError>({ queryKey: TOKENS_QUERY_KEY, queryFn: listMyTokens });

export interface ITokenRevoke {
  Open: string | null;
  Busy: boolean;
  Notice: string | null;
  Ask: (id: string) => void;
  Cancel: () => void;
  Confirm: (token: TApiToken) => void;
}

export const useTokenRevoke = (): ITokenRevoke => {
  const t = useT();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const [Open, setOpen] = useState<string | null>(null);
  const [Busy, setBusy] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);

  return {
    Open,
    Busy,
    Notice,
    Ask: (id) => {
      setNotice(null);
      setOpen(id);
    },
    Cancel: () => {
      setOpen(null);
    },
    Confirm: (token) => {
      setBusy(true);
      setNotice(null);
      revokeMyToken(token.Id)
        .then(async () => {
          setOpen(null);
          await client.invalidateQueries({ queryKey: TOKENS_QUERY_KEY });
          Push({ Id: `token-revoked-${token.Id}`, Accent: 'live', Title: t('tokens.revoked', { name: token.Name }) });
        })
        .catch((error: unknown) => {
          setNotice(errorText(error));
        })
        .finally(() => {
          setBusy(false);
        });
    },
  };
};
