import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { NAME_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { StrengthMeter } from '@/Core/Ui/StrengthMeter';
import { useI18n } from '@/I18n/useI18n';
import { acceptInvitation, type TInvitationAccepted, type TInvitationPreview } from '@/Modules/Identity/Api/IdentityApi';
import { INVITE_ACCEPT_FIELDS, inviteAcceptSchema, type TInviteAcceptForm } from '@/Modules/Identity/Types/InviteForm';

export interface IInviteAcceptFormProps {
  token: string;
  preview: TInvitationPreview;
  onAccepted: (accepted: TInvitationAccepted) => void;
  onGone: (message: string) => void;
}

export const InviteAcceptForm = ({ token, preview, onAccepted, onGone }: IInviteAcceptFormProps) => {
  const { Locale, t } = useI18n();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const [Notice, setNotice] = useState<string | null>(null);
  const [Busy, setBusy] = useState(false);
  const form = useForm<TInviteAcceptForm>({ resolver: zodResolver(inviteAcceptSchema), defaultValues: { Name: '', Password: '' } });
  const { errors } = form.formState;
  const password = useWatch({ control: form.control, name: 'Password' });

  const HandleValid = async (values: TInviteAcceptForm) => {
    setBusy(true);
    setNotice(null);
    try {
      onAccepted(await acceptInvitation({ Token: token, Name: values.Name, Password: values.Password, Locale }));
    } catch (error) {
      form.resetField('Password');
      if (!isApiError(error)) setNotice(t('api.error.unknown'));
      else if (error.Code === 'INVITATION_INVALID') onGone(errorText(error));
      else if (error.Code === 'PASSWORD_BREACHED') form.setError('Password', { type: 'server', message: errorText(error) });
      else if (!applyFieldErrors(form, error, INVITE_ACCEPT_FIELDS)) setNotice(errorText(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('field.email')}>
        <Input type="email" dir="ltr" value={preview.Email} readOnly autoComplete="username" />
      </Field>
      <Field label={t('invite.page.name')} error={message(errors.Name?.message)}>
        <Input maxLength={NAME_MAX} autoComplete="name" {...form.register('Name')} />
      </Field>
      <div className="grid gap-2">
        <Field label={t('field.password')} error={message(errors.Password?.message)}>
          <PasswordInput autoComplete="new-password" {...form.register('Password')} />
        </Field>
        <StrengthMeter password={password} />
        <p className="m-0 text-body-sm text-tertiary">{t('password.hint')}</p>
      </div>
      <Button type="submit" variant="primary" className="w-full" disabled={Busy}>
        {Busy ? t('invite.page.submitting') : t('invite.page.submit')}
      </Button>
    </form>
  );
};
