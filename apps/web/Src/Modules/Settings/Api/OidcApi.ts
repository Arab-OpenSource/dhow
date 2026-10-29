import { ApiClient, request } from '@/Core/Api/Client';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import type { TOidcSettings, TOidcTest, TOidcType } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcCreate {
  Id: string;
  Type: TOidcType;
  Name: string;
  Issuer: string;
  ClientId: string;
  ClientSecret: string;
  AllowedDomains: string[];
}

export interface IOidcChanges {
  Type?: TOidcType;
  Name?: string;
  Issuer?: string;
  ClientId?: string;
  ClientSecret?: string;
  AllowedDomains?: string[];
}

export const getOidcSettings = (): Promise<TOidcSettings> => request(() => ApiClient.GET('/instance/settings/oidc'));

export const createOidcProvider = (body: IOidcCreate, factor: TCurrentFactor): Promise<TOidcSettings> =>
  request(() => ApiClient.POST('/instance/settings/oidc', { body: { ...body, CurrentFactor: factor } }));

export const updateOidcProvider = (id: string, version: number, changes: IOidcChanges, factor: TCurrentFactor): Promise<TOidcSettings> =>
  request(() =>
    ApiClient.PUT('/instance/settings/oidc/{providerId}', {
      params: { path: { providerId: id } },
      body: { ...changes, Version: version, CurrentFactor: factor },
    }),
  );

export const removeOidcProvider = (id: string, version: number, factor: TCurrentFactor): Promise<TOidcSettings> =>
  request(() =>
    ApiClient.POST('/instance/settings/oidc/{providerId}/remove', {
      params: { path: { providerId: id } },
      body: { Version: version, CurrentFactor: factor },
    }),
  );

export const testOidcProvider = (type: TOidcType, issuer: string): Promise<TOidcTest> =>
  request(() => ApiClient.POST('/instance/settings/oidc/test', { body: { Type: type, Issuer: issuer } }));
