import { useState } from 'react';

import { useT } from '@/I18n/useT';
import type { TSettingsChanges } from '@/Modules/Settings/Api/SettingsApi';
import { fieldsOf, KEEP, type TSecretDraft } from '@/Modules/Settings/Types/SettingsFields';
import { isSecret, valueText, type IGroupSpec, type TInstanceSettings } from '@/Modules/Settings/Types/SettingsTypes';

const WHOLE = /^\d+$/;

export interface ISettingsDraft {
  Drafts: Record<string, string>;
  Secrets: Record<string, TSecretDraft>;
  Errors: Record<string, string>;
  Dirty: boolean;
  SetDraft: (name: string, value: string) => void;
  SetSecret: (name: string, draft: TSecretDraft) => void;
  SetErrors: (errors: Record<string, string>) => void;
  Changes: () => TSettingsChanges | null;
  Reset: () => void;
}

export const useSettingsDraft = (spec: IGroupSpec, settings: TInstanceSettings | undefined): ISettingsDraft => {
  const t = useT();
  const [Drafts, setDrafts] = useState<Record<string, string>>({});
  const [Secrets, setSecrets] = useState<Record<string, TSecretDraft>>({});
  const [Errors, SetErrors] = useState<Record<string, string>>({});

  const Changes = (): TSettingsChanges | null => {
    if (settings === undefined) return null;
    const out: TSettingsChanges = {};
    const bad: Record<string, string> = {};
    for (const { field, setting } of fieldsOf(settings, spec)) {
      if (isSecret(setting)) {
        const secret = Secrets[field.Name] ?? KEEP;
        if (secret.Mode === 'replace' && secret.Value !== '') out[field.Name] = secret.Value;
        if (secret.Mode === 'clear') out[field.Name] = '';
        continue;
      }
      const draft = Drafts[field.Name];
      if (draft === undefined || draft === valueText(setting)) continue;
      if (field.Kind === 'number') {
        if (WHOLE.test(draft.trim())) out[field.Name] = Number(draft.trim());
        else bad[field.Name] = t('settings.wholeNumber');
      } else if (typeof setting.Value === 'boolean') out[field.Name] = draft === 'true';
      else out[field.Name] = draft;
    }
    SetErrors(bad);
    return Object.keys(bad).length > 0 ? null : out;
  };

  return {
    Drafts,
    Secrets,
    Errors,
    Dirty: Object.keys(Drafts).length > 0 || Object.values(Secrets).some((secret) => secret.Mode !== 'keep'),
    SetDraft: (name, value) => {
      setDrafts((current) => ({ ...current, [name]: value }));
    },
    SetSecret: (name, draft) => {
      setSecrets((current) => ({ ...current, [name]: draft }));
    },
    SetErrors,
    Changes,
    Reset: () => {
      setDrafts({});
      setSecrets({});
      SetErrors({});
    },
  };
};
