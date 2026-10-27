import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { IconRail, type IRailItem } from '@/Core/Ui/IconRail';
import { useT } from '@/I18n/useT';

export const ShellRail = () => {
  const t = useT();
  const { goWorkspace, openPanel } = useShellNavigation();

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
      href: '/?panel=system-status',
      onSelect: () => {
        openPanel('system-status');
      },
    },
  ];

  return <IconRail label={t('shell.nav.label')} items={items} bottom={bottom} />;
};
