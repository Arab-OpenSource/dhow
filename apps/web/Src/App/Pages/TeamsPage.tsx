import { useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { BackLink } from '@/App/Pages/BackLink';
import { isUuid } from '@/App/Pages/Params';
import { canManageMembers } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { Button } from '@/Core/Ui/Button';
import { MasterDetail } from '@/Core/Ui/MasterDetail';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { useT } from '@/I18n/useT';
import { TeamBody } from '@/Modules/Organization/Components/TeamBody';
import { TeamsCard } from '@/Modules/Organization/Components/TeamsCard';

export const TeamsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();
  const membership = useCurrentMembership();
  const { teamId } = useParams({ strict: false });
  if (teamId !== undefined && !isUuid(teamId)) return <NotFoundPage />;
  const manager = canManageMembers(membership?.Role);
  const HandleNew = () => {
    open('team-new');
  };

  return (
    <Page
      title={t('shell.nav.teams')}
      lead={t('page.teams.lead')}
      action={
        manager ? (
          <Button variant="primary" iconStart="plus" onClick={HandleNew}>
            {t('teams.new')}
          </Button>
        ) : undefined
      }
      nav={<OrgNav />}
    >
      {membership ? (
        <MasterDetail
          listLabel={t('teams.title')}
          detailLabel={t('shell.crumb.team')}
          list={
            <TeamsCard
              organizationId={membership.OrganizationId}
              selectedId={teamId}
              hrefOf={(id) => `/org/teams/${id}`}
              onOpenTeam={(id) => {
                void navigate({ to: '/org/teams/$teamId', params: { teamId: id } });
              }}
              onNewTeam={manager ? HandleNew : undefined}
            />
          }
          detail={
            teamId === undefined ? undefined : (
              <>
                <BackLink to="/org/teams" label={t('teams.back')} />
                <TeamBody
                  key={teamId}
                  organizationId={membership.OrganizationId}
                  actor={membership.Role}
                  teamId={teamId}
                  onDeleted={() => {
                    void navigate({ to: '/org/teams' });
                  }}
                />
              </>
            )
          }
        />
      ) : (
        <Skeleton label={t('ui.loading')} lines={5} />
      )}
    </Page>
  );
};
