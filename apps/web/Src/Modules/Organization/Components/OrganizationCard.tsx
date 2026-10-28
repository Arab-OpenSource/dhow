import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import type { ApiError } from '@/Core/Api/ApiError';
import type { TOrganization } from '@/Core/Auth/AuthTypes';
import { NAME_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { useRelativeTime } from '@/Core/Hooks/useRelativeTime';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import { useRenameOrganization } from '@/Modules/Organization/Hooks/useOrganization';
import { ORGANIZATION_FIELDS, organizationFormSchema, type TOrganizationForm } from '@/Modules/Organization/Types/OrganizationForm';

export interface IOrganizationCardProps {
  organization: TOrganization;
  editable: boolean;
}

const RenameForm = ({ organization }: { organization: TOrganization }) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const Push = useToastStore((state) => state.Push);
  const rename = useRenameOrganization(organization.Id);
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TOrganizationForm>({
    resolver: zodResolver(organizationFormSchema),
    values: { Name: organization.Name },
  });
  const { errors, isDirty } = form.formState;

  const HandleValid = (values: TOrganizationForm) => {
    setNotice(null);
    rename.mutate(values.Name, {
      onSuccess: (updated) => {
        Push({ Id: `organization-renamed-${updated.Id}`, Accent: 'live', Title: t('organization.renamed', { name: updated.Name }) });
      },
      onError: (error: ApiError) => {
        if (!applyFieldErrors(form, error, ORGANIZATION_FIELDS)) setNotice(errorText(error));
      },
    });
  };

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-3 px-4 py-3">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('organization.name')} error={message(errors.Name?.message)}>
        <Input maxLength={NAME_MAX} autoComplete="organization" {...form.register('Name')} />
      </Field>
      <Button type="submit" variant="primary" className="justify-self-start" disabled={!isDirty || rename.isPending}>
        {rename.isPending ? t('organization.saving') : t('organization.save')}
      </Button>
    </form>
  );
};

export const OrganizationCard = ({ organization, editable }: IOrganizationCardProps) => {
  const t = useT();
  const time = useRelativeTime();

  return (
    <Card title={t('organization.card.title')} icon="building">
      {editable ? <RenameForm organization={organization} /> : null}
      <Rows label={t('organization.card.title')}>
        {editable ? null : <Row title={organization.Name} sub={t('organization.name')} />}
        <Row title={organization.Slug} sub={t('organization.slug')} mono />
        <Row title={time.full(organization.CreatedAt)} sub={t('organization.created')} />
      </Rows>
    </Card>
  );
};
