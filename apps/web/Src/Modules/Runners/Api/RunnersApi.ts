import { ApiClient, request, requestPage, type IPage } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TRunner = components['schemas']['Runner'];
export type TRunnerStatus = components['schemas']['RunnerStatus'];
export type TRunnerIsolation = components['schemas']['RunnerIsolation'];
export type TUpdateRunner = components['schemas']['UpdateRunnerRequest'];
export type TRegistrationToken = components['schemas']['RunnerRegistrationToken'];
export type TRegistrationTokenStatus = components['schemas']['RunnerRegistrationTokenStatus'];
export type TCreateRegistrationToken = components['schemas']['CreateRunnerRegistrationTokenRequest'];
export type TCreatedRegistrationToken = components['schemas']['CreatedRunnerRegistrationToken'];

export const RUNNERS_PAGE_SIZE = 20;
export const TOKENS_PAGE_SIZE = 10;

export const runnersKey = (organizationId: string) => ['runners', organizationId] as const;
export const runnerListKey = (organizationId: string) => [...runnersKey(organizationId), 'list'] as const;
export const runnerKey = (organizationId: string, runnerId: string) => [...runnersKey(organizationId), 'detail', runnerId] as const;
export const registrationTokensKey = (organizationId: string) => ['runner-registration-tokens', organizationId] as const;

export const listRunners = (organizationId: string, page: number): Promise<IPage<TRunner>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/runners', {
      params: { path: { id: organizationId }, query: { Page: page, Limit: RUNNERS_PAGE_SIZE } },
    }),
  );

export const getRunner = (organizationId: string, runnerId: string): Promise<TRunner> =>
  request(() => ApiClient.GET('/organizations/{id}/runners/{runnerId}', { params: { path: { id: organizationId, runnerId } } }));

export const updateRunner = (organizationId: string, runnerId: string, body: TUpdateRunner): Promise<TRunner> =>
  request(() =>
    ApiClient.PATCH('/organizations/{id}/runners/{runnerId}', { params: { path: { id: organizationId, runnerId } }, body }),
  );

export const revokeRunner = (organizationId: string, runnerId: string): Promise<TRunner> =>
  request(() =>
    ApiClient.POST('/organizations/{id}/runners/{runnerId}/revoke', { params: { path: { id: organizationId, runnerId } } }),
  );

export const listRegistrationTokens = (organizationId: string, page: number): Promise<IPage<TRegistrationToken>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/runner-registration-tokens', {
      params: { path: { id: organizationId }, query: { Page: page, Limit: TOKENS_PAGE_SIZE } },
    }),
  );

export const createRegistrationToken = (
  organizationId: string,
  body: TCreateRegistrationToken,
): Promise<TCreatedRegistrationToken> =>
  request(() => ApiClient.POST('/organizations/{id}/runner-registration-tokens', { params: { path: { id: organizationId } }, body }));

export const revokeRegistrationToken = (organizationId: string, tokenId: string): Promise<TRegistrationToken> =>
  request(() =>
    ApiClient.POST('/organizations/{id}/runner-registration-tokens/{tokenId}/revoke', {
      params: { path: { id: organizationId, tokenId } },
    }),
  );
