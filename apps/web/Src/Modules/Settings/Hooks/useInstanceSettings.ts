import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import type { ApiError } from '@/Core/Api/ApiError';
import { getInstanceSettings } from '@/Modules/Settings/Api/SettingsApi';
import type { TInstanceSettings } from '@/Modules/Settings/Types/SettingsTypes';

export const INSTANCE_SETTINGS_KEY = ['instance', 'settings'] as const;

export const useInstanceSettings = (): UseQueryResult<TInstanceSettings, ApiError> =>
  useQuery<TInstanceSettings, ApiError>({ queryKey: INSTANCE_SETTINGS_KEY, queryFn: getInstanceSettings });
