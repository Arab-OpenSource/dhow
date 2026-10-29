import { useNavigate, useParams } from '@tanstack/react-router';

import { NotFoundPage } from '@/App/Layout/NotFoundPage';
import { Page } from '@/App/Layout/Page';
import { OrgNav } from '@/App/Layout/SectionNavs';
import { useSlide } from '@/App/Navigation/useSlide';
import { BackLink } from '@/App/Pages/BackLink';
import { InviteAction } from '@/App/Pages/InviteAction';
import { OrganizationGate } from '@/App/Pages/OrganizationGate';
import { isUuid } from '@/App/Pages/Params';
import { MasterDetail } from '@/Core/Ui/MasterDetail';
import { useT } from '@/I18n/useT';
import { MemberBody } from '@/Modules/Organization/Components/MemberBody';
import { MembersCard } from '@/Modules/Organization/Components/MembersCard';

export const MembersPage = () => {
  const t = useT();
  const navigate = useNavigate();
  const { open } = useSlide();
  const { memberId } = useParams({ strict: false });
  if (memberId !== undefined && !isUuid(memberId)) return <NotFoundPage />;

  return (
    <Page title={t('shell.nav.members')} lead={t('page.members.lead')} action={<InviteAction />} nav={<OrgNav />}>
      <OrganizationGate>
        {(organization, membership) => (
          <MasterDetail
            listLabel={t('members.title')}
            detailLabel={t('shell.crumb.member')}
            list={
              <MembersCard
                organizationId={organization.Id}
                role={membership.Role}
                rule={organization.RequireSecondFactor}
                selectedId={memberId}
                hrefOf={(userId) => `/org/members/${userId}`}
                onOpenMember={(userId) => {
                  void navigate({ to: '/org/members/$memberId', params: { memberId: userId } });
                }}
              />
            }
            detail={
              memberId === undefined ? undefined : (
                <>
                  <BackLink to="/org/members" label={t('members.back')} />
                  <MemberBody
                    key={memberId}
                    organizationId={organization.Id}
                    actor={membership.Role}
                    rule={organization.RequireSecondFactor}
                    userId={memberId}
                    onRemove={() => {
                      open('member-remove');
                    }}
                    onOpenAccount={() => {
                      void navigate({ to: '/account/security' });
                    }}
                    teamHrefOf={(teamId) => `/org/teams/${teamId}`}
                    onOpenTeam={(teamId) => {
                      void navigate({ to: '/org/teams/$teamId', params: { teamId } });
                    }}
                  />
                </>
              )
            }
          />
        )}
      </OrganizationGate>
    </Page>
  );
};
