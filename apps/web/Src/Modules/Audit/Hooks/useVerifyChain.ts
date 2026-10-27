import { useMutation, useMutationState } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { verifyAuditChain, type TAuditVerify } from '@/Modules/Audit/Api/AuditApi';

export type TVerifyState =
  | { Status: 'idle' }
  | { Status: 'running' }
  | { Status: 'error'; Error: ApiError }
  | { Status: 'done'; Result: TAuditVerify };

const verifyKey = (organizationId: string) => ['audit', 'verify', organizationId] as const;

export const useVerifyChain = (organizationId: string) =>
  useMutation<TAuditVerify, ApiError>({
    mutationKey: verifyKey(organizationId),
    mutationFn: () => verifyAuditChain(organizationId),
    gcTime: 0,
  });

export const useVerifyState = (organizationId: string): TVerifyState => {
  const states = useMutationState({
    filters: { mutationKey: verifyKey(organizationId) },
    select: (mutation) => mutation.state,
  });
  const latest = states.at(-1);
  if (latest === undefined || latest.status === 'idle') return { Status: 'idle' };
  if (latest.status === 'pending') return { Status: 'running' };
  if (latest.status === 'error') return { Status: 'error', Error: latest.error as ApiError };
  return { Status: 'done', Result: latest.data as TAuditVerify };
};
