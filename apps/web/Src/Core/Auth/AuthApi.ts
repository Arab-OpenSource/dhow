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
export type TOidcProvider = components['schemas']['OidcProvider'];
export type TOidcCallback = components['schemas']['OidcCallbackRequest'];
export type TOidcAuthorization = components['schemas']['OidcAuthorization'];

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

const outcome = async (
  client: QueryClient,
  call: () => Promise<IFetchResult<TLoginResult | TMfaPending>>,
): Promise<TSignInOutcome> => {
  let data: TLoginResult | TMfaPending;
  try {
    data = unwrap(await call());
  } catch (error) {
    throw asApiError(error);
  }
  if (isPending(data)) return { Kind: 'mfa', Pending: data };
  return { Kind: 'session', Result: established(client, data) };
};

export const signIn = (client: QueryClient, credentials: ICredentials): Promise<TSignInOutcome> =>
  outcome(client, () => ApiClient.POST('/sessions', { body: credentials }));

export const listOidcProviders = (): Promise<TOidcProvider[]> => request(() => ApiClient.GET('/oidc/providers'));

export const beginOidcLogin = (provider: string): Promise<TOidcAuthorization> =>
  request(() => ApiClient.POST('/oidc/authorize', { body: { Provider: provider } }));

export const completeOidcLogin = (client: QueryClient, body: TOidcCallback): Promise<TSignInOutcome> =>
  outcome(client, () => ApiClient.POST('/oidc/callback', { body }));

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
