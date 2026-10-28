import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { factorMethods, getMfaStatus, MFA_QUERY_KEY, type TFactorMethod, type TMfaStatus } from '@/Core/Auth/Factor';
import { passkeysSupported } from '@/Core/Auth/WebAuthn';

export const useMfaStatus = (): UseQueryResult<TMfaStatus, ApiError> =>
  useQuery<TMfaStatus, ApiError>({ queryKey: MFA_QUERY_KEY, queryFn: getMfaStatus });

export const useFactorMethods = (): TFactorMethod[] | undefined => {
  const status = useMfaStatus();
  return status.data === undefined ? undefined : factorMethods(status.data, passkeysSupported());
};
