import { useState } from 'react';

import { canManageMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useMembers } from '@/Core/Auth/useMembers';
import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { IconButton } from '@/Core/Ui/IconButton';
import { Row, Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import {
  addTeamMember,
  bindTeamRole,
  changeTeamRole,
  removeTeamMember,
  unbindTeamRole,
  type TTeamDetail,
} from '@/Modules/Organization/Api/OrganizationApi';
import { NO_ROLE, TeamRolePicker, type TTeamRoleChoice } from '@/Modules/Organization/Components/TeamRolePicker';
import { orgRoleOf } from '@/Modules/Organization/Components/TeamsCard';
import { useTeamChange } from '@/Modules/Organization/Hooks/useTeamChange';

export interface ITeamCardProps {
  organizationId: string;
  actor: TRole;
  team: TTeamDetail;
}

const ALL_MEMBERS = 100;

export const TeamRoleCard = ({ organizationId, actor, team }: ITeamCardProps) => {
  const t = useT();
  const binding = orgRoleOf(team);
  const [Role, setRole] = useState<TTeamRoleChoice>(binding?.Role ?? NO_ROLE);
  const change = useTeamChange(organizationId, team.Id, ({ role }: { role: TTeamRoleChoice }) => {
    if (role === NO_ROLE) return unbindTeamRole(organizationId, team.Id, binding?.Id ?? '');
    if (binding) return changeTeamRole(organizationId, team.Id, binding.Id, role);
    return bindTeamRole(organizationId, team.Id, role);
  });
  const manager = canManageMembers(actor);

  return (
    <Card title={t('team.role')} icon="shield" padded>
      {manager ? (
        <div className="grid gap-3">
          <TeamRolePicker actor={actor} value={Role} onChange={setRole} disabled={change.isPending} />
          <p className="m-0 text-body-sm text-tertiary">{t('team.new.subtitle')}</p>
          <Button
            size="sm"
            variant="primary"
            className="justify-self-start"
            disabled={change.isPending || Role === (binding?.Role ?? NO_ROLE)}
            onClick={() => {
              change.mutate({ role: Role, done: t('team.role.saved') });
            }}
          >
            {t('team.role.save')}
          </Button>
        </div>
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
  const add = useTeamChange(organizationId, team.Id, ({ userId }: { userId: string }) => addTeamMember(organizationId, team.Id, userId));
  const inTeam = new Set(team.Members.map((member) => member.UserId));
  const candidates = (members.data?.Items ?? []).filter((member) => !inTeam.has(member.UserId));

  if (!Open) {
    return (
      <div className="px-card py-3">
        <Button
          size="sm"
          iconStart="plus"
          onClick={() => {
            setOpen(true);
          }}
        >
          {t('team.members.add')}
        </Button>
      </div>
    );
  }
  return (
    <div className="grid gap-2 border-t border-line px-card py-3">
      <span className="text-body-sm text-secondary">{t('team.members.pick')}</span>
      {members.data !== undefined && candidates.length === 0 ? (
        <p className="m-0 text-body-sm text-tertiary">{t('team.members.none')}</p>
      ) : null}
      <div className="flex flex-wrap gap-2">
        {candidates.map((member) => (
          <Button
            key={member.UserId}
            size="sm"
            iconStart="plus"
            disabled={add.isPending}
            onClick={() => {
              add.mutate({ userId: member.UserId, done: t('team.members.added', { name: member.Name }) });
            }}
          >
            {member.Name}
          </Button>
        ))}
      </div>
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
        <EmptyState icon="users" text={t('team.members.empty')} />
      ) : (
        <Rows label={t('team.members.title')}>
          {team.Members.map((member) => (
            <Row
              key={member.UserId}
              start={<Avatar name={member.Name} size="sm" />}
              title={member.Name}
              sub={
                <bdi dir="ltr" className="font-mono">
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
