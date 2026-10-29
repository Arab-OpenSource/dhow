import { useState } from 'react';

import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useT } from '@/I18n/useT';
import { sendTestMail } from '@/Modules/Settings/Api/SettingsApi';

export interface IMailTestProps {
  enabled: boolean;
}

type TResult = { Ok: true; To: string } | { Ok: false; Text: string };

export const MailTest = ({ enabled }: IMailTestProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const [Busy, setBusy] = useState(false);
  const [Result, setResult] = useState<TResult | null>(null);

  const HandleSend = () => {
    setBusy(true);
    setResult(null);
    sendTestMail()
      .then((sent) => {
        setResult({ Ok: true, To: sent.SentTo });
      })
      .catch((error: unknown) => {
        setResult({ Ok: false, Text: errorText(error) });
      })
      .finally(() => {
        setBusy(false);
      });
  };

  return (
    <>
      <Button iconStart="mail" disabled={!enabled || Busy} onClick={HandleSend}>
        {Busy ? t('settings.mail.testSending') : t('settings.mail.test')}
      </Button>
      {enabled ? null : <span className="text-body-sm text-tertiary">{t('settings.mail.testNeedsHost')}</span>}
      {Result === null ? null : (
        <div className="basis-full" aria-live="polite">
          {Result.Ok ? (
            <InlineAlert accent="live" title={t('settings.mail.testSent', { email: Result.To })}>
              {t('settings.mail.testSentAbout')}
            </InlineAlert>
          ) : (
            <InlineAlert accent="failed" title={Result.Text} />
          )}
        </div>
      )}
    </>
  );
};
