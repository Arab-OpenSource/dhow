import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from '@tanstack/react-router';
import { Suspense } from 'react';

import { PageLoading } from '@/App/Layout/PageStates';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { useToastStore } from '@/Core/Store/useToastStore';
import { lazyComponent } from '@/Core/Ui/LazyComponent';
import { useT } from '@/I18n/useT';
import { IDENTITIES_QUERY_KEY } from '@/Modules/Session/Api/OidcApi';
import { OIDC_CALLBACK_PATH } from '@/Modules/Session/Store/OidcCallback';

const OidcCallbackPage = lazyComponent(() =>
  import('@/Modules/Session/Pages/OidcCallbackPage').then((module) => module.OidcCallbackPage),
);

export const preloadOidcCallbackPage = (): void => {
  void OidcCallbackPage.preload();
};

export const OidcCallbackRoute = () => {
  const t = useT();
  const navigate = useNavigate();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const { provider } = useParams({ from: OIDC_CALLBACK_PATH });

  const toAccount = () => {
    void navigate({ to: '/account/security', replace: true });
  };

  return (
    <Suspense fallback={<PageLoading />}>
      <OidcCallbackPage
        provider={provider}
        onSignedIn={() => {
          void navigate({ to: '/', replace: true });
        }}
        onLinked={(change) => {
          void client.invalidateQueries({ queryKey: IDENTITIES_QUERY_KEY });
          void client.invalidateQueries({ queryKey: ME_QUERY_KEY });
          Push({
            Id: `identity-linked-${change.Identity.Id}`,
            Accent: 'live',
            Title: t('oidc.callback.linked', { provider: change.Identity.ProviderName }),
            Detail: change.Revoked > 0 ? t('oidc.callback.linkedDetail') : undefined,
          });
          toAccount();
        }}
        onBack={(purpose) => {
          if (purpose === 'link') toAccount();
          else void navigate({ to: '/login', replace: true });
        }}
      />
    </Suspense>
  );
};
