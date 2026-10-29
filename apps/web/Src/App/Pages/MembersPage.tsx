import { useNavigate } from '@tanstack/react-router';

import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { InviteAction } from '@/App/Pages/InviteAction';
import { OrganizationGate } from '@/App/Pages/OrganizationGate';
import { canManageMembers } from '@/Core/Auth/AuthTypes';
import { useT } from '@/I18n/useT';
import { MembersCard } from '@/Modules/Organization/Components/MembersCard';

export const MembersPage = () => {
  const t = useT();
  const navigate = useNavigate();

  return (
    <Page title={t('shell.nav.members')} lead={t('page.members.lead')} action={<InviteAction />} nav={<OrgNav />}>
      <OrganizationGate>
        {(organization, membership) => (
          <MembersCard
            organizationId={organization.Id}
            role={membership.Role}
            rule={organization.RequireSecondFactor}
            onOpenMember={
              canManageMembers(membership.Role)
                ? (memberId) => {
                    void navigate({ to: '/org/members/$memberId', params: { memberId } });
                  }
                : undefined
            }
          />
        )}
      </OrganizationGate>
    </Page>
  );
};
