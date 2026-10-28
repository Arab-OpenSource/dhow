import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { completeOidcLogin, type TMfaPending, type TOidcCallback } from '@/Core/Auth/AuthApi';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { AuthPage } from '@/Core/Ui/AuthPage';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import type { TTranslate } from '@/I18n/I18nContext';
import { completeLink, type TLinkedIdentityChange } from '@/Modules/Session/Api/OidcApi';
import { MfaStep } from '@/Modules/Session/Components/MfaStep';
import { clearOidcPurpose, readOidcPurpose, type TOidcPurpose } from '@/Modules/Session/Store/OidcCallback';
import { useOidcCallbackStore, type IOidcCallback } from '@/Modules/Session/Store/useOidcCallbackStore';

export interface IOidcCallbackPageProps {
  provider: string;
  onSignedIn: () => void;
  onLinked: (change: TLinkedIdentityChange) => void;
  onBack: (purpose: TOidcPurpose) => void;
}

type TPhase = { Kind: 'working' } | { Kind: 'failed'; Message: string } | { Kind: 'mfa'; Pending: TMfaPending };

interface IFirst {
  Phase: TPhase;
  Body: TOidcCallback | null;
}

const firstPhase = (callback: IOidcCallback | null, provider: string, t: TTranslate): IFirst => {
  if (callback === null) return { Phase: { Kind: 'failed', Message: t('oidc.callback.missing') }, Body: null };
  if (callback.Error !== '') return { Phase: { Kind: 'failed', Message: t('oidc.callback.cancelled') }, Body: null };
  if (callback.Code === '' || callback.State === '' || callback.Provider !== provider) {
    return { Phase: { Kind: 'failed', Message: t('oidc.callback.missing') }, Body: null };
  }
  return { Phase: { Kind: 'working' }, Body: { Provider: callback.Provider, Code: callback.Code, State: callback.State } };
};

export const OidcCallbackPage = ({ provider, onSignedIn, onLinked, onBack }: IOidcCallbackPageProps) => {
  const t = useT();
  const client = useQueryClient();
  const errorText = useApiErrorText();
  const [Purpose] = useState<TOidcPurpose>(readOidcPurpose);
  const [First] = useState<IFirst>(() => firstPhase(useOidcCallbackStore.getState().Callback, provider, t));
  const [Phase, setPhase] = useState<TPhase>(First.Phase);
  const Started = useRef(false);

  useEffect(() => {
    if (Started.current) return;
    Started.current = true;
    clearOidcPurpose();
    useOidcCallbackStore.getState().Take();
    const body = First.Body;
    if (body === null) return;
    const fail = (error: unknown) => {
      setPhase({ Kind: 'failed', Message: isApiError(error) ? errorText(error) : t('api.error.unknown') });
    };
    if (Purpose === 'link') {
      completeLink(body).then(onLinked, fail);
      return;
    }
    completeOidcLogin(client, body).then((outcome) => {
      if (outcome.Kind === 'mfa') setPhase({ Kind: 'mfa', Pending: outcome.Pending });
      else onSignedIn();
    }, fail);
  }, [client, errorText, First, onLinked, onSignedIn, Purpose, t]);

  if (Phase.Kind === 'mfa') {
    return (
      <MfaStep
        pending={Phase.Pending}
        onSignedIn={onSignedIn}
        onBack={() => {
          onBack('login');
        }}
      />
    );
  }

  return (
    <AuthPage
      title={Phase.Kind === 'failed' ? t('oidc.callback.failed') : t('oidc.callback.title')}
      lead={Phase.Kind === 'failed' ? undefined : Purpose === 'link' ? t('oidc.callback.linkLead') : t('oidc.callback.lead')}
    >
      {Phase.Kind === 'failed' ? (
        <>
          <InlineAlert accent="failed" title={Phase.Message} />
          <Button
            variant="primary"
            iconStart="back"
            className="w-full"
            onClick={() => {
              onBack(Purpose);
            }}
          >
            {Purpose === 'link' ? t('oidc.callback.backToAccount') : t('oidc.callback.backToLogin')}
          </Button>
        </>
      ) : (
        <div role="status" aria-live="polite">
          <Skeleton label={t('oidc.callback.busy')} lines={2} />
        </div>
      )}
    </AuthPage>
  );
};
