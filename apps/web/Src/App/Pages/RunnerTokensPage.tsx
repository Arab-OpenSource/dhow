import { Page } from '@/App/Layout/Page';
import { RunnersNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useT } from '@/I18n/useT';
import { RegistrationTokensCard } from '@/Modules/Runners/Components/RegistrationTokensCard';
import { RunnersFooter } from '@/Modules/Runners/Components/RunnersBody';

export const RunnerTokensPage = () => {
  const t = useT();
  const { open } = useSlide();
  const membership = useCurrentMembership();

  return (
    <Page
      title={t('runners.tokens.title')}
      lead={t('page.runnerTokens.lead')}
      action={
        <RunnersFooter
          onAddRunner={() => {
            open('runner-new');
          }}
        />
      }
      nav={<RunnersNav />}
    >
      {membership ? <RegistrationTokensCard organizationId={membership.OrganizationId} /> : null}
    </Page>
  );
};
