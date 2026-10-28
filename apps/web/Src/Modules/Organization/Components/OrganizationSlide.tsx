import { useId } from 'react';

import { canManageMembers, type TRole } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useApiErrorText } from '@/Core/Hooks/useApiErrorText';
import { Button } from '@/Core/Ui/Button';
import { InlineAlert } from '@/Core/Ui/InlineAlert';
import { Skeleton } from '@/Core/Ui/Skeleton';
import { tabId, tabPanelId, Tabs, type ITab } from '@/Core/Ui/Tabs';
import { useT } from '@/I18n/useT';
import { InvitationsCard } from '@/Modules/Organization/Components/InvitationsCard';
import { MembersCard } from '@/Modules/Organization/Components/MembersCard';
import { SettingsSection } from '@/Modules/Organization/Components/SettingsSection';
import { TeamsCard } from '@/Modules/Organization/Components/TeamsCard';
import { useOrganization } from '@/Modules/Organization/Hooks/useOrganization';
import { useOrganizationTabStore, type TOrganizationTab } from '@/Modules/Organization/Store/useOrganizationTabStore';

export interface IOrganizationSlideProps {
  onOpenMember: (userId: string) => void;
  onOpenTeam: (teamId: string) => void;
  onNewTeam: () => void;
  onTransfer: () => void;
  onLeft: () => void;
}

const Loaded = ({ organizationId, role, ...actions }: IOrganizationSlideProps & { organizationId: string; role: TRole }) => {
  const t = useT();
  const base = useId();
  const errorText = useApiErrorText();
  const organization = useOrganization(organizationId);
  const Stored = useOrganizationTabStore((state) => state.Tab);
  const SetTab = useOrganizationTabStore((state) => state.SetTab);
  const tabs: ITab<TOrganizationTab>[] = [
    { id: 'members', label: t('organization.tab.members') },
    { id: 'teams', label: t('organization.tab.teams') },
    ...(canManageMembers(role) ? [{ id: 'invitations' as const, label: t('organization.tab.invitations') }] : []),
    { id: 'settings', label: t('organization.tab.settings') },
  ];
  const Tab = tabs.some((tab) => tab.id === Stored) ? Stored : 'members';

  if (organization.isError) {
    return (
      <div className="grid justify-items-start gap-3">
        <InlineAlert accent="failed" title={t('organization.error')}>
          {errorText(organization.error)}
        </InlineAlert>
        <Button
          size="sm"
          onClick={() => {
            void organization.refetch();
          }}
        >
          {t('status.retry')}
        </Button>
      </div>
    );
  }
  if (organization.data === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;

  return (
    <>
      <Tabs base={base} label={t('organization.tabs')} tabs={tabs} value={Tab} onChange={SetTab} />
      <div role="tabpanel" id={tabPanelId(base, Tab)} aria-labelledby={tabId(base, Tab)} className="grid gap-4">
        {Tab === 'members' ? (
          <MembersCard
            organizationId={organizationId}
            role={role}
            onOpenMember={canManageMembers(role) ? actions.onOpenMember : undefined}
          />
        ) : null}
        {Tab === 'teams' ? (
          <TeamsCard organizationId={organizationId} role={role} onOpenTeam={actions.onOpenTeam} onNewTeam={actions.onNewTeam} />
        ) : null}
        {Tab === 'invitations' ? <InvitationsCard organizationId={organizationId} role={role} /> : null}
        {Tab === 'settings' ? (
          <SettingsSection organization={organization.data} role={role} onTransfer={actions.onTransfer} onLeft={actions.onLeft} />
        ) : null}
      </div>
    </>
  );
};

export const OrganizationSlide = (props: IOrganizationSlideProps) => {
  const t = useT();
  const membership = useCurrentMembership();
  if (membership === undefined) return <Skeleton label={t('ui.loading')} lines={5} />;
  return <Loaded key={membership.OrganizationId} organizationId={membership.OrganizationId} role={membership.Role} {...props} />;
};

export const OrganizationSubtitle = () => {
  const membership = useCurrentMembership();
  const t = useT();
  return <span>{membership ? `${membership.OrganizationName} · ${t(`role.${membership.Role}`)}` : t('organization.subtitle')}</span>;
};

export interface IOrganizationFooterProps {
  onInvite: () => void;
  onAudit: () => void;
}

export const OrganizationFooter = ({ onInvite, onAudit }: IOrganizationFooterProps) => {
  const t = useT();
  const role = useCurrentMembership()?.Role;
  return (
    <>
      {role === 'owner' || role === 'admin' ? (
        <Button size="sm" variant="ghost" iconStart="file" onClick={onAudit}>
          {t('organization.audit')}
        </Button>
      ) : null}
      <span className="flex-1" />
      {canManageMembers(role) ? (
        <Button size="sm" variant="primary" iconStart="plus" onClick={onInvite}>
          {t('organization.invite')}
        </Button>
      ) : null}
    </>
  );
};
