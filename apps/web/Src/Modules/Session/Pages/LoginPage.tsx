import { useState } from 'react';

import type { TMfaPending } from '@/Core/Auth/AuthApi';
import { AuthPage } from '@/Core/Ui/AuthPage';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { LoginForm } from '@/Modules/Session/Components/LoginForm';
import { MfaStep } from '@/Modules/Session/Components/MfaStep';
import { SignInChoices } from '@/Modules/Session/Components/SignInChoices';

export interface ILoginPageProps {
  onSignedIn: () => void;
}

export const LoginPage = ({ onSignedIn }: ILoginPageProps) => {
  const t = useT();
  const [Pending, setPending] = useState<TMfaPending | null>(null);
  const [Expired, setExpired] = useState(false);

  if (Pending !== null) {
    return (
      <MfaStep
        pending={Pending}
        onSignedIn={onSignedIn}
        onBack={(expired) => {
          setExpired(expired);
          setPending(null);
        }}
      />
    );
  }

  return (
    <AuthPage title={t('login.title')} lead={t('login.lead')} footer={t('login.footer')}>
      {Expired ? <InlineAlert accent="awaiting" title={t('mfa.step.expired')} /> : null}
      <SignInChoices onSignedIn={onSignedIn} />
      <LoginForm
        onSignedIn={onSignedIn}
        onMfa={(pending) => {
          setExpired(false);
          setPending(pending);
        }}
      />
    </AuthPage>
  );
};
