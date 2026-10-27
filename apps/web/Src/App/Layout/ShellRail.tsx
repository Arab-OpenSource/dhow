import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { IconRail, type IRailItem } from '@/Core/Ui/IconRail';
import { useT } from '@/I18n/useT';

export const ShellRail = () => {
  const t = useT();
  const { goWorkspace, openStack } = useShellNavigation();

  const items: IRailItem[] = [
    {
      id: 'workspace',
      icon: 'project',
      label: t('shell.nav.workspace'),
      href: '/',
      current: true,
      onSelect: goWorkspace,
    },
  ];

  const bottom: IRailItem[] = [
    {
      id: 'system-status',
      icon: 'health',
      label: t('shell.nav.systemStatus'),
      href: '/?stack=system-status',
      onSelect: () => {
        openStack(['system-status']);
      },
    },
    {
      id: 'appearance',
      icon: 'settings',
      label: t('shell.nav.appearance'),
      href: '/?stack=appearance',
      onSelect: () => {
        openStack(['appearance']);
      },
    },
  ];

  return <IconRail label={t('shell.nav.label')} items={items} bottom={bottom} />;
};
