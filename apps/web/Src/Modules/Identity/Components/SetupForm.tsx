import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { isApiError, type ApiError } from '@/Core/Api/ApiError';
import { signIn } from '@/Core/Auth/AuthApi';
import { useAction } from '@/Core/Hooks/useAction';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Icon } from '@/Core/Ui/Icon';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { useI18n } from '@/I18n/useI18n';
import { OrganizationFields, OwnerFields, PasswordFields } from '@/Modules/Identity/Components/SetupFields';
import { setupInstance } from '@/Modules/Identity/Api/IdentityApi';
import { useSetupTokenStore } from '@/Modules/Identity/Store/useSetupTokenStore';
import { SETUP_FIELDS, setupFormSchema, type TSetupForm } from '@/Modules/Identity/Types/SetupForm';

export interface ISetupFormProps {
  token: string;
  onDone: (signedIn: boolean) => void;
  onAlreadyDone: () => void;
}

const EMPTY: TSetupForm = {
  OwnerName: '',
  Email: '',
  Password: '',
  ConfirmPassword: '',
  OrganizationName: '',
  OrganizationSlug: '',
};

export const SetupForm = ({ token, onDone, onAlreadyDone }: ISetupFormProps) => {
  const { Locale, t } = useI18n();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TSetupForm>({ resolver: zodResolver(setupFormSchema), defaultValues: EMPTY });

  const submit = useAction(async (values: TSetupForm): Promise<boolean> => {
    await setupInstance({
      SetupToken: token,
      OwnerName: values.OwnerName,
      Email: values.Email,
      Password: values.Password,
      OrganizationName: values.OrganizationName,
      OrganizationSlug: values.OrganizationSlug,
      Locale,
    });
    try {
      await signIn(client, { Email: values.Email, Password: values.Password });
      return true;
    } catch {
      return false;
    }
  });

  const Reject = (error: ApiError) => {
    if (error.Code === 'SETUP_TOKEN_INVALID') {
      useSetupTokenStore.getState().Reject(errorText(error));
    } else if (error.Code === 'SETUP_COMPLETED') {
      useSetupTokenStore.getState().Clear();
      onAlreadyDone();
    } else if (error.Code === 'PASSWORD_BREACHED') {
      form.setError('Password', { type: 'server', message: errorText(error) });
      form.setFocus('Password');
    } else if (!applyFieldErrors(form, error, SETUP_FIELDS)) {
      setNotice(errorText(error));
    }
  };

  const HandleValid = async (values: TSetupForm) => {
    setNotice(null);
    const result = await submit.Run(values);
    form.resetField('Password');
    form.resetField('ConfirmPassword');
    if (isApiError(result)) {
      Reject(result);
      return;
    }
    useSetupTokenStore.getState().Clear();
    Push({
      Id: 'setup-done',
      Accent: result ? 'live' : 'awaiting',
      Title: result ? t('setup.done.title') : t('setup.signinFailed.title'),
      Detail: result ? t('setup.done.detail', { name: values.OwnerName }) : t('setup.signinFailed.detail'),
    });
    onDone(result);
  };

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-5">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <fieldset className="m-0 grid gap-4 border-0 p-0">
        <legend className="mb-3 p-0 text-title-sm">{t('setup.section.owner')}</legend>
        <OwnerFields form={form} />
        <PasswordFields form={form} />
      </fieldset>
      <fieldset className="m-0 grid gap-4 border-0 p-0">
        <legend className="mb-3 p-0 text-title-sm">{t('setup.section.organization')}</legend>
        <OrganizationFields form={form} />
      </fieldset>
      <Button type="submit" variant="primary" disabled={submit.Pending} className="w-full">
        {submit.Pending ? t('setup.submitting') : t('setup.submit')}
      </Button>
      <p className="m-0 flex items-start gap-2 text-caption text-secondary">
        <Icon name="shield" size={16} className="mt-px text-tertiary" />
        <span>{t('setup.closeNotice')}</span>
      </p>
    </form>
  );
};
