import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { Icon } from '@/Core/Ui/Icon';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { Select } from '@/Core/Ui/Select';
import { useT } from '@/I18n/useT';
import type { TSecretDraft } from '@/Modules/Settings/Types/SettingsFields';
import {
  isSecret,
  valueText,
  type IFieldSpec,
  type TAnySetting,
  type TSecretSetting,
  type TValueSetting,
} from '@/Modules/Settings/Types/SettingsTypes';

export interface ISettingFieldProps {
  spec: IFieldSpec;
  setting: TAnySetting;
  draft: string;
  secret: TSecretDraft;
  error?: string;
  disabled: boolean;
  onChange: (value: string) => void;
  onSecret: (draft: TSecretDraft) => void;
}

const EnvNote = ({ env, byPassword = false }: { env: string; byPassword?: boolean }) => {
  const t = useT();
  return (
    <span className="flex flex-wrap items-center gap-1.5 text-body-sm text-tertiary">
      <Icon name="key" size={14} />
      {byPassword ? t('settings.lockedByPassword') : t('settings.lockedByEnv')}
      <code dir="ltr" className="rounded-md border border-line bg-surface-inset px-1.5 font-mono text-code text-secondary">
        {env}
      </code>
    </span>
  );
};

const LockedValue = ({ spec, setting }: { spec: IFieldSpec; setting: TValueSetting }) => {
  const t = useT();
  const raw = valueText(setting);
  const choice = spec.Choices?.find((option) => option.Value === raw);
  const shown = choice ? t(choice.Label) : raw;
  return (
    <div className="grid gap-1.5">
      <span className="text-body-sm text-secondary">{t(spec.Label)}</span>
      <span dir={spec.Ltr ? 'ltr' : undefined} className="text-start text-body text-primary">
        {shown === '' ? t('settings.empty') : shown}
      </span>
      <EnvNote env={setting.LockedBy ?? setting.Env} byPassword={setting.LockedBy !== undefined && setting.LockedBy !== setting.Env} />
    </div>
  );
};

type TLockedCopyProps = Pick<ISettingFieldProps, 'secret' | 'disabled' | 'onSecret'>;

const LockedCopy = ({ secret, disabled, onSecret }: TLockedCopyProps) => {
  const t = useT();
  const clearing = secret.Mode === 'clear';
  return (
    <div className="grid justify-items-start gap-1.5">
      <p className="m-0 text-body-sm text-secondary">{clearing ? t('settings.secret.willClearCopy') : t('settings.secret.unusedCopy')}</p>
      <Button
        size="sm"
        variant={clearing ? 'ghost' : 'secondary'}
        disabled={disabled}
        onClick={() => {
          onSecret(clearing ? { Mode: 'keep' } : { Mode: 'clear' });
        }}
      >
        {clearing ? t('settings.secret.keepCopy') : t('settings.secret.clearCopy')}
      </Button>
    </div>
  );
};

const SecretValue = ({
  spec,
  setting,
  secret,
  disabled,
  error,
  onSecret,
}: Pick<ISettingFieldProps, 'spec' | 'secret' | 'disabled' | 'error' | 'onSecret'> & { setting: TSecretSetting }) => {
  const t = useT();
  const saved = setting.Set && (setting.Locked || secret.Mode !== 'clear');
  return (
    <div className="grid gap-1.5">
      <span className="flex flex-wrap items-center gap-2 text-body-sm text-secondary">
        {t(spec.Label)}
        <Badge tone={saved ? 'ok' : 'mute'}>{saved ? t('settings.secret.saved') : t('settings.secret.notSaved')}</Badge>
      </span>
      {setting.Locked ? <EnvNote env={setting.Env} /> : null}
      {setting.Locked && setting.Saved ? <LockedCopy secret={secret} disabled={disabled} onSecret={onSecret} /> : null}
      {!setting.Locked && secret.Mode === 'replace' ? (
        <Field label={t('settings.secret.new')} error={error}>
          <PasswordInput
            autoComplete="new-password"
            value={secret.Value}
            disabled={disabled}
            onChange={(event) => {
              onSecret({ Mode: 'replace', Value: event.target.value });
            }}
          />
        </Field>
      ) : null}
      {!setting.Locked && secret.Mode === 'clear' ? <p className="m-0 text-body-sm text-warn-fg">{t('settings.secret.willClear')}</p> : null}
      {setting.Locked ? null : (
        <div className="flex flex-wrap gap-2">
          {secret.Mode === 'keep' ? (
            <Button
              size="sm"
              disabled={disabled}
              onClick={() => {
                onSecret({ Mode: 'replace', Value: '' });
              }}
            >
              {setting.Set ? t('settings.secret.replace') : t('settings.secret.add')}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              disabled={disabled}
              onClick={() => {
                onSecret({ Mode: 'keep' });
              }}
            >
              {t('settings.secret.keep')}
            </Button>
          )}
          {setting.Set && secret.Mode === 'keep' ? (
            <Button
              size="sm"
              variant="danger"
              iconStart="trash"
              disabled={disabled}
              onClick={() => {
                onSecret({ Mode: 'clear' });
              }}
            >
              {t('settings.secret.clear')}
            </Button>
          ) : null}
        </div>
      )}
    </div>
  );
};

export const SettingField = ({ spec, setting, draft, secret, error, disabled, onChange, onSecret }: ISettingFieldProps) => {
  const t = useT();
  if (isSecret(setting)) {
    return <SecretValue spec={spec} setting={setting} secret={secret} disabled={disabled} error={error} onSecret={onSecret} />;
  }
  if (setting.Locked) return <LockedValue spec={spec} setting={setting} />;
  const label = setting.Source === 'default' ? t('settings.label.default', { label: t(spec.Label) }) : t(spec.Label);
  return (
    <Field label={label} error={error}>
      {spec.Kind === 'choice' ? (
        <Select
          value={draft}
          disabled={disabled}
          options={(spec.Choices ?? []).map((choice) => ({ value: choice.Value, label: t(choice.Label) }))}
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
      ) : (
        <Input
          dir={spec.Ltr || spec.Kind === 'number' ? 'ltr' : undefined}
          inputMode={spec.Kind === 'number' ? 'numeric' : undefined}
          value={draft}
          disabled={disabled}
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
      )}
      {spec.Hint ? <span className="text-body-sm text-tertiary">{t(spec.Hint, spec.HintParams)}</span> : null}
    </Field>
  );
};
