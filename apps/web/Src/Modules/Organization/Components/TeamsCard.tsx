import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Badge } from '@/Core/Ui/Badge';
import { Button } from '@/Core/Ui/Button';
import { Card } from '@/Core/Ui/Card';
import { CardStates, cardStateOf } from '@/Core/Ui/CardStates';
import { Icon } from '@/Core/Ui/Icon';
import { Rows } from '@/Core/Ui/Row';
import { useT } from '@/I18n/useT';
import type { TTeam } from '@/Modules/Organization/Api/OrganizationApi';
import { LinkRow } from '@/Modules/Organization/Components/LinkRow';
import { useTeams } from '@/Modules/Organization/Hooks/useOrganization';

export interface ITeamsCardProps {
  organizationId: string;
  selectedId?: string;
  hrefOf: (teamId: string) => string;
  onOpenTeam: (teamId: string) => void;
  onNewTeam?: () => void;
}

export const orgRoleOf = (team: Pick<TTeam, 'Bindings'>) => team.Bindings.at(0);

const TeamSub = ({ team }: { team: TTeam }) => {
  const t = useT();
  return (
    <>
      <span className="block">{t('teams.count', { count: team.MemberCount })}</span>
      {team.Description === '' ? null : (
        <bdi className="block truncate">{team.Description}</bdi>
      )}
    </>
  );
};

export const TeamsCard = ({ organizationId, selectedId, hrefOf, onOpenTeam, onNewTeam }: ITeamsCardProps) => {
  const t = useT();
  const errorText = useApiErrorText();
  const teams = useTeams(organizationId);

  return (
    <Card title={t('teams.title')} icon="users" end={teams.data ? t('teams.total', { count: teams.data.length }) : undefined}>
      <CardStates
        state={cardStateOf(teams, teams.data?.length === 0)}
        loadingLabel={t('teams.loading')}
        errorTitle={t('teams.error')}
        errorText={teams.error ? errorText(teams.error) : undefined}
        retryLabel={t('status.retry')}
        onRetry={() => {
          void teams.refetch();
        }}
        emptyIcon="users"
        emptyText={t('teams.empty')}
        emptyAction={
          onNewTeam ? (
            <Button size="sm" iconStart="plus" onClick={onNewTeam}>
              {t('teams.new')}
            </Button>
          ) : undefined
        }
      >
        <Rows label={t('teams.title')}>
          {teams.data?.map((team) => {
            const binding = orgRoleOf(team);
            return (
              <LinkRow
                key={team.Id}
                href={hrefOf(team.Id)}
                current={team.Id === selectedId}
                start={<Icon name="users" size={18} className="text-secondary" />}
                title={<bdi>{team.Name}</bdi>}
                sub={<TeamSub team={team} />}
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
      </CardStates>
    </Card>
  );
};
