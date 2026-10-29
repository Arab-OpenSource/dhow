import { isSecret, type IFieldSpec, type IGroupSpec, type TAnySetting, type TInstanceSettings } from '@/Modules/Settings/Types/SettingsTypes';

export type TSecretDraft = { Mode: 'keep' } | { Mode: 'replace'; Value: string } | { Mode: 'clear' };

export const KEEP: TSecretDraft = { Mode: 'keep' };

export interface IBoundField {
  field: IFieldSpec;
  setting: TAnySetting;
}

export const fieldsOf = (settings: TInstanceSettings, spec: IGroupSpec): IBoundField[] => {
  const group = settings[spec.Key] as unknown as Record<string, TAnySetting | undefined>;
  return spec.Fields.flatMap((field) => {
    const setting = group[field.Name];
    return setting === undefined ? [] : [{ field, setting }];
  });
};

export const versionOf = (settings: TInstanceSettings, spec: IGroupSpec): number => settings[spec.Key].Version;

export const isDefaultGroup = (settings: TInstanceSettings, spec: IGroupSpec): boolean =>
  fieldsOf(settings, spec).every(({ setting }) => (isSecret(setting) ? !setting.Set : setting.Source === 'default'));
