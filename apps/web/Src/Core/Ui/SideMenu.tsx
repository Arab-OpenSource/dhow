import { useId, type ReactNode } from 'react';

import { Avatar } from '@/Core/Ui/Avatar';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { IconButton } from '@/Core/Ui/IconButton';
import type { TIconName } from '@/Core/Ui/Icons';
import { followInApp } from '@/Core/Ui/PlainClick';

export interface IMenuItem {
  id: string;
  icon: TIconName;
  label: string;
  href: string;
  current?: boolean;
  count?: number;
  countLabel?: string;
  countTone?: 'warn';
  onSelect: () => void;
  onIntent?: () => void;
}

export interface IMenuGroup {
  id: string;
  label?: string;
  items: IMenuItem[];
}

export interface IMenuSignOut {
  label: string;
  busy: boolean;
  onSelect: () => void;
}

export interface IMenuAccount {
  id: string;
  name: string;
  label: string;
  sub?: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
  onIntent?: () => void;
  signOut?: IMenuSignOut;
}

export interface ISideMenuProps {
  label: string;
  brand: ReactNode;
  groups: IMenuGroup[];
  account: IMenuAccount;
}

const ITEM = 'flex items-center gap-3 rounded-sm px-2.5 py-2 text-label no-underline transition-colors duration-140';

const tone = (current: boolean | undefined): string =>
  current ? 'bg-sel text-primary' : 'text-secondary hover:bg-hover hover:text-primary';

const MenuLink = ({ item }: { item: IMenuItem }) => (
  <a
    href={item.href}
    aria-current={item.current ? 'page' : undefined}
    onPointerEnter={item.onIntent}
    onFocus={item.onIntent}
    onClick={followInApp(item.onSelect)}
    className={cx(ITEM, tone(item.current))}
  >
    <Icon name={item.icon} size={18} className={item.current ? 'text-accent' : undefined} />
    <span className="min-w-0 flex-1 truncate">{item.label}</span>
    {item.count !== undefined && item.count > 0 ? (
      <span
        aria-label={item.countLabel}
        data-tone={item.countTone}
        className={cx(
          'min-w-6 rounded-pill border px-1.5 text-center text-caption font-medium tabular-nums',
          item.countTone === 'warn' ? 'border-warn-line bg-warn-wash text-warn-fg' : 'border-line-2 text-secondary',
        )}
      >
        {item.count}
      </span>
    ) : null}
  </a>
);

const MenuGroup = ({ group }: { group: IMenuGroup }) => {
  const id = useId();
  if (group.items.length === 0) return null;
  const list = (
    <ul className="m-0 grid list-none gap-0.5 p-0">
      {group.items.map((item) => (
        <li key={item.id}>
          <MenuLink item={item} />
        </li>
      ))}
    </ul>
  );
  if (group.label === undefined) return <div data-menu-group={group.id}>{list}</div>;
  return (
    <div role="group" aria-labelledby={id} data-menu-group={group.id} className="grid gap-1 pt-4">
      <span id={id} className="px-2.5 text-caption font-medium text-tertiary">
        {group.label}
      </span>
      {list}
    </div>
  );
};

const AccountRow = ({ account }: { account: IMenuAccount }) => (
  <div className="flex items-center gap-1 rounded-md border border-line p-1.5">
    <a
      href={account.href}
      aria-current={account.current ? 'page' : undefined}
      onPointerEnter={account.onIntent}
      onFocus={account.onIntent}
      onClick={followInApp(account.onSelect)}
      className={cx(
        'flex min-w-0 flex-1 items-center gap-2.5 rounded-sm px-1.5 py-1.5 text-label no-underline transition-colors duration-140',
        tone(account.current),
      )}
    >
      <Avatar name={account.name} size="sm" />
      <span className="grid min-w-0">
        <span className="truncate">{account.label}</span>
        {account.sub ? <span className="truncate text-caption text-tertiary">{account.sub}</span> : null}
      </span>
    </a>
    {account.signOut ? (
      <span className="flex-none">
        <IconButton
          plain
          size="sm"
          icon="out"
          label={account.signOut.label}
          disabled={account.signOut.busy}
          onClick={account.signOut.onSelect}
        />
      </span>
    ) : null}
  </div>
);

export const SideMenu = ({ label, brand, groups, account }: ISideMenuProps) => (
  <nav
    aria-label={label}
    className="relative z-30 flex w-menu flex-none flex-col overflow-y-auto border-e border-line bg-rail px-3 py-3.5 max-md:hidden"
  >
    <div className="flex items-center gap-2.5 px-1.5 pt-1 pb-3.5">{brand}</div>
    {groups.map((group) => (
      <MenuGroup key={group.id} group={group} />
    ))}
    <div className="mt-auto pt-4">
      <AccountRow account={account} />
    </div>
  </nav>
);
