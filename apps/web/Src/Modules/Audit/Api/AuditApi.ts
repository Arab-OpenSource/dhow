import { ApiClient, request, requestPage, type IPage } from '@/Core/Api/Client';
import type { components } from '@/Core/Api/Schema';

export type TAuditEvent = components['schemas']['AuditEvent'];
export type TAuditVerify = components['schemas']['AuditVerifyResult'];
export type TAuditCategory = components['schemas']['AuditCategory'];

export interface IAuditQuery {
  Category?: TAuditCategory;
  ActorUserId?: string;
  From?: string;
  To?: string;
}

export const listAuditEvents = (organizationId: string, page: number, limit: number, query: IAuditQuery = {}): Promise<IPage<TAuditEvent>> =>
  requestPage(() =>
    ApiClient.GET('/organizations/{id}/audit-events', {
      params: { path: { id: organizationId }, query: { Page: page, Limit: limit, ...query } },
    }),
  );

export const getAuditEvent = (organizationId: string, eventId: string): Promise<TAuditEvent> =>
  request(() => ApiClient.GET('/organizations/{id}/audit-events/{eventId}', { params: { path: { id: organizationId, eventId } } }));

export const verifyAuditChain = (organizationId: string): Promise<TAuditVerify> =>
  request(() => ApiClient.GET('/organizations/{id}/audit-events/verify', { params: { path: { id: organizationId } } }));
