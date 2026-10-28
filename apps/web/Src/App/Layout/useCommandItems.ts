import { useShellNavigation } from '@/App/Layout/useShellNavigation';
import { canManageMembers, canReadAudit } from '@/Core/Auth/AuthTypes';
import { useCurrentMembership } from '@/Core/Auth/useMe';
import { useSignOut } from '@/Core/Auth/useSignOut';
import type { ICommandItem } from '@/Core/Ui/CommandPalette';
import { otherLocale } from '@/I18n/Locale';
import { useI18n } from '@/I18n/useI18n';

export interface ICommands {
  items: ICommandItem[];
  run: (item: ICommandItem) => void;
}

export const useCommandItems = (): ICommands => {
  const { Locale, SetLocale, t } = useI18n();
  const { goWorkspace, openStack } = useShellNavigation();
  const { SignOut } = useSignOut();
  const role = useCurrentMembership()?.Role;
  const target = otherLocale(Locale);

  const items: ICommandItem[] = [
    { id: 'home', icon: 'home', label: t('palette.command.home'), kind: t('palette.kind.page') },
    { id: 'system-status', icon: 'shield', label: t('palette.command.systemStatus'), kind: t('palette.kind.panel') },
    { id: 'notifications', icon: 'bell', label: t('palette.command.notifications'), kind: t('palette.kind.panel') },
    { id: 'appearance', icon: 'settings', label: t('palette.command.appearance'), kind: t('palette.kind.panel') },
    { id: 'theme-new', icon: 'plus', label: t('palette.command.createTheme'), kind: t('palette.kind.panel') },
    { id: 'account', icon: 'user', label: t('palette.command.account'), kind: t('palette.kind.panel') },
    { id: 'jobs', icon: 'play', label: t('palette.command.jobs'), kind: t('palette.kind.panel') },
    { id: 'organization', icon: 'users', label: t('palette.command.organization'), kind: t('palette.kind.panel') },
    { id: 'organizations', icon: 'building', label: t('palette.command.organizations'), kind: t('palette.kind.panel') },
    ...(canManageMembers(role)
      ? [{ id: 'invite', icon: 'plus' as const, label: t('palette.command.invite'), kind: t('palette.kind.panel') }]
      : []),
    { id: 'password', icon: 'key', label: t('palette.command.password'), kind: t('palette.kind.panel') },
    ...(canReadAudit(role)
      ? [{ id: 'audit', icon: 'file' as const, label: t('palette.command.audit'), kind: t('palette.kind.panel') }]
      : []),
    { id: 'sign-out', icon: 'out', label: t('palette.command.signOut'), kind: t('palette.kind.action') },
    {
      id: 'language',
      label: t('palette.command.language', { language: t(`language.${target}`) }),
      kind: t('palette.kind.action'),
    },
  ];

  const run = (item: ICommandItem) => {
    if (item.id === 'home') goWorkspace();
    else if (item.id === 'system-status') openStack(['system-status']);
    else if (item.id === 'notifications') openStack(['notifications']);
    else if (item.id === 'appearance') openStack(['appearance']);
    else if (item.id === 'theme-new') openStack(['appearance', 'theme-edit']);
    else if (item.id === 'account') openStack(['account']);
    else if (item.id === 'jobs') openStack(['jobs']);
    else if (item.id === 'organization') openStack(['organization']);
    else if (item.id === 'organizations') openStack(['organizations']);
    else if (item.id === 'invite') openStack(['organization', 'organization-invite']);
    else if (item.id === 'audit') openStack(['audit']);
    else if (item.id === 'password') openStack(['account', 'account-password']);
    else if (item.id === 'sign-out') void SignOut();
    else if (item.id === 'language') SetLocale(target);
  };

  return { items, run };
};
