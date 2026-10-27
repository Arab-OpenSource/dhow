import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState, type RefObject } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { completeMfa, mfaPasskeyCredential, type TMfaLoginRequest, type TMfaPending } from '@/Core/Auth/AuthApi';
import { PasskeyCancelled, passkeysSupported } from '@/Core/Auth/WebAuthn';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useCountdown, type ICountdown } from '@/Core/Hooks/useCountdown';
import { useT } from '@/I18n/useT';
import { cleanCode, firstCodeMethod, type TCodeMethod } from '@/Modules/Session/Types/MfaCode';

const EXPIRED = 'MFA_CHALLENGE_EXPIRED';

export interface IMfaStepOptions {
  pending: TMfaPending;
  onSignedIn: () => void;
  onBack: (expired: boolean) => void;
  inputRef: RefObject<HTMLInputElement | null>;
}

export interface IMfaStep {
  Method: TCodeMethod | null;
  Other: TCodeMethod | null;
  Passkey: boolean;
  Code: string;
  CodeError: string | null;
  Notice: string | null;
  Busy: boolean;
  Countdown: ICountdown;
  SetCode: (value: string) => void;
  Switch: () => void;
  SubmitCode: () => void;
  UsePasskey: () => void;
}

const otherOf = (method: TCodeMethod | null, pending: TMfaPending): TCodeMethod | null => {
  if (method !== 'recovery_code' && pending.Methods.includes('recovery_code')) return 'recovery_code';
  if (method === 'recovery_code' && pending.Methods.includes('totp')) return 'totp';
  return null;
};

export const useMfaStep = ({ pending, onSignedIn, onBack, inputRef: InputRef }: IMfaStepOptions): IMfaStep => {
  const t = useT();
  const client = useQueryClient();
  const errorText = useApiErrorText();
  const Countdown = useCountdown();
  const [Method, setMethod] = useState<TCodeMethod | null>(firstCodeMethod(pending.Methods));
  const [Code, SetCode] = useState('');
  const [CodeError, setCodeError] = useState<string | null>(null);
  const [Notice, setNotice] = useState<string | null>(null);
  const [Busy, setBusy] = useState(false);

  useEffect(() => {
    InputRef.current?.focus();
  }, [InputRef, Method]);

  const finish = async (body: () => Promise<TMfaLoginRequest>) => {
    setBusy(true);
    setNotice(null);
    try {
      await completeMfa(client, await body());
      onSignedIn();
    } catch (error) {
      if (isApiError(error) && error.Code === EXPIRED) onBack(true);
      else if (isApiError(error) && error.RetryAfterSeconds !== null) Countdown.Start(error.RetryAfterSeconds);
      else if (error instanceof PasskeyCancelled) setNotice(t('mfa.passkeys.cancelled'));
      else setNotice(errorText(error));
      SetCode('');
      InputRef.current?.focus();
    } finally {
      setBusy(false);
    }
  };

  return {
    Method,
    Other: otherOf(Method, pending),
    Passkey: pending.Methods.includes('passkey') && passkeysSupported(),
    Code,
    CodeError,
    Notice,
    Busy,
    Countdown,
    SetCode,
    Switch: () => {
      setMethod(otherOf(Method, pending));
      SetCode('');
      setCodeError(null);
    },
    SubmitCode: () => {
      if (Method === null || Countdown.Remaining > 0) return;
      const clean = cleanCode(Method, Code);
      if (clean === null) {
        setCodeError(Method === 'totp' ? t('mfa.step.error.code') : t('mfa.step.error.recovery'));
        InputRef.current?.focus();
        return;
      }
      setCodeError(null);
      void finish(() => Promise.resolve({ Method, Code: clean }));
    },
    UsePasskey: () => {
      void finish(async () => ({ Method: 'passkey', Credential: await mfaPasskeyCredential() }));
    },
  };
};
