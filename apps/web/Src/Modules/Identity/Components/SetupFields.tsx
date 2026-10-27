import { useState } from 'react';
import { useWatch, type UseFormReturn } from 'react-hook-form';

import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { Field } from '@/Core/Ui/Field';
import { Input } from '@/Core/Ui/Input';
import { PasswordInput } from '@/Core/Ui/PasswordInput';
import { StrengthMeter } from '@/Core/Ui/StrengthMeter';
import { useT } from '@/I18n/useT';
import { EMAIL_MAX, NAME_MAX, SLUG_MAX, slugify, type TSetupForm } from '@/Modules/Identity/Types/SetupForm';

export interface ISetupFieldsProps {
  form: UseFormReturn<TSetupForm>;
}

export const OwnerFields = ({ form }: ISetupFieldsProps) => {
  const t = useT();
  const message = useFormMessage();
  const { errors } = form.formState;

  return (
    <>
      <Field label={t('setup.ownerName')} error={message(errors.OwnerName?.message)}>
        <Input maxLength={NAME_MAX} autoComplete="name" {...form.register('OwnerName')} />
      </Field>
      <Field label={t('field.email')} error={message(errors.Email?.message)}>
        <Input
          type="email"
          dir="ltr"
          inputMode="email"
          autoComplete="email"
          autoCapitalize="none"
          spellCheck={false}
          maxLength={EMAIL_MAX}
          {...form.register('Email')}
        />
      </Field>
    </>
  );
};

export const PasswordFields = ({ form }: ISetupFieldsProps) => {
  const t = useT();
  const message = useFormMessage();
  const { errors } = form.formState;
  const password = useWatch({ control: form.control, name: 'Password' });

  return (
    <>
      <div className="grid gap-2">
        <Field label={t('field.password')} error={message(errors.Password?.message)}>
          <PasswordInput autoComplete="new-password" {...form.register('Password')} />
        </Field>
        <StrengthMeter password={password} />
        <p className="m-0 text-caption text-tertiary">{t('password.hint')}</p>
      </div>
      <Field label={t('field.confirmPassword')} error={message(errors.ConfirmPassword?.message)}>
        <PasswordInput autoComplete="new-password" {...form.register('ConfirmPassword')} />
      </Field>
    </>
  );
};

export const OrganizationFields = ({ form }: ISetupFieldsProps) => {
  const t = useT();
  const message = useFormMessage();
  const { errors } = form.formState;
  const [SlugEdited, setSlugEdited] = useState(false);

  return (
    <>
      <Field label={t('setup.organizationName')} error={message(errors.OrganizationName?.message)}>
        <Input
          maxLength={NAME_MAX}
          autoComplete="organization"
          {...form.register('OrganizationName', {
            onChange: (event: { target: { value: string } }) => {
              if (!SlugEdited) form.setValue('OrganizationSlug', slugify(event.target.value));
            },
          })}
        />
      </Field>
      <div className="grid gap-1.5">
        <Field label={t('setup.slug')} error={message(errors.OrganizationSlug?.message)}>
          <Input
            dir="ltr"
            maxLength={SLUG_MAX}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            {...form.register('OrganizationSlug', {
              onChange: () => {
                setSlugEdited(true);
              },
            })}
          />
        </Field>
        <p className="m-0 text-caption text-tertiary">{t('setup.slug.hint')}</p>
      </div>
    </>
  );
};
