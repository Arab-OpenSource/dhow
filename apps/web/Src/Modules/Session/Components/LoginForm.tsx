import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { signIn, type TMfaPending } from '@/Core/Auth/AuthApi';
import { EMAIL_MAX } from '@/Core/Forms/Fields';
import { formatClock } from '@/Core/Format/TimeFormat';
import { useAction } from '@/Core/Hooks/useAction';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useCountdown } from '@/Core/Hooks/useCountdown';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { useT } from '@/I18n/useT';
import { loginFormSchema, type TLoginForm } from '@/Modules/Session/Types/LoginForm';

export interface ILoginFormProps {
  onSignedIn: () => void;
  onMfa?: (pending: TMfaPending) => void;
}

export const LoginForm = ({ onSignedIn, onMfa }: ILoginFormProps) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const countdown = useCountdown();
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TLoginForm>({ resolver: zodResolver(loginFormSchema), defaultValues: { Email: '', Password: '' } });
  const { errors } = form.formState;
  const locked = countdown.Remaining > 0;

  const submit = useAction((values: TLoginForm) => signIn(client, values));

  const HandleValid = async (values: TLoginForm) => {
    if (locked) return;
    setNotice(null);
    const result = await submit.Run(values);
    form.resetField('Password');
    if (!isApiError(result)) {
      if (result.Kind === 'mfa') onMfa?.(result.Pending);
      else onSignedIn();
      return;
    }
    if (result.RetryAfterSeconds === null) setNotice(errorText(result));
    else countdown.Start(result.RetryAfterSeconds);
    form.setFocus('Password');
  };

  const time = formatClock(countdown.Remaining);
  const label = locked ? t('throttle.retryIn', { time }) : submit.Pending ? t('login.submitting') : t('login.submit');

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-4">
      {locked ? (
        <>
          <InlineAlert accent="awaiting" title={t('login.locked.title')}>
            <span>{t('login.locked.hint')} </span>
            <b role="timer" aria-live="off" className="font-mono">
              {time}
            </b>
          </InlineAlert>
          <p role="status" className="sr-only">
            {t('login.locked.announce', { seconds: countdown.Remaining })}
          </p>
        </>
      ) : null}
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('field.email')} error={message(errors.Email?.message)}>
        <Input
          type="email"
          dir="ltr"
          inputMode="email"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          maxLength={EMAIL_MAX}
          {...form.register('Email')}
        />
      </Field>
      <Field label={t('field.password')} error={message(errors.Password?.message)}>
        <PasswordInput autoComplete="current-password" {...form.register('Password')} />
      </Field>
      <Button type="submit" variant="primary" disabled={locked || submit.Pending} className="w-full">
        {label}
      </Button>
    </form>
  );
};
