import { useNavigate, useRouter } from '@tanstack/react-router';

import { OrganizationChip } from '@/App/Layout/OrganizationChip';
import { GROUP_LABELS, MENU_GROUPS, sectionById, visibleSections, type ISection } from '@/App/Navigation/Sections';
import { useSection } from '@/App/Navigation/useSection';
import { useCurrentMembership, useMe } from '@/Core/Auth/useMe';
import { useSignOut } from '@/Core/Auth/useSignOut';
import { Mark } from '@/Core/Ui/Mark';
import { SideMenu, type IMenuGroup, type IMenuItem } from '@/Core/Ui/SideMenu';
import { useT } from '@/I18n/useT';

export const ShellMenu = () => {
  const t = useT();
  const navigate = useNavigate();
  const router = useRouter();
  const match = useSection();
  const me = useMe();
  const { SignOut, Pending } = useSignOut();
  const role = useCurrentMembership()?.Role;
  const currentId = match ? (match.section.parent ?? match.section.id) : null;

  const item = (section: ISection): IMenuItem => ({
    id: section.id,
    icon: section.icon,
    label: t(section.label),
    href: section.path,
    current: currentId === section.id,
    onSelect: () => {
      void navigate({ to: section.path });
    },
    onIntent: () => {
      void router.preloadRoute({ to: section.path }).catch(() => undefined);
    },
  });

  const groups: IMenuGroup[] = [
    { id: 'home', items: [item(sectionById('home'))] },
    ...MENU_GROUPS.map((group) => ({
      id: group,
      label: t(GROUP_LABELS[group]),
      items: visibleSections(role, group)
        .filter((section) => section.menu)
        .map(item),
    })),
  ];

  return (
    <SideMenu
      label={t('shell.nav.label')}
      brand={
        <div className="grid min-w-0 flex-1 gap-2.5">
          <span className="flex items-center gap-2.5">
            <Mark size={28} />
            <span className="text-title-section">{t('shell.brand')}</span>
          </span>
          <OrganizationChip />
        </div>
      }
      groups={groups}
      account={{
        id: 'account',
        name: me.data?.User.Name ?? '',
        label: me.data?.User.Name ?? t('shell.nav.account'),
        sub: role ? t(`role.${role}`) : undefined,
        href: '/account/profile',
        current: match?.section.group === 'account',
        onSelect: () => {
          void navigate({ to: '/account/profile' });
        },
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
