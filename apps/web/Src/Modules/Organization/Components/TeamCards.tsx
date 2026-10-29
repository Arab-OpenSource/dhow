import { useState } from 'react';

import { canManageMembers, grantableRoles, type TRole } from '@/Core/Auth/AuthTypes';
import { useMembers } from '@/Core/Auth/useMembers';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { Field } from '@/Core/Ui/Field';
import { IconButton } from '@/Core/Ui/IconButton';
import { Row, Rows } from '@/Core/Ui/Row';
import { Select } from '@/Core/Ui/Select';
import { useT } from '@/I18n/useT';
import {
  addTeamMember,
  bindTeamRole,
  changeTeamRole,
  removeTeamMember,
  unbindTeamRole,
  type TTeamDetail,
} from '@/Modules/Organization/Api/OrganizationApi';
import { NO_ROLE, type TTeamRoleChoice } from '@/Modules/Organization/Components/TeamRolePicker';
import { orgRoleOf } from '@/Modules/Organization/Components/TeamsCard';
import { ALL_MEMBERS } from '@/Modules/Organization/Hooks/useOrganization';
import { useTeamChange } from '@/Modules/Organization/Hooks/useTeamChange';

export interface ITeamCardProps {
  organizationId: string;
  actor: TRole;
  team: TTeamDetail;
}

const TeamRoleForm = ({ organizationId, actor, team }: ITeamCardProps) => {
  const t = useT();
  const binding = orgRoleOf(team);
  const current: TTeamRoleChoice = binding?.Role ?? NO_ROLE;
  const [Role, setRole] = useState<TTeamRoleChoice>(current);
  const change = useTeamChange(organizationId, team.Id, ({ role }: { role: TTeamRoleChoice }) => {
    if (role === NO_ROLE) return unbindTeamRole(organizationId, team.Id, binding?.Id ?? '');
    if (binding) return changeTeamRole(organizationId, team.Id, binding.Id, role);
    return bindTeamRole(organizationId, team.Id, role);
  });
  const choices: TTeamRoleChoice[] = [NO_ROLE, ...grantableRoles(actor)];
  const label = (choice: TTeamRoleChoice) => (choice === NO_ROLE ? t('team.role.none') : t(`role.${choice}`));

  return (
    <div className="grid gap-3">
      <Field label={t('team.role.pick')}>
        <Select
          value={Role}
          options={choices.map((choice) => ({ value: choice, label: label(choice) }))}
          disabled={change.isPending}
          onChange={(event) => {
            const next = choices.find((choice) => choice === event.target.value);
            if (next !== undefined) setRole(next);
          }}
        />
      </Field>
      <p className="m-0 text-body-sm text-secondary">{Role === NO_ROLE ? t('team.role.noneAbout') : t(`role.${Role}.about`)}</p>
      <p className="m-0 text-body-sm text-tertiary">{t('team.new.subtitle')}</p>
      <Button
        size="sm"
        className="justify-self-start"
        disabled={change.isPending || Role === current}
        onClick={() => {
          change.mutate({ role: Role, done: t('team.role.saved') });
        }}
      >
        {t('team.role.save')}
      </Button>
    </div>
  );
};

export const TeamRoleCard = ({ organizationId, actor, team }: ITeamCardProps) => {
  const t = useT();
  const binding = orgRoleOf(team);

  return (
    <Card title={t('team.role')} icon="shield" padded>
      {canManageMembers(actor) ? (
        <TeamRoleForm organizationId={organizationId} actor={actor} team={team} />
      ) : (
        <p className="m-0 text-body-sm text-secondary">
          {binding ? t('teams.roleIn', { role: t(`role.${binding.Role}`) }) : t('teams.noRole')}
        </p>
      )}
    </Card>
  );
};

type TAddMemberProps = Omit<ITeamCardProps, 'actor'>;

const AddMember = ({ organizationId, team }: TAddMemberProps) => {
  const t = useT();
  const [Open, setOpen] = useState(false);
  const members = useMembers(Open ? organizationId : undefined, 1, ALL_MEMBERS);
  const [Choice, setChoice] = useState('');
  const add = useTeamChange(organizationId, team.Id, ({ userId }: { userId: string }) => addTeamMember(organizationId, team.Id, userId));
  const inTeam = new Set(team.Members.map((member) => member.UserId));
  const candidates = (members.data?.Items ?? []).filter((member) => !inTeam.has(member.UserId));
  const picked = candidates.find((member) => member.UserId === Choice) ?? candidates.at(0);

  if (!Open || members.data === undefined) {
    return (
      <div className="border-t border-line px-card py-3">
        <Button
          size="sm"
          iconStart="plus"
          disabled={Open}
          onClick={() => {
            setOpen(true);
          }}
        >
          {t('team.members.add')}
        </Button>
      </div>
    );
  }
  if (picked === undefined) {
    return <p className="m-0 border-t border-line px-card py-3 text-body-sm text-tertiary">{t('team.members.none')}</p>;
  }
  return (
    <div className="flex flex-wrap items-end gap-2 border-t border-line px-card py-3">
      <Field label={t('team.members.pick')} className="min-w-[12rem] flex-1">
        <Select
          compact
          value={picked.UserId}
          options={candidates.map((member) => ({ value: member.UserId, label: `${member.Name} · ${member.Email}` }))}
          disabled={add.isPending}
          onChange={(event) => {
            setChoice(event.target.value);
          }}
        />
      </Field>
      <Button
        size="sm"
        iconStart="plus"
        disabled={add.isPending}
        onClick={() => {
          add.mutate({ userId: picked.UserId, done: t('team.members.added', { name: picked.Name }) });
        }}
      >
        {t('team.members.addPicked')}
      </Button>
    </div>
  );
};

export const TeamMembersCard = ({ organizationId, actor, team }: ITeamCardProps) => {
  const t = useT();
  const manager = canManageMembers(actor);
  const remove = useTeamChange(organizationId, team.Id, ({ userId }: { userId: string }) =>
    removeTeamMember(organizationId, team.Id, userId),
  );

  return (
    <Card title={t('team.members.title')} icon="users" end={t('teams.count', { count: team.Members.length })}>
      {team.Members.length === 0 ? (
        <EmptyState icon="users" text={manager ? t('team.members.emptyManager') : t('team.members.empty')} className="py-6" />
      ) : (
        <Rows label={t('team.members.title')}>
          {team.Members.map((member) => (
            <Row
              key={member.UserId}
              start={<Avatar name={member.Name} size="sm" />}
              title={<bdi>{member.Name}</bdi>}
              sub={
                <bdi dir="ltr" className="block truncate text-start">
                  {member.Email}
                </bdi>
              }
              end={
                <>
                  <Badge tone="mute">{t(`role.${member.Role}`)}</Badge>
                  {manager ? (
                    <IconButton
                      icon="close"
                      label={t('team.members.remove', { name: member.Name })}
                      disabled={remove.isPending}
                      onClick={() => {
                        remove.mutate({ userId: member.UserId, done: t('team.members.removed', { name: member.Name }) });
                      }}
                    />
                  ) : null}
                </>
              }
            />
          ))}
        </Rows>
      )}
      {manager ? <AddMember organizationId={organizationId} team={team} /> : null}
    </Card>
  );
};
