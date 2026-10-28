import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import type { ApiError } from '@/Core/Api/ApiError';
import { canManageMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { NAME_MAX } from '@/Core/Forms/Fields';
import { applyFieldErrors } from '@/Core/Hooks/FormErrors';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { useFormMessage } from '@/Core/Hooks/useFormMessage';
import { useToastStore } from '@/Core/Store/useToastStore';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Field } from '@/Core/Ui/Field';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Input } from '@/Core/Ui/Input';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { deleteTeam, updateTeam, type TTeamDetail } from '@/Modules/Organization/Api/OrganizationApi';
import { TeamMembersCard, TeamRoleCard } from '@/Modules/Organization/Components/TeamCards';
import { teamKey, useRefreshOrganization, useTeam } from '@/Modules/Organization/Hooks/useOrganization';
import { TEAM_DESCRIPTION_MAX, TEAM_FIELDS, teamFormSchema, type TTeamForm } from '@/Modules/Organization/Types/Forms';

export interface ITeamBodyProps {
  teamId: string;
  onDeleted: () => void;
}

interface ITeamPartProps {
  organizationId: string;
  team: TTeamDetail;
}

const DetailsCard = ({ organizationId, team }: ITeamPartProps) => {
  const t = useT();
  const message = useFormMessage();
  const errorText = useApiErrorText();
  const client = useQueryClient();
  const Push = useToastStore((state) => state.Push);
  const [Notice, setNotice] = useState<string | null>(null);
  const form = useForm<TTeamForm>({
    resolver: zodResolver(teamFormSchema),
    values: { Name: team.Name, Description: team.Description },
  });
  const save = useMutation<TTeamDetail, ApiError, TTeamForm>({
    mutationFn: (values) => updateTeam(organizationId, team.Id, values.Name, values.Description),
    onSuccess: (updated) => {
      client.setQueryData(teamKey(organizationId, team.Id), updated);
      void client.invalidateQueries({ queryKey: ['teams', organizationId] });
      Push({ Id: `team-saved-${team.Id}`, Accent: 'live', Title: t('team.saved') });
    },
    onError: (error) => {
      if (!applyFieldErrors(form, error, TEAM_FIELDS)) setNotice(errorText(error));
    },
  });
  const { errors, isDirty } = form.formState;

  return (
    <Card title={t('team.details')} icon="users" padded>
      <form
        noValidate
        onSubmit={(event) =>
          void form.handleSubmit((values) => {
            setNotice(null);
            save.mutate(values);
          })(event)
        }
        className="grid gap-3"
      >
        {Notice ? <InlineAlert accent="failed" title={Notice} /> : null}
        <Field label={t('team.name')} error={message(errors.Name?.message)}>
          <Input maxLength={NAME_MAX} autoComplete="off" {...form.register('Name')} />
        </Field>
        <Field label={t('team.description')} error={message(errors.Description?.message)}>
          <Input maxLength={TEAM_DESCRIPTION_MAX} autoComplete="off" {...form.register('Description')} />
        </Field>
        <Button type="submit" size="sm" variant="primary" className="justify-self-start" disabled={!isDirty || save.isPending}>
          {t('team.save')}
        </Button>
      </form>
    </Card>
  );
};

const DeleteCard = ({ organizationId, team, onDeleted }: ITeamPartProps & { onDeleted: () => void }) => {
  const t = useT();
  const errorText = useApiErrorText();
  const refresh = useRefreshOrganization(organizationId);
  const Push = useToastStore((state) => state.Push);
  const [Confirming, setConfirming] = useState(false);
  const remove = useMutation<unknown, ApiError>({
    mutationFn: () => deleteTeam(organizationId, team.Id),
    onSuccess: () => {
      Push({ Id: `team-deleted-${team.Id}`, Accent: 'live', Title: t('team.deleted', { name: team.Name }) });
      void refresh(team.Id);
      onDeleted();
    },
  });

  return (
    <Card title={t('team.delete')} icon="trash" padded>
      <div className="grid gap-3">
        <p className="m-0 text-body-sm text-secondary">{t('team.delete.about')}</p>
        {remove.error ? <InlineAlert accent="failed" title={errorText(remove.error)} /> : null}
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="danger"
            iconStart="trash"
            disabled={remove.isPending}
            onClick={() => {
              if (Confirming) remove.mutate();
              else setConfirming(true);
            }}
          >
            {Confirming ? t('team.delete.confirm') : t('team.delete')}
          </Button>
          {Confirming ? (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setConfirming(false);
              }}
            >
              {t('member.cancel')}
            </Button>
          ) : null}
        </div>
      </div>
    </Card>
  );
};

const Loaded = ({ organizationId, actor, teamId, onDeleted }: ITeamBodyProps & { organizationId: string; actor: TRole }) => {
  const t = useT();
  const team = useTeam(organizationId, teamId);
  if (team.isError) return <EmptyState icon="users" text={t('team.error')} />;
  if (team.data === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  const manager = canManageMembers(actor);
  return (
    <>
      {manager ? (
        <DetailsCard organizationId={organizationId} team={team.data} />
      ) : (
        <Card padded>
          <div className="grid gap-1">
            <span className="text-title-sm">{team.data.Name}</span>
            {team.data.Description === '' ? null : <p className="m-0 text-body-sm text-secondary">{team.data.Description}</p>}
          </div>
        </Card>
      )}
      <TeamRoleCard key={team.data.Bindings.map((binding) => binding.Role).join()} organizationId={organizationId} actor={actor} team={team.data} />
      <TeamMembersCard organizationId={organizationId} actor={actor} team={team.data} />
      {manager ? <DeleteCard organizationId={organizationId} team={team.data} onDeleted={onDeleted} /> : null}
    </>
  );
};

export const TeamBody = ({ teamId, onDeleted }: ITeamBodyProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return <Loaded organizationId={membership.OrganizationId} actor={membership.Role} teamId={teamId} onDeleted={onDeleted} />;
};
