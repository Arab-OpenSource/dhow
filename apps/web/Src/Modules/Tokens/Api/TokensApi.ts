import { ApiClient, request } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';
import type { TCurrentFactor } from '@/Core/Auth/Factor';

export type TApiToken = components['schemas']['ApiToken'];
export type TApiTokenScope = components['schemas']['ApiTokenScope'];
export type TCreatedApiToken = components['schemas']['CreatedApiToken'];

export const ALL_TOKENS_QUERY_KEY = ['api-tokens'] as const;
export const TOKENS_QUERY_KEY = [...ALL_TOKENS_QUERY_KEY, 'mine'] as const;
export const organizationTokensKey = (organizationId: string) => [...ALL_TOKENS_QUERY_KEY, 'organization', organizationId] as const;

export interface ICreateTokenInput {
  OrganizationId: string;
  Name: string;
  Scopes: TApiTokenScope[];
  ExpiresInDays: number;
}

export const listMyTokens = (): Promise<TApiToken[]> => request(() => ApiClient.GET('/me/api-tokens'));

export const createToken = (input: ICreateTokenInput, factor: TCurrentFactor): Promise<TCreatedApiToken> =>
  request(() =>
    ApiClient.POST('/organizations/{id}/api-tokens', {
      params: { path: { id: input.OrganizationId } },
      body: { Name: input.Name, Scopes: input.Scopes, ExpiresInDays: input.ExpiresInDays, CurrentFactor: factor },
    }),
  );

export const revokeMyToken = (id: string): Promise<TApiToken> =>
  request(() => ApiClient.POST('/me/api-tokens/{id}/revoke', { params: { path: { id } } }));

export const listOrganizationTokens = (organizationId: string): Promise<TApiToken[]> =>
  request(() => ApiClient.GET('/organizations/{id}/api-tokens', { params: { path: { id: organizationId } } }));

export const revokeOrganizationToken = (organizationId: string, tokenId: string): Promise<TApiToken> =>
  request(() =>
    ApiClient.POST('/organizations/{id}/api-tokens/{tokenId}/revoke', { params: { path: { id: organizationId, tokenId } } }),
  );
