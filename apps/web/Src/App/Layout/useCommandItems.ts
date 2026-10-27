import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import type { ICommandItem } from '@/Core/Ui/CommandPalette';
import { otherLocale } from '@/I18n/Locale';
import { useI18n } from '@/I18n/useI18n';

export interface ICommands {
  items: ICommandItem[];
  run: (item: ICommandItem) => void;
}

export const useCommandItems = (): ICommands => {
  const { Locale, SetLocale, t } = useI18n();
  const { goWorkspace, openPanel } = useShellNavigation();
  const target = otherLocale(Locale);

  const items: ICommandItem[] = [
    { id: 'workspace', icon: 'project', label: t('palette.command.workspace'), kind: t('palette.kind.page') },
    { id: 'notifications', icon: 'bell', label: t('palette.command.notifications'), kind: t('palette.kind.panel') },
    {
      id: 'language',
      label: t('palette.command.language', { language: t(`language.${target}`) }),
      kind: t('palette.kind.action'),
    },
  ];

  const run = (item: ICommandItem) => {
    if (item.id === 'workspace') goWorkspace();
    else if (item.id === 'notifications') openPanel('notifications');
    else if (item.id === 'language') SetLocale(target);
  };

  return { items, run };
};
