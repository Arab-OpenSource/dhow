import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { ME_QUERY_KEY } from '@/Core/Auth/MeQuery';
import { NAME_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { useOrganizationStore } from '@/Core/Store/useOrganizationStore';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { useT } from '@/I18n/useT';
import { createOrganization } from '@/Modules/Organization/Api/OrganizationApi';
import {
  NEW_ORGANIZATION_FIELDS,
  newOrganizationSchema,
  SLUG_MAX,
  type TNewOrganizationForm,
} from '@/Modules/Organization/Types/Forms';

export interface IOrganizationNewBodyProps {
  onCreated: () => void;
}

const slugify = (name: string): string =>
  name
    .normalize('NFKD')
    .replaceAll(/\p{M}/gu, '')
    .toLowerCase()
    .replaceAll(/[^a-z0-9]+/g, '-')
    .replaceAll(/^-+|-+$/g, '')
    .slice(0, SLUG_MAX)
    .replaceAll(/-+$/g, '');

export const OrganizationNewBody = ({ onCreated }: IOrganizationNewBodyProps) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const [Notice, setNotice] = useState<string | null>(null);
  const [Busy, setBusy] = useState(false);
  const [SlugTouched, setSlugTouched] = useState(false);
  const form = useForm<TNewOrganizationForm>({ resolver: zodResolver(newOrganizationSchema), defaultValues: { Name: '', Slug: '' } });
  const { errors } = form.formState;
  const name = form.register('Name');
  const slug = form.register('Slug');

  const HandleValid = async (values: TNewOrganizationForm) => {
    setBusy(true);
    setNotice(null);
    try {
      const organization = await createOrganization(values.Name, values.Slug);
      useOrganizationStore.getState().Set(organization.Id);
      await client.invalidateQueries({ queryKey: ME_QUERY_KEY });
      Push({ Id: `organization-created-${organization.Id}`, Accent: 'live', Title: t('organization.new.created', { name: organization.Name }) });
      onCreated();
    } catch (error) {
      if (!(isApiError(error) && applyFieldErrors(form, error, NEW_ORGANIZATION_FIELDS))) {
        setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('organization.name')} error={message(errors.Name?.message)}>
        <Input
          maxLength={NAME_MAX}
          autoComplete="organization"
          {...name}
          onChange={(event) => {
            void name.onChange(event);
            if (!SlugTouched) form.setValue('Slug', slugify(event.target.value));
          }}
        />
      </Field>
      <Field label={t('organization.slug')} error={message(errors.Slug?.message)}>
        <Input
          dir="ltr"
          maxLength={SLUG_MAX}
          autoCapitalize="none"
          spellCheck={false}
          {...slug}
          onChange={(event) => {
            setSlugTouched(true);
            void slug.onChange(event);
          }}
        />
      </Field>
      <Button type="submit" variant="primary" className="justify-self-start" disabled={Busy}>
        {Busy ? t('organization.new.creating') : t('organization.new.create')}
      </Button>
    </form>
  );
};
