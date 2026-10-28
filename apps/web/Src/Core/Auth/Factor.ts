import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';
import { getPasskey } from '@/Core/Auth/WebAuthn';

type TSchemas = components['schemas'];

export type TMfaStatus = TSchemas['MfaStatus'];
export type TCurrentFactor = TSchemas['CurrentFactor'];
export type TFactorMethod = TSchemas['FactorMethod'];

export interface IFactorDraft {
  Method: TFactorMethod;
  Value: string;
}

export const MFA_QUERY_KEY = ['mfa'] as const;

export const getMfaStatus = (): Promise<TMfaStatus> => request(() => ApiClient.GET('/me/mfa'));

export const passkeyFactor = async (): Promise<TCurrentFactor> => {
  const challenge = await request(() => ApiClient.POST('/me/mfa/passkey-options'));
  const credential = await getPasskey(challenge.Options);
  return { Method: 'passkey', ChallengeId: challenge.ChallengeId, Credential: credential };
};

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
