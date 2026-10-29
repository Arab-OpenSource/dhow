import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { InviteAction } from '@/App/Pages/InviteAction';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useT } from '@/I18n/useT';
import { InvitationsCard } from '@/Modules/Organization/Components/InvitationsCard';

export const InvitationsPage = () => {
  const t = useT();
  const membership = useCurrentMembership();

  return (
    <Page title={t('shell.nav.invitations')} lead={t('page.invitations.lead')} action={<InviteAction />} nav={<OrgNav />}>
      {membership ? <InvitationsCard organizationId={membership.OrganizationId} role={membership.Role} /> : null}
    </Page>
  );
};
