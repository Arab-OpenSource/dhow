import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { canReadAudit } from '@/Core/Auth/AuthTypes';
import { useMe } from '@/Core/Auth/useMe';
import { Mark } from '@/Core/Ui/Mark';
import { SideMenu, type IMenuItem, type TMenuEntry } from '@/Core/Ui/SideMenu';
import { useT } from '@/I18n/useT';

export const ShellMenu = () => {
  const t = useT();
  const { stack, goWorkspace, openStack } = useShellNavigation();

  const me = useMe();
  const role = me.data?.Memberships[0]?.Role;

  const items: TMenuEntry[] = [
    { id: 'home', icon: 'home', label: t('shell.nav.home'), href: '/', current: stack.length === 0, onSelect: goWorkspace },
    { id: 'projects', icon: 'layers', label: t('shell.nav.projects'), hint: t('shell.soon') },
    { id: 'runners', icon: 'server', label: t('shell.nav.runners'), hint: t('shell.soon') },
    { id: 'team', icon: 'users', label: t('shell.nav.team'), hint: t('shell.soon') },
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
    },
  ];

  return (
    <SideMenu
      label={t('shell.nav.label')}
      brand={
        <>
          <Mark size={28} />
          <span className="text-title-xl">{t('shell.brand')}</span>
        </>
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
      }}
    />
  );
};
