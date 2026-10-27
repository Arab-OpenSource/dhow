import { ApiClient, request, requestPage, type IPage } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TAuditEvent = components['schemas']['AuditEvent'];
export type TAuditVerify = components['schemas']['AuditVerifyResult'];

export const listAuditEvents = (organizationId: string, page: number, limit: number): Promise<IPage<TAuditEvent>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/audit-events', {
      params: { path: { id: organizationId }, query: { Page: page, Limit: limit } },
    }),
  );

export const verifyAuditChain = (organizationId: string): Promise<TAuditVerify> =>
  request(() => ApiClient.GET('/organizations/{id}/audit-events/verify', { params: { path: { id: organizationId } } }));
