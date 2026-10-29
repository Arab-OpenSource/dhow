import { ApiClient, request } from '@/Core/Api/Client';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import type { TGroupPath, TInstanceSettings, TMailTest } from '@/Modules/Settings/Types/SettingsTypes';

export type TSettingsChanges = Record<string, string | number | boolean>;

export const getInstanceSettings = (): Promise<TInstanceSettings> => request(() => ApiClient.GET('/instance/settings'));

export const saveSettings = (
  path: TGroupPath,
  version: number,
  changes: TSettingsChanges,
  factor: TCurrentFactor,
): Promise<TInstanceSettings> => {
  const body = { ...changes, Version: version, CurrentFactor: factor };
  switch (path) {
    case 'mail':
      return request(() => ApiClient.PUT('/instance/settings/mail', { body }));
    case 'organizations':
      return request(() => ApiClient.PUT('/instance/settings/organizations', { body }));
    case 'passwords':
      return request(() => ApiClient.PUT('/instance/settings/passwords', { body }));
    case 'rate-limits':
      return request(() => ApiClient.PUT('/instance/settings/rate-limits', { body }));
    case 'logs':
      return request(() => ApiClient.PUT('/instance/settings/logs', { body }));
  }
};

export const sendTestMail = (): Promise<TMailTest> => request(() => ApiClient.POST('/instance/settings/mail/test'));
