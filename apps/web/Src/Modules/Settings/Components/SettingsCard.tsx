import { useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import type { TCurrentFactor } from '@/Core/Auth/Factor';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, type TCardState } from '@/Core/Ui/CardStates';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { saveSettings } from '@/Modules/Settings/Api/SettingsApi';
import { SettingField } from '@/Modules/Settings/Components/SettingField';
import { SettingsConfirm } from '@/Modules/Settings/Components/SettingsConfirm';
import { INSTANCE_SETTINGS_KEY } from '@/Modules/Settings/Hooks/useInstanceSettings';
import { useSettingsDraft, type ISettingsDraft } from '@/Modules/Settings/Hooks/useSettingsDraft';
import { fieldsOf, isDefaultGroup, KEEP, versionOf } from '@/Modules/Settings/Types/SettingsFields';
import { isSecret, valueText, type IGroupSpec, type TInstanceSettings } from '@/Modules/Settings/Types/SettingsTypes';

export interface ISettingsCardProps {
  spec: IGroupSpec;
  settings: TInstanceSettings | undefined;
  state: TCardState;
  onRetry: () => void;
  footer?: ReactNode;
  emptyNote?: string;
}

interface IFieldsProps {
  spec: IGroupSpec;
  settings: TInstanceSettings;
  draft: ISettingsDraft;
  disabled: boolean;
}

const Fields = ({ spec, settings, draft, disabled }: IFieldsProps) => (
  <div className="grid items-start gap-4 md:grid-cols-2">
    {fieldsOf(settings, spec).map(({ field, setting }) => (
      <SettingField
        key={field.Name}
        spec={field}
        setting={setting}
        draft={draft.Drafts[field.Name] ?? (isSecret(setting) ? '' : valueText(setting))}
        secret={draft.Secrets[field.Name] ?? KEEP}
        error={draft.Errors[field.Name]}
        disabled={disabled}
        onChange={(value) => {
          draft.SetDraft(field.Name, value);
        }}
        onSecret={(value) => {
          draft.SetSecret(field.Name, value);
        }}
      />
    ))}
  </div>
);

export const SettingsCard = ({ spec, settings, state, onRetry, footer, emptyNote }: ISettingsCardProps) => {
  const t = useT();
  const client = useQueryClient();
  const Push = useToastStore((s) => s.Push);
  const draft = useSettingsDraft(spec, settings);
  const [Confirming, setConfirming] = useState(false);

  const HandleSave = () => {
    const list = draft.Changes();
    if (list === null) return;
    if (Object.keys(list).length === 0) draft.Reset();
    else setConfirming(true);
  };

  const save = async (factor: TCurrentFactor) => {
    try {
      return await saveSettings(spec.Path, settings === undefined ? 0 : versionOf(settings, spec), draft.Changes() ?? {}, factor);
    } catch (error) {
      if (isApiError(error) && error.Details.length > 0) {
        draft.SetErrors(Object.fromEntries(error.Details.map((detail) => [detail.Field, detail.Message])));
        setConfirming(false);
      }
      if (isApiError(error) && error.Code === 'SETTING_CHANGED') void client.invalidateQueries({ queryKey: INSTANCE_SETTINGS_KEY });
      throw error;
    }
  };

  return (
    <Card title={t(spec.Title)} icon={spec.Icon}>
      <CardStates
        state={state}
        loadingLabel={t('settings.loading')}
        errorTitle={t('settings.error')}
        retryLabel={t('settings.retry')}
        onRetry={onRetry}
        emptyIcon={spec.Icon}
        emptyText={t('settings.error')}
        lines={spec.Fields.length + 1}
      >
        {settings === undefined ? null : (
          <div className="grid gap-4 px-card pt-1 pb-card">
            <p className="m-0 text-body-sm text-secondary">{t(spec.Lead)}</p>
            {emptyNote && isDefaultGroup(settings, spec) ? <InlineAlert accent="building" title={emptyNote} /> : null}
            <Fields spec={spec} settings={settings} draft={draft} disabled={Confirming} />
            {Confirming ? null : (
              <div className="flex flex-wrap items-center gap-2">
                <Button variant="primary" disabled={!draft.Dirty} onClick={HandleSave}>
                  {t('settings.save')}
                </Button>
                {draft.Dirty ? (
                  <Button variant="ghost" onClick={draft.Reset}>
                    {t('settings.discard')}
                  </Button>
                ) : null}
                {footer}
              </div>
            )}
          </div>
        )}
        {Confirming && settings !== undefined ? (
          <SettingsConfirm
            confirm={t(spec.Confirm)}
            save={save}
            onSaved={(saved) => {
              client.setQueryData(INSTANCE_SETTINGS_KEY, saved);
              Push({ Id: `settings-saved-${spec.Path}-${String(Date.now())}`, Accent: 'live', Title: t('settings.saved') });
              draft.Reset();
              setConfirming(false);
            }}
            onCancel={() => {
              setConfirming(false);
            }}
          />
        ) : null}
      </CardStates>
    </Card>
  );
};
