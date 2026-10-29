import { useQueryClient } from '@tanstack/react-query';

import { isApiError } from '@/Core/Api/ApiError';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { removeOidcProvider } from '@/Modules/Settings/Api/OidcApi';
import { SettingsConfirm } from '@/Modules/Settings/Components/SettingsConfirm';
import { OIDC_PUBLIC_KEY, OIDC_SETTINGS_KEY } from '@/Modules/Settings/Hooks/useOidcSettings';
import type { TOidcProvider } from '@/Modules/Settings/Types/OidcTypes';

export interface IOidcRemoveProps {
  provider: TOidcProvider;
  onClose: () => void;
}

export const OidcRemove = ({ provider, onClose }: IOidcRemoveProps) => {
  const t = useT();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const name = provider.Name;

  if (provider.OnlySignInUsers > 0) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('settings.oidc.remove.blocked')}>
          {t('settings.oidc.remove.blockedAbout', { count: String(provider.OnlySignInUsers) })}
        </InlineAlert>
        <Button size="sm" variant="ghost" onClick={onClose}>
          {t('mfa.cancel')}
        </Button>
      </div>
    );
  }

  return (
    <div className="-mx-card grid">
      <SettingsConfirm
        danger
        question={`${t('settings.oidc.remove.question', { name, count: String(provider.LinkedIdentities) })} ${t('settings.oidc.remove.about')}`}
        confirm={t('settings.oidc.remove.confirm', { name })}
        save={async (factor) => {
          try {
            return await removeOidcProvider(provider.Id, provider.Version, factor);
          } catch (error) {
            if (isApiError(error) && (error.Code === 'OIDC_PROVIDER_ONLY_SIGN_IN' || error.Code === 'SETTING_CHANGED')) {
              void client.invalidateQueries({ queryKey: OIDC_SETTINGS_KEY });
            }
            throw error;
          }
        }}
        onSaved={(saved) => {
          client.setQueryData(OIDC_SETTINGS_KEY, saved);
          void client.invalidateQueries({ queryKey: OIDC_PUBLIC_KEY });
          Push({ Id: `oidc-removed-${provider.Id}`, Accent: 'live', Title: t('settings.oidc.removed') });
          onClose();
        }}
        onCancel={onClose}
      />
    </div>
  );
};
