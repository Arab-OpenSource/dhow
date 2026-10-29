import { useState } from 'react';

import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { OidcRemove } from '@/Modules/Settings/Components/OidcRemove';
import { OidcTestResult } from '@/Modules/Settings/Components/OidcTestResult';
import { useOidcTest } from '@/Modules/Settings/Hooks/useOidcTest';
import { OIDC_TYPE_NAMES, type TOidcProvider } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcProviderRowProps {
  provider: TOidcProvider;
  locked: boolean;
  onEdit: (id: string) => void;
}

export const OidcProviderRow = ({ provider, locked, onEdit }: IOidcProviderRowProps) => {
  const t = useT();
  const test = useOidcTest();
  const [Removing, setRemoving] = useState(false);

  return (
    <li className="grid min-w-0 gap-3 border-t border-line px-card py-3.5 first:border-t-0">
      <div className="flex min-w-0 items-start gap-3">
        <Icon name="key" size={18} className="mt-0.5 flex-none text-secondary" />
        <span className="grid min-w-0 flex-1 gap-1">
          <span className="flex flex-wrap items-center gap-2 text-label">
            <bdi>{provider.Name}</bdi>
            {provider.Enabled ? null : <Badge tone="mute">{t('settings.oidc.off')}</Badge>}
          </span>
          <small className="flex min-w-0 flex-wrap gap-x-2 text-body-sm text-tertiary">
            <bdi dir="ltr">{OIDC_TYPE_NAMES[provider.Type]}</bdi>
            <bdi dir="ltr" className="min-w-0 break-all">
              {provider.Issuer}
            </bdi>
          </small>
          <small className="text-body-sm text-tertiary">{t('settings.oidc.linked', { count: String(provider.LinkedIdentities) })}</small>
          {provider.Enabled ? null : <small className="text-body-sm text-warn-fg">{t('settings.oidc.offAbout')}</small>}
        </span>
      </div>
      <div role="group" aria-label={t('settings.oidc.actions', { name: provider.Name })} className="flex flex-wrap gap-2">
        <Button
          size="sm"
          iconStart="check"
          disabled={test.Busy}
          onClick={() => {
            test.Run(provider.Type, provider.Issuer);
          }}
        >
          {test.Busy ? t('settings.oidc.testing') : t('settings.oidc.test')}
        </Button>
        {locked ? null : (
          <>
            <Button
              size="sm"
              iconStart="settings"
              onClick={() => {
                onEdit(provider.Id);
              }}
            >
              {t('settings.oidc.edit')}
            </Button>
            <Button
              size="sm"
              variant="danger"
              iconStart="trash"
              disabled={Removing}
              onClick={() => {
                setRemoving(true);
              }}
            >
              {t('settings.oidc.remove')}
            </Button>
          </>
        )}
      </div>
      {test.Problem ? <InlineAlert accent="failed" title={test.Problem} /> : null}
      {test.Result ? <OidcTestResult result={test.Result} /> : null}
      {Removing ? (
        <OidcRemove
          provider={provider}
          onClose={() => {
            setRemoving(false);
          }}
        />
      ) : null}
    </li>
  );
};
