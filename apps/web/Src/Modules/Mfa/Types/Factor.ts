import type { TCurrentFactor, TFactorMethod, TMfaStatus } from '@/Modules/Mfa/Api/MfaApi';

export interface IFactorDraft {
  Method: TFactorMethod;
  Value: string;
}

export const factorMethods = (status: TMfaStatus, passkeys: boolean): TFactorMethod[] => {
  if (!status.Enabled) return ['password'];
  const out: TFactorMethod[] = [];
  if (status.TotpEnabled) out.push('totp');
  if (passkeys && status.PasskeyCount > 0) out.push('passkey');
  if (status.RecoveryCodesRemaining > 0) out.push('recovery_code');
  return out;
};

export const firstDraft = (methods: readonly TFactorMethod[]): IFactorDraft => ({ Method: methods[0] ?? 'password', Value: '' });

export const typedFactor = (draft: IFactorDraft): TCurrentFactor | null => {
  const value = draft.Value.trim();
  if (draft.Method === 'passkey' || value === '') return null;
  return draft.Method === 'password' ? { Method: 'password', Password: draft.Value } : { Method: draft.Method, Code: value };
};
