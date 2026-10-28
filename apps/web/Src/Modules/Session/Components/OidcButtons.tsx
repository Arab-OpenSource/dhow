import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { beginOidcLogin, type TOidcProvider } from '@/Core/Auth/AuthApi';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { rememberOidcPurpose } from '@/Modules/Session/Store/OidcCallback';

export interface IOidcButtonsProps {
  providers: readonly TOidcProvider[];
  navigate?: (url: string) => void;
}

const leave = (url: string): void => {
  window.location.assign(url);
};

export const OidcButtons = ({ providers, navigate = leave }: IOidcButtonsProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Busy, setBusy] = useState<string | null>(null);
  const [Notice, setNotice] = useState<string | null>(null);

  const HandleClick = async (provider: TOidcProvider) => {
    setBusy(provider.Id);
    setNotice(null);
    try {
      const authorization = await beginOidcLogin(provider.Id);
      rememberOidcPurpose('login');
      navigate(authorization.AuthorizationUrl);
    } catch (error) {
      setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
      setBusy(null);
    }
  };

  return (
    <>
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      {providers.map((provider) => (
        <Button
          key={provider.Id}
          iconStart="user"
          className="w-full"
          disabled={Busy !== null}
          onClick={() => void HandleClick(provider)}
        >
          {Busy === provider.Id ? t('login.oidc.busy', { provider: provider.Name }) : t('login.oidc.with', { provider: provider.Name })}
        </Button>
      ))}
    </>
  );
};
