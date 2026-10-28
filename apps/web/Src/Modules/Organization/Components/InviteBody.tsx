import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { grantableRoles } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { EMAIL_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { ChoiceList } from '@/Core/Ui/ChoiceList';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { createInvitation } from '@/Modules/Organization/Api/OrganizationApi';
import { InviteLink } from '@/Modules/Organization/Components/InviteLink';
import { invitationsKey } from '@/Modules/Organization/Hooks/useOrganization';
import { useInviteStore } from '@/Modules/Organization/Store/useInviteStore';
import { INVITE_FIELDS, INVITE_FORM_ID, inviteFormSchema, type TInviteForm } from '@/Modules/Organization/Types/Forms';

const InviteForm = ({ organizationId, roles }: { organizationId: string; roles: TInviteForm['Role'][] }) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const SetBusy = useInviteStore((state) => state.SetBusy);
  const SetIssued = useInviteStore((state) => state.SetIssued);
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TInviteForm>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { Email: '', Role: roles.includes('developer') ? 'developer' : (roles[roles.length - 1] ?? 'viewer') },
  });
  const { errors } = form.formState;

  const HandleValid = async (values: TInviteForm) => {
    setNotice(null);
    SetBusy(true);
    try {
      const issued = await createInvitation(organizationId, values.Email, values.Role);
      void client.invalidateQueries({ queryKey: invitationsKey(organizationId) });
      SetIssued(issued);
    } catch (error) {
      SetBusy(false);
      if (isApiError(error) && applyFieldErrors(form, error, INVITE_FIELDS)) return;
      setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
    }
  };

  return (
    <form id={INVITE_FORM_ID} noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('invite.email')} error={message(errors.Email?.message)}>
        <Input
          type="email"
          dir="ltr"
          inputMode="email"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          maxLength={EMAIL_MAX}
          {...form.register('Email')}
        />
      </Field>
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('invite.role')}</span>
        <Controller
          control={form.control}
          name="Role"
          render={({ field }) => (
            <ChoiceList
              label={t('invite.role')}
              value={field.value}
              onChange={field.onChange}
              choices={roles.map((role) => ({ value: role, title: t(`role.${role}`), about: t(`role.${role}.about`) }))}
            />
          )}
        />
      </div>
    </form>
  );
};

export const InviteBody = () => {
  const t = useT();
  const membership = useCurrentMembership();
  const Issued = useInviteStore((state) => state.Issued);
  const SetIssued = useInviteStore((state) => state.SetIssued);

  useEffect(
    () => () => {
      SetIssued(null);
    },
    [SetIssued],
  );

  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={4} />;
  if (Issued) return <InviteLink issued={Issued} copy={false} />;
  return <InviteForm organizationId={membership.OrganizationId} roles={grantableRoles(membership.Role)} />;
};
