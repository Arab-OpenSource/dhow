import { useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { isUuid } from '@/App/Pages/Params';
import { useT } from '@/I18n/useT';
import { TeamBody } from '@/Modules/Organization/Components/TeamBody';

export const TeamPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { teamId } = useParams({ strict: false });
  if (!isUuid(teamId)) return <NotFoundPage />;

  return (
    <Page title={t('shell.crumb.team')} lead={t('team.subtitle')}>
      <TeamBody
        key={teamId}
        teamId={teamId}
        onDeleted={() => {
          void navigate({ to: '/org/teams' });
        }}
      />
    </Page>
  );
};
