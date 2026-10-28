import { keepPreviousData, useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import type { IPage } from '@/Core/Api/Client';
import { getAuditEvent, listAuditEvents, type IAuditQuery, type TAuditEvent } from '@/Modules/Audit/Api/AuditApi';

export const AUDIT_PAGE_SIZE = 20;
export const AUDIT_QUERY_KEY = ['audit', 'events'] as const;

export const useAuditEvents = (
  organizationId: string | undefined,
  page: number,
  query: IAuditQuery = {},
): UseQueryResult<IPage<TAuditEvent>, ApiError> =>
  useQuery<IPage<TAuditEvent>, ApiError>({
    queryKey: [...AUDIT_QUERY_KEY, organizationId, page, query],
    queryFn: () => listAuditEvents(organizationId ?? '', page, AUDIT_PAGE_SIZE, query),
    enabled: organizationId !== undefined,
    placeholderData: keepPreviousData,
    staleTime: 0,
  });

export const useAuditEvent = (organizationId: string, eventId: string): UseQueryResult<TAuditEvent, ApiError> =>
  useQuery<TAuditEvent, ApiError>({
    queryKey: ['audit', 'event', organizationId, eventId],
    queryFn: () => getAuditEvent(organizationId, eventId),
  });
