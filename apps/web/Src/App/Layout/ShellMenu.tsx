import { preloadFrame } from '@/App/Layout/LazyFrames';
import { OrganizationChip } from '@/App/Layout/OrganizationChip';
import type { TFrameId } from '@/App/Panels';
import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useSignOut } from '@/Core/Auth/useSignOut';
import { Mark } from '@/Core/Ui/Mark';
import { SideMenu, type IMenuItem, type TMenuEntry } from '@/Core/Ui/SideMenu';
import { useT } from '@/I18n/useT';

const intentFor = (id: TFrameId) => () => {
  void preloadFrame(id);
};

export const ShellMenu = () => {
  const t = useT();
  const { stack, goWorkspace, openStack } = useShellNavigation();

  const me = useMe();
  const { SignOut, Pending } = useSignOut();
  const role = useCurrentMembership()?.Role;

  const items: TMenuEntry[] = [
    { id: 'home', icon: 'home', label: t('shell.nav.home'), href: '/', current: stack.length === 0, onSelect: goWorkspace },
    { id: 'projects', icon: 'layers', label: t('shell.nav.projects'), hint: t('shell.soon') },
    {
      id: 'jobs',
      icon: 'play',
      label: t('shell.nav.jobs'),
      href: '/?stack=jobs',
      current: stack.some((frame) => frame === 'jobs' || frame.startsWith('job-')),
      onSelect: () => {
        openStack(['jobs']);
      },
      onIntent: intentFor('jobs'),
    },
    { id: 'runners', icon: 'server', label: t('shell.nav.runners'), hint: t('shell.soon') },
    {
      id: 'team',
      icon: 'users',
      label: t('shell.nav.team'),
      href: '/?stack=organization',
      current: stack.includes('organization'),
      onSelect: () => {
        openStack(['organization']);
      },
      onIntent: intentFor('organization'),
    },
  ];
  if (canReadAudit(role)) {
    items.push({
      id: 'audit',
      icon: 'file',
      label: t('shell.nav.audit'),
      href: '/?stack=audit',
      current: stack.includes('audit'),
      onSelect: () => {
        openStack(['audit']);
      },
      onIntent: intentFor('audit'),
    });
  }

  const bottom: IMenuItem[] = [
    {
      id: 'settings',
      icon: 'settings',
      label: t('shell.nav.settings'),
      href: '/?stack=appearance',
      current: stack.includes('appearance'),
      onSelect: () => {
        openStack(['appearance']);
      },
      onIntent: intentFor('appearance'),
    },
  ];

  return (
    <SideMenu
      label={t('shell.nav.label')}
      brand={
        <div className="grid min-w-0 gap-2.5">
          <span className="flex items-center gap-2.5">
            <Mark size={28} />
            <span className="text-title-xl">{t('shell.brand')}</span>
          </span>
          <OrganizationChip />
        </div>
      }
      items={items}
      bottom={bottom}
      account={{
        id: 'account',
        name: me.data?.User.Name ?? '',
        label: me.data?.User.Name ?? t('shell.nav.account'),
        href: '/?stack=account',
        current: stack.includes('account'),
        onSelect: () => {
          openStack(['account']);
        },
        onIntent: intentFor('account'),
        signOut: {
          label: t('account.signOut'),
          busy: Pending,
          onSelect: () => {
            void SignOut();
          },
        },
      }}
    />
  );
};
