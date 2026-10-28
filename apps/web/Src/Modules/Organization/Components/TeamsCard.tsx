import { canManageMembers, canReadMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { EmptyState } from '@/Core/Ui/EmptyState';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Row, Rows } from '@/Core/Ui/Row';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import type { TTeam } from '@/Modules/Organization/Api/OrganizationApi';
import { useTeams } from '@/Modules/Organization/Hooks/useOrganization';

export interface ITeamsCardProps {
  organizationId: string;
  role: TRole;
  onOpenTeam: (teamId: string) => void;
  onNewTeam: () => void;
}

export const orgRoleOf = (team: TTeam) => team.Bindings.at(0);

type TTeamsListProps = Pick<ITeamsCardProps, 'organizationId' | 'onOpenTeam'>;

const TeamsList = ({ organizationId, onOpenTeam }: TTeamsListProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const teams = useTeams(organizationId);

  if (teams.isError) {
    return (
      <div className="p-4">
        <InlineAlert accent="failed" title={t('teams.error')}>
          {errorText(teams.error)}
        </InlineAlert>
      </div>
    );
  }
  if (teams.data === undefined) return <Skeleton label={t('teams.loading')} lines={3} className="p-4" />;
  if (teams.data.length === 0) return <EmptyState icon="users" text={t('teams.empty')} />;
  return (
    <Rows label={t('teams.title')}>
      {teams.data.map((team) => {
        const binding = orgRoleOf(team);
        return (
          <Row
            key={team.Id}
            icon="users"
            title={team.Name}
            sub={
              <>
                {t('teams.count', { count: team.MemberCount })}
                {team.Description === '' ? null : ` · ${team.Description}`}
              </>
            }
            end={
              <Badge tone="mute">{binding ? t('teams.roleIn', { role: t(`role.${binding.Role}`) }) : t('teams.noRole')}</Badge>
            }
            onSelect={() => {
              onOpenTeam(team.Id);
            }}
          />
        );
      })}
    </Rows>
  );
};

export const TeamsCard = ({ organizationId, role, onOpenTeam, onNewTeam }: ITeamsCardProps) => {
  const t = useT();
  return (
    <Card
      title={t('teams.title')}
      icon="users"
      end={
        canManageMembers(role) ? (
          <Button size="sm" iconStart="plus" onClick={onNewTeam}>
            {t('teams.new')}
          </Button>
        ) : undefined
      }
    >
      {canReadMembers(role) ? (
        <TeamsList organizationId={organizationId} onOpenTeam={onOpenTeam} />
      ) : (
        <EmptyState icon="users" text={t('members.forbidden')} />
      )}
    </Card>
  );
};
