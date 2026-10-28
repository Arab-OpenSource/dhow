import { useQuery } from '@tanstack/react-query';

import { listOidcProviders } from '@/Core/Auth/AuthApi';
import { passkeysSupported } from '@/Core/Auth/WebAuthn';
import { useT } from '@/I18n/useT';
import { OIDC_PROVIDERS_QUERY_KEY } from '@/Modules/Session/Api/OidcApi';
import { OidcButtons } from '@/Modules/Session/Components/OidcButtons';
import { PasskeySignIn } from '@/Modules/Session/Components/PasskeySignIn';

const PROVIDERS_STALE_MS = 5 * 60_000;

export interface ISignInChoicesProps {
  onSignedIn: () => void;
  navigate?: (url: string) => void;
}

export const SignInChoices = ({ onSignedIn, navigate }: ISignInChoicesProps) => {
  const t = useT();
  const providers = useQuery({ queryKey: OIDC_PROVIDERS_QUERY_KEY, queryFn: listOidcProviders, staleTime: PROVIDERS_STALE_MS });
  const list = Array.isArray(providers.data) ? providers.data : [];
  const passkey = passkeysSupported();

  if (!passkey && list.length === 0) return null;

  return (
    <div role="group" aria-label={t('login.oidc.label')} className="grid gap-3">
      {passkey ? <PasskeySignIn onSignedIn={onSignedIn} /> : null}
      <OidcButtons providers={list} navigate={navigate} />
      <p className="m-0 mt-1 flex items-center gap-3 text-caption text-tertiary">
        <span className="h-px flex-1 bg-line" />
        {t('login.or')}
        <span className="h-px flex-1 bg-line" />
      </p>
    </div>
  );
};
