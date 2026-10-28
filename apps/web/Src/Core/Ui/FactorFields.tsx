import type { IFactorDraft, TFactorMethod } from '@/Core/Auth/Factor';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { useT } from '@/I18n/useT';

export interface IFactorFieldsProps {
  methods: readonly TFactorMethod[];
  draft: IFactorDraft;
  onChange: (draft: IFactorDraft) => void;
  disabled?: boolean;
}

export const FactorFields = ({ methods, draft, onChange, disabled = false }: IFactorFieldsProps) => {
  const t = useT();
  const label = t(`factor.method.${draft.Method}`);
  const change = (value: string) => {
    onChange({ ...draft, Value: value });
  };

  return (
    <fieldset className="m-0 grid gap-3 rounded-md border border-line bg-surface-inset p-3" disabled={disabled}>
      <legend className="px-1 text-body-sm text-secondary">{t('factor.title')}</legend>
      {methods.length > 1 ? (
        <div role="group" aria-label={t('factor.methods')} className="flex flex-wrap gap-1.5">
          {methods.map((method) => (
            <Button
              key={method}
              size="sm"
              variant={method === draft.Method ? 'primary' : 'ghost'}
              aria-pressed={method === draft.Method}
              onClick={() => {
                onChange({ Method: method, Value: '' });
              }}
            >
              {t(`factor.method.${method}`)}
            </Button>
          ))}
        </div>
      ) : null}
      {draft.Method === 'passkey' ? (
        <p className="m-0 text-body-sm text-secondary">{t('factor.passkey.hint')}</p>
      ) : draft.Method === 'password' ? (
        <Field label={label}>
          <PasswordInput
            autoComplete="current-password"
            value={draft.Value}
            onChange={(event) => {
              change(event.target.value);
            }}
          />
        </Field>
      ) : (
        <Field label={label}>
          <Input
            dir="ltr"
            spellCheck={false}
            className="font-mono"
            inputMode={draft.Method === 'totp' ? 'numeric' : 'text'}
            autoComplete={draft.Method === 'totp' ? 'one-time-code' : 'off'}
            maxLength={draft.Method === 'totp' ? 8 : 24}
            value={draft.Value}
            onChange={(event) => {
              change(event.target.value);
            }}
          />
        </Field>
      )}
    </fieldset>
  );
};
