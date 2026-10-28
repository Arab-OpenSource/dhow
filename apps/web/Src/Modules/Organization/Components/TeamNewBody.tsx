import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { isApiError } from '@/Core/Api/ApiError';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { NAME_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { createTeam } from '@/Modules/Organization/Api/OrganizationApi';
import { NO_ROLE, TeamRolePicker, type TTeamRoleChoice } from '@/Modules/Organization/Components/TeamRolePicker';
import { teamsKey } from '@/Modules/Organization/Hooks/useOrganization';
import { TEAM_DESCRIPTION_MAX, TEAM_FIELDS, teamFormSchema, type TTeamForm } from '@/Modules/Organization/Types/Forms';

export interface ITeamNewBodyProps {
  onCreated: (teamId: string) => void;
}

export const TeamNewBody = ({ onCreated }: ITeamNewBodyProps) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const membership = useCurrentMembership();
  const Push = useToastStore((state) => state.Push);
  const [Role, setRole] = useState<TTeamRoleChoice>(NO_ROLE);
  const [Busy, setBusy] = useState(false);
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TTeamForm>({ resolver: zodResolver(teamFormSchema), defaultValues: { Name: '', Description: '' } });
  const { errors } = form.formState;

  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={4} />;
  const organizationId = membership.OrganizationId;

  const HandleValid = async (values: TTeamForm) => {
    setBusy(true);
    setNotice(null);
    try {
      const team = await createTeam(organizationId, values.Name, values.Description, Role === NO_ROLE ? undefined : Role);
      void client.invalidateQueries({ queryKey: teamsKey(organizationId) });
      Push({ Id: `team-created-${team.Id}`, Accent: 'live', Title: t('team.created', { name: team.Name }) });
      onCreated(team.Id);
    } catch (error) {
      if (!(isApiError(error) && applyFieldErrors(form, error, TEAM_FIELDS))) {
        setNotice(isApiError(error) ? errorText(error) : t('api.error.unknown'));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <form noValidate onSubmit={(event) => void form.handleSubmit(HandleValid)(event)} className="grid gap-4">
      {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
      <Field label={t('team.name')} error={message(errors.Name?.message)}>
        <Input maxLength={NAME_MAX} autoComplete="off" {...form.register('Name')} />
      </Field>
      <Field label={t('team.description')} error={message(errors.Description?.message)}>
        <Input maxLength={TEAM_DESCRIPTION_MAX} autoComplete="off" {...form.register('Description')} />
      </Field>
      <div className="grid gap-1.5">
        <span className="text-body-sm text-secondary">{t('team.role')}</span>
        <TeamRolePicker actor={membership.Role} value={Role} onChange={setRole} disabled={Busy} />
      </div>
      <Button type="submit" variant="primary" className="justify-self-start" disabled={Busy}>
        {Busy ? t('team.creating') : t('team.create')}
      </Button>
    </form>
  );
};
