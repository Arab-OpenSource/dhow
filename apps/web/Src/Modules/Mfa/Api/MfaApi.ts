import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';
import { createPasskey, getPasskey } from '@/Core/Auth/WebAuthn';

type TSchemas = components['schemas'];

export type TMfaStatus = TSchemas['MfaStatus'];
export type TPasskey = TSchemas['Passkey'];
export type TMfaChange = TSchemas['MfaChange'];
export type TTotpEnrollment = TSchemas['TotpEnrollment'];
export type TCurrentFactor = TSchemas['CurrentFactor'];
export type TFactorMethod = TSchemas['FactorMethod'];
export type TAddPasskeyResult = TSchemas['AddPasskeyResult'];

export const getMfaStatus = (): Promise<TMfaStatus> => request(() => ApiClient.GET('/me/mfa'));

export const listPasskeys = (): Promise<TPasskey[]> => request(() => ApiClient.GET('/me/passkeys'));

export const startTotp = (): Promise<TTotpEnrollment> => request(() => ApiClient.POST('/me/totp'));

export const confirmTotp = (code: string, factor: TCurrentFactor): Promise<TMfaChange> =>
  request(() => ApiClient.POST('/me/totp/confirm', { body: { Code: code, CurrentFactor: factor } }));

export const disableTotp = (factor: TCurrentFactor): Promise<TMfaChange> =>
  request(() => ApiClient.POST('/me/totp/disable', { body: { CurrentFactor: factor } }));

export const regenerateRecoveryCodes = (factor: TCurrentFactor): Promise<TMfaChange> =>
  request(() => ApiClient.POST('/me/recovery-codes', { body: { CurrentFactor: factor } }));

export const renamePasskey = (id: string, name: string): Promise<TPasskey> =>
  request(() => ApiClient.PATCH('/me/passkeys/{id}', { params: { path: { id } }, body: { Name: name } }));

export const removePasskey = (id: string, factor: TCurrentFactor): Promise<TMfaChange> =>
  request(() => ApiClient.POST('/me/passkeys/{id}/remove', { params: { path: { id } }, body: { CurrentFactor: factor } }));

export const passkeyFactor = async (): Promise<TCurrentFactor> => {
  const challenge = await request(() => ApiClient.POST('/me/mfa/passkey-options'));
  const credential = await getPasskey(challenge.Options);
  return { Method: 'passkey', ChallengeId: challenge.ChallengeId, Credential: credential };
};

export const addPasskey = async (name: string, factor: () => Promise<TCurrentFactor>): Promise<TAddPasskeyResult> => {
  const challenge = await request(() => ApiClient.POST('/me/passkeys/options'));
  const credential = await createPasskey(challenge.Options);
  const current = await factor();
  return request(() =>
    ApiClient.POST('/me/passkeys', {
      body: { ChallengeId: challenge.ChallengeId, Credential: credential, Name: name, CurrentFactor: current },
    }),
  );
};
