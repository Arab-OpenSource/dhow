import { useId, type MouseEvent, type ReactNode } from 'react';

import { Avatar } from '@/Core/Ui/Avatar';
import { Badge } from '@/Core/Ui/Badge';
import { cx } from '@/Core/Ui/ClassNames';
import { Icon } from '@/Core/Ui/Icon';
import { IconButton } from '@/Core/Ui/IconButton';
import type { TIconName } from '@/Core/Ui/Icons';
import { Tooltip } from '@/Core/Ui/Tooltip';

export interface IMenuItem {
  id: string;
  icon: TIconName;
  label: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
  onIntent?: () => void;
}

export interface IPendingMenuItem {
  id: string;
  icon: TIconName;
  label: string;
  hint: string;
}

export type TMenuEntry = IMenuItem | IPendingMenuItem;

export interface IMenuSignOut {
  label: string;
  busy: boolean;
  onSelect: () => void;
}

export interface IMenuAccount {
  id: string;
  name: string;
  label: string;
  href: string;
  current?: boolean;
  onSelect: () => void;
  onIntent?: () => void;
  signOut?: IMenuSignOut;
}

export interface ISideMenuProps {
  label: string;
  brand: ReactNode;
  items: TMenuEntry[];
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
      onPointerEnter={item.onIntent}
      onFocus={item.onIntent}
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

const PendingLink = ({ item }: { item: IPendingMenuItem }) => {
  const id = useId();
  return (
    <Tooltip label={`${item.label} · ${item.hint}`} side="top" phoneOnly>
      <button
        type="button"
        aria-disabled="true"
        aria-labelledby={`${id}-label`}
        aria-describedby={`${id}-hint`}
        data-pending=""
        onClick={(event) => {
          event.preventDefault();
        }}
        className={cx(ITEM, 'cursor-not-allowed border-0 bg-transparent text-start text-tertiary')}
      >
        <Icon name={item.icon} size={18} className="opacity-60" />
        <span id={`${id}-label`} className="opacity-80 max-md:sr-only">
          {item.label}
        </span>
        <span id={`${id}-hint`} className="ms-auto max-md:sr-only">
          <Badge tone="mute">{item.hint}</Badge>
        </span>
      </button>
    </Tooltip>
  );
};

const AccountRow = ({ account }: { account: IMenuAccount }) => (
  <div className="flex items-center gap-1 max-md:contents">
    <Tooltip label={account.label} side="top" phoneOnly>
      <a
        href={account.href}
        aria-current={account.current ? 'page' : undefined}
        onPointerEnter={account.onIntent}
        onFocus={account.onIntent}
        onClick={(event) => {
          if (isPlainClick(event)) {
            event.preventDefault();
            account.onSelect();
          }
        }}
        className={cx(
          'flex min-w-0 flex-1 items-center gap-2.5 rounded-sm px-1.5 py-2 text-body-sm no-underline transition-colors duration-140',
          'max-md:size-11 max-md:flex-none max-md:justify-center max-md:p-0',
          account.current ? 'bg-sel text-primary' : 'text-secondary hover:bg-hover hover:text-primary',
        )}
      >
        <Avatar name={account.name} size="sm" />
        <span className="truncate max-md:sr-only">{account.label}</span>
      </a>
    </Tooltip>
    {account.signOut ? (
      <span className="flex-none max-md:hidden">
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

export const SideMenu = ({ label, brand, items, bottom, account }: ISideMenuProps) => (
  <nav
    aria-label={label}
    className={cx(
      'pointer-events-auto flex bg-rail',
      'max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:z-30 max-md:h-bottombar max-md:items-center max-md:justify-around max-md:border-t max-md:border-line max-md:px-2',
      'md:relative md:z-30 md:w-menu md:flex-none md:flex-col md:gap-1 md:border-e md:border-line md:px-2.5 md:py-3.5',
    )}
  >
    <div className="flex items-center gap-2.5 px-1.5 pt-1 pb-3.5 max-md:hidden">{brand}</div>
    {items.map((item) => ('href' in item ? <MenuLink key={item.id} item={item} /> : <PendingLink key={item.id} item={item} />))}
    <div className="grid gap-1 max-md:contents md:mt-auto">
      {bottom.map((item) => (
        <MenuLink key={item.id} item={item} />
      ))}
      <AccountRow account={account} />
    </div>
  </nav>
);
