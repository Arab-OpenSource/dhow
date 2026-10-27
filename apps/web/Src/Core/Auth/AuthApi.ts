import type { QueryClient } from '@tanstack/react-query';

import { ApiClient, type IFetchResult, request, requestVoid, unwrap } from '@/Core/Api/Client';
import { API_ERROR_NETWORK, ApiError, isApiError } from '@/Core/Api/ApiError';
import type { components } from '@/Core/Api/Schema';
import { getPasskey, type TWebAuthnJson } from '@/Core/Auth/WebAuthn';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';

export type TLoginResult = components['schemas']['LoginResult'];
export type TMfaPending = components['schemas']['MfaPending'];
export type TMfaMethod = components['schemas']['MfaMethod'];
export type TMfaLoginRequest = components['schemas']['MfaLoginRequest'];

export interface ICredentials {
  Email: string;
  Password: string;
}

export type TSignInOutcome = { Kind: 'session'; Result: TLoginResult } | { Kind: 'mfa'; Pending: TMfaPending };

const isPending = (value: TLoginResult | TMfaPending): value is TMfaPending => 'Methods' in value;

const established = (client: QueryClient, result: TLoginResult): TLoginResult => {
  client.clear();
  useCsrfStore.getState().Set(result.CsrfToken);
  return result;
};

const asApiError = (error: unknown): ApiError =>
  isApiError(error) ? error : new ApiError({ Code: API_ERROR_NETWORK, Message: '' }, 0);

export const signIn = async (client: QueryClient, credentials: ICredentials): Promise<TSignInOutcome> => {
  let data: TLoginResult | TMfaPending;
  try {
    const result: IFetchResult<TLoginResult | TMfaPending> = await ApiClient.POST('/sessions', { body: credentials });
    data = unwrap(result);
  } catch (error) {
    throw asApiError(error);
  }
  if (isPending(data)) return { Kind: 'mfa', Pending: data };
  return { Kind: 'session', Result: established(client, data) };
};

export const completeMfa = async (client: QueryClient, body: TMfaLoginRequest): Promise<TLoginResult> =>
  established(client, await request(() => ApiClient.POST('/sessions/mfa', { body })));

export const mfaPasskeyCredential = async (): Promise<TWebAuthnJson> => {
  const options = await request(() => ApiClient.POST('/sessions/mfa/passkey-options'));
  return getPasskey(options.Options);
};

export const signInWithPasskey = async (client: QueryClient): Promise<TLoginResult> => {
  const challenge = await request(() => ApiClient.POST('/sessions/passkey-options'));
  const credential = await getPasskey(challenge.Options);
  const result = await request(() =>
    ApiClient.POST('/sessions/passkey', { body: { ChallengeId: challenge.ChallengeId, Credential: credential } }),
  );
  return established(client, result);
};

export const signOut = async (): Promise<void> => {
  await requestVoid(() => ApiClient.DELETE('/sessions/current'));
  useCsrfStore.getState().Clear();
};
