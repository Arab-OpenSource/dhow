import { Button } from '@/Core/Ui/Button';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { useT } from '@/I18n/useT';
import { OidcCallback } from '@/Modules/Settings/Components/OidcCallback';
import type { IOidcForm } from '@/Modules/Settings/Hooks/useOidcForm';
import { OIDC_NAME_MAX, OIDC_PRESETS, PRODUCT_NAMES, type TOidcPreset } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcFieldsProps {
  form: IOidcForm;
  creating: boolean;
  linked: boolean;
  callbackBase: string;
  disabled: boolean;
}

type TSecretFieldProps = Pick<IOidcFieldsProps, 'form' | 'creating' | 'disabled'>;

const SecretField = ({ form, creating, disabled }: TSecretFieldProps) => {
  const t = useT();
  const { Draft, Errors, Set, SecretMode, SecretNeeded } = form;
  const input = (
    <Field label={creating ? t('settings.oidc.field.secret') : t('settings.oidc.field.secret.new')} error={Errors.ClientSecret}>
      <PasswordInput
        autoComplete="new-password"
        value={Draft.Secret}
        disabled={disabled}
        onChange={(event) => {
          Set('Secret', event.target.value);
        }}
      />
    </Field>
  );
  if (creating) return input;
  return (
    <div className="grid gap-2">
      <span className="text-body-sm text-secondary">{t('settings.oidc.field.secret')}</span>
      {SecretNeeded ? <p className="m-0 text-body-sm text-warn-fg">{t('settings.oidc.field.secret.again')}</p> : null}
      {SecretMode === 'replace' ? input : null}
      {SecretMode === 'clear' ? <p className="m-0 text-body-sm text-warn-fg">{t('settings.oidc.field.secret.willClear')}</p> : null}
      <div className="flex flex-wrap gap-2">
        {SecretMode === 'keep' ? (
          <>
            <Button
              size="sm"
              disabled={disabled}
              onClick={() => {
                Set('SecretMode', 'replace');
              }}
            >
              {t('settings.oidc.field.secret.replace')}
            </Button>
            <Button
              size="sm"
              variant="danger"
              iconStart="trash"
              disabled={disabled}
              onClick={() => {
                Set('SecretMode', 'clear');
              }}
            >
              {t('settings.oidc.field.secret.clear')}
            </Button>
          </>
        ) : SecretNeeded ? null : (
          <Button
            size="sm"
            variant="ghost"
            disabled={disabled}
            onClick={() => {
              Set('SecretMode', 'keep');
              Set('Secret', '');
            }}
          >
            {t('settings.oidc.field.secret.keep')}
          </Button>
        )}
      </div>
    </div>
  );
};

export const OidcFields = ({ form, creating, linked, callbackBase, disabled }: IOidcFieldsProps) => {
  const t = useT();
  const { Draft, Errors, Set } = form;
  const github = Draft.Preset === 'github';

  return (
    <>
      <ChoiceList<TOidcPreset>
        label={t('settings.oidc.preset.label')}
        value={Draft.Preset}
        disabled={disabled || linked}
        choices={OIDC_PRESETS.map((preset) => ({ value: preset.Id, title: preset.Title, about: t(preset.About, PRODUCT_NAMES) }))}
        onChange={form.Choose}
      />
      {creating ? (
        <Field label={t('settings.oidc.field.id')} error={Errors.Id}>
          <Input
            dir="ltr"
            autoComplete="off"
            spellCheck={false}
            maxLength={32}
            value={Draft.Id}
            disabled={disabled}
            onChange={(event) => {
              Set('Id', event.target.value.toLowerCase());
            }}
          />
          <span className="text-body-sm text-tertiary">{t('settings.oidc.field.id.hint')}</span>
        </Field>
      ) : null}
      <OidcCallback base={callbackBase} id={Draft.Id} />
      <Field label={t('settings.oidc.field.name')} error={Errors.Name}>
        <Input
          maxLength={OIDC_NAME_MAX}
          autoComplete="off"
          value={Draft.Name}
          disabled={disabled}
          onChange={(event) => {
            Set('Name', event.target.value);
          }}
        />
      </Field>
      <Field label={t('settings.oidc.field.issuer')} error={Errors.Issuer}>
        <Input
          dir="ltr"
          inputMode="url"
          autoComplete="off"
          spellCheck={false}
          value={Draft.Issuer}
          disabled={disabled || linked}
          onChange={(event) => {
            Set('Issuer', event.target.value);
          }}
        />
        {linked ? <span className="text-body-sm text-tertiary">{t('settings.oidc.field.issuer.locked')}</span> : null}
        {github && !linked ? <span className="text-body-sm text-tertiary">{t('settings.oidc.field.issuer.github', PRODUCT_NAMES)}</span> : null}
      </Field>
      <Field label={t('settings.oidc.field.clientId')} error={Errors.ClientId}>
        <Input
          dir="ltr"
          autoComplete="off"
          spellCheck={false}
          value={Draft.ClientId}
          disabled={disabled}
          onChange={(event) => {
            Set('ClientId', event.target.value);
          }}
        />
      </Field>
      <SecretField form={form} creating={creating} disabled={disabled} />
      <Field label={t('settings.oidc.field.domains')} error={Errors.AllowedDomains}>
        <Input
          dir="ltr"
          autoComplete="off"
          spellCheck={false}
          value={Draft.Domains}
          disabled={disabled}
          onChange={(event) => {
            Set('Domains', event.target.value);
          }}
        />
        <span className="text-body-sm text-tertiary">{t('settings.oidc.field.domains.hint', PRODUCT_NAMES)}</span>
      </Field>
    </>
  );
};
