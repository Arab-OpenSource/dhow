import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { IconRail, type IRailItem } from '@/Core/Ui/IconRail';
import { useT } from '@/I18n/useT';

export const ShellRail = () => {
  const t = useT();
  const { goWorkspace } = useShellNavigation();

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

  return <IconRail label={t('shell.nav.label')} items={items} />;
};
