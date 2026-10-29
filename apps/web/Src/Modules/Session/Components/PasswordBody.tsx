import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useAction } from '@/Core/Hooks/useAction';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useCountdown } from '@/Core/Hooks/useCountdown';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { StrengthMeter } from '@/Core/Ui/StrengthMeter';
import { useT } from '@/I18n/useT';
import { changePassword } from '@/Modules/Session/Api/SessionApi';
import { SESSIONS_QUERY_KEY } from '@/Modules/Session/Hooks/useSessions';
import { usePasswordFormStore } from '@/Modules/Session/Store/usePasswordFormStore';
import {
  PASSWORD_FIELDS,
  PASSWORD_FORM_ID,
  passwordFormSchema,
  type TPasswordForm,
} from '@/Modules/Session/Types/PasswordForm';

export interface IPasswordBodyProps {
  onDone: () => void;
}

const EMPTY: TPasswordForm = { CurrentPassword: '', NewPassword: '', ConfirmPassword: '' };
const PASSWORD_FIELD_NAMES = ['CurrentPassword', 'NewPassword', 'ConfirmPassword'] as const;

export const PasswordBody = ({ onDone }: IPasswordBodyProps) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const Report = usePasswordFormStore((state) => state.Report);
  const Reset = usePasswordFormStore((state) => state.Reset);
  const countdown = useCountdown();
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TPasswordForm>({ resolver: zodResolver(passwordFormSchema), defaultValues: EMPTY });
  const { errors } = form.formState;
  const next = useWatch({ control: form.control, name: 'NewPassword' });
  const submit = useAction(async (values: TPasswordForm): Promise<number> =>
    changePassword({ CurrentPassword: values.CurrentPassword, NewPassword: values.NewPassword }),
  );

  useEffect(() => {
    Report(submit.Pending, countdown.Remaining);
  }, [Report, submit.Pending, countdown.Remaining]);

  useEffect(() => Reset, [Reset]);

  const HandleValid = async (values: TPasswordForm) => {
    if (countdown.Remaining > 0) return;
    setNotice(null);
    const result = await submit.Run(values);
    PASSWORD_FIELD_NAMES.forEach((name) => {
      form.resetField(name);
    });
    if (!isApiError(result)) {
      void client.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY });
      Push({
        Id: 'password-changed',
        Accent: 'live',
        Title: t('password.change.done'),
        Detail: result > 0 ? t('password.change.done.others', { count: result }) : t('password.change.done.none'),
      });
      onDone();
      return;
    }
    if (result.Code === 'INVALID_CURRENT_PASSWORD') {
      form.setError('CurrentPassword', { type: 'server', message: errorText(result) });
      form.setFocus('CurrentPassword');
      if (result.RetryAfterSeconds !== null) countdown.Start(result.RetryAfterSeconds);
    } else if (result.Code === 'PASSWORD_BREACHED') {
      form.setError('NewPassword', { type: 'server', message: errorText(result) });
      form.setFocus('NewPassword');
    } else if (!applyFieldErrors(form, result, PASSWORD_FIELDS)) {
      setNotice(errorText(result));
    }
  };

  return (
    <form
      id={PASSWORD_FORM_ID}
      noValidate
      onSubmit={(event) => void form.handleSubmit(HandleValid)(event)}
      className="grid gap-4"
    >
      <InlineAlert accent="building" title={t('password.change.notice')} />
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('password.current')} error={message(errors.CurrentPassword?.message)}>
        <PasswordInput autoComplete="current-password" {...form.register('CurrentPassword')} />
      </Field>
      <div className="grid gap-2">
        <Field label={t('password.new')} error={message(errors.NewPassword?.message)}>
          <PasswordInput autoComplete="new-password" {...form.register('NewPassword')} />
        </Field>
        <StrengthMeter password={next} />
        <p className="m-0 text-body-sm text-tertiary">{t('password.hint')}</p>
      </div>
      <Field label={t('password.confirm')} error={message(errors.ConfirmPassword?.message)}>
        <PasswordInput autoComplete="new-password" {...form.register('ConfirmPassword')} />
      </Field>
    </form>
  );
};
