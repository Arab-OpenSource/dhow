import type { components } from '@/Core/Api/Schema';
import type { TMessageKey } from '@/I18n/Messages';

type TSchemas = components['schemas'];

export type TInstanceSettings = TSchemas['InstanceSettings'];
export type TStringSetting = TSchemas['StringSetting'];
export type TIntegerSetting = TSchemas['IntegerSetting'];
export type TBooleanSetting = TSchemas['BooleanSetting'];
export type TSecretSetting = TSchemas['SecretSetting'];
export type TMailTest = TSchemas['MailTest'];

export type TGroupKey = keyof TInstanceSettings;
export type TGroupPath = 'mail' | 'organizations' | 'passwords' | 'rate-limits' | 'logs';
export type TValueSetting = TStringSetting | TIntegerSetting | TBooleanSetting;
export type TAnySetting = TValueSetting | TSecretSetting;

export type TFieldKind = 'text' | 'number' | 'choice' | 'secret';

export interface IChoiceSpec {
  Value: string;
  Label: TMessageKey;
}

export interface IFieldSpec {
  Name: string;
  Kind: TFieldKind;
  Label: TMessageKey;
  Hint?: TMessageKey;
  HintParams?: Record<string, string>;
  Choices?: readonly IChoiceSpec[];
  Ltr?: boolean;
}

export interface IGroupSpec {
  Key: TGroupKey;
  Path: TGroupPath;
  Title: TMessageKey;
  Lead: TMessageKey;
  Icon: 'mail' | 'building' | 'key' | 'shield' | 'file';
  Confirm: TMessageKey;
  Fields: readonly IFieldSpec[];
}

export const isSecret = (setting: TAnySetting): setting is TSecretSetting => 'Set' in setting;

export const valueText = (setting: TValueSetting): string => String(setting.Value);
