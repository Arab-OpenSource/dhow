import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useT } from '@/I18n/useT';
import { TeamsCard } from '@/Modules/Organization/Components/TeamsCard';

export const TeamsPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();
  const membership = useCurrentMembership();

  return (
    <Page title={t('shell.nav.teams')} lead={t('page.teams.lead')} nav={<OrgNav />}>
      {membership ? (
        <TeamsCard
          organizationId={membership.OrganizationId}
          role={membership.Role}
          onOpenTeam={(teamId) => {
            void navigate({ to: '/org/teams/$teamId', params: { teamId } });
          }}
          onNewTeam={() => {
            open('team-new');
          }}
        />
      ) : null}
    </Page>
  );
};
