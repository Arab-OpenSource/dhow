import { useQueryClient } from '@tanstack/react-query';

import { isApiError } from '@/Core/Api/ApiError';
import { useFactorMethods, useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { formatClock } from '@/Core/Format/TimeFormat';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { FactorFields } from '@/Core/Ui/FactorFields';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { createOidcProvider, updateOidcProvider } from '@/Modules/Settings/Api/OidcApi';
import { OidcFields } from '@/Modules/Settings/Components/OidcFields';
import { OidcTestResult } from '@/Modules/Settings/Components/OidcTestResult';
import { typeOf, useOidcForm, type IOidcForm, type TOidcErrors } from '@/Modules/Settings/Hooks/useOidcForm';
import { OIDC_PUBLIC_KEY, OIDC_SETTINGS_KEY, useOidcSettings } from '@/Modules/Settings/Hooks/useOidcSettings';
import { useOidcTest } from '@/Modules/Settings/Hooks/useOidcTest';
import type { TOidcProvider, TOidcSettings } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcProviderBodyProps {
  providerId?: string;
  onDone: () => void;
}

interface IOidcFormProps {
  settings: TOidcSettings;
  provider: TOidcProvider | undefined;
  onDone: () => void;
}

const fieldErrors = (error: unknown, form: IOidcForm, taken: string): void => {
  if (!isApiError(error)) return;
  if (error.Code === 'OIDC_PROVIDER_EXISTS') {
    form.SetErrors({ Id: taken });
    return;
  }
  if (error.Code !== 'SETTING_INVALID' && error.Code !== 'OIDC_PROVIDER_LINKED') return;
  const out: TOidcErrors = {};
  for (const detail of error.Details) {
    if (detail.Field in out || detail.Field === '') continue;
    Object.assign(out, { [detail.Field]: detail.Message });
  }
  form.SetErrors(out);
};

const OidcForm = ({ settings, provider, onDone }: IOidcFormProps) => {
  const t = useT();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const methods = useFactorMethods();
  const form = useOidcForm(provider);
  const test = useOidcTest();
  const creating = provider === undefined;
  const linked = provider !== undefined && provider.LinkedIdentities > 0;

  const action = useFactorAction<TOidcSettings>(methods ?? ['password'], async (proof) => {
    try {
      const factor = await proof();
      return provider === undefined
        ? await createOidcProvider(form.Create(), factor)
        : await updateOidcProvider(provider.Id, provider.Version, form.Changes(provider), factor);
    } catch (error) {
      fieldErrors(error, form, t('settings.oidc.error.taken'));
      if (isApiError(error) && error.Code === 'SETTING_CHANGED') void client.invalidateQueries({ queryKey: OIDC_SETTINGS_KEY });
      throw error;
    }
  });

  const HandleSubmit = () => {
    const problems = form.Problems();
    form.SetErrors(Object.fromEntries(Object.entries(problems).map(([field, key]) => [field, t(key)])));
    if (Object.keys(problems).length > 0) return;
    void action.Run().then((saved) => {
      if (saved === null) return;
      client.setQueryData(OIDC_SETTINGS_KEY, saved);
      void client.invalidateQueries({ queryKey: OIDC_PUBLIC_KEY });
      Push({ Id: `oidc-saved-${String(Date.now())}`, Accent: 'live', Title: t('settings.oidc.saved') });
      onDone();
    });
  };
  const locked = action.Countdown.Remaining > 0;
  const submit = creating ? t('settings.oidc.form.create') : t('settings.oidc.form.save');

  return (
    <form
      noValidate
      className="grid gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        HandleSubmit();
      }}
    >
      {action.Notice ? <InlineAlert accent="failed" title={action.Notice} /> : null}
      <OidcFields form={form} creating={creating} linked={linked} callbackBase={settings.CallbackBase} disabled={action.Busy} />
      <div className="grid justify-items-start gap-3">
        <Button
          size="sm"
          iconStart="check"
          disabled={test.Busy || action.Busy}
          onClick={() => {
            test.Run(typeOf(form.Draft.Preset), form.Draft.Issuer.trim());
          }}
        >
          {test.Busy ? t('settings.oidc.testing') : t('settings.oidc.test')}
        </Button>
        {test.Problem ? <InlineAlert accent="failed" title={test.Problem} /> : null}
        {test.Result ? <OidcTestResult result={test.Result} /> : null}
      </div>
      <div className="grid gap-3 border-t border-line pt-4">
        <p className="m-0 text-body-sm text-secondary">{t('settings.oidc.form.factor')}</p>
        <FactorFields methods={methods ?? ['password']} draft={action.Draft} onChange={action.SetDraft} disabled={action.Busy} />
        <Button type="submit" variant="primary" iconStart="key" className="justify-self-start" disabled={action.Busy || locked}>
          {locked ? t('throttle.retryIn', { time: formatClock(action.Countdown.Remaining) }) : action.Busy ? t('settings.oidc.form.saving') : submit}
        </Button>
      </div>
    </form>
  );
};

export const OidcProviderBody = ({ providerId, onDone }: IOidcProviderBodyProps) => {
  const t = useT();
  const query = useOidcSettings();
  const status = useMfaStatus();
  const methods = useFactorMethods();

  if (query.isError || status.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('settings.oidc.error')} />
        <Button
          size="sm"
          onClick={() => {
            void query.refetch();
            void status.refetch();
          }}
        >
          {t('settings.retry')}
        </Button>
      </div>
    );
  }
  if (query.data === undefined || status.data === undefined || methods === undefined) return <Skeleton label={t('settings.oidc.loading')} lines={6} />;
  if (query.data.Locked) return <InlineAlert accent="awaiting" title={t('settings.oidc.lockedForm')} />;
  if (!status.data.Enabled) {
    return (
      <InlineAlert accent="awaiting" title={t('settings.factor.needed')}>
        {t('settings.factor.neededAbout')}
      </InlineAlert>
    );
  }
  const provider = providerId === undefined ? undefined : query.data.Providers.find((item) => item.Id === providerId);
  if (providerId !== undefined && provider === undefined) return <InlineAlert accent="awaiting" title={t('settings.oidc.notFound')} />;
  return <OidcForm key={provider?.Id ?? 'new'} settings={query.data} provider={provider} onDone={onDone} />;
};
