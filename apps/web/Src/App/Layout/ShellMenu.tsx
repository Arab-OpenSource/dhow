import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { Mark } from '@/Core/Ui/Mark';
import { SideMenu, type IMenuItem, type IPendingMenuItem } from '@/Core/Ui/SideMenu';
import { useT } from '@/I18n/useT';

export const ShellMenu = () => {
  const t = useT();
  const { stack, goWorkspace, openStack } = useShellNavigation();

  const items: IMenuItem[] = [
    { id: 'home', icon: 'home', label: t('shell.nav.home'), href: '/', current: stack.length === 0, onSelect: goWorkspace },
  ];

  const pending: IPendingMenuItem[] = [
    { id: 'projects', icon: 'layers', label: t('shell.nav.projects'), hint: t('shell.soon') },
    { id: 'runners', icon: 'server', label: t('shell.nav.runners'), hint: t('shell.soon') },
    { id: 'team', icon: 'users', label: t('shell.nav.team'), hint: t('shell.soon') },
    { id: 'audit', icon: 'file', label: t('shell.nav.audit'), hint: t('shell.soon') },
  ];

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
      pending={pending}
      bottom={bottom}
      account={{ id: 'account', icon: 'user', avatar: 'user', label: t('shell.nav.account'), hint: t('shell.soon') }}
    />
  );
};
