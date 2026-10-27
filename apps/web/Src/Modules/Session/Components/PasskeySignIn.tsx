import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { isApiError } from '@/Core/Api/ApiError';
import { signInWithPasskey } from '@/Core/Auth/AuthApi';
import { PasskeyCancelled, passkeysSupported } from '@/Core/Auth/WebAuthn';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';

export interface IPasskeySignInProps {
  onSignedIn: () => void;
}

export const PasskeySignIn = ({ onSignedIn }: IPasskeySignInProps) => {
  const t = useT();
  const client = useQueryClient();
  const errorText = useApiErrorText();
  const [Busy, setBusy] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);

  if (!passkeysSupported()) return null;

  const HandleClick = async () => {
    setBusy(true);
    setNotice(null);
    try {
      await signInWithPasskey(client);
      onSignedIn();
    } catch (error) {
      if (error instanceof PasskeyCancelled) setNotice(t('login.passkey.cancelled'));
      else setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Button iconStart="key" className="w-full" disabled={Busy} onClick={() => void HandleClick()}>
        {Busy ? t('login.passkey.busy') : t('login.passkey')}
      </Button>
      <p className="m-0 flex items-center gap-3 text-caption text-tertiary">
        <span className="h-px flex-1 bg-line" />
        {t('login.or')}
        <span className="h-px flex-1 bg-line" />
      </p>
    </div>
  );
};
