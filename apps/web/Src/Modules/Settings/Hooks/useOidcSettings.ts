import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { getOidcSettings } from '@/Modules/Settings/Api/OidcApi';
import type { TOidcSettings } from '@/Modules/Settings/Types/OidcTypes';

export const OIDC_SETTINGS_KEY = ['instance', 'settings', 'oidc'] as const;
export const OIDC_PUBLIC_KEY = ['oidc', 'providers'] as const;

export const useOidcSettings = (): UseQueryResult<TOidcSettings, ApiError> =>
  useQuery<TOidcSettings, ApiError>({ queryKey: OIDC_SETTINGS_KEY, queryFn: getOidcSettings });
