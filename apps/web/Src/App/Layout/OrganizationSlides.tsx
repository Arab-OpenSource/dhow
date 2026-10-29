import type { IFrameContext } from '@/App/Layout/FrameContext';
import {
  LazyInviteBody,
  LazyInviteFooter,
  LazyMemberBody,
  LazyOrganizationFooter,
  LazyOrganizationNewBody,
  LazyOrganizationsBody,
  LazyOrganizationSlide,
  LazyOrganizationSubtitle,
  LazyOrganizationTokenNewBody,
  LazyOrganizationTokensBody,
  LazyOrganizationTokensFooter,
  LazyTeamBody,
  LazyTeamNewBody,
  LazyTransferBody,
} from '@/App/Layout/LazyFrames';
import { memberFrame, memberOf, teamFrame, teamOf, type TFrameId } from '@/App/Panels';
import type { ISlide } from '@/Core/Ui/SlideStack';

const organizationSlide = (id: TFrameId, { t, navigation }: IFrameContext): ISlide => ({
  id,
  title: t('organization.title'),
  subtitle: <LazyOrganizationSubtitle />,
  children: (
    <LazyOrganizationSlide
      onOpenMember={(userId) => {
        navigation.pushFrame(memberFrame(userId));
      }}
      onOpenTeam={(teamId) => {
        navigation.pushFrame(teamFrame(teamId));
      }}
      onNewTeam={() => {
        navigation.pushFrame('team-new');
      }}
      onTransfer={() => {
        navigation.pushFrame('organization-transfer');
      }}
      onTokens={() => {
        navigation.pushFrame('organization-tokens');
      }}
      onLeft={navigation.closeAll}
    />
  ),
  footer: (
    <LazyOrganizationFooter
      onInvite={() => {
        navigation.pushFrame('organization-invite');
      }}
      onAudit={() => {
        navigation.pushFrame('audit');
      }}
      onTokens={() => {
        navigation.pushFrame('organization-tokens');
      }}
    />
  ),
});

const childSlide = (id: TFrameId, { t, navigation }: IFrameContext): ISlide | null => {
  const member = memberOf(id);
  if (member) {
    return {
      id,
      title: t('organization.tab.members'),
      subtitle: <span>{t('member.subtitle')}</span>,
      children: <LazyMemberBody userId={member} onRemoved={navigation.popFrame} />,
    };
  }
  const team = teamOf(id);
  if (team) {
    return {
      id,
      title: t('organization.tab.teams'),
      subtitle: <span>{t('team.subtitle')}</span>,
      children: <LazyTeamBody teamId={team} onDeleted={navigation.popFrame} />,
    };
  }
  return null;
};

export const buildOrganizationSlide = (id: TFrameId, context: IFrameContext): ISlide | null => {
  const { t, navigation } = context;
  if (id === 'organization') return organizationSlide(id, context);
  if (id === 'organizations') {
    return {
      id,
      title: t('organizations.title'),
      subtitle: <span>{t('organizations.subtitle')}</span>,
      children: (
        <LazyOrganizationsBody
          onSwitched={navigation.closeAll}
          onSettings={() => {
            navigation.openStack(['organization']);
          }}
          onCreate={() => {
            navigation.pushFrame('organization-new');
          }}
        />
      ),
    };
  }
  if (id === 'organization-new') {
    return {
      id,
      title: t('organization.new.title'),
      subtitle: <span>{t('organization.new.subtitle')}</span>,
      children: (
        <LazyOrganizationNewBody
          onCreated={() => {
            navigation.openStack(['organization']);
          }}
        />
      ),
    };
  }
  if (id === 'organization-invite') {
    return {
      id,
      title: t('invite.title'),
      subtitle: <span>{t('invite.subtitle')}</span>,
      children: <LazyInviteBody />,
      footer: <LazyInviteFooter onClose={navigation.popFrame} />,
    };
  }
  if (id === 'organization-transfer') {
    return {
      id,
      title: t('organization.transfer.title'),
      subtitle: <span>{t('organization.transfer.subtitle')}</span>,
      children: <LazyTransferBody onDone={navigation.popFrame} />,
    };
  }
  if (id === 'organization-tokens') {
    const onNew = () => {
      navigation.pushFrame('organization-token-new');
    };
    return {
      id,
      title: t('orgTokens.title'),
      subtitle: <span>{t('orgTokens.subtitle')}</span>,
      children: <LazyOrganizationTokensBody onNew={onNew} />,
      footer: <LazyOrganizationTokensFooter onNew={onNew} />,
    };
  }
  if (id === 'organization-token-new') {
    return {
      id,
      title: t('tokens.slide.title'),
      subtitle: <span>{t('orgTokens.new.subtitle')}</span>,
      children: <LazyOrganizationTokenNewBody onDone={navigation.popFrame} />,
    };
  }
  if (id === 'team-new') {
    return {
      id,
      title: t('team.new.title'),
      subtitle: <span>{t('team.new.subtitle')}</span>,
      children: (
        <LazyTeamNewBody
          onCreated={(teamId) => {
            navigation.openStack(['organization', teamFrame(teamId)]);
          }}
        />
      ),
    };
  }
  return childSlide(id, context);
};
