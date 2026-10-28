import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';
import type { TOidcAuthorization, TOidcCallback } from '@/Core/Auth/AuthApi';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { fetchMe } from '@/Core/Auth/MeQuery';
import { useCsrfStore } from '@/Core/Store/useCsrfStore';

export type TLinkedIdentity = components['schemas']['LinkedIdentity'];
export type TLinkedIdentityChange = components['schemas']['LinkedIdentityChange'];

export const IDENTITIES_QUERY_KEY = ['identities'] as const;
export const OIDC_PROVIDERS_QUERY_KEY = ['oidc-providers'] as const;

export const listIdentities = (): Promise<TLinkedIdentity[]> => request(() => ApiClient.GET('/me/identities'));

export const beginLink = (provider: string, factor: TCurrentFactor): Promise<TOidcAuthorization> =>
  request(() => ApiClient.POST('/me/identities/authorize', { body: { Provider: provider, CurrentFactor: factor } }));

const ensureCsrf = async (): Promise<void> => {
  if (useCsrfStore.getState().Token === null) await fetchMe();
};

export const completeLink = async (body: TOidcCallback): Promise<TLinkedIdentityChange> => {
  await ensureCsrf();
  return request(() => ApiClient.POST('/me/identities/callback', { body }));
};

export const removeIdentity = async (id: string, factor: TCurrentFactor): Promise<number> => {
  const result = await request(() =>
    ApiClient.POST('/me/identities/{id}/remove', { params: { path: { id } }, body: { CurrentFactor: factor } }),
  );
  return result.Revoked;
};
