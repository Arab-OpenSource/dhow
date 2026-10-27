import { useRef } from 'react';

import type { TMfaPending } from '@/Core/Auth/AuthApi';
import { formatClock } from '@/Core/Format/TimeFormat';
import { AuthPage } from '@/Core/Ui/AuthPage';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { useT } from '@/I18n/useT';
import { useMfaStep } from '@/Modules/Session/Hooks/useMfaStep';

export interface IMfaStepProps {
  pending: TMfaPending;
  onSignedIn: () => void;
  onBack: (expired: boolean) => void;
}

export const MfaStep = (props: IMfaStepProps) => {
  const t = useT();
  const inputRef = useRef<HTMLInputElement>(null);
  const step = useMfaStep({ ...props, inputRef });
  const locked = step.Countdown.Remaining > 0;
  const time = formatClock(step.Countdown.Remaining);
  const totp = step.Method === 'totp';
  const lead = totp ? t('mfa.step.lead.totp') : step.Method === null ? t('mfa.step.lead.passkey') : t('mfa.step.lead.recovery');

  return (
    <AuthPage title={t('mfa.step.title')} lead={lead} footer={t('login.footer')}>
      <form
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          step.SubmitCode();
        }}
        className="grid gap-4"
      >
        {locked ? (
          <>
            <InlineAlert accent="awaiting" title={t('mfa.step.locked.title')}>
              <b role="timer" aria-live="off" className="font-mono">
                {time}
              </b>
            </InlineAlert>
            <p role="status" className="sr-only">
              {t('mfa.step.locked.announce', { seconds: step.Countdown.Remaining })}
            </p>
          </>
        ) : null}
        {step.Notice ? <InlineAlert accent="failed" title={step.Notice} /> : null}
        {step.Method !== null ? (
          <>
            <Field label={totp ? t('mfa.step.code') : t('mfa.step.recoveryCode')} error={step.CodeError ?? undefined}>
              <Input
                ref={inputRef}
                dir="ltr"
                value={step.Code}
                onChange={(event) => {
                  step.SetCode(event.target.value);
                }}
                inputMode={totp ? 'numeric' : 'text'}
                autoComplete={totp ? 'one-time-code' : 'off'}
                autoCapitalize={totp ? 'none' : 'characters'}
                spellCheck={false}
                maxLength={totp ? 8 : 24}
                className="text-center font-mono text-title-sm tracking-[0.3em]"
              />
            </Field>
            <Button type="submit" variant="primary" className="w-full" disabled={step.Busy || locked}>
              {locked ? t('throttle.retryIn', { time }) : step.Busy ? t('mfa.step.submitting') : t('mfa.step.submit')}
            </Button>
          </>
        ) : null}
        {step.Passkey ? (
          <Button
            iconStart="key"
            variant={step.Method === null ? 'primary' : 'secondary'}
            className="w-full"
            disabled={step.Busy || locked}
            onClick={step.UsePasskey}
          >
            {step.Busy && step.Method === null ? t('mfa.step.passkeyBusy') : t('mfa.step.usePasskey')}
          </Button>
        ) : null}
        <div className="flex flex-wrap justify-between gap-2">
          {step.Other ? (
            <Button variant="ghost" size="sm" onClick={step.Switch}>
              {step.Other === 'totp' ? t('mfa.step.useTotp') : t('mfa.step.useRecovery')}
            </Button>
          ) : (
            <span />
          )}
          <Button
            variant="ghost"
            size="sm"
            iconStart="back"
            onClick={() => {
              props.onBack(false);
            }}
          >
            {t('mfa.step.back')}
          </Button>
        </div>
      </form>
    </AuthPage>
  );
};
