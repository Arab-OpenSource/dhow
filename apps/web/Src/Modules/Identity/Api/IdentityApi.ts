import { ApiClient, request, requestPage, type IPage } from '@/Core/Api/Client';
import type { TOrganization } from '@/Core/Auth/AuthTypes';
import type { components } from '@/Core/Api/Schema';

export type TSetupRequest = components['schemas']['SetupRequest'];
export type TSetupResult = components['schemas']['SetupResult'];
export type TMember = components['schemas']['Member'];

export const setupInstance = (body: TSetupRequest): Promise<TSetupResult> =>
  request(() => ApiClient.POST('/setup', { body }));

export const getOrganization = (id: string): Promise<TOrganization> =>
  request(() => ApiClient.GET('/organizations/{id}', { params: { path: { id } } }));

export const renameOrganization = (id: string, name: string): Promise<TOrganization> =>
  request(() => ApiClient.PATCH('/organizations/{id}', { params: { path: { id } }, body: { Name: name } }));

export const listMembers = (id: string, page: number, limit: number): Promise<IPage<TMember>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/members', { params: { path: { id }, query: { Page: page, Limit: limit } } }),
  );
