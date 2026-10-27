import type { MouseEvent, ReactNode } from 'react';

import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import type { TIconName } from '@/Core/Ui/Icons';
import { Tooltip } from '@/Core/Ui/Tooltip';

export interface IMenuItem {
  id: string;
  icon: TIconName;
  label: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
}

export interface IPendingMenuItem {
  id: string;
  icon: TIconName;
  label: string;
  hint: string;
}

export interface IMenuAccount extends IPendingMenuItem {
  avatar: TIconName;
}

export interface ISideMenuProps {
  label: string;
  brand: ReactNode;
  items: IMenuItem[];
  pending: IPendingMenuItem[];
  bottom: IMenuItem[];
  account: IMenuAccount;
}

const ITEM =
  'flex items-center gap-3 rounded-sm px-2.5 py-2 text-body-sm no-underline transition-colors duration-140 max-md:size-11 max-md:justify-center max-md:p-0';

const isPlainClick = (event: MouseEvent<HTMLAnchorElement>): boolean =>
  event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

const MenuLink = ({ item }: { item: IMenuItem }) => (
  <Tooltip label={item.label} side="top" phoneOnly>
    <a
      href={item.href}
      aria-current={item.current ? 'page' : undefined}
      onClick={(event) => {
        if (isPlainClick(event)) {
          event.preventDefault();
          item.onSelect();
        }
      }}
      className={cx(ITEM, item.current ? 'bg-sel text-primary' : 'text-secondary hover:bg-hover hover:text-primary')}
    >
      <Icon name={item.icon} size={18} className={item.current ? 'text-accent' : undefined} />
      <span className="max-md:sr-only">{item.label}</span>
    </a>
  </Tooltip>
);

const PendingLink = ({ item }: { item: IPendingMenuItem }) => (
  <Tooltip label={item.hint} side="top">
    <button type="button" aria-disabled="true" className={cx(ITEM, 'cursor-not-allowed border-0 bg-transparent text-start text-tertiary')}>
      <Icon name={item.icon} size={18} />
      <span className="max-md:sr-only">{item.label}</span>
    </button>
  </Tooltip>
);

const AccountRow = ({ account }: { account: IMenuAccount }) => (
  <Tooltip label={account.hint} side="top">
    <button
      type="button"
      aria-disabled="true"
      className="flex cursor-not-allowed items-center gap-2.5 rounded-sm border-0 bg-transparent px-1.5 py-2 text-start text-body-sm text-tertiary max-md:size-11 max-md:justify-center max-md:p-0"
    >
      <span className="inline-grid size-8 flex-none place-items-center rounded-pill bg-surface-raised text-primary">
        <Icon name={account.avatar} size={16} />
      </span>
      <span className="truncate max-md:sr-only">{account.label}</span>
    </button>
  </Tooltip>
);

export const SideMenu = ({ label, brand, items, pending, bottom, account }: ISideMenuProps) => (
  <nav
    aria-label={label}
    className={cx(
      'pointer-events-auto flex bg-rail',
      'max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:z-30 max-md:h-bottombar max-md:items-center max-md:justify-around max-md:border-t max-md:border-line max-md:px-2',
      'md:relative md:z-30 md:w-menu md:flex-none md:flex-col md:gap-1 md:border-e md:border-line md:px-2.5 md:py-3.5',
    )}
  >
    <div className="flex items-center gap-2.5 px-1.5 pt-1 pb-3.5 max-md:hidden">{brand}</div>
    {items.map((item) => (
      <MenuLink key={item.id} item={item} />
    ))}
    {pending.map((item) => (
      <PendingLink key={item.id} item={item} />
    ))}
    <div className="grid gap-1 max-md:contents md:mt-auto">
      {bottom.map((item) => (
        <MenuLink key={item.id} item={item} />
      ))}
      <AccountRow account={account} />
    </div>
  </nav>
);
