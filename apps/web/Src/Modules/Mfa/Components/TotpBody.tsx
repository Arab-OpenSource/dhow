import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import type { ApiError } from '@/Core/Api/ApiError';
import { factorMethods, type TMfaStatus } from '@/Core/Auth/Factor';
import { passkeysSupported } from '@/Core/Auth/WebAuthn';
import { useMfaStatus } from '@/Core/Auth/useMfaStatus';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFactorAction } from '@/Core/Hooks/useFactorAction';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { FactorFields } from '@/Core/Ui/FactorFields';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { confirmTotp, startTotp, type TTotpEnrollment } from '@/Modules/Mfa/Api/MfaApi';
import { QrCode } from '@/Modules/Mfa/Components/QrCode';
import { RecoveryCodes } from '@/Modules/Mfa/Components/RecoveryCodes';
import { useRefreshMfa } from '@/Modules/Mfa/Hooks/useMfa';
import { TOTP_FORM_ID, useTotpFormStore } from '@/Modules/Mfa/Store/useTotpFormStore';

export interface ITotpBodyProps {
  onDone: () => void;
}

const ENROLLMENT_KEY = ['totp-enrollment'] as const;
const KEY_GROUP = /.{1,4}/g;

const TotpForm = ({ enrollment, status, onDone }: { enrollment: TTotpEnrollment; status: TMfaStatus; onDone: () => void }) => {
  const t = useT();
  const refresh = useRefreshMfa();
  const Push = useToastStore((state) => state.Push);
  const Report = useTotpFormStore((state) => state.Report);
  const Reset = useTotpFormStore((state) => state.Reset);
  const methods = factorMethods(status, passkeysSupported());
  const [Code, setCode] = useState('');
  const [Codes, setCodes] = useState<string[] | null>(null);
  const action = useFactorAction(methods, async (factor) => confirmTotp(Code.replace(/\s/g, ''), await factor()));

  useEffect(() => {
    Report(action.Busy, action.Countdown.Remaining, Codes === null);
  }, [Report, action.Busy, action.Countdown.Remaining, Codes]);

  useEffect(() => Reset, [Reset]);

  if (Codes !== null) return <RecoveryCodes codes={Codes} onDone={onDone} />;

  return (
    <form
      id={TOTP_FORM_ID}
      noValidate
      className="grid gap-4"
      onSubmit={(event) => {
        event.preventDefault();
        void action.Run().then(async (change) => {
          if (change === null) return;
          await refresh();
          Push({ Id: `totp-on-${Date.now()}`, Accent: 'live', Title: t('totp.enabled'), Detail: t('mfa.revoked') });
          if (change.RecoveryCodes && change.RecoveryCodes.length > 0) setCodes(change.RecoveryCodes);
          else onDone();
        });
      }}
    >
      <QrCode value={enrollment.Uri} />
      <div className="grid gap-1 rounded-md border border-dashed border-line-2 px-4 py-3">
        <small className="text-body-sm text-secondary">{t('totp.manual')}</small>
        <bdi dir="ltr" className="font-mono text-body-sm tracking-wider select-all">
          {enrollment.Secret.match(KEY_GROUP)?.join(' ')}
        </bdi>
      </div>
      {action.Notice ? <InlineAlert accent="failed" title={action.Notice} /> : null}
      <Field label={t('totp.code')}>
        <Input
          dir="ltr"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={8}
          className="text-center font-mono tracking-[0.3em]"
          value={Code}
          onChange={(event) => {
            setCode(event.target.value);
          }}
        />
      </Field>
      <FactorFields methods={methods} draft={action.Draft} onChange={action.SetDraft} disabled={action.Busy} />
    </form>
  );
};

const TotpStart = ({ onStart }: { onStart: () => void }) => {
  const t = useT();
  return (
    <div className="grid justify-items-start gap-3">
      <p className="m-0 text-body-sm text-secondary">{t('totp.start.about')}</p>
      <Button variant="primary" iconStart="phone" onClick={onStart}>
        {t('totp.start')}
      </Button>
    </div>
  );
};

export const TotpBody = ({ onDone }: ITotpBodyProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const status = useMfaStatus();
  const [Started, setStarted] = useState(false);
  const enrollment = useQuery<TTotpEnrollment, ApiError>({
    queryKey: ENROLLMENT_KEY,
    queryFn: startTotp,
    enabled: Started,
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });

  if (!Started) {
    return (
      <TotpStart
        onStart={() => {
          setStarted(true);
        }}
      />
    );
  }

  if (enrollment.isError || status.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('totp.error')}>
          {errorText(enrollment.error ?? status.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void enrollment.refetch();
            void status.refetch();
          }}
        >
          {t('totp.restart')}
        </Button>
      </div>
    );
  }
  if (enrollment.data === undefined || status.data === undefined) return <Skeleton label={t('totp.loading')} lines={6} />;
  return <TotpForm enrollment={enrollment.data} status={status.data} onDone={onDone} />;
};
