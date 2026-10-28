import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { firstDraft, passkeyFactor, typedFactor, type IFactorDraft, type TCurrentFactor, type TFactorMethod } from '@/Core/Auth/Factor';
import { PasskeyCancelled } from '@/Core/Auth/WebAuthn';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useCountdown, type ICountdown } from '@/Core/Hooks/useCountdown';
import { useT } from '@/I18n/useT';

export interface IFactorAction<TResult> {
  Draft: IFactorDraft;
  SetDraft: (draft: IFactorDraft) => void;
  Busy: boolean;
  Notice: string | null;
  Countdown: ICountdown;
  Run: () => Promise<TResult | null>;
  Reset: () => void;
}

export const useFactorAction = <TResult>(
  methods: readonly TFactorMethod[],
  action: (factor: () => Promise<TCurrentFactor>) => Promise<TResult>,
): IFactorAction<TResult> => {
  const t = useT();
  const errorText = useApiErrorText();
  const Countdown = useCountdown();
  const [Stored, SetDraft] = useState<IFactorDraft>(firstDraft(methods));
  const Draft = methods.includes(Stored.Method) ? Stored : firstDraft(methods);
  const [Busy, setBusy] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);

  const factor = (): Promise<TCurrentFactor> => {
    if (Draft.Method === 'passkey') return passkeyFactor();
    const typed = typedFactor(Draft);
    return typed === null ? Promise.reject(new Error(t('factor.error.required'))) : Promise.resolve(typed);
  };

  const Run = async (): Promise<TResult | null> => {
    if (Countdown.Remaining > 0) return null;
    if (Draft.Method !== 'passkey' && typedFactor(Draft) === null) {
      setNotice(t('factor.error.required'));
      return null;
    }
    setBusy(true);
    setNotice(null);
    try {
      return await action(factor);
    } catch (error) {
      if (isApiError(error) && error.RetryAfterSeconds !== null) Countdown.Start(error.RetryAfterSeconds);
      if (error instanceof PasskeyCancelled) setNotice(t('mfa.passkeys.cancelled'));
      else setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
      SetDraft((current) => ({ ...current, Value: '' }));
      return null;
    } finally {
      setBusy(false);
    }
  };

  return {
    Draft,
    SetDraft,
    Busy,
    Notice,
    Countdown,
    Run,
    Reset: () => {
      setNotice(null);
      SetDraft(firstDraft(methods));
    },
  };
};
