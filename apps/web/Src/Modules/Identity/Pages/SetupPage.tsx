import { useState } from 'react';

import { AuthPage } from '@/Core/Ui/AuthPage';
import { Button } from '@/Core/Ui/Button';
import { useT } from '@/I18n/useT';
import { SetupForm } from '@/Modules/Identity/Components/SetupForm';
import { SetupGuide } from '@/Modules/Identity/Components/SetupGuide';
import { useSetupTokenStore } from '@/Modules/Identity/Store/useSetupTokenStore';

export interface ISetupPageProps {
  setupRequired: boolean;
  onDone: (signedIn: boolean) => void;
  onLogin: () => void;
}

export const SetupPage = ({ setupRequired, onDone, onLogin }: ISetupPageProps) => {
  const t = useT();
  const Token = useSetupTokenStore((state) => state.Token);
  const Rejection = useSetupTokenStore((state) => state.Rejection);
  const [AlreadyDone, setAlreadyDone] = useState(false);
  const [Finished, setFinished] = useState(false);

  if (!Finished && (!setupRequired || AlreadyDone)) {
    return (
      <AuthPage title={t('setup.completed.title')} lead={t('setup.completed.lead')}>
        <Button variant="primary" onClick={onLogin}>
          {t('setup.completed.action')}
        </Button>
      </AuthPage>
    );
  }

  if (Token === null && !Finished) {
    return (
      <AuthPage title={t('setup.guide.title')} lead={t('setup.guide.lead')}>
        <SetupGuide rejection={Rejection} />
      </AuthPage>
    );
  }

  return (
    <AuthPage title={t('setup.title')} lead={t('setup.lead')}>
      <SetupForm
        token={Token ?? ''}
        onDone={(signedIn) => {
          setFinished(true);
          onDone(signedIn);
        }}
        onAlreadyDone={() => {
          setAlreadyDone(true);
        }}
      />
    </AuthPage>
  );
};
